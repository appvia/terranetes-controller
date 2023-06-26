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

package revision

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

	terraformv1alpha1 "github.com/appvia/terranetes-controller/pkg/apis/terraform/v1alpha1"
	"github.com/appvia/terranetes-controller/pkg/handlers/revisions"
	"github.com/appvia/terranetes-controller/pkg/schema"
)

// Controller handles the reconciliation of the policy resource
type Controller struct {
	// cc is the kubernetes client to the cluster
	cc client.Client
	// recorder is a event recorder
	recorder record.EventRecorder
	// EnableWebhooks indicates if the webhooks should be enabled
	EnableWebhooks bool
	// EnableUpdateProtection indicates if the update protection should be enabled
	EnableUpdateProtection bool
}

const controllerName = "revision.terraform.appvia.io"

// Add is called to setup the manager for the controller
func (c *Controller) Add(mgr manager.Manager) error {
	log.Info("adding the revision controller")

	c.cc = mgr.GetClient()
	c.recorder = mgr.GetEventRecorderFor(controllerName)

	if c.EnableWebhooks {
		mgr.GetWebhookServer().Register(
			fmt.Sprintf("/mutate/%s/revisions", terraformv1alpha1.GroupName),
			admission.WithCustomDefaulter(schema.GetScheme(), &terraformv1alpha1.Revision{}, revisions.NewMutator(c.cc)),
		)
		mgr.GetWebhookServer().Register(
			fmt.Sprintf("/validate/%s/revisions", terraformv1alpha1.GroupName),
			admission.WithCustomValidator(schema.GetScheme(), &terraformv1alpha1.Revision{},
				revisions.NewValidator(c.cc, c.EnableUpdateProtection),
			),
		)
	}

	return ctrl.NewControllerManagedBy(mgr).
		For(&terraformv1alpha1.Revision{}).
		Named(controllerName).
		WithOptions(controller.Options{MaxConcurrentReconciles: 10}).
		WithEventFilter(&predicate.GenerationChangedPredicate{}).
		Complete(c)
}
