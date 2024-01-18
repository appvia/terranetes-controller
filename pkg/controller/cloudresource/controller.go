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

package cloudresource

import (
	"context"
	"fmt"

	log "github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"

	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	terraformv1alpha1 "github.com/appvia/terranetes-controller/pkg/apis/terraform/v1alpha1"
	"github.com/appvia/terranetes-controller/pkg/handlers/cloudresources"
	"github.com/appvia/terranetes-controller/pkg/schema"
)

const controllerName = "cloudresource.terraform.appvia.io"

// Controller handles the reconciliation of the cloudresource resource
type Controller struct {
	// cc is the kubernetes client to the cluster
	cc client.Client
	// recorder is the kubernetes event recorder
	recorder record.EventRecorder
	// EnableTerraformVersions enables the use of the cloudresource's Terraform version
	EnableTerraformVersions bool
	// EnableWebhooks indicates if we should register the webhooks
	EnableWebhooks bool
}

// Add is called to setup the manager for the controller
func (c *Controller) Add(mgr manager.Manager) error {
	log.Info("adding the cloudresource controller")

	c.cc = mgr.GetClient()
	c.recorder = mgr.GetEventRecorderFor(controllerName)

	if c.EnableWebhooks {
		mgr.GetWebhookServer().Register(
			fmt.Sprintf("/validate/%s/cloudresources", terraformv1alpha1.GroupName),
			admission.WithCustomValidator(schema.GetScheme(), &terraformv1alpha1.CloudResource{}, cloudresources.NewValidator(c.cc)),
		)
		mgr.GetWebhookServer().Register(
			fmt.Sprintf("/mutate/%s/cloudresources", terraformv1alpha1.GroupName),
			admission.WithCustomDefaulter(schema.GetScheme(), &terraformv1alpha1.CloudResource{}, cloudresources.NewMutator(c.cc)),
		)
	}

	return ctrl.NewControllerManagedBy(mgr).
		For(&terraformv1alpha1.CloudResource{}).
		Named(controllerName).
		WithOptions(controller.Options{MaxConcurrentReconciles: 10}).
		WithEventFilter(predicate.Or(
			&predicate.AnnotationChangedPredicate{},
			&predicate.GenerationChangedPredicate{},
			&predicate.ResourceVersionChangedPredicate{},
		)).
		Watches(
			&terraformv1alpha1.Plan{},
			handler.EnqueueRequestsFromMapFunc(func(_ context.Context, o client.Object) []reconcile.Request {
				list := &terraformv1alpha1.CloudResourceList{}
				if err := c.cc.List(context.Background(), list,
					client.MatchingLabels(map[string]string{
						terraformv1alpha1.CloudResourcePlanNameLabel: o.GetName(),
					}),
				); err != nil {
					log.WithError(err).Error("failed to list cloudresources")

					return nil
				}
				if len(list.Items) == 0 {
					return nil
				}

				var requests []reconcile.Request

				for _, x := range list.Items {
					requests = append(requests, reconcile.Request{
						NamespacedName: types.NamespacedName{
							Name:      x.Name,
							Namespace: x.Namespace,
						},
					})
				}

				return requests
			}),
			builder.WithPredicates(
				&predicate.GenerationChangedPredicate{},
			),
		).
		Watches(
			&terraformv1alpha1.Configuration{},
			handler.EnqueueRequestsFromMapFunc(func(_ context.Context, o client.Object) []reconcile.Request {
				logger := log.WithFields(log.Fields{
					"name":      o.GetName(),
					"namespace": o.GetNamespace(),
				})

				switch {
				case o.GetLabels() == nil:
					return nil
				case o.GetLabels()[terraformv1alpha1.CloudResourceNameLabel] == "":
					return nil
				}
				name := o.GetLabels()[terraformv1alpha1.CloudResourceNameLabel]

				logger.WithFields(log.Fields{
					"cloudresource": name,
				}).Debug("configuration change will trigger cloudresource reconcile")

				return []reconcile.Request{
					{
						NamespacedName: types.NamespacedName{
							Name:      name,
							Namespace: o.GetNamespace(),
						},
					},
				}
			}),
			builder.WithPredicates(&predicate.ResourceVersionChangedPredicate{})).
		Complete(c)
}
