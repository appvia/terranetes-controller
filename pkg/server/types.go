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

import "time"

// Config is the configuration for the controller
type Config struct {
	// APIServerPort is the port to listen on
	APIServerPort int
	// BackendTemplate is the name of a secret in the controller namespace which
	// contains an optional template to use for the backend state - unless this
	// is set we use the default backend state i.e. kubernetes state
	BackendTemplate string
	// DriftControllerInterval is the interval for the controller to check for drift
	DriftControllerInterval time.Duration
	// DriftInterval is the minimum interval between drift checks
	DriftInterval time.Duration
	// DriftThreshold is the max number of drifts we are running to run - this prevents the
	// controller from running many configurations at the same time
	DriftThreshold float64
	// EnableContextInjection indicates the controller should always inject the context
	// into the terraform variables - i.e. namespace and name under a terraform variable
	// called 'terranetes'
	EnableContextInjection bool
	// EnableNamespaceProtection indicates the controller should protect the namespace
	// from being deleted if there are any terranetes resources in the namespace
	EnableNamespaceProtection bool
	// EnableWebhooks enables the webhooks registration
	EnableWebhooks bool
	// EnableWatchers enables the creation of watcher jobs
	EnableWatchers bool
	// EnableTerraformVersions indicates if configurations can override the default terraform version
	EnableTerraformVersions bool
	// ExecutorImage is the image to use for the executor
	ExecutorImage string
	// ExecutorSecrets is a list of additional secrets to be added to the executor
	ExecutorSecrets []string
	// InfracostsSecretName is the name of the secret that contains the cost token and endpoint
	InfracostsSecretName string
	// InfracostsImage is the image to use for infracosts
	InfracostsImage string
	// JobTemplate is the name of the configmap containing a template for the jobs
	JobTemplate string
	// MetricsPort is the port to listen on
	MetricsPort int
	// Namespace is namespace the controller is running
	Namespace string
	// PolicyImage is the image to use for policy
	PolicyImage string
	// PreloadImage is the image to use for the preload job
	PreloadImage string
	// RegisterCRDs indicated we register our crds
	RegisterCRDs bool
	// ResyncPeriod is the period to resync the controller manager
	ResyncPeriod time.Duration
	// TerraformImage is the image to use for terraform
	TerraformImage string
	// TLSDir is the directory where the TLS certificates are stored
	TLSDir string
	// TLSAuthority is the path to the ca certificate
	TLSAuthority string
	// TLSCert is the path to the certificate
	TLSCert string
	// TLSKey is the path to the key
	TLSKey string
	// WebhookPort is the port to listen on
	WebhookPort int
}
