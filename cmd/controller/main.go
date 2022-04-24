/*
 * Copyright (C) 2022 Rohith Jayawardene <gambol99@gmail.com>
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

package main

import (
	"context"
	"fmt"
	"os"
	"time"

	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	ctrl "sigs.k8s.io/controller-runtime"

	"github.com/appvia/terraform-controller/pkg/server"
	"github.com/appvia/terraform-controller/pkg/version"
)

func init() {
	log.SetFormatter(&log.JSONFormatter{})
}

var config server.Config

func main() {
	cmd := &cobra.Command{
		Use:     "terraform-controller",
		Short:   "Runs the terraform controller to managed workflows",
		Version: version.Version,
		RunE: func(cmd *cobra.Command, args []string) error {
			if v, _ := cmd.Flags().GetBool("verbose"); v {
				log.SetLevel(log.DebugLevel)
			}
			if v, _ := cmd.Flags().GetBool("trace"); v {
				log.SetLevel(log.TraceLevel)
			}

			return Run(context.Background())
		},
	}

	flags := cmd.Flags()
	flags.Bool("verbose", false, "Enable verbose logging")
	flags.Bool("trace", false, "Enable trace logging")
	flags.BoolVar(&config.EnableWebhook, "enable-webhook", true, "Indicates we should register the webhooks")
	flags.DurationVar(&config.ResyncPeriod, "resync-period", 1*time.Hour, "The resync period for the controller")
	flags.IntVar(&config.APIServerPort, "apiserver-port", 10080, "The port the apiserver should be listening on")
	flags.IntVar(&config.MetricsPort, "metrics-port", 9090, "The port the metric endpoint binds to")
	flags.IntVar(&config.WebhookPort, "webhooks-port", 10081, "The port the webhook endpoint binds to")
	flags.StringVar(&config.CostSecretName, "cost-secret", "", "Name of the secret on the controller namespace containing your infracost token")
	flags.StringVar(&config.ExecutorImage, "executor-image", "quay.io/appvia/terraform-executor:latest", "The image to use for the executor")
	flags.StringVar(&config.GitImage, "git-image", "quay.io/appvia/terraform-controller:latest", "The image to use for git operations")
	flags.StringVar(&config.Namespace, "namespace", os.Getenv("KUBE_NAMESPACE"), "The namespace the controller is running in and where jobs will run")
	flags.StringVar(&config.TLSAuthority, "tls-ca", "", "The filename to the ca certificate")
	flags.StringVar(&config.TLSCert, "tls-cert", "tls.pem", "The name of the file containing the TLS certificate")
	flags.StringVar(&config.TLSDir, "tls-dir", "", "The directory the certificates are held")
	flags.StringVar(&config.TLSKey, "tls-key", "tls-key.pem", "The name of the file containing the TLS key")

	if err := cmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "[error] %s\n", err)

		os.Exit(1)
	}
}

// Run is called to execute the action
func Run(ctx context.Context) error {
	svc, err := server.New(ctrl.GetConfigOrDie(), config)
	if err != nil {
		return err
	}

	return svc.Start(ctx)
}
