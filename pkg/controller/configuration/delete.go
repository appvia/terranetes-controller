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

package configuration

import (
	"context"
	"fmt"
	"time"

	log "github.com/sirupsen/logrus"
	batchv1 "k8s.io/api/batch/v1"
	v1 "k8s.io/api/core/v1"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	corev1alpha1 "github.com/appvia/terranetes-controller/pkg/apis/core/v1alpha1"
	terraformv1alpha1 "github.com/appvia/terranetes-controller/pkg/apis/terraform/v1alpha1"
	"github.com/appvia/terranetes-controller/pkg/controller"
	"github.com/appvia/terranetes-controller/pkg/utils"
	"github.com/appvia/terranetes-controller/pkg/utils/filters"
	"github.com/appvia/terranetes-controller/pkg/utils/jobs"
	"github.com/appvia/terranetes-controller/pkg/utils/kubernetes"
)

// ensureTerraformDestroy is responsible for deleting any associated terraform configuration
func (c *Controller) ensureTerraformDestroy(configuration *terraformv1alpha1.Configuration, state *state) controller.EnsureFunc {
	cond := controller.ConditionMgr(configuration, corev1alpha1.ConditionReady, c.recorder)
	generation := fmt.Sprintf("%d", configuration.GetGeneration())

	return func(ctx context.Context) (reconcile.Result, error) {
		// @step: if the configuration has the orphan label we can skip the deletion step
		if configuration.GetAnnotations()[terraformv1alpha1.OrphanAnnotation] == "true" {
			return reconcile.Result{}, nil
		}

		// else we are deleting the resource
		configuration.Status.ResourceStatus = terraformv1alpha1.DestroyingResources

		// @step: ensure we have a status and the resource count has been defined
		if configuration.Status.Resources != nil {
			if ptr.Deref(configuration.Status.Resources, 0) == 0 {
				c.recorder.Event(configuration, v1.EventTypeNormal, "DeletionSkipped", "Configuration had zero resources, skipping terraform destroy")

				return reconcile.Result{}, nil
			}
		}

		// @step: check we have a terraform state - else we can just continue
		secret := &v1.Secret{}
		secret.Namespace = c.ControllerNamespace
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
			WithGeneration(generation).
			WithName(configuration.GetName()).
			WithLabel(terraformv1alpha1.RetryAnnotation, configuration.GetAnnotations()[terraformv1alpha1.RetryAnnotation]).
			WithNamespace(configuration.GetNamespace()).
			WithStage(terraformv1alpha1.StageTerraformDestroy).
			WithUID(string(configuration.GetUID())).
			Latest()

		// @step: generate the destroy job
		batch := jobs.New(configuration, state.provider)
		runner, err := batch.NewTerraformDestroy(jobs.Options{
			AdditionalJobAnnotations: state.provider.JobAnnotations(),
			AdditionalJobSecrets:     state.additionalJobSecrets,
			AdditionalJobLabels: utils.MergeStringMaps(
				c.ControllerJobLabels,
				state.provider.JobLabels(),
				configuration.GetLabels(),
				map[string]string{
					terraformv1alpha1.RetryAnnotation: configuration.GetAnnotations()[terraformv1alpha1.RetryAnnotation],
				}),
			BackoffLimit:     c.BackoffLimit,
			BinaryPath:       c.BinaryPath,
			EnableInfraCosts: c.EnableInfracosts,
			ExecutorImage:    c.ExecutorImage,
			ExecutorSecrets:  c.ExecutorSecrets,
			InfracostsImage:  c.InfracostsImage,
			InfracostsSecret: c.InfracostsSecretName,
			Namespace:        c.ControllerNamespace,
			Template:         state.jobTemplate,
			Image:   GetTerraformImage(configuration, c.TerraformImage),
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

			if c.EnableWatchers {
				// @step: retrieve the current state of the namespace
				ns := &v1.Namespace{}
				ns.Name = configuration.GetNamespace()

				// @step: retrieve the current state of the namespace
				if _, err := kubernetes.GetIfExists(ctx, c.cc, ns); err != nil {
					cond.Failed(err, "Failed to check for the namespace")

					return reconcile.Result{}, err
				}

				// @step: as long as the namespace is not terminating we can create a job
				if ns.DeletionTimestamp.IsZero() {
					if err := c.CreateWatcher(ctx, configuration, terraformv1alpha1.StageTerraformDestroy); err != nil {
						cond.Failed(err, "Failed to create the terraform destroy watcher")

						return reconcile.Result{}, err
					}
				}
			}
			cond.InProgress("Terraform destroy is running")

			return reconcile.Result{RequeueAfter: 5 * time.Second}, nil
		}

		switch {
		case jobs.IsComplete(job):
			cond.Success("Terraform destroy is complete")
			return reconcile.Result{}, nil

		case jobs.IsFailed(job):
			cond.Failed(nil, "Terraform destroy has failed")
			configuration.Status.ResourceStatus = terraformv1alpha1.DestroyingResourcesFailed

			return reconcile.Result{}, controller.ErrIgnore

		case jobs.IsActive(job):
			cond.InProgress("Terraform destroy is running")
		}

		return reconcile.Result{RequeueAfter: 5 * time.Second}, nil
	}
}

// ensureConfigurationSecretsDeleted is responsible for deleting any associated terraform state
func (c *Controller) ensureConfigurationSecretsDeleted(configuration *terraformv1alpha1.Configuration) controller.EnsureFunc {
	cond := controller.ConditionMgr(configuration, corev1alpha1.ConditionReady, c.recorder)

	return func(ctx context.Context) (reconcile.Result, error) {
		names := []string{
			configuration.GetTerraformConfigSecretName(),
			configuration.GetTerraformCostSecretName(),
			configuration.GetTerraformPolicySecretName(),
			configuration.GetTerraformStateSecretName(),
			configuration.GetTerraformPlanOutSecretName(),
			configuration.GetTerraformPlanJSONSecretName(),
		}

		for _, name := range names {
			secret := &v1.Secret{}
			secret.Namespace = c.ControllerNamespace
			secret.Name = name

			if err := kubernetes.DeleteIfExists(ctx, c.cc, secret); err != nil {
				cond.Failed(err, "Failed to delete the configuration secret (%s/%s)", secret.Namespace, secret.Name)

				return reconcile.Result{}, err
			}
		}

		return reconcile.Result{}, nil
	}
}

// ensureConfigurationJobsDeleted is responsible for deleting any associated terraform configuration jobs
func (c *Controller) ensureConfigurationJobsDeleted(configuration *terraformv1alpha1.Configuration) controller.EnsureFunc {
	cond := controller.ConditionMgr(configuration, corev1alpha1.ConditionReady, c.recorder)

	return func(ctx context.Context) (reconcile.Result, error) {
		list := &batchv1.JobList{}

		err := c.cc.List(ctx, list, &client.MatchingLabels{
			terraformv1alpha1.ConfigurationNameLabel:      configuration.Name,
			terraformv1alpha1.ConfigurationNamespaceLabel: configuration.Namespace,
		})
		if err != nil {
			cond.Failed(err, "Failed to list all the configuration jobs")

			return reconcile.Result{}, err
		}

		if len(list.Items) == 0 {
			return reconcile.Result{}, nil
		}

		for i := 0; i < len(list.Items); i++ {
			job := list.Items[i]
			if err := kubernetes.DeleteIfExists(ctx, c.cc, &job); err != nil {
				cond.Failed(err, "Failed to delete the configuration job (%s/%s)", job.Namespace, job.Name)

				return reconcile.Result{}, err
			}
		}

		// @step: log information around the removal
		log.WithFields(log.Fields{
			"name":      configuration.GetName(),
			"namespace": configuration.GetNamespace(),
		}).Info("successfully deleted the configuration")

		c.recorder.Event(configuration, v1.EventTypeNormal, "ConfigurationDeleted", "Configuration has been deleted")

		return reconcile.Result{}, nil
	}
}
