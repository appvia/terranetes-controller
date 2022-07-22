/*
 * Copyright (C) 2022  Appvia Ltd <info@appvia.io>
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

package drift

import (
	"context"
	"fmt"
	"time"

	log "github.com/sirupsen/logrus"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	terraformv1alphav1 "github.com/appvia/terranetes-controller/pkg/apis/terraform/v1alpha1"
	"github.com/appvia/terranetes-controller/pkg/controller"
)

// ensureConfigurationReadyForDrift is responsible for checking the configuration is ready for drift
func (c *Controller) ensureConfigurationReadyForDrift(configuration *terraformv1alphav1.Configuration) controller.EnsureFunc {
	return func(ctx context.Context) (reconcile.Result, error) {
		list := &terraformv1alphav1.ConfigurationList{}
		if err := c.cc.List(ctx, list, client.InNamespace("")); err != nil {
			log.WithError(err).Error("failed to retrieve a list of configurations in cluster")

			return reconcile.Result{}, err
		}

		// @step: count the number of configuration with a drift annotation
		var count float64
		for _, configuration := range list.Items {
			if configuration.Annotations[terraformv1alphav1.DriftAnnotation] != "" {
				count++
			}
		}
		// is the percentage of configurations running a drift now
		running := count / float64(len(list.Items))

		switch {
		// can't really happen due the the predicate - but better safe than sorry; if not enabled or deleting, we ignore
		case !configuration.Spec.EnableDriftDetection, configuration.DeletionTimestamp != nil:
			return reconcile.Result{RequeueAfter: c.CheckInterval}, nil
		case len(configuration.Status.Conditions) == 0:
			return reconcile.Result{RequeueAfter: c.CheckInterval}, nil
		// if the plan condition does not exist, we ignore
		case !configuration.Status.HasCondition(terraformv1alphav1.ConditionTerraformPlan):
			return reconcile.Result{RequeueAfter: c.CheckInterval}, nil
		// if the apply condition does not exist, we ignore
		case !configuration.Status.HasCondition(terraformv1alphav1.ConditionTerraformApply):
			return reconcile.Result{RequeueAfter: c.CheckInterval}, nil
		// if the last plan for this generation failed, we ignore
		case configuration.Status.GetCondition(terraformv1alphav1.ConditionTerraformPlan).IsFailed(configuration.GetGeneration()):
			return reconcile.Result{RequeueAfter: c.CheckInterval}, nil
		// if the last apply for this generation failed, we ignore
		case configuration.Status.GetCondition(terraformv1alphav1.ConditionTerraformApply).IsFailed(configuration.GetGeneration()):
			return reconcile.Result{RequeueAfter: c.CheckInterval}, nil
		// if the configuration plan is already in progress
		case configuration.Status.GetCondition(terraformv1alphav1.ConditionTerraformPlan).InProgress():
			return reconcile.Result{RequeueAfter: c.CheckInterval}, nil
		// if the configuration apply is already in progress
		case configuration.Status.GetCondition(terraformv1alphav1.ConditionTerraformApply).InProgress():
			return reconcile.Result{RequeueAfter: c.CheckInterval}, nil
		// if a plan has not been run on the current generation, we ignore
		case !configuration.Status.GetCondition(terraformv1alphav1.ConditionTerraformPlan).IsComplete(configuration.GetGeneration()):
			return reconcile.Result{RequeueAfter: c.CheckInterval}, nil
		// if the apply for the generation has not been run, we ignore
		case !configuration.Status.GetCondition(terraformv1alphav1.ConditionTerraformApply).IsComplete(configuration.GetGeneration()):
			return reconcile.Result{RequeueAfter: c.CheckInterval}, nil
		// if the last transition on a plan was less than the interval, i.e we've had activity, we ignore
		case configuration.Status.GetCondition(terraformv1alphav1.ConditionTerraformPlan).LastTransitionTime.Add(c.DriftInterval).After(time.Now()):
			return reconcile.Result{RequeueAfter: c.CheckInterval}, nil
		// if the last transition on a apply was less than the interval, i.e we've had activity, we ignore
		case configuration.Status.GetCondition(terraformv1alphav1.ConditionTerraformApply).LastTransitionTime.Add(c.DriftInterval).After(time.Now()):
			return reconcile.Result{RequeueAfter: c.CheckInterval}, nil
		// if the number of active configuration running a drift exceeds the max percentage, we ignore
		case len(list.Items) > 1 && running >= c.DriftThreshold:
			return reconcile.Result{RequeueAfter: c.CheckInterval}, nil
		}

		return reconcile.Result{}, nil
	}
}

// ensureDriftDetection is responsible for triggering off drift detection on the configuration
func (c *Controller) ensureDriftDetection(configuration *terraformv1alphav1.Configuration) controller.EnsureFunc {
	return func(ctx context.Context) (reconcile.Result, error) {
		original := configuration.DeepCopy()

		// @step: we tag the resource with an annotation to trigger the terraform plan
		if configuration.Annotations == nil {
			configuration.Annotations = map[string]string{}
		}

		configuration.Annotations[terraformv1alphav1.DriftAnnotation] = fmt.Sprintf("%d", time.Now().Unix())

		if err := c.cc.Patch(ctx, configuration, client.MergeFrom(original)); err != nil {
			c.recorder.Event(configuration, "Warning", "DriftDetection", "Failed to patch configuration with drift detection annotation")

			return reconcile.Result{}, err
		}
		c.recorder.Event(configuration, "Normal", "DriftDetection", "Triggered drift detection on configuration")

		return reconcile.Result{}, nil
	}
}
