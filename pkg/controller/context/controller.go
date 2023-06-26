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

package context

import (
	"fmt"

	log "github.com/sirupsen/logrus"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	terraformv1alpha1 "github.com/appvia/terranetes-controller/pkg/apis/terraform/v1alpha1"
	"github.com/appvia/terranetes-controller/pkg/handlers/contexts"
)

// Controller handles the reconciliation of the resource
type Controller struct {
	// EnableWebhooks indicates if the webhooks should be enabled
	EnableWebhooks bool
}

// Add is called to setup the manager for the controller
func (c *Controller) Add(mgr manager.Manager) error {
	log.Info("adding the contexts controller")

	if c.EnableWebhooks {
		mgr.GetWebhookServer().Register(
			fmt.Sprintf("/validate/%s/contexts", terraformv1alpha1.GroupName),
			admission.WithCustomValidator(mgr.GetScheme(), &terraformv1alpha1.Context{}, contexts.NewValidator(mgr.GetClient())),
		)
	}

	return nil
}
