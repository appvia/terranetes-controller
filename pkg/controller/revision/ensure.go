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
	"context"

	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	corev1alpha1 "github.com/appvia/terranetes-controller/pkg/apis/core/v1alpha1"
	terraformv1alpha1 "github.com/appvia/terranetes-controller/pkg/apis/terraform/v1alpha1"
	"github.com/appvia/terranetes-controller/pkg/controller"
	"github.com/appvia/terranetes-controller/pkg/utils/kubernetes"
)

// ensurePlanExists is responsible for ensuring that a plan exists, and if not, creating one
func (c *Controller) ensurePlanExists(revision *terraformv1alpha1.Revision) controller.EnsureFunc {
	cond := controller.ConditionMgr(revision, corev1alpha1.ConditionReady, c.recorder)
	cc := c.cc

	return func(ctx context.Context) (reconcile.Result, error) {
		plan := &terraformv1alpha1.Plan{}
		plan.Name = revision.Spec.Plan.Name

		if found, err := kubernetes.GetIfExists(ctx, cc, plan); err != nil {
			cond.Failed(err, "Failed to check if a configuration plan exists: %v", err)

			return reconcile.Result{}, err

		} else if !found {
			plan.Spec.Revisions = []terraformv1alpha1.PlanRevision{{
				Name:     revision.Name,
				Revision: revision.Spec.Plan.Revision,
			}}

			if err := cc.Create(ctx, plan); err != nil {
				cond.Failed(err, "Failed to create a configuration plan: %v", err)

				return reconcile.Result{}, err
			}

			return controller.RequeueImmediate, nil
		}

		// @step: else we need check if the revision exists in the plan
		if plan.HasRevision(revision.Spec.Plan.Revision) {
			return reconcile.Result{}, nil
		}

		original := plan.DeepCopy()

		// @step: update the revision list
		plan.Spec.Revisions = append(plan.Spec.Revisions, terraformv1alpha1.PlanRevision{
			Name:     revision.Name,
			Revision: revision.Spec.Plan.Revision,
		})
		if err := cc.Patch(ctx, plan, client.MergeFrom(original)); err != nil {
			cond.Failed(err, "Failed to patch the configuration plan: %v", err)

			return reconcile.Result{}, err
		}

		return reconcile.Result{}, nil
	}
}

// ensureInUseCount is responsible for ensuring the in-use count is correct
func (c *Controller) ensureInUseCount(revision *terraformv1alpha1.Revision) controller.EnsureFunc {
	cond := controller.ConditionMgr(revision, corev1alpha1.ConditionReady, c.recorder)

	return func(ctx context.Context) (reconcile.Result, error) {
		list := &terraformv1alpha1.CloudResourceList{}
		if err := c.cc.List(ctx, list, client.MatchingLabels{
			terraformv1alpha1.CloudResourcePlanNameLabel: revision.Spec.Plan.Name,
			terraformv1alpha1.CloudResourceRevisionLabel: revision.Spec.Plan.Revision,
		}); err != nil {
			cond.Failed(err, "Failed to list cloud resources: %v", err)

			return reconcile.Result{}, err
		}
		// @step: update the in-use count
		revision.Status.InUse = len(list.Items)

		// @step: update the prometheus metric
		revisionTotal.WithLabelValues(
			revision.Spec.Plan.Name,
			revision.Spec.Plan.Revision,
		).Set(float64(revision.Status.InUse))

		return reconcile.Result{}, nil
	}
}
