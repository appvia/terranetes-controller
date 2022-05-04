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

package configuration

import (
	"errors"
	"fmt"
	"time"

	cache "github.com/patrickmn/go-cache"
	log "github.com/sirupsen/logrus"
	batchv1 "k8s.io/api/batch/v1"

	v1 "k8s.io/api/core/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	terraformv1alphav1 "github.com/appvia/terraform-controller/pkg/apis/terraform/v1alpha1"
	"github.com/appvia/terraform-controller/pkg/handlers/configurations"
)

const controllerName = "configuration.terraform.appvia.io"

// Controller handles the reconciliation of the configuration resource
type Controller struct {
	// cc is the kubernetes client to the cluster
	cc client.Client
	// cache is a local cache of resources to make lookups faster
	cache *cache.Cache
	// EnableCostAnalytics enables the cost analytics via infracost
	EnableCostAnalytics bool
	// CostAnalyticsSecretName is the name of the secret containing the api and token
	CostAnalyticsSecretName string
	// JobNamespace is the namespace where the runner is running
	JobNamespace string
	// ExecutorImage is the image to use for the executor
	ExecutorImage string
}

// Add is called to setup the manager for the controller
func (c *Controller) Add(mgr manager.Manager) error {
	log.Info("adding the configuration controller")

	switch {
	case c.JobNamespace == "":
		return errors.New("job namespace is required")
	case c.ExecutorImage == "":
		return errors.New("executor image is required")
	}

	c.cc = mgr.GetClient()
	c.cache = cache.New(3*time.Hour, 5*time.Minute)

	mgr.GetWebhookServer().Register(
		fmt.Sprintf("/validate/%s/configurations", terraformv1alphav1.GroupName),
		admission.WithCustomValidator(&terraformv1alphav1.Configuration{}, configurations.NewValidator(c.cc)),
	)
	mgr.GetWebhookServer().Register(
		fmt.Sprintf("/mutate/%s/configurations", terraformv1alphav1.GroupName),
		admission.WithCustomDefaulter(&terraformv1alphav1.Configuration{}, configurations.NewMutator(c.cc)),
	)

	return ctrl.NewControllerManagedBy(mgr).
		For(&terraformv1alphav1.Configuration{}).
		Named(controllerName).
		WithOptions(controller.Options{MaxConcurrentReconciles: 10}).
		Watches(
			// We use this it keep a local cache of all namespaces in the cluster
			&source.Kind{Type: &v1.Namespace{}},
			handler.EnqueueRequestsFromMapFunc(func(o client.Object) []reconcile.Request {
				switch {
				case !o.GetDeletionTimestamp().IsZero():
					c.cache.Delete(o.GetName())
				default:
					c.cache.SetDefault(o.GetName(), o)
				}

				return nil
			}),
		).
		Watches(
			&source.Kind{Type: &batchv1.Job{}},
			// allows us to requeue the resource when the job has updated
			handler.EnqueueRequestsFromMapFunc(func(a client.Object) []reconcile.Request {
				return []ctrl.Request{
					{NamespacedName: client.ObjectKey{
						Namespace: c.JobNamespace,
						Name:      a.GetLabels()["job-name"],
					}},
				}
			}),
			// we only care about jobs in our namespace
			builder.WithPredicates(predicate.Funcs{
				GenericFunc: func(e event.GenericEvent) bool {
					return e.Object.GetNamespace() == c.JobNamespace
				},
				CreateFunc: func(e event.CreateEvent) bool {
					return e.Object.GetNamespace() == c.JobNamespace
				},
				UpdateFunc: func(e event.UpdateEvent) bool {
					return e.ObjectNew.GetNamespace() == c.JobNamespace
				},
				DeleteFunc: func(e event.DeleteEvent) bool {
					return false
				},
			}),
			builder.WithPredicates(&predicate.ResourceVersionChangedPredicate{})).
		Complete(c)
}
