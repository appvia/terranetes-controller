/*
 * Copyright (C) 2022 Rohith Jayawardene <gambol99@gmail.com>
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
	"strings"
	"time"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	corev1alphav1 "github.com/appvia/terraform-controller/pkg/apis/core/v1alpha1"
	terraformv1alphav1 "github.com/appvia/terraform-controller/pkg/apis/terraform/v1alpha1"
	"github.com/appvia/terraform-controller/pkg/controller"
	"github.com/appvia/terraform-controller/pkg/utils/filters"
	"github.com/appvia/terraform-controller/pkg/utils/jobs"
	"github.com/appvia/terraform-controller/pkg/utils/kubernetes"
	"github.com/appvia/terraform-controller/pkg/utils/terraform"
)

// ensureJobsList is responsible for retrieving all the current jobs for this configuration
func (c *Controller) ensureJobsList(configuration *terraformv1alphav1.Configuration, state *state) controller.EnsureFunc {
	cond := controller.ConditionMgr(configuration, corev1alphav1.ConditionReady)

	return func(ctx context.Context) (reconcile.Result, error) {
		list, err := c.ListJobs(ctx, configuration)
		if err != nil {
			cond.Failed(err, "Failed to find the configuration jobs")

			return reconcile.Result{}, err
		}
		state.jobs = list

		return reconcile.Result{}, nil
	}
}

// ensureNoPreviousGeneration is responsible for ensuring there active jobs are running for this
// configuration, if so we act safely and wait for the job to finish
func (c *Controller) ensureNoPreviousGeneration(configuration *terraformv1alphav1.Configuration, state *state) controller.EnsureFunc {
	return func(ctx context.Context) (reconcile.Result, error) {
		currentGeneration := configuration.Generation
		if currentGeneration <= 1 {
			return reconcile.Result{}, nil
		}

		return reconcile.Result{}, nil
	}
}

// ensureProviderIsReady is responsible for ensuring the provider referenced by this configuration is ready
func (c *Controller) ensureProviderIsReady(configuration *terraformv1alphav1.Configuration, state *state) controller.EnsureFunc {
	cond := controller.ConditionMgr(configuration, terraformv1alphav1.ConditionProviderReady)

	return func(ctx context.Context) (reconcile.Result, error) {
		provider := &terraformv1alphav1.Provider{}
		provider.Namespace = configuration.Spec.ProviderRef.Namespace
		provider.Name = configuration.Spec.ProviderRef.Name

		// @step: we can default the provider namespace to the running namespace if not set
		if provider.Namespace == "" {
			provider.Namespace = c.JobNamespace
		}

		found, err := kubernetes.GetIfExists(ctx, c.cc, provider)
		if err != nil {
			cond.Failed(err, "Failed to retrieve the provider for the configuration")

			return reconcile.Result{}, err
		}
		if !found {
			cond.ActionRequired("Provider referenced (%s/%s) does not exist", provider.Namespace, provider.Name)

			return reconcile.Result{RequeueAfter: 5 * time.Minute}, nil
		}

		// @step: check the status of the provider
		status := provider.Status.GetCondition(corev1alphav1.ConditionReady)
		if status.Status != metav1.ConditionTrue {
			cond.Warning("Provider is not ready")

			return reconcile.Result{RequeueAfter: 30 * time.Second}, nil
		}
		cond.Success("Provider ready")

		state.provider = provider

		return reconcile.Result{}, nil
	}
}

// ensureGeneratedConfig is responsible the terraform configuration is generated for this job to run. This
// includes the backend configuration and the variables
func (c *Controller) ensureGeneratedConfig(configuration *terraformv1alphav1.Configuration, state *state) controller.EnsureFunc {
	cond := controller.ConditionMgr(configuration, corev1alphav1.ConditionReady)
	name := string(configuration.GetUID())

	return func(ctx context.Context) (reconcile.Result, error) {
		cm := &v1.ConfigMap{}
		cm.Namespace = c.JobNamespace
		cm.Name = name

		// @step: check if the configmap exists
		if _, err := kubernetes.GetIfExists(ctx, c.cc, cm); err != nil {
			cond.Failed(err, "Failed to retrieve the terraform job configuration configmap")

			return reconcile.Result{}, err
		}
		cm.Labels = map[string]string{
			terraformv1alphav1.ConfigurationNameLabel:      configuration.Name,
			terraformv1alphav1.ConfigurationNamespaceLabel: configuration.Namespace,
		}

		// @step: generate the terraform backend configuration - this creates a kubernetes terraform backend
		// pointing at a secret
		cfg, err := terraform.NewKubernetesBackend(c.JobNamespace, name)
		if err != nil {
			cond.Failed(err, "Failed to generate the terraform backend configuration")

			return reconcile.Result{}, err
		}
		cm.Data = map[string]string{
			terraformv1alphav1.TerraformBackendConfigMapKey: string(cfg),
		}

		// @step: generate the provider for the terraform configuration
		cfg, err = terraform.NewTerraformProvider(string(state.provider.Spec.Provider))
		if err != nil {
			cond.Failed(err, "Failed to generate the terraform provider configuration")

			return reconcile.Result{}, err
		}
		cm.Data[terraformv1alphav1.TerraformProviderConfigMapKey] = string(cfg)

		// @step: if the any variables for this job lets create the variables configmap
		if configuration.HasVariables() {
			cm.Data[terraformv1alphav1.TerraformVariablesConfigMapKey] = string(configuration.Spec.Variables.Raw)
		} else {
			delete(cm.Data, terraformv1alphav1.TerraformVariablesConfigMapKey)
		}

		// @step: create or update the configmap
		if err := kubernetes.CreateOrPatch(ctx, c.cc, cm); err != nil {
			cond.Failed(err, "Failed to create or update the terraform backend configmap")

			return reconcile.Result{}, err
		}

		return reconcile.Result{}, nil
	}
}

// ensureTerraformPlan is responsible for ensuring the terraform plan is running or ran
func (c *Controller) ensureTerraformPlan(configuration *terraformv1alphav1.Configuration, state *state) controller.EnsureFunc {
	cond := controller.ConditionMgr(configuration, terraformv1alphav1.ConditionTerraformPlan)
	generation := fmt.Sprintf("%d", configuration.GetGeneration())

	return func(ctx context.Context) (reconcile.Result, error) {
		switch {
		case cond.GetCondition().IsComplete(configuration.GetGeneration()):
			return reconcile.Result{}, nil
		}

		// @step: generate the job
		batch := jobs.New(configuration, state.provider)
		runner, err := batch.NewTerraformPlan(jobs.Options{
			GitImage:         c.GitImage,
			Namespace:        c.JobNamespace,
			TerraformImage:   c.TerraformImage,
			TerraformVersion: c.TerraformVersion,
		})
		if err != nil {
			cond.Failed(err, "Failed to create the terraform plan job")

			return reconcile.Result{}, err
		}

		// @step: search for any current jobs
		job, found := filters.Jobs(state.jobs).
			WithStage(terraformv1alphav1.StageTerraformPlan).
			WithGeneration(generation).
			Latest()

		if !found {
			// @step: if auto approval is not enabled we should annotate the configuration with
			// the need to approve
			if !configuration.Spec.EnableAutoApproval && !configuration.NeedsApproval() {

				original := configuration.DeepCopy()
				if configuration.Annotations == nil {
					configuration.Annotations = map[string]string{}
				}
				configuration.Annotations[terraformv1alphav1.ApplyAnnotation] = "false"

				if err := c.cc.Patch(ctx, configuration, client.MergeFrom(original)); err != nil {
					cond.Failed(err, "Failed to create or update the terraform configuration")

					return reconcile.Result{}, err
				}

				return controller.RequeueImmediate, nil
			}

			// @step: create a watch job in the configuration namespace to allow the user to witness
			// the terraform output
			if err := c.CreateWatcher(ctx, configuration, terraformv1alphav1.StageTerraformPlan); err != nil {
				cond.Failed(err, "Failed to create the terraform plan watcher")

				return reconcile.Result{}, err
			}

			// @step: create the terraform plan job
			if err := c.cc.Create(ctx, runner); err != nil {
				cond.Failed(err, "Failed to create the terraform plan job")

				return reconcile.Result{}, err
			}
			cond.InProgress("Terraform plan in progress")

			return reconcile.Result{RequeueAfter: 5 * time.Second}, nil
		}

		// @step: we only shift out of this state of the job is complete
		switch {
		case jobs.IsComplete(job):
			cond.Success("Terraform plan is complete")
			return reconcile.Result{}, nil

		case jobs.IsFailed(job):
			cond.Failed(nil, "Terraform plan is failed")
			return reconcile.Result{}, controller.ErrIgnore

		case jobs.IsActive(job):
			cond.InProgress("Terraform plan is running")
		}

		return reconcile.Result{RequeueAfter: 10 * time.Second}, nil
	}
}

// ensureTerraformApply is responsible for ensuring the terraform apply is running or run
func (c *Controller) ensureTerraformApply(configuration *terraformv1alphav1.Configuration, state *state) controller.EnsureFunc {
	cond := controller.ConditionMgr(configuration, terraformv1alphav1.ConditionTerraformApply)
	generation := fmt.Sprintf("%d", configuration.GetGeneration())

	return func(ctx context.Context) (reconcile.Result, error) {
		switch {
		case configuration.NeedsApproval():
			cond.ActionRequired("Waiting for terraform apply annotation to be set to true")
			return reconcile.Result{}, controller.ErrIgnore

		case cond.GetCondition().IsComplete(configuration.GetGeneration()):
			return reconcile.Result{}, nil
		}

		// @step: create the terraform job
		runner, err := jobs.New(configuration, state.provider).NewTerraformApply(jobs.Options{
			GitImage:         c.GitImage,
			Namespace:        c.JobNamespace,
			TerraformImage:   c.TerraformImage,
			TerraformVersion: c.TerraformVersion,
		})
		if err != nil {
			cond.Failed(err, "Failed to create the terraform apply job")

			return reconcile.Result{}, err
		}

		// @step: find the job which is implementing this stage if any
		job, found := filters.Jobs(state.jobs).
			WithStage(terraformv1alphav1.StageTerraformApply).
			WithGeneration(generation).
			Latest()

		// @step: we can requeue or move on depending on the status
		if !found {
			if err := c.CreateWatcher(ctx, configuration, terraformv1alphav1.StageTerraformApply); err != nil {
				cond.Failed(err, "Failed to create the terraform apply watcher")

				return reconcile.Result{}, err
			}

			if err := c.cc.Create(ctx, runner); err != nil {
				cond.Failed(err, "Failed to create the terraform apply job")

				return reconcile.Result{}, err
			}
			cond.InProgress("Terraform apply is running")

			return reconcile.Result{RequeueAfter: 5 * time.Second}, nil
		}

		// @step: we only shift out of this state of the job is complete
		switch {
		case jobs.IsComplete(job):
			cond.Success("Terraform apply is complete")
			return reconcile.Result{}, nil

		case jobs.IsFailed(job):
			cond.Failed(nil, "Terraform apply has failed")
			return reconcile.Result{}, controller.ErrIgnore

		case jobs.IsActive(job):
			cond.InProgress("Terraform apply in progress")

		}

		return reconcile.Result{RequeueAfter: 5 * time.Second}, nil
	}
}

// ensureTerraformStatus is responsible for updating the configuration status
func (c *Controller) ensureTerraformStatus(configuration *terraformv1alphav1.Configuration, state *state) controller.EnsureFunc {
	condition := controller.ConditionMgr(configuration, corev1alphav1.ConditionReady)

	return func(ctx context.Context) (reconcile.Result, error) {
		// @step: check we have a terraform state - else we can just continue
		secret := &v1.Secret{}
		secret.Namespace = c.JobNamespace
		secret.Name = configuration.GetTerraformStateSecretName()

		found, err := kubernetes.GetIfExists(ctx, c.cc, secret)
		if err != nil {
			condition.Failed(err, "Failed to get the terraform state secret")

			return reconcile.Result{}, err
		}
		if !found {
			condition.Failed(nil, "Terraform state secret not found")

			return reconcile.Result{}, nil
		}

		state, err := terraform.DecodeState(secret.Data["tfstate"])
		if err != nil {
			condition.Failed(err, "Failed to decode the terraform state")

			return reconcile.Result{}, err
		}

		configuration.Status.TerraformVersion = state.TerraformVersion
		configuration.Status.Resources = state.CountResources()

		return reconcile.Result{}, nil
	}
}

// ensureTerraformSecret is responsible for ensuring the jobs ran successfully
func (c *Controller) ensureTerraformSecret(configuration *terraformv1alphav1.Configuration) controller.EnsureFunc {
	condition := controller.ConditionMgr(configuration, corev1alphav1.ConditionReady)
	name := configuration.GetTerraformStateSecretName()

	return func(ctx context.Context) (reconcile.Result, error) {
		// @step: if no secrets have been created we can defer and move successful
		if configuration.Spec.WriteConnectionSecretToRef == nil {
			condition.Success("Terraform has completed successfully")

			return reconcile.Result{}, nil
		}

		// @step: read the terraform state
		ss := &v1.Secret{}
		ss.Name = name
		ss.Namespace = c.JobNamespace

		found, err := kubernetes.GetIfExists(ctx, c.cc, ss)
		if err != nil {
			condition.Failed(err, "Failed to get terraform state secret (%s/%s)", c.JobNamespace, name)

			return reconcile.Result{}, err
		}
		if !found {
			condition.Failed(nil, "Terraform state secret (%s/%s) not found", c.JobNamespace, name)

			return reconcile.Result{}, controller.ErrIgnore
		}

		// @step: decode the terraform state (essentially just returning the uncompressed content)
		state, err := terraform.DecodeState(ss.Data["tfstate"])
		if err != nil {
			condition.Failed(err, "Failed to decode the terraform state")

			return reconcile.Result{}, err
		}

		// @step: check if we have any module outputs
		if state.HasOutputs() {
			secret := &v1.Secret{}
			secret.Namespace = configuration.Namespace
			secret.Name = configuration.Spec.WriteConnectionSecretToRef.Name
			secret.OwnerReferences = []metav1.OwnerReference{
				*metav1.NewControllerRef(configuration, configuration.GroupVersionKind()),
			}
			secret.Labels = map[string]string{
				terraformv1alphav1.ConfigurationNameLabel:      configuration.Name,
				terraformv1alphav1.ConfigurationNamespaceLabel: configuration.Namespace,
			}
			secret.Data = make(map[string][]byte)

			for k, v := range state.Outputs {
				value, err := v.ToValue()
				if err != nil {
					condition.Failed(err, "Failed to convert the terraform output to a value, key: %s, value: %v", k, v)

					return reconcile.Result{}, err
				}
				secret.Data[strings.ToUpper(k)] = []byte(value)
			}

			// @step: create the terraform secret
			if err := kubernetes.CreateOrForceUpdate(ctx, c.cc, secret); err != nil {
				condition.Failed(err, "Failed to create the terraform state secret")

				return reconcile.Result{}, err
			}
		}
		condition.Success("Terraform has completed successfully")

		return reconcile.Result{}, nil
	}
}
