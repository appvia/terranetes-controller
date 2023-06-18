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

	v1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	corev1alpha1 "github.com/appvia/terranetes-controller/pkg/apis/core/v1alpha1"
	terraformv1alpha1 "github.com/appvia/terranetes-controller/pkg/apis/terraform/v1alpha1"
	"github.com/appvia/terranetes-controller/pkg/controller"
	"github.com/appvia/terranetes-controller/pkg/utils/kubernetes"
)

// ensureRevisionDeleted is responsible for making sure the revision is removed any associated plans
func (c *Controller) ensureRevisionDeleted(revision *terraformv1alpha1.Revision) controller.EnsureFunc {
	cond := controller.ConditionMgr(revision, corev1alpha1.ConditionReady, c.recorder)
	cc := c.cc

	return func(ctx context.Context) (reconcile.Result, error) {
		plan := &terraformv1alpha1.Plan{}
		plan.Name = revision.Spec.Plan.Name

		if found, err := kubernetes.GetIfExists(ctx, cc, plan); err != nil {
			cond.Failed(err, "Failed to check for the plan (%s) existence", plan.Name)

			return reconcile.Result{}, err
		} else if !found {
			c.recorder.Eventf(revision, v1.EventTypeNormal, "PlanNotFound",
				"Plan associated to revision: %s not found", revision.Spec.Plan.Name)

			return reconcile.Result{}, nil
		}

		// @step: do we need to remove revision from the plan
		if plan.HasRevision(revision.Spec.Plan.Revision) {
			original := plan.DeepCopy()

			// @step: update and remove the revision from the plan
			plan.RemoveRevision(revision.Spec.Plan.Revision)
			if err := cc.Patch(ctx, plan, client.MergeFrom(original)); err != nil {
				cond.Failed(err, "Failed to remove the revision (%s) from the plan (%s)", revision.Spec.Plan.Revision, plan.Name)

				return reconcile.Result{}, err
			}
		}

		c.recorder.Eventf(revision, v1.EventTypeNormal, "RevisionRemoved",
			"Revision: %s removed from plan: %s", revision.Spec.Plan.Name, plan.Name)

		return reconcile.Result{}, nil
	}
}
