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
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/sirupsen/logrus"
	batchv1 "k8s.io/api/batch/v1"
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

// ensureCostAnalyticsSecret is responsible for ensuring the cost analytics secret is available. This secret is added into
// the job namespace by the platform administrator - but it's possible someone has deleted / changed it - so better to
// place guard around it
func (c *Controller) ensureCostAnalyticsSecret(configuration *terraformv1alphav1.Configuration) controller.EnsureFunc {
	cond := controller.ConditionMgr(configuration, corev1alphav1.ConditionReady)

	return func(ctx context.Context) (reconcile.Result, error) {
		if !c.EnableCostAnalytics {
			return reconcile.Result{}, nil
		}

		secret := &v1.Secret{}
		secret.Namespace = c.JobNamespace
		secret.Name = c.CostAnalyticsSecretName

		found, err := kubernetes.GetIfExists(ctx, c.cc, secret)
		if err != nil {
			cond.Failed(err, "Failed to retrieve the cost analytics secret")

			return reconcile.Result{}, err
		}
		if !found {
			cond.ActionRequired("Cost analytics secret (%s/%s) does not exist, contact platform administrator", secret.Namespace, secret.Name)

			return reconcile.Result{RequeueAfter: 10 * time.Minute}, nil
		}

		// @step: check the secret contains a token
		if secret.Data["INFRACOST_API_KEY"] == nil {
			cond.ActionRequired("Cost analytics secret (%s/%s) does not contain a token, contact platform administrator", secret.Namespace, secret.Name)

			return reconcile.Result{RequeueAfter: 10 * time.Minute}, nil
		}

		return reconcile.Result{}, nil
	}
}

// ensurePoliciesList is responsible for retrieving all the policies in the cluster before we start processing this job. These
// policies are used further down the line by other ensure methods
func (c *Controller) ensurePoliciesList(configuration *terraformv1alphav1.Configuration, state *state) controller.EnsureFunc {
	cond := controller.ConditionMgr(configuration, corev1alphav1.ConditionReady)

	return func(ctx context.Context) (reconcile.Result, error) {
		list := &terraformv1alphav1.PolicyList{}

		if err := c.cc.List(ctx, list); err != nil {
			cond.Failed(err, "Failed to list the policies in cluster")

			return reconcile.Result{}, err
		}

		state.policies = list

		return reconcile.Result{}, nil
	}
}

// ensureAuthenticationSecret is responsible for verifying that any secret which is referenced by the
// configuration does exist
func (c *Controller) ensureAuthenticationSecret(configuration *terraformv1alphav1.Configuration, state *state) controller.EnsureFunc {
	cond := controller.ConditionMgr(configuration, corev1alphav1.ConditionReady)

	return func(ctx context.Context) (reconcile.Result, error) {
		if configuration.Spec.Auth == nil {
			return reconcile.Result{}, nil
		}

		secret := &v1.Secret{}
		secret.Namespace = configuration.Namespace
		secret.Name = configuration.Spec.Auth.Name

		found, err := kubernetes.GetIfExists(ctx, c.cc, secret)
		if err != nil {
			cond.Failed(err, "Failed to retrieve the authentication secret: (%s/%s)", secret.Namespace, secret.Name)

			return reconcile.Result{}, err
		}
		if !found {
			cond.ActionRequired("Authentication secret (spec.scmAuth) does not exist")

			return reconcile.Result{}, controller.ErrIgnore
		}

		// @step: ensure the format of the secret
		switch {
		case secret.Data["SSH_AUTH_KEY"] != nil:
		case secret.Data["GIT_USERNAME"] != nil && secret.Data["GIT_PASSWORD"] != nil:
		default:
			cond.ActionRequired("Authentication secret needs either GIT_USERNAME & GIT_PASSWORD or SSH_AUTH_KEY")

			return reconcile.Result{}, controller.ErrIgnore
		}

		state.auth = secret

		return reconcile.Result{}, nil
	}
}

// ensureJobsList is responsible for retrieving all the jobs in the configuration namespace - these are used by ensure methods
// further down the line
func (c *Controller) ensureJobsList(configuration *terraformv1alphav1.Configuration, state *state) controller.EnsureFunc {
	cond := controller.ConditionMgr(configuration, corev1alphav1.ConditionReady)

	return func(ctx context.Context) (reconcile.Result, error) {
		list := &batchv1.JobList{}

		if err := c.cc.List(ctx, list, client.InNamespace(c.JobNamespace)); err != nil {
			cond.Failed(err, "Failed to list the jobs in controller namespace")

			return reconcile.Result{}, err
		}

		state.jobs = list

		return reconcile.Result{}, nil
	}
}

// ensureNoPreviousGeneration is responsible for ensuring there active jobs are running for this configuration, if so we act
// safely and wait for the job to finish
func (c *Controller) ensureNoPreviousGeneration(configuration *terraformv1alphav1.Configuration, state *state) controller.EnsureFunc {

	return func(ctx context.Context) (reconcile.Result, error) {
		list, found := filters.Jobs(state.jobs).
			WithNamespace(configuration.GetNamespace()).
			WithName(configuration.GetName()).
			WithUID(string(configuration.GetUID())).
			List()
		if !found {
			return reconcile.Result{}, nil
		}

		// @step: iterate the items, if running AND from another generation, we wait
		for _, x := range list.Items {

			if !jobs.IsComplete(&x) && !jobs.IsFailed(&x) {
				if x.GetGeneration() != configuration.GetGeneration() {
					logrus.WithFields(logrus.Fields{
						"generation": x.GetGeneration(),
						"name":       x.GetName(),
						"namespace":  x.GetNamespace(),
						"uid":        x.GetUID(),
					}).Info("found a previous generation job, waiting for it to finish")

					return reconcile.Result{RequeueAfter: 10 * time.Second}, nil
				}
			}
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

		found, err := kubernetes.GetIfExists(ctx, c.cc, provider)
		if err != nil {
			cond.Failed(err, "Failed to retrieve the provider for the configuration: (%s/%s)", provider.Namespace, provider.Name)

			return reconcile.Result{}, err
		}
		if !found {
			cond.ActionRequired("Provider referenced (%s/%s) does not exist", provider.Namespace, provider.Name)

			return reconcile.Result{RequeueAfter: 5 * time.Minute}, nil
		}

		// @step: we need to check the status of the provider to ensure it's ready to be used
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

// ensureGeneratedConfig is responsible in ensuring the terraform configuration is generated for this job. This
// includes the backend configuration and the variables which have been included in the configuration
func (c *Controller) ensureGeneratedConfig(configuration *terraformv1alphav1.Configuration, state *state) controller.EnsureFunc {
	cond := controller.ConditionMgr(configuration, corev1alphav1.ConditionReady)
	backend := string(configuration.GetUID())
	name := configuration.GetTerraformConfigSecretName()

	return func(ctx context.Context) (reconcile.Result, error) {
		secret := &v1.Secret{}
		secret.Namespace = c.JobNamespace
		secret.Name = name

		if _, err := kubernetes.GetIfExists(ctx, c.cc, secret); err != nil {
			cond.Failed(err, "Failed to check for configuration secret")

			return reconcile.Result{}, err
		}

		secret.Labels = map[string]string{
			terraformv1alphav1.ConfigurationNameLabel:      configuration.Name,
			terraformv1alphav1.ConfigurationNamespaceLabel: configuration.Namespace,
		}

		// @step: generate the terraform backend configuration - this creates a kubernetes terraform backend
		// pointing at a secret
		cfg, err := terraform.NewKubernetesBackend(c.JobNamespace, backend)
		if err != nil {
			cond.Failed(err, "Failed to generate the terraform backend configuration")

			return reconcile.Result{}, err
		}
		secret.Data = map[string][]byte{
			terraformv1alphav1.TerraformBackendConfigMapKey: cfg,
		}

		// @step: generate the provider for the terraform configuration
		cfg, err = terraform.NewTerraformProvider(string(state.provider.Spec.Provider))
		if err != nil {
			cond.Failed(err, "Failed to generate the terraform provider configuration")

			return reconcile.Result{}, err
		}
		secret.Data[terraformv1alphav1.TerraformProviderConfigMapKey] = cfg

		// @step: if the any variables for this job lets create the variables configmap
		if configuration.HasVariables() {
			secret.Data[terraformv1alphav1.TerraformVariablesConfigMapKey] = configuration.Spec.Variables.Raw
		} else {
			delete(secret.Data, terraformv1alphav1.TerraformVariablesConfigMapKey)
		}

		// @step: copy any authentication details into the secret
		if state.auth != nil {
			for k, v := range state.auth.Data {
				secret.Data[k] = v
			}
		}

		if err := kubernetes.CreateOrPatch(ctx, c.cc, secret); err != nil {
			cond.Failed(err, "Failed to create or update the configuration secret")

			return reconcile.Result{}, err
		}

		return reconcile.Result{}, nil
	}
}

// ensureTerraformPlan is responsible for ensuring the terraform plan is running or has already ran for this generation. We
// consult the status of the resource to check the status of a stage at generation x
func (c *Controller) ensureTerraformPlan(configuration *terraformv1alphav1.Configuration, state *state) controller.EnsureFunc {
	cond := controller.ConditionMgr(configuration, terraformv1alphav1.ConditionTerraformPlan)
	generation := fmt.Sprintf("%d", configuration.GetGeneration())

	return func(ctx context.Context) (reconcile.Result, error) {
		switch {
		// @note: this is effectively checking the status of plan condition - if the condition is True for the given generation
		// we can say the plan has already been run and move on
		case cond.GetCondition().IsComplete(configuration.GetGeneration()):
			return reconcile.Result{}, nil
		}

		// @step: generate the job
		batch := jobs.New(configuration, state.provider)
		runner, err := batch.NewTerraformPlan(jobs.Options{
			ExecutorImage:      c.ExecutorImage,
			Namespace:          c.JobNamespace,
			EnableCostAnalysis: c.EnableCostAnalytics,
			CostAnalysisSecret: c.CostAnalyticsSecretName,
		})
		if err != nil {
			cond.Failed(err, "Failed to create the terraform plan job")

			return reconcile.Result{}, err
		}

		// @step: search for any current jobs
		job, found := filters.Jobs(state.jobs).
			WithGeneration(generation).
			WithNamespace(configuration.GetNamespace()).
			WithName(configuration.GetName()).
			WithStage(terraformv1alphav1.StageTerraformPlan).
			WithUID(string(configuration.GetUID())).
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

// ensureCostStatus is responsible for updating the cost status post a plan
func (c *Controller) ensureCostStatus(configuration *terraformv1alphav1.Configuration) controller.EnsureFunc {
	cond := controller.ConditionMgr(configuration, corev1alphav1.ConditionReady)

	return func(ctx context.Context) (reconcile.Result, error) {
		if !c.EnableCostAnalytics {
			return reconcile.Result{}, nil
		}

		secret := &v1.Secret{}
		secret.Namespace = c.JobNamespace
		secret.Name = configuration.GetTerraformCostSecretName()

		found, err := kubernetes.GetIfExists(ctx, c.cc, secret)
		if err != nil {
			cond.Failed(err, "Failed to get the terraform costs secret")

			return reconcile.Result{}, err
		}
		if !found {
			configuration.Status.Costs = &terraformv1alphav1.CostStatus{Enabled: false}

			return reconcile.Result{}, nil
		}

		// @step: parse the cost report json
		if secret.Data["costs.json"] != nil {
			report := make(map[string]interface{})
			if err := json.NewDecoder(bytes.NewReader(secret.Data["costs.json"])).Decode(&report); err != nil {
				cond.Failed(err, "Failed to decode the terraform costs report")

				return reconcile.Result{}, err
			}

			configuration.Status.Costs = &terraformv1alphav1.CostStatus{
				Enabled: true,
				Hourly:  fmt.Sprintf("$%v", report["totalHourlyCost"]),
				Monthly: fmt.Sprintf("$%v", report["totalMonthlyCost"]),
			}
		}

		return reconcile.Result{}, nil
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
			ExecutorImage: c.ExecutorImage,
			Namespace:     c.JobNamespace,
		})
		if err != nil {
			cond.Failed(err, "Failed to create the terraform apply job")

			return reconcile.Result{}, err
		}

		// @step: find the job which is implementing this stage if any
		job, found := filters.Jobs(state.jobs).
			WithGeneration(generation).
			WithNamespace(configuration.GetNamespace()).
			WithName(configuration.GetName()).
			WithStage(terraformv1alphav1.StageTerraformApply).
			WithUID(string(configuration.GetUID())).
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
func (c *Controller) ensureTerraformStatus(configuration *terraformv1alphav1.Configuration) controller.EnsureFunc {
	cond := controller.ConditionMgr(configuration, corev1alphav1.ConditionReady)

	return func(ctx context.Context) (reconcile.Result, error) {

		// @step: check we have a terraform state - else we can just continue
		secret := &v1.Secret{}
		secret.Namespace = c.JobNamespace
		secret.Name = configuration.GetTerraformStateSecretName()

		found, err := kubernetes.GetIfExists(ctx, c.cc, secret)
		if err != nil {
			cond.Failed(err, "Failed to get the terraform state secret")

			return reconcile.Result{}, err
		}
		if !found {
			cond.Failed(nil, "Terraform state secret not found")

			return reconcile.Result{}, nil
		}

		state, err := terraform.DecodeState(secret.Data["tfstate"])
		if err != nil {
			cond.Failed(err, "Failed to decode the terraform state")

			return reconcile.Result{}, err
		}

		configuration.Status.Resources = state.CountResources()

		return reconcile.Result{}, nil
	}
}

// ensureTerraformSecret is responsible for ensuring the jobs ran successfully
func (c *Controller) ensureTerraformSecret(configuration *terraformv1alphav1.Configuration) controller.EnsureFunc {
	cond := controller.ConditionMgr(configuration, corev1alphav1.ConditionReady)
	name := configuration.GetTerraformStateSecretName()

	return func(ctx context.Context) (reconcile.Result, error) {
		// @step: if no secrets have been created we can defer and move successful
		if configuration.Spec.WriteConnectionSecretToRef == nil {
			cond.Success("Terraform has completed successfully")

			return reconcile.Result{}, nil
		}

		// @step: read the terraform state
		ss := &v1.Secret{}
		ss.Name = name
		ss.Namespace = c.JobNamespace

		found, err := kubernetes.GetIfExists(ctx, c.cc, ss)
		if err != nil {
			cond.Failed(err, "Failed to get terraform state secret (%s/%s)", c.JobNamespace, name)

			return reconcile.Result{}, err
		}
		if !found {
			cond.Failed(nil, "Terraform state secret (%s/%s) not found", c.JobNamespace, name)

			return reconcile.Result{}, controller.ErrIgnore
		}

		// @step: decode the terraform state (essentially just returning the uncompressed content)
		state, err := terraform.DecodeState(ss.Data["tfstate"])
		if err != nil {
			cond.Failed(err, "Failed to decode the terraform state")

			return reconcile.Result{}, err
		}

		// @step: check if we have any module outputs and if found, we convert the outputs to a
		// kubernetes secret
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
					cond.Failed(err, "Failed to convert the terraform output to a value, key: %s, value: %v", k, v)

					return reconcile.Result{}, err
				}
				secret.Data[strings.ToUpper(k)] = []byte(value)
			}

			// @step: create the terraform secret
			if err := kubernetes.CreateOrForceUpdate(ctx, c.cc, secret); err != nil {
				cond.Failed(err, "Failed to create the terraform state secret")

				return reconcile.Result{}, err
			}
		}
		cond.Success("Terraform has completed successfully")

		return reconcile.Result{}, nil
	}
}
