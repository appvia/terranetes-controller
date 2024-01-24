/*
 * Copyright (C) 2022  Appvia Ltd <info@appvia.io>
 *
 * This program is free software; you can redistribute it and/or
 * modify it under the terms of the GNU General Public License
 * as published by the Free Software Foundation; either version 2
 * of the License, or (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU General Public License for more details.
 *
 * You should have received a copy of the GNU General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 */

package server

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"os"
	"strings"
	"time"

	log "github.com/sirupsen/logrus"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"
	"sigs.k8s.io/controller-runtime/pkg/webhook"

	"github.com/appvia/terranetes-controller/pkg/apiserver"
	"github.com/appvia/terranetes-controller/pkg/controller/cloudresource"
	"github.com/appvia/terranetes-controller/pkg/controller/configuration"
	ctrlcontext "github.com/appvia/terranetes-controller/pkg/controller/context"
	"github.com/appvia/terranetes-controller/pkg/controller/drift"
	"github.com/appvia/terranetes-controller/pkg/controller/expire"
	"github.com/appvia/terranetes-controller/pkg/controller/namespace"
	"github.com/appvia/terranetes-controller/pkg/controller/plan"
	"github.com/appvia/terranetes-controller/pkg/controller/policy"
	"github.com/appvia/terranetes-controller/pkg/controller/preload"
	"github.com/appvia/terranetes-controller/pkg/controller/provider"
	"github.com/appvia/terranetes-controller/pkg/controller/revision"
	"github.com/appvia/terranetes-controller/pkg/register"
	"github.com/appvia/terranetes-controller/pkg/schema"
	"github.com/appvia/terranetes-controller/pkg/utils"
	k8sutils "github.com/appvia/terranetes-controller/pkg/utils/kubernetes"
	"github.com/appvia/terranetes-controller/pkg/version"
)

// Server is a wrapper around the services
type Server struct {
	cfg      *rest.Config
	config   Config
	mgr      manager.Manager
	hs       *http.Server
	listener net.Listener
}

// New returns and starts a new server
func New(cfg *rest.Config, config Config) (*Server, error) {
	switch {
	case config.DriftThreshold >= 1:
		return nil, fmt.Errorf("drift threshold must be less than 1")
	case config.DriftThreshold <= 0:
		return nil, fmt.Errorf("drift threshold must be greater than 0")
	}

	log.WithFields(log.Fields{
		"gitsha":  version.GitCommit,
		"version": version.Version,
	}).Info("starting the terranetes controller")

	cc, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		return nil, err
	}

	// @step: lets register our own crds
	if config.RegisterCRDs {
		log.Info("registering the custom resources")
		for _, path := range register.AssetNames() {
			if !strings.HasPrefix(path, "charts/terranetes-controller/crds/") {
				continue
			}

			ca, err := k8sutils.NewExtentionsAPIClient(cfg)
			if err != nil {
				return nil, err
			}
			if err := k8sutils.ApplyCustomResourceRawDefinitions(context.Background(), ca, register.MustAsset(path)); err != nil {
				return nil, err
			}
		}
	}

	// @step: create the apiserver
	listener, err := net.Listen("tcp", fmt.Sprintf(":%d", config.APIServerPort))
	if err != nil {
		return nil, err
	}

	ns := os.Getenv("KUBE_NAMESPACE")
	if ns == "" {
		ns = "terraform-system"
	}

	hs := &http.Server{
		Addr:              listener.Addr().String(),
		IdleTimeout:       30 * time.Second,
		ReadHeaderTimeout: 5 * time.Second,
		Handler: (&apiserver.Server{
			Client:    cc,
			Namespace: config.Namespace,
		}).Serve(),
	}

	options := manager.Options{
		Cache:                         cache.Options{SyncPeriod: &config.ResyncPeriod},
		LeaderElection:                true,
		LeaderElectionID:              "controller.terraform.appvia.io",
		LeaderElectionNamespace:       ns,
		LeaderElectionReleaseOnCancel: true,
		Metrics: metricsserver.Options{
			BindAddress: fmt.Sprintf(":%d", config.MetricsPort),
		},
		Scheme: schema.GetScheme(),
	}

	if config.EnableWebhooks {
		log.Info("creating the webhook server for validation and mutations")
		options.WebhookServer = webhook.NewServer(webhook.Options{
			CertDir:  config.TLSDir,
			CertName: config.TLSCert,
			KeyName:  config.TLSKey,
			Port:     config.WebhookPort,
		})
	}

	log.Info("creating a new manager for the controllers")
	mgr, err := manager.New(cfg, options)
	if err != nil {
		return nil, fmt.Errorf("failed to create the controller manager: %w", err)
	}

	if config.InfracostsSecretName != "" && config.InfracostsImage != "" {
		log.Info("enabling the infracost integration")
	}

	// @step: ensure the contexts controller is enabled
	if err := (&ctrlcontext.Controller{
		EnableWebhooks: config.EnableWebhooks,
	}).Add(mgr); err != nil {
		return nil, fmt.Errorf("failed to add the contexts controller: %w", err)
	}

	jobLabels := map[string]string{}
	if len(config.JobLabels) > 0 {
		labels, err := utils.ToMap(config.JobLabels)
		if err != nil {
			return nil, fmt.Errorf("failed to parse the job labels: %w", err)
		}
		jobLabels = labels
	}

	// @step: ensure the configuration controller is enabled
	if err := (&configuration.Controller{
		BackendTemplate:         config.BackendTemplate,
		BackoffLimit:            config.BackoffLimit,
		ControllerJobLabels:     jobLabels,
		ControllerNamespace:     config.Namespace,
		EnableInfracosts:        (config.InfracostsSecretName != ""),
		EnableTerraformVersions: config.EnableTerraformVersions,
		EnableWatchers:          config.EnableWatchers,
		EnableWebhooks:          config.EnableWebhooks,
		ExecutorImage:           config.ExecutorImage,
		ExecutorSecrets:         config.ExecutorSecrets,
		InfracostsImage:         config.InfracostsImage,
		InfracostsSecretName:    config.InfracostsSecretName,
		JobTemplate:             config.JobTemplate,
		PolicyImage:             config.PolicyImage,
		TerraformImage:          config.TerraformImage,
	}).Add(mgr); err != nil {
		return nil, fmt.Errorf("failed to create the configuration controller, error: %w", err)
	}

	// @step: ensure the drift controller is enabled
	if err := (&drift.Controller{
		CheckInterval:  config.DriftControllerInterval,
		DriftInterval:  config.DriftInterval,
		DriftThreshold: config.DriftThreshold,
	}).Add(mgr); err != nil {
		return nil, fmt.Errorf("failed to create the drift controller, error: %w", err)
	}

	// @step: ensure the provider controller is enabled
	if err := (&provider.Controller{
		ControllerNamespace: config.Namespace,
		EnableWebhooks:      config.EnableWebhooks,
	}).Add(mgr); err != nil {
		return nil, fmt.Errorf("failed to create the provider controller, error: %w", err)
	}

	// @step: ensure the policy controller is enabled
	if err := (&policy.Controller{
		EnableWebhooks: config.EnableWebhooks,
	}).Add(mgr); err != nil {
		return nil, fmt.Errorf("failed to create the policy controller, error: %w", err)
	}

	// @step: ensure the namespace controller is enabled
	if err := (&namespace.Controller{
		EnableNamespaceProtection: config.EnableNamespaceProtection,
		EnableWebhooks:            config.EnableWebhooks,
	}).Add(mgr); err != nil {
		return nil, fmt.Errorf("failed to create the namespace controller, error: %w", err)
	}

	// @step: ensure the plan controller
	if err := (&plan.Controller{}).Add(mgr); err != nil {
		return nil, fmt.Errorf("failed to create the plan controller, error: %w", err)
	}

	// @step: ensure the revision controller is enabled
	if err := (&revision.Controller{
		EnableUpdateProtection: config.EnableRevisionUpdateProtection,
		EnableWebhooks:         config.EnableWebhooks,
	}).Add(mgr); err != nil {
		return nil, fmt.Errorf("failed to create the revision controller, error: %w", err)
	}

	// @step: ensure the cloudresource controller is enabled
	if err := (&cloudresource.Controller{
		EnableTerraformVersions: config.EnableTerraformVersions,
		EnableWebhooks:          config.EnableWebhooks,
	}).Add(mgr); err != nil {
		return nil, fmt.Errorf("failed to create the cloudresource controller, error: %w", err)
	}

	// @step: ensure the preload controller is enabled
	if err := (&preload.Controller{
		ContainerImage:      config.PreloadImage,
		ControllerNamespace: config.Namespace,
		EnableWebhooks:      config.EnableWebhooks,
	}).Add(mgr); err != nil {
		return nil, fmt.Errorf("failed to create the preload controller, error: %w", err)
	}

	// @step: ensure the revision expiration controller is started
	if err := (&expire.Controller{
		RevisionExpiration: config.RevisionExpiration,
	}).Add(mgr); err != nil {
		return nil, fmt.Errorf("failed to create the expiration controller, error: %w", err)
	}

	return &Server{
		cfg:      cfg,
		config:   config,
		hs:       hs,
		listener: listener,
		mgr:      mgr,
	}, nil
}

// Start is called to begin the service
func (s *Server) Start(ctx context.Context) error {
	if s.config.EnableWebhooks {
		if err := s.registerWebhooks(ctx); err != nil {
			return fmt.Errorf("failed to register the webhooks, error: %w", err)
		}
	}

	go func() {
		log.Info("starting the api server")
		if err := s.hs.Serve(s.listener); err != nil {
			log.WithError(err).Fatal("trying to start the apiserver")
		}
	}()

	return s.mgr.Start(ctrl.SetupSignalHandler())
}
