/*
 * Copyright (C) 2022  Rohith Jayawardene <gambol99@gmail.com>
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
	"fmt"
	"time"

	batchv1 "k8s.io/api/batch/v1"
	v1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	corev1alphav1 "github.com/appvia/terraform-controller/pkg/apis/core/v1alpha1"
	terraformv1alphav1 "github.com/appvia/terraform-controller/pkg/apis/terraform/v1alpha1"
	"github.com/appvia/terraform-controller/pkg/controller"
	"github.com/appvia/terraform-controller/pkg/utils/filters"
	"github.com/appvia/terraform-controller/pkg/utils/jobs"
	"github.com/appvia/terraform-controller/pkg/utils/kubernetes"
)

// ensureTerraformDestroy is responsible for deleting any associated terraform configuration
func (c *Controller) ensureTerraformDestroy(configuration *terraformv1alphav1.Configuration, state *state) controller.EnsureFunc {
	cond := controller.ConditionMgr(configuration, corev1alphav1.ConditionReady)
	generation := fmt.Sprintf("%d", configuration.GetGeneration())

	return func(ctx context.Context) (reconcile.Result, error) {
		// @step: if the configuration has the orphan label we can skip the deletion step
		if configuration.GetAnnotations()[terraformv1alphav1.OrphanAnnotation] == "true" {
			return reconcile.Result{}, nil
		}

		// @step: check we have a terraform state - else we can just continue
		secret := &v1.Secret{}
		secret.Namespace = c.JobNamespace
		secret.Name = configuration.GetTerraformStateSecretName()

		found, err := kubernetes.GetIfExists(ctx, c.cc, secret)
		if err != nil {
			cond.Failed(err, "Failed to check for the terraform state secret")

			return reconcile.Result{}, err
		}
		if !found {
			return reconcile.Result{}, nil
		}

		// @step: find any currently running destroy jobs
		job, found := filters.Jobs(state.jobs).
			WithStage(terraformv1alphav1.StageTerraformDestroy).
			WithGeneration(generation).
			Latest()

		// @step: generate the destroy job
		batch := jobs.New(configuration, state.provider)
		runner, err := batch.NewTerraformDestroy(jobs.Options{
			ExecutorImage: c.ExecutorImage,
			GitImage:      c.GitImage,
			Namespace:     c.JobNamespace,
		})
		if err != nil {
			cond.Failed(err, "Failed to create the terraform destroy job")

			return reconcile.Result{}, err
		}

		// @step: we can requeue or move on depending on the status
		if !found {
			if err := c.cc.Create(ctx, runner); err != nil {
				cond.Failed(err, "Failed to create the terraform destroy job")

				return reconcile.Result{}, err
			}

			return reconcile.Result{RequeueAfter: 5 * time.Second}, nil
		}

		switch {
		case jobs.IsComplete(job):
			cond.Success("Terraform destroy is complete")
			return reconcile.Result{}, nil

		case jobs.IsFailed(job):
			cond.Failed(nil, "Terraform destroy is failing")
			return reconcile.Result{RequeueAfter: 30 * time.Second}, nil

		case jobs.IsActive(job):
			cond.InProgress("Terraform destroy is running")
		}

		return reconcile.Result{RequeueAfter: 10 * time.Second}, nil
	}
}

// ensureTerraformConfigDeleted is responsible for deleting any associated terraform configuration configmap
func (c *Controller) ensureTerraformConfigDeleted(configuration *terraformv1alphav1.Configuration) controller.EnsureFunc {
	cond := controller.ConditionMgr(configuration, corev1alphav1.ConditionReady)
	name := string(configuration.GetUID())

	return func(ctx context.Context) (reconcile.Result, error) {
		cm := &v1.ConfigMap{}
		cm.Namespace = c.JobNamespace
		cm.Name = name

		if err := kubernetes.DeleteIfExists(ctx, c.cc, cm); err != nil {
			cond.Failed(err, "Failed to delete the terraform configuration configmap")

			return reconcile.Result{}, err
		}

		return reconcile.Result{}, nil
	}
}

// ensureConfigurationJobsDeleted is responsible for deleting any associated terraform configuration jobs
func (c *Controller) ensureConfigurationJobsDeleted(configuration *terraformv1alphav1.Configuration) controller.EnsureFunc {
	cond := controller.ConditionMgr(configuration, corev1alphav1.ConditionReady)

	return func(ctx context.Context) (reconcile.Result, error) {
		list := &batchv1.JobList{}

		err := c.cc.List(ctx, list, &client.MatchingLabels{
			terraformv1alphav1.ConfigurationNameLabel:      configuration.Name,
			terraformv1alphav1.ConfigurationNamespaceLabel: configuration.Namespace,
		})
		if err != nil {
			cond.Failed(err, "Failed to list all the configuration jobs")

			return reconcile.Result{}, err
		}

		if len(list.Items) < 0 {
			return reconcile.Result{}, nil
		}

		for _, job := range list.Items {
			if err := kubernetes.DeleteIfExists(ctx, c.cc, &job); err != nil {
				cond.Failed(err, "Failed to delete the configuration job (%s/%s)", job.Namespace, job.Name)

				return reconcile.Result{}, err
			}
		}

		return reconcile.Result{}, nil
	}
}

// ensureTerraformStateDeleted is responsible for deleting any associated terraform state
func (c *Controller) ensureTerraformStateDeleted(configuration *terraformv1alphav1.Configuration) controller.EnsureFunc {
	cond := controller.ConditionMgr(configuration, corev1alphav1.ConditionReady)

	return func(ctx context.Context) (reconcile.Result, error) {
		secret := &v1.Secret{}
		secret.Namespace = c.JobNamespace
		secret.Name = configuration.GetTerraformStateSecretName()

		if err := kubernetes.DeleteIfExists(ctx, c.cc, secret); err != nil {
			cond.Failed(err, "Failed to delete the terraform state secret")

			return reconcile.Result{}, err
		}

		return reconcile.Result{}, nil
	}
}
