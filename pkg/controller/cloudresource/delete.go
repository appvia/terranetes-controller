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
	"time"

	v1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	corev1alpha1 "github.com/appvia/terranetes-controller/pkg/apis/core/v1alpha1"
	terraformv1alpha1 "github.com/appvia/terranetes-controller/pkg/apis/terraform/v1alpha1"
	"github.com/appvia/terranetes-controller/pkg/controller"
	"github.com/appvia/terranetes-controller/pkg/utils/kubernetes"
)

// ensureConfigurationRemoved is responsible for ensuring the configuration is removed
// when the cloudresource is deleted
func (c *Controller) ensureConfigurationRemoved(cloudresource *terraformv1alpha1.CloudResource) controller.EnsureFunc {
	cond := controller.ConditionMgr(cloudresource, corev1alpha1.ConditionReady, c.recorder)
	cc := c.cc

	return func(ctx context.Context) (reconcile.Result, error) {
		cloudresource.Status.ResourceStatus = terraformv1alpha1.DestroyingResources

		// @step: lets try and use the cloudresource status
		configuration := &terraformv1alpha1.Configuration{}
		configuration.Name = cloudresource.Status.ConfigurationName
		configuration.Namespace = cloudresource.Namespace

		found, err := kubernetes.GetIfExists(ctx, cc, configuration)
		if err != nil {
			cond.Failed(err, "Failed to retrieve the configuration")

			return reconcile.Result{}, err
		}
		if !found {
			c.recorder.Event(cloudresource, v1.EventTypeNormal, "Deleted", "The configuration has been deleted")

			return reconcile.Result{}, nil
		}
		if configuration.DeletionTimestamp.IsZero() {
			if err := cc.Delete(ctx, configuration); err != nil {
				cond.Failed(err, "Failed to delete the configuration")

				return reconcile.Result{}, err
			}
			cond.Deleting("Waiting for the configuration to be deleted")

			return reconcile.Result{RequeueAfter: 5 * time.Second}, nil
		}

		// @step: update the status from the configuration
		status := configuration.Status.GetCondition(corev1alpha1.ConditionReady)
		cond.GetCondition().Status = status.Status
		cond.GetCondition().Reason = status.Reason
		cond.GetCondition().Message = status.Message
		cond.GetCondition().Type = status.Type

		// @step: has the configuration failed to delete?
		if configuration.Status.ResourceStatus == terraformv1alpha1.DestroyingResourcesFailed {
			cond.ActionRequired("Failed to delete CloudResource, please check Configuration status")

			return reconcile.Result{}, controller.ErrIgnore
		}

		return reconcile.Result{RequeueAfter: 5 * time.Second}, nil
	}
}
