/*
 * Copyright (C) 2023  Appvia Ltd <info@appvia.io>
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

package namespace

import (
	"fmt"

	log "github.com/sirupsen/logrus"
	v1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	terraformv1alpha1 "github.com/appvia/terranetes-controller/pkg/apis/terraform/v1alpha1"
	"github.com/appvia/terranetes-controller/pkg/handlers/namespaces"
)

// Controller handles the reconciliation of the resource
type Controller struct {
	// cc is the kubernetes client to the cluster
	cc client.Client
	// EnableWebhooks indicates if the webhooks should be enabled
	EnableWebhooks bool
	// EnableNamespaceProtection indicates if the namespace protection should be enabled
	EnableNamespaceProtection bool
}

// Add is called to setup the manager for the controller
func (c *Controller) Add(mgr manager.Manager) error {
	log.Info("adding the namespace controller")

	c.cc = mgr.GetClient()

	if c.EnableWebhooks {
		mgr.GetWebhookServer().Register(
			fmt.Sprintf("/validate/%s/namespaces", terraformv1alpha1.GroupName),
			admission.WithCustomValidator(&v1.Namespace{}, namespaces.NewValidator(c.cc, c.EnableNamespaceProtection)),
		)
	}

	return nil
}
