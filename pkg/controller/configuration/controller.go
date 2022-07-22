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
	"context"
	"errors"
	"fmt"
	"time"

	cache "github.com/patrickmn/go-cache"
	log "github.com/sirupsen/logrus"
	batchv1 "k8s.io/api/batch/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/record"

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

	terraformv1alphav1 "github.com/appvia/terranetes-controller/pkg/apis/terraform/v1alpha1"
	"github.com/appvia/terranetes-controller/pkg/handlers/configurations"
	"github.com/appvia/terranetes-controller/pkg/utils/policies"
)

const controllerName = "configuration.terraform.appvia.io"

// Controller handles the reconciliation of the configuration resource
type Controller struct {
	// cc is the kubernetes client to the cluster
	cc client.Client
	// kc is a client kubernetes - this is required as the controller runtime client
	// does not support subresources, check https://github.com/kubernetes-sigs/controller-runtime/pull/1922
	kc kubernetes.Interface
	// cache is a local cache of resources to make lookups faster
	cache *cache.Cache
	// recorder is the kubernetes event recorder
	recorder record.EventRecorder
	// ExecutorSecrets is a collection of secrets which should be added to the
	// executors job everytime - these are configured by the platform team on the
	// cli options
	ExecutorSecrets []string
	// ControllerNamespace is the namespace where the runner is running
	ControllerNamespace string
	// EnableInfracosts enables the cost analytics via infracost
	EnableInfracosts bool
	// EnableWatchers indicates we should create watcher jobs in the user namespace
	EnableWatchers bool
	// ExecutorImage is the image to use for the executor
	ExecutorImage string
	// EnableTerraformVersions enables the use of the configuration's Terraform version
	EnableTerraformVersions bool
	// InfracostsImage is the image to use for all infracost jobs
	InfracostsImage string
	// InfracostsSecretName is the name of the secret containing the api and token
	InfracostsSecretName string
	// JobTemplate is a custom override for the template to use
	JobTemplate string
	// PolicyImage is the image to use for all policy / checkov jobs
	PolicyImage string
	// TerraformImage is the image to use for all terraform jobs
	TerraformImage string
}

// Add is called to setup the manager for the controller
func (c *Controller) Add(mgr manager.Manager) error {
	log.WithFields(log.Fields{
		"additional_secrets": len(c.ExecutorSecrets),
		"enable_costs":       c.EnableInfracosts,
		"enable_watchers":    c.EnableWatchers,
		"namespace":          c.ControllerNamespace,
		"policy_image":       c.PolicyImage,
		"terraform_image":    c.TerraformImage,
	}).Info("adding the configuration controller")

	switch {
	case c.ControllerNamespace == "":
		return errors.New("job namespace is required")
	case c.TerraformImage == "":
		return errors.New("terraform image is required")
	case c.PolicyImage == "":
		return errors.New("policy image is required")
	case c.EnableInfracosts && c.InfracostsImage == "":
		return errors.New("infracost image is required")
	case c.EnableInfracosts && c.InfracostsSecretName == "":
		return errors.New("infracost secret is required")
	}

	c.cc = mgr.GetClient()
	c.cache = cache.New(12*time.Hour, 10*time.Minute)
	c.recorder = mgr.GetEventRecorderFor(controllerName)

	kc, err := kubernetes.NewForConfig(mgr.GetConfig())
	if err != nil {
		return err
	}
	c.kc = kc

	mgr.GetWebhookServer().Register(
		fmt.Sprintf("/validate/%s/configurations", terraformv1alphav1.GroupName),
		admission.WithCustomValidator(&terraformv1alphav1.Configuration{}, configurations.NewValidator(c.cc, c.EnableTerraformVersions)),
	)
	mgr.GetWebhookServer().Register(
		fmt.Sprintf("/mutate/%s/configurations", terraformv1alphav1.GroupName),
		admission.WithCustomDefaulter(&terraformv1alphav1.Configuration{}, configurations.NewMutator(c.cc)),
	)

	return ctrl.NewControllerManagedBy(mgr).
		For(&terraformv1alphav1.Configuration{}).
		Named(controllerName).
		WithOptions(controller.Options{MaxConcurrentReconciles: 10}).
		// @note: we will avoid reconciliation on any resource where the annotation is set
		WithEventFilter(&predicate.Funcs{
			CreateFunc: func(e event.CreateEvent) bool {
				return !(e.Object.GetLabels()[terraformv1alphav1.ReconcileAnnotation] == "false")
			},
			UpdateFunc: func(e event.UpdateEvent) bool {
				return !(e.ObjectNew.GetLabels()[terraformv1alphav1.ReconcileAnnotation] == "false")
			},
			GenericFunc: func(e event.GenericEvent) bool {
				return !(e.Object.GetLabels()[terraformv1alphav1.ReconcileAnnotation] == "false")
			},
		}).
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
						Namespace: c.ControllerNamespace,
						Name:      a.GetLabels()["job-name"],
					}},
				}
			}),
			// we only care about jobs in our namespace
			builder.WithPredicates(predicate.Funcs{
				GenericFunc: func(e event.GenericEvent) bool {
					return e.Object.GetNamespace() == c.ControllerNamespace
				},
				CreateFunc: func(e event.CreateEvent) bool {
					return e.Object.GetNamespace() == c.ControllerNamespace
				},
				UpdateFunc: func(e event.UpdateEvent) bool {
					return e.ObjectNew.GetNamespace() == c.ControllerNamespace
				},
				DeleteFunc: func(e event.DeleteEvent) bool {
					return false
				},
			}),
			builder.WithPredicates(&predicate.ResourceVersionChangedPredicate{})).
		Complete(c)
}

// findMatchingPolicies is used to find a matching policy for the configuration. Note, ONLY one policy
// can be returned - we weight multiple policy least to most specific - i.e. no selector (i.e match all, weight=1),
// namespace labels=10, resource labels=20 per label. If multiple policies equal the same weight we throw
// an error.
func (c *Controller) findMatchingPolicy(
	ctx context.Context,
	configuration *terraformv1alphav1.Configuration,
	list *terraformv1alphav1.PolicyList) (*terraformv1alphav1.PolicyConstraint, error) {

	if len(list.Items) == 0 {
		return nil, nil
	}

	namespace, found := c.cache.Get(configuration.Namespace)
	if !found {
		return nil, fmt.Errorf("namespace: %q was not found in the cache", configuration.Namespace)
	}

	return policies.FindMatchingPolicy(ctx, configuration, namespace.(client.Object), list)
}
