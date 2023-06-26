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

package preload

import (
	"context"
	"errors"

	log "github.com/sirupsen/logrus"
	batchv1 "k8s.io/api/batch/v1"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	terraformv1alpha1 "github.com/appvia/terranetes-controller/pkg/apis/terraform/v1alpha1"
)

// Controller handles the reconciliation of the policy resource
type Controller struct {
	// cc is the kubernetes client to the cluster
	cc client.Client
	// recorder is a event recorder
	recorder record.EventRecorder
	// ControllerNamespace is the namespace the controller is running in
	ControllerNamespace string
	// ContainerImage is the image we should use for the preloading job
	ContainerImage string
	// EnableWebhooks indicates if the webhooks should be enabled
	EnableWebhooks bool
}

const controllerName = "preload.terraform.appvia.io"

// Add is called to setup the manager for the controller
func (c *Controller) Add(mgr manager.Manager) error {
	log.Info("adding the context preload controller")

	switch {
	case c.ControllerNamespace == "":
		return errors.New("controller namespace is required")
	case c.ContainerImage == "":
		return errors.New("container image is required")
	}

	c.cc = mgr.GetClient()
	c.recorder = mgr.GetEventRecorderFor(controllerName)

	return ctrl.NewControllerManagedBy(mgr).
		For(&terraformv1alpha1.Provider{}).
		Named(controllerName).
		WithOptions(controller.Options{MaxConcurrentReconciles: 10}).
		WithEventFilter(&predicate.GenerationChangedPredicate{}).
		WithEventFilter(&predicate.ResourceVersionChangedPredicate{}).
		Watches(
			&batchv1.Job{},
			handler.EnqueueRequestsFromMapFunc(func(_ context.Context, o client.Object) []ctrl.Request {
				if o.GetLabels()[terraformv1alpha1.PreloadJobLabel] == "true" {
					return []ctrl.Request{
						{
							NamespacedName: client.ObjectKey{
								Name: o.GetLabels()[terraformv1alpha1.PreloadProviderLabel],
							},
						},
					}
				}

				return nil
			}),
		).
		Complete(c)
}
