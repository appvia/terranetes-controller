/*
 * Copyright (C) 2022 Appvia Ltd <info@appvia.io>
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
	"flag"
	"fmt"
	"os"
	"time"

	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	"github.com/appvia/terranetes-controller/pkg/server"
	"github.com/appvia/terranetes-controller/pkg/version"
)

func init() {
	log.SetFormatter(&log.JSONFormatter{})
}

var config server.Config
var zapOpts zap.Options

func main() {
	cmd := &cobra.Command{
		Use:     "terranetes-controller",
		Short:   "Runs the terraform controller to managed workflows",
		Version: version.GetVersion(),
		RunE: func(cmd *cobra.Command, args []string) error {
			if v, _ := cmd.Flags().GetBool("verbose"); v {
				log.SetLevel(log.DebugLevel)
			}

			return Run(context.Background())
		},
	}

	flags := cmd.Flags()
	flags.Bool("verbose", false, "Enable verbose logging")
	flags.IntVar(&config.BackoffLimit, "backoff-limit", 1, "The number of times we are willing to allow a terraform job to error before marking as a failure")
	flags.BoolVar(&config.EnableContextInjection, "enable-context-injection", false, "Indicates the controller should inject Configuration context into the terraform variables")
	flags.BoolVar(&config.EnableNamespaceProtection, "enable-namespace-protection", false, "Indicates the controller should protect the controller namespace from being deleted")
	flags.BoolVar(&config.EnableRevisionUpdateProtection, "enable-revision-update-protection", false, "Indicates we should protect the revisions in use from being updated")
	flags.BoolVar(&config.EnableTerraformVersions, "enable-terraform-versions", true, "Indicates the terraform version can be overridden by configurations")
	flags.BoolVar(&config.EnableWatchers, "enable-watchers", true, "Indicates we create watcher jobs in the configuration namespaces")
	flags.BoolVar(&config.EnableWebhooks, "enable-webhooks", true, "Indicates we should register the webhooks")
	flags.BoolVar(&config.RegisterCRDs, "register-crds", true, "Indicates the controller to register its own CRDs")
	flags.BoolVar(&config.EnableWebhookPrefix, "enable-webhook-prefix", false, "Indicates the controller should prefix webhook configuration names with the controller name")
	flags.DurationVar(&config.DriftControllerInterval, "drift-controller-interval", 5*time.Minute, "Is the check interval for the controller to search for configurations which should be checked for drift")
	flags.DurationVar(&config.DriftInterval, "drift-interval", 3*time.Hour, "The minimum duration the controller will wait before triggering a drift check")
	flags.DurationVar(&config.ResyncPeriod, "resync-period", 5*time.Hour, "The resync period for the controller")
	flags.DurationVar(&config.RevisionExpiration, "revision-expiration", 0, "The duration a revision should be kept is not referenced or latest (zero means disabled)")
	flags.Float64Var(&config.DriftThreshold, "drift-threshold", 0.10, "The maximum percentage of configurations that can be run drift detection at any one time")
	flags.IntVar(&config.APIServerPort, "apiserver-port", 10080, "The port the apiserver should be listening on")
	flags.IntVar(&config.MetricsPort, "metrics-port", 9090, "The port the metric endpoint binds to")
	flags.IntVar(&config.WebhookPort, "webhooks-port", 10081, "The port the webhook endpoint binds to")
	flags.StringSliceVar(&config.ExecutorSecrets, "executor-secret", []string{}, "Name of a secret in controller namespace which should be added to the job")
	flags.StringVar(&config.BackendTemplate, "backend-template", "", "Name of secret in the controller namespace containing a template for the terraform state")
	flags.StringVar(&config.ExecutorImage, "executor-image", fmt.Sprintf("ghcr.io/appvia/terranetes-executor:%s", version.Version), "The image to use for the executor")
	flags.StringVar(&config.InfracostsImage, "infracost-image", "infracosts/infracost:latest", "The image to use for the infracosts")
	flags.StringVar(&config.InfracostsSecretName, "cost-secret", "", "Name of the secret on the controller namespace containing your infracost token")
	flags.StringVar(&config.JobTemplate, "job-template", "", "Name of configmap in the controller namespace containing a template for the job")
	flags.StringSliceVar(&config.JobLabels, "job-label", []string{}, "A collection of key=values to add to all jobs")
	flags.StringVar(&config.Namespace, "namespace", os.Getenv("KUBE_NAMESPACE"), "The namespace the controller is running in and where jobs will run")
	flags.StringVar(&config.PolicyImage, "policy-image", "bridgecrew/checkov:latest", "The image to use for the policy")
	flags.StringVar(&config.PreloadImage, "preload-image", fmt.Sprintf("ghcr.io/appvia/terranetes-executor:%s", version.Version), "The image to use for the preload")
	flags.StringVar(&config.TLSAuthority, "tls-ca", "", "The filename to the ca certificate")
	flags.StringVar(&config.TLSCert, "tls-cert", "tls.pem", "The name of the file containing the TLS certificate")
	flags.StringVar(&config.TLSDir, "tls-dir", "", "The directory the certificates are held")
	flags.StringVar(&config.TLSKey, "tls-key", "tls-key.pem", "The name of the file containing the TLS key")
	flags.StringVar(&config.TerraformImage, "terraform-image", "hashicorp/terraform:latest", "The image to use for the terraform")

	crFlags := flag.NewFlagSet("controller-runtime", flag.ContinueOnError)
	zapOpts.BindFlags(crFlags)
	ctrl.RegisterFlags(crFlags)
	flags.AddGoFlagSet(crFlags)

	if err := cmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "[error] %s\n", err)

		os.Exit(1)
	}
}

// Run is called to execute the action
func Run(ctx context.Context) error {
	ctrl.SetLogger(zap.New(zap.UseFlagOptions(&zapOpts)))
	svc, err := server.New(ctrl.GetConfigOrDie(), config)
	if err != nil {
		return err
	}

	return svc.Start(ctx)
}
