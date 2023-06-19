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

package plan

import (
	"context"

	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	corev1alpha1 "github.com/appvia/terranetes-controller/pkg/apis/core/v1alpha1"
	terraformv1alpha1 "github.com/appvia/terranetes-controller/pkg/apis/terraform/v1alpha1"
	"github.com/appvia/terranetes-controller/pkg/controller"
	"github.com/appvia/terranetes-controller/pkg/utils"
)

// ensureLatestOnPlan is responsible for ensuring the latest revision is on the status
func (c *Controller) ensureLatestOnPlan(plan *terraformv1alpha1.Plan) controller.EnsureFunc {
	cond := controller.ConditionMgr(plan, corev1alpha1.ConditionReady, c.recorder)

	return func(ctx context.Context) (reconcile.Result, error) {
		switch {
		case len(plan.Spec.Revisions) <= 0:
			return reconcile.Result{}, nil
		}

		// @step: grab the latest
		latest, err := utils.LatestSemverVersion(plan.ListRevisions())
		if err != nil {
			cond.ActionRequired("Failed to sort the revision in plan: %s, error: %v", plan.Name, err)

			return reconcile.Result{}, controller.ErrIgnore
		}

		var revision terraformv1alpha1.PlanRevision
		for _, x := range plan.Spec.Revisions {
			if x.Version == latest {
				revision = x
			}
		}

		// @step: update the status
		plan.Status.Latest = revision

		return reconcile.Result{}, nil
	}
}

// ensurePlanDeleted is responsible for deleting any plans which no longer have any revisions
func (c *Controller) ensurePlanDeleted(plan *terraformv1alpha1.Plan) controller.EnsureFunc {
	cond := controller.ConditionMgr(plan, corev1alpha1.ConditionReady, c.recorder)
	cc := c.cc

	return func(ctx context.Context) (reconcile.Result, error) {
		if len(plan.Spec.Revisions) <= 0 {
			if err := cc.Delete(ctx, plan); err != nil {
				cond.Failed(err, "Failed to delete plan, as it no longer has any revisions")

				return reconcile.Result{}, err
			}
			c.recorder.Event(plan, "Normal", "DeletedPlan", "Plan has been deleted")
		}

		return reconcile.Result{}, nil
	}
}
