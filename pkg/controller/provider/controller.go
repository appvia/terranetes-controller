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

package provider

import (
	"fmt"

	log "github.com/sirupsen/logrus"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	terraformv1alphav1 "github.com/appvia/terranetes-controller/pkg/apis/terraform/v1alpha1"
	"github.com/appvia/terranetes-controller/pkg/handlers/providers"
)

const controllerName = "provider.terraform.appvia.io"

// Controller handles the reconciliation of the provider resource
type Controller struct {
	// cc is the client connection
	cc client.Client
	// recorder is a event recorder
	recorder record.EventRecorder
	// ControllerNamespace is the namespace the controller lives
	ControllerNamespace string
}

// Add is called to setup the manager for the controller
func (c *Controller) Add(mgr manager.Manager) error {
	log.Info("creating the provider controller")

	c.cc = mgr.GetClient()
	c.recorder = mgr.GetEventRecorderFor(controllerName)

	mgr.GetWebhookServer().Register(
		fmt.Sprintf("/validate/%s/providers", terraformv1alphav1.GroupName),
		admission.WithCustomValidator(&terraformv1alphav1.Provider{}, providers.NewValidator(c.cc, c.ControllerNamespace)),
	)

	return ctrl.NewControllerManagedBy(mgr).
		For(&terraformv1alphav1.Provider{}).
		Named(controllerName).
		WithOptions(controller.Options{MaxConcurrentReconciles: 10}).
		WithEventFilter(&predicate.GenerationChangedPredicate{}).
		Complete(c)
}
