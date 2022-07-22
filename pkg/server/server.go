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
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/webhook"

	"github.com/appvia/terranetes-controller/pkg/apiserver"
	"github.com/appvia/terranetes-controller/pkg/controller/configuration"
	"github.com/appvia/terranetes-controller/pkg/controller/drift"
	"github.com/appvia/terranetes-controller/pkg/controller/policy"
	"github.com/appvia/terranetes-controller/pkg/controller/provider"
	"github.com/appvia/terranetes-controller/pkg/register"
	"github.com/appvia/terranetes-controller/pkg/schema"
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

	namespace := os.Getenv("KUBE_NAMESPACE")
	if namespace == "" {
		namespace = "terranetes-system"
	}

	hs := &http.Server{
		Addr:        listener.Addr().String(),
		IdleTimeout: 30 * time.Second,
		Handler: (&apiserver.Server{
			Client:    cc,
			Namespace: config.Namespace,
		}).Serve(),
	}

	options := manager.Options{
		LeaderElection:                false,
		LeaderElectionID:              "controller.terraform.appvia.io",
		LeaderElectionNamespace:       namespace,
		LeaderElectionReleaseOnCancel: true,
		MetricsBindAddress:            fmt.Sprintf(":%d", config.MetricsPort),
		Port:                          config.WebhookPort,
		Scheme:                        schema.GetScheme(),
		SyncPeriod:                    &config.ResyncPeriod,
	}

	if config.EnableWebhook {
		log.Info("creating the webhook server for validation and mutations")
		options.WebhookServer = &webhook.Server{
			CertDir:  config.TLSDir,
			CertName: config.TLSCert,
			KeyName:  config.TLSKey,
			Port:     config.WebhookPort,
		}
	}

	log.Info("creating a new manager for the controllers")
	mgr, err := manager.New(cfg, options)
	if err != nil {
		return nil, fmt.Errorf("failed to create the controller manager: %v", err)
	}

	if config.InfracostsSecretName != "" && config.InfracostsImage != "" {
		log.Info("enabling the infracost integration")
	}

	if err := (&configuration.Controller{
		ControllerNamespace:     config.Namespace,
		EnableInfracosts:        (config.InfracostsSecretName != ""),
		EnableTerraformVersions: config.EnableTerraformVersions,
		EnableWatchers:          config.EnableWatchers,
		ExecutorImage:           config.ExecutorImage,
		ExecutorSecrets:         config.ExecutorSecrets,
		InfracostsImage:         config.InfracostsImage,
		InfracostsSecretName:    config.InfracostsSecretName,
		JobTemplate:             config.JobTemplate,
		PolicyImage:             config.PolicyImage,
		TerraformImage:          config.TerraformImage,
	}).Add(mgr); err != nil {
		return nil, fmt.Errorf("failed to create the configuration controller, error: %v", err)
	}

	if err := (&drift.Controller{
		CheckInterval:  config.DriftControllerInterval,
		DriftInterval:  config.DriftInterval,
		DriftThreshold: config.DriftThreshold,
	}).Add(mgr); err != nil {
		return nil, fmt.Errorf("failed to create the drift controller, error: %v", err)
	}

	if err := (&provider.Controller{
		ControllerNamespace: config.Namespace,
	}).Add(mgr); err != nil {
		return nil, fmt.Errorf("failed to create the provider controller, error: %v", err)
	}

	if err := (&policy.Controller{}).Add(mgr); err != nil {
		return nil, fmt.Errorf("failed to create the policy controller, error: %v", err)
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
	if s.config.EnableWebhook {
		if err := s.registerWebhooks(ctx); err != nil {
			return fmt.Errorf("failed to register the webhooks, error: %v", err)
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
