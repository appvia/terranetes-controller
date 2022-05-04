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

	log "github.com/sirupsen/logrus"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/webhook"

	"github.com/appvia/terraform-controller/pkg/apiserver"
	"github.com/appvia/terraform-controller/pkg/controller/configuration"
	"github.com/appvia/terraform-controller/pkg/controller/policy"
	"github.com/appvia/terraform-controller/pkg/controller/provider"
	"github.com/appvia/terraform-controller/pkg/schema"
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
	cc, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		return nil, err
	}

	// @step: create the apiserver
	listener, err := net.Listen("tcp", fmt.Sprintf(":%d", config.APIServerPort))
	if err != nil {
		return nil, err
	}

	hs := &http.Server{
		Addr: listener.Addr().String(),
		Handler: (&apiserver.Server{
			Client:    cc,
			Namespace: config.Namespace,
		}).ServerHTTP(),
	}

	options := manager.Options{
		LeaderElection:                false,
		LeaderElectionID:              "controller.terraform.appvia.io",
		LeaderElectionNamespace:       os.Getenv("KUBE_NAMESPACE"),
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

	if err := (&configuration.Controller{
		CostAnalyticsSecretName: config.CostSecretName,
		EnableCostAnalytics:     (config.CostSecretName != ""),
		ExecutorImage:           config.ExecutorImage,
		JobNamespace:            config.Namespace,
	}).Add(mgr); err != nil {
		return nil, fmt.Errorf("failed to create the configuration controller, error: %v", err)
	}

	if err := (&provider.Controller{}).Add(mgr); err != nil {
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
