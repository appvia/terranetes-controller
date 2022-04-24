/*
 * Copyright (C) 2022  Rohith Jayawardene <gambol99@gmail.com>
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
	// EnableWebhook enables the webhook registration
	EnableWebhook bool
	// CostSecretName is the name of the secret that contains the cost token and endpoint
	CostSecretName string
	// Namespace is namespace the controller is running
	Namespace string
	// ExecutorImage is the image to use for the executor
	ExecutorImage string
	// ResyncPeriod is the period to resync the controller manager
	ResyncPeriod time.Duration
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
	// APIServerPort is the port to listen on
	APIServerPort int
	// MetricsPort is the port to listen on
	MetricsPort int
	// GitImage is the image version to use for git in terraform jobs
	GitImage string
}
