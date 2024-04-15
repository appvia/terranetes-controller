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
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	log "github.com/sirupsen/logrus"
	"github.com/tidwall/gjson"
	batchv1 "k8s.io/api/batch/v1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	corev1alpha1 "github.com/appvia/terranetes-controller/pkg/apis/core/v1alpha1"
	terraformv1alpha1 "github.com/appvia/terranetes-controller/pkg/apis/terraform/v1alpha1"
	"github.com/appvia/terranetes-controller/pkg/assets"
	"github.com/appvia/terranetes-controller/pkg/controller"
	"github.com/appvia/terranetes-controller/pkg/utils"
	"github.com/appvia/terranetes-controller/pkg/utils/filters"
	"github.com/appvia/terranetes-controller/pkg/utils/jobs"
	"github.com/appvia/terranetes-controller/pkg/utils/kubernetes"
	"github.com/appvia/terranetes-controller/pkg/utils/terraform"
)

// ensureReconcileAnnotation is responsible for ignoring the configuration if the annotation is set
func (c *Controller) ensureReconcileAnnotation(configuration *terraformv1alpha1.Configuration) controller.EnsureFunc {
	cond := controller.ConditionMgr(configuration, corev1alpha1.ConditionReady, c.recorder)

	return func(ctx context.Context) (reconcile.Result, error) {
		// @step: check if we are ignoring the configuration
		// nolint:goconst
		if configuration.GetAnnotations()[terraformv1alpha1.ReconcileAnnotation] == "false" {
			cond.Warning("Configuration has reconciling annotation set as false, ignoring changes")

			return reconcile.Result{}, controller.ErrIgnore
		}

		return reconcile.Result{}, nil
	}
}

// ensureCapturedState is responsible for retrieving various resources required for later ensure methods
func (c *Controller) ensureCapturedState(configuration *terraformv1alpha1.Configuration, state *state) controller.EnsureFunc {
	cond := controller.ConditionMgr(configuration, corev1alpha1.ConditionReady, c.recorder)

	return func(ctx context.Context) (reconcile.Result, error) {
		// @step: retrieve a list of policies in the cluster
		policies := &terraformv1alpha1.PolicyList{}
		if err := c.cc.List(ctx, policies); err != nil {
			cond.Failed(err, "Failed to list the policies in cluster")

			return reconcile.Result{}, err
		}

		// @step: retrieve a list of jobs
		jobs := &batchv1.JobList{}
		if err := c.cc.List(ctx, jobs, client.InNamespace(c.ControllerNamespace)); err != nil {
			cond.Failed(err, "Failed to list the jobs in controller namespace")

			return reconcile.Result{}, err
		}

		// @step: if no sync state is set we determine it out of sync
		if configuration.Status.ResourceStatus == terraformv1alpha1.UnknownResourceStatus {
			configuration.Status.ResourceStatus = terraformv1alpha1.ResourcesOutOfSync
		}

		state.jobs = jobs
		state.policies = policies

		return reconcile.Result{}, nil
	}
}

// ensureNoActivity is responsible for ensuring there active jobs are running for this configuration, if so we act
// safely and wait for the job to finish
func (c *Controller) ensureNoActivity(configuration *terraformv1alpha1.Configuration, state *state) controller.EnsureFunc {

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
		for i := 0; i < len(list.Items); i++ {
			x := list.Items[i]

			if !jobs.IsComplete(&x) && !jobs.IsFailed(&x) {
				if x.GetGeneration() != configuration.GetGeneration() {
					log.WithFields(log.Fields{
						"generation": x.GetGeneration(),
						"name":       x.GetName(),
						"namespace":  x.GetNamespace(),
						"uid":        x.GetUID(),
					}).Info("found a generation already running, waiting to complete")

					return reconcile.Result{RequeueAfter: 10 * time.Second}, nil
				}
			}
		}

		return reconcile.Result{}, nil
	}
}

// ensureCustomBackendTemplate is called to ensure that if configured the backend template for the terraform
// is available and ready. The template can come from the default backend template or from an optional field
// in the provider configuration
func (c *Controller) ensureCustomBackendTemplate(configuration *terraformv1alpha1.Configuration, state *state) controller.EnsureFunc {
	cond := controller.ConditionMgr(configuration, corev1alpha1.ConditionReady, c.recorder)

	return func(ctx context.Context) (reconcile.Result, error) {
		switch {
		case c.HasBackendTemplate():
			break
		case state == nil:
			return reconcile.Result{}, nil
		case state.provider == nil:
			return reconcile.Result{}, nil
		case state.provider.HasBackendTemplate():
			break
		default:
			return reconcile.Result{}, nil
		}

		// @step: we default the controller token
		secret := &v1.Secret{}
		secret.Namespace = c.ControllerNamespace
		secret.Name = c.BackendTemplate
		provider := state.provider

		// @step: are we pulling the backend template from the default or the provider configuration
		if provider != nil && provider.HasBackendTemplate() {
			secret.Name = provider.Spec.BackendTemplate.Name
			secret.Namespace = c.ControllerNamespace
			if provider.Spec.BackendTemplate.Namespace != "" {
				secret.Namespace = provider.Spec.BackendTemplate.Namespace
			}
		}

		reference := fmt.Sprintf("%s/%s", secret.Namespace, secret.Name)

		// @step: else we need to check for the secret and source it in for later
		found, err := kubernetes.GetIfExists(ctx, c.cc, secret)
		if err != nil {
			cond.Failed(err, "Failed to retrieve the backend template secret")

			return reconcile.Result{}, err
		}
		if !found {
			cond.ActionRequired(fmt.Sprintf("Backend template secret %q not found, contact administrator", reference))

			return reconcile.Result{}, controller.ErrIgnore
		}

		// @step: ensure we have the backend.tf key in the secret
		backend, ok := secret.Data[terraformv1alpha1.TerraformBackendSecretKey]
		if !ok || len(backend) == 0 {
			cond.ActionRequired(fmt.Sprintf("Backend template secret %q does not contain the %s key",
				reference, terraformv1alpha1.TerraformBackendSecretKey),
			)

			return reconcile.Result{}, controller.ErrIgnore
		}

		// @step: update the template in the state
		state.backendTemplate = string(backend)

		return reconcile.Result{}, nil
	}
}

// ensureCostSecret is responsible for ensuring the cost analytics secret is available. This secret is added into
// the job namespace by the platform administrator - but it's possible someone has deleted / changed it - so better to
// place guard around it
func (c *Controller) ensureCostSecret(configuration *terraformv1alpha1.Configuration) controller.EnsureFunc {
	cond := controller.ConditionMgr(configuration, corev1alpha1.ConditionReady, c.recorder)

	return func(ctx context.Context) (reconcile.Result, error) {
		if !c.EnableInfracosts {
			return reconcile.Result{}, nil
		}

		secret := &v1.Secret{}
		secret.Namespace = c.ControllerNamespace
		secret.Name = c.InfracostsSecretName

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

// ensureValueFromSecret is responsible for checking any valuefrom secrets are available and placing them into the state
func (c *Controller) ensureValueFromSecret(configuration *terraformv1alpha1.Configuration, state *state) controller.EnsureFunc {
	cond := controller.ConditionMgr(configuration, corev1alpha1.ConditionReady, c.recorder)

	return func(ctx context.Context) (reconcile.Result, error) {
		switch {
		case len(configuration.Spec.ValueFrom) == 0:
			return reconcile.Result{}, nil
		}

		for i, x := range configuration.Spec.ValueFrom {
			switch {
			case x.Secret != nil && x.Context != nil:
				cond.Failed(fmt.Errorf("spec.valuefrom[%d] secret and context cannot be used together", i), "options are mutually exclusive")

				return reconcile.Result{}, controller.ErrIgnore

			// @step: if the value is from a secret, we need to check it exists
			case x.Secret != nil:
				secret, found, err := kubernetes.GetSecretIfExists(ctx, c.cc, configuration.Namespace, *x.Secret)
				if err != nil {
					cond.Failed(err, "Failed to retrieve the secret spec.valueFrom[%d]", i)

					return reconcile.Result{}, err
				}

				// @step: we either error or move on if the secret is not found
				if !found {
					if !x.Optional {
						cond.ActionRequired("spec.valueFrom[%d].secret (%s/%s) does not exist", i, configuration.Namespace, *x.Secret)

						return reconcile.Result{RequeueAfter: 5 * time.Minute}, nil
					}
					continue
				}

				// @step: we need to ensure the key is present in the secret
				if (secret.Data == nil || len(secret.Data[x.Key]) == 0) && !x.Optional {
					cond.ActionRequired("spec.valueFrom[%d] (%s/%s) does not contain key: %q", i, configuration.Namespace, secret.Name, x.Key)

					return reconcile.Result{RequeueAfter: 5 * time.Minute}, nil
				}

				if secret.Data != nil {
					state.valueFrom[x.GetName()] = string(secret.Data[x.Key])
				}

			case x.Context != nil:
				config := &terraformv1alpha1.Context{}
				config.Name = *x.Context

				// @step: first we check for the terranetes context resource
				found, err := kubernetes.GetIfExists(ctx, c.cc, config)
				if err != nil {
					cond.Failed(err, "Failed to retrieve the context spec.valueFrom[%d].context: %s", i, config.Name)

					return reconcile.Result{}, err
				}
				if !found {
					if !x.Optional {
						cond.ActionRequired("spec.valueFrom[%d].context (%s) does not exist", i, config.Name)

						return reconcile.Result{RequeueAfter: 5 * time.Minute}, nil
					}
					continue
				}

				// @step: we need to check terranetes context has a value
				raw, found := config.Spec.GetVariableValue(x.Key)
				if !found {
					if !x.Optional {
						cond.ActionRequired("spec.valueFrom[%d] does not contain key: %s", i, x.Key)

						return reconcile.Result{RequeueAfter: 5 * time.Minute}, nil
					}
					continue
				}
				if len(raw.Raw) == 0 {
					if !x.Optional {
						cond.ActionRequired("spec.valueFrom[%d] key: %s is empty", i, x.Key)

						return reconcile.Result{RequeueAfter: 5 * time.Minute}, nil
					}
					continue
				}

				av := make(map[string]interface{})
				if err := json.NewDecoder(bytes.NewBuffer(raw.Raw)).Decode(&av); err != nil {
					cond.Failed(err, "Failed to decode the context spec.valueFrom[%d].context: %s", i, config.Name)

					return reconcile.Result{}, err
				}
				state.valueFrom[x.GetName()] = av["value"]

			default:
				cond.Failed(fmt.Errorf("spec.valueFrom[%d] not has no type", i), "missing type in spec.valueFrom[%d]", i)
			}
		}

		return reconcile.Result{}, nil
	}
}

// ensureCustomJobTemplate is used to verify the job template exists if we have been configured to override the template
func (c *Controller) ensureCustomJobTemplate(configuration *terraformv1alpha1.Configuration, state *state) controller.EnsureFunc {
	cond := controller.ConditionMgr(configuration, corev1alpha1.ConditionReady, c.recorder)

	return func(ctx context.Context) (reconcile.Result, error) {
		var err error
		if c.JobTemplate == "" {
			state.jobTemplate = assets.MustAsset("job.yaml.tpl") // lets default to the embedded template
			state.jobTemplateHash, err = jobs.TemplateHash(state.jobTemplate)
			if err != nil {
				return reconcile.Result{}, err
			}

			return reconcile.Result{}, nil
		}

		cm := &v1.ConfigMap{}
		cm.Namespace = c.ControllerNamespace
		cm.Name = c.JobTemplate

		found, err := kubernetes.GetIfExists(ctx, c.cc, cm)
		if err != nil {
			cond.Failed(err, "Failed to retrieve the custom job template")

			return reconcile.Result{}, err
		}
		if !found {
			cond.ActionRequired("Custom job template (%s/%s) does not exists", c.ControllerNamespace, c.JobTemplate)

			return reconcile.Result{}, controller.ErrIgnore
		}

		template, found := cm.Data[terraformv1alpha1.TerraformJobTemplateConfigMapKey]
		if !found {
			cond.ActionRequired("Custom job template (%s/%s) does not contain the %q key",
				c.ControllerNamespace, c.JobTemplate, terraformv1alpha1.TerraformJobTemplateConfigMapKey)

			return reconcile.Result{}, controller.ErrIgnore
		}

		state.jobTemplate = []byte(template)
		state.jobTemplateHash, err = jobs.TemplateHash(state.jobTemplate)
		if err != nil {
			return reconcile.Result{}, err
		}

		return reconcile.Result{}, nil
	}
}

// ensureAuthenticationSecret is responsible for verifying that any secret which is referenced by the
// configuration does exist
func (c *Controller) ensureAuthenticationSecret(configuration *terraformv1alpha1.Configuration, state *state) controller.EnsureFunc {
	cond := controller.ConditionMgr(configuration, corev1alpha1.ConditionReady, c.recorder)

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
			cond.ActionRequired("Authentication secret (spec.auth) does not exist")

			return reconcile.Result{RequeueAfter: 5 * time.Minute}, nil
		}

		state.auth = secret

		return reconcile.Result{}, nil
	}
}

// ensureProviderReady is responsible for ensuring the provider referenced by this configuration is ready to be used
func (c *Controller) ensureProviderReady(configuration *terraformv1alpha1.Configuration, state *state) controller.EnsureFunc {
	cond := controller.ConditionMgr(configuration, terraformv1alpha1.ConditionProviderReady, c.recorder)

	return func(ctx context.Context) (reconcile.Result, error) {
		provider := &terraformv1alpha1.Provider{}
		provider.Name = configuration.Spec.ProviderRef.Name

		found, err := kubernetes.GetIfExists(ctx, c.cc, provider)
		if err != nil {
			cond.Failed(err, "Failed to retrieve the provider for the configuration: %q", provider.Name)

			return reconcile.Result{}, err
		}
		if !found {
			cond.ActionRequired("Provider referenced %q does not exist", provider.Name)

			return reconcile.Result{RequeueAfter: 5 * time.Minute}, nil
		}

		// @step: we need to check the status of the provider to ensure it's ready to be used
		if provider.Status.GetCondition(corev1alpha1.ConditionReady).Status != metav1.ConditionTrue {
			cond.Warning("Provider is not ready")

			return reconcile.Result{RequeueAfter: 30 * time.Second}, nil
		}
		state.provider = provider

		// @step: ensure we are permitted to use the provider
		if provider.Spec.Selector != nil {
			namespace, err := c.getNamespaceFromCache(ctx, configuration.Namespace)
			if err != nil {
				cond.Failed(err, "Failed to retrieve the namespace from the cache")

				return reconcile.Result{RequeueAfter: 30 * time.Second}, nil
			}

			// @step: ensure we have match the selector of the provider - i.e our namespace and resource labels must match
			match, err := kubernetes.IsSelectorMatch(*provider.Spec.Selector, configuration.GetLabels(), namespace.GetLabels())
			if err != nil {
				cond.Failed(err, "Failed to check against the provider policy")

				return reconcile.Result{}, err
			}
			if !match {
				cond.ActionRequired("Provider policy does not permit the configuration to use it")

				return reconcile.Result{}, controller.ErrIgnore
			}
		}
		cond.Success("Provider ready")

		return reconcile.Result{}, nil
	}
}

// ensurePolicyDefaultsExist is responsible for ensuring any default secrets which are being injected
// are available for the this configuration
func (c *Controller) ensurePolicyDefaultsExist(configuration *terraformv1alpha1.Configuration, state *state) controller.EnsureFunc {
	cond := controller.ConditionMgr(configuration, corev1alpha1.ConditionReady, c.recorder)

	return func(ctx context.Context) (reconcile.Result, error) {
		switch {
		// if we have no policies, we can move on
		case state.policies == nil, len(state.policies.Items) == 0:
			return reconcile.Result{}, nil
		}

		namespace, err := c.getNamespaceFromCache(ctx, configuration.Namespace)
		if err != nil {
			cond.Failed(err, "Failed to retrieve the namespace")

			return reconcile.Result{RequeueAfter: 30 * time.Second}, nil
		}

		var list []string

		// @step: find any policies who's using secrets and match this configuration
		for i := 0; i < len(state.policies.Items); i++ {
			if state.policies.Items[i].Spec.Defaults == nil {
				continue
			}

			// @step: iterate the defaults and check if we have any secrets
			for _, x := range state.policies.Items[i].Spec.Defaults {
				switch {
				case len(x.Secrets) == 0:
					continue

				case len(x.Selector.Modules) == 0 && x.Selector.Namespace == nil:
					list = append(list, x.Secrets...)
				}

				// @step: check if the configuration matches a module selector
				if len(x.Selector.Modules) > 0 {
					match, err := x.Selector.IsModulesMatch(configuration)
					if err != nil {
						cond.Failed(err, "Failed to check against the policy: %q", state.policies.Items[i].Name)

						return reconcile.Result{}, err
					}
					if match {
						list = append(list, x.Secrets...)
					}
				}

				// @step: check if the configuration matches a namespace selector
				if x.Selector.Namespace != nil {
					if namespace == nil {
						cond.Failed(errors.New("namespace missing from cachce"), "Failed to retrieve the namespace from the cache")

						return reconcile.Result{RequeueAfter: 30 * time.Second}, nil
					}

					match, err := x.Selector.IsLabelsMatch(namespace)
					if err != nil {
						cond.Failed(err, "Failed to check against the policy: %q", state.policies.Items[i].Name)

						return reconcile.Result{}, err
					}
					if match {
						list = append(list, x.Secrets...)
					}
				}
			}
		}

		// @step: we iterate the referenced secrets and check they exist in the controller namespace
		for i := 0; i < len(list); i++ {
			secret := &v1.Secret{}
			secret.Namespace = c.ControllerNamespace
			secret.Name = list[i]

			found, err := kubernetes.GetIfExists(ctx, c.cc, secret)
			if err != nil {
				cond.Failed(err, "Failed to retrieve the default secret: (%s/%s)", secret.Namespace, secret.Name)

				return reconcile.Result{}, err
			}
			if !found {
				cond.ActionRequired("Default secret (%s/%s) does not exist, please contact administrator", secret.Namespace, secret.Name)

				return reconcile.Result{RequeueAfter: 5 * time.Minute}, nil
			}
		}

		// @step: add the secrets to the state
		state.additionalJobSecrets = list

		return reconcile.Result{}, nil
	}
}

// ensureJobConfigurationSecret is responsible in ensuring the terraform configuration is generated for this job. This
// includes the backend configuration and the variables which have been included in the configuration
func (c *Controller) ensureJobConfigurationSecret(configuration *terraformv1alpha1.Configuration, state *state) controller.EnsureFunc {
	cond := controller.ConditionMgr(configuration, corev1alpha1.ConditionReady, c.recorder)
	policyCondition := controller.ConditionMgr(configuration, terraformv1alpha1.ConditionTerraformPolicy, c.recorder)
	suffix := string(configuration.GetUID())
	name := configuration.GetTerraformConfigSecretName()

	return func(ctx context.Context) (reconcile.Result, error) {
		secret := &v1.Secret{}
		secret.Namespace = c.ControllerNamespace
		secret.Name = name

		if _, err := kubernetes.GetIfExists(ctx, c.cc, secret); err != nil {
			cond.Failed(err, "Failed to check for configuration secret")

			return reconcile.Result{}, err
		}
		secret.Labels = map[string]string{
			terraformv1alpha1.ConfigurationNameLabel:      configuration.Name,
			terraformv1alpha1.ConfigurationNamespaceLabel: configuration.Namespace,
			terraformv1alpha1.ConfigurationUIDLabel:       string(configuration.GetUID()),
		}

		// @step: generate the terraform backend configuration - this creates a kubernetes terraform
		// backend pointing at a secret
		cfg, err := terraform.NewKubernetesBackend(terraform.BackendOptions{
			Configuration: configuration,
			Namespace:     c.ControllerNamespace,
			Suffix:        suffix,
			Template:      state.backendTemplate,
		})
		if err != nil {
			cond.Failed(err, "Failed to generate the terraform backend configuration")

			return reconcile.Result{}, err
		}
		secret.Data = map[string][]byte{terraformv1alpha1.TerraformBackendSecretKey: cfg}

		// @step: generate the provider for the terraform configuration
		cfg, err = terraform.NewTerraformProvider(string(state.provider.Spec.Provider), state.provider.GetConfiguration())
		if err != nil {
			cond.Failed(err, "Failed to generate the terraform provider configuration")

			return reconcile.Result{}, err
		}
		secret.Data[terraformv1alpha1.TerraformProviderConfigMapKey] = cfg

		// @step: we need to generate the value from the variables
		variables, err := configuration.Spec.GetVariables()
		if err != nil {
			cond.Failed(err, "Failed to retrieve the variables for the configuration")

			return reconcile.Result{}, err
		}
		for key, value := range state.valueFrom {
			variables[key] = value
		}

		// @step: should we inject the context?
		if c.EnableContextInjection {
			variables["terranetes"] = map[string]interface{}{
				"name":      configuration.Name,
				"namespace": configuration.Namespace,
			}
		}

		// @step: if the any variables for this job lets add them
		switch len(variables) == 0 {
		case true:
			delete(secret.Data, terraformv1alpha1.TerraformVariablesConfigMapKey)

		default:
			encoded := &bytes.Buffer{}
			if err := json.NewEncoder(encoded).Encode(&variables); err != nil {
				cond.Failed(err, "Failed to encode the variables for the configuration")

				return reconcile.Result{}, err
			}
			secret.Data[terraformv1alpha1.TerraformVariablesConfigMapKey] = encoded.Bytes()
		}

		// @step: copy any authentication details into the secret
		if state.auth != nil {
			for k, v := range state.auth.Data {
				secret.Data[k] = v
			}
		}

		// @step: we need to find any matching policy which should be attached to this configuration.
		policy, err := c.findMatchingPolicy(ctx, configuration, state.policies)
		if err != nil {
			policyCondition.Failed(err, "Failed to find matching policy constraints")

			return reconcile.Result{}, err
		}
		if policy == nil {
			delete(secret.Data, terraformv1alpha1.CheckovJobTemplateConfigMapKey)
		} else {
			state.checkovConstraint = policy

			if policy.Source == nil {
				config, err := terraform.NewCheckovPolicy(map[string]interface{}{
					"Policy": policy,
				})
				if err != nil {
					cond.Failed(err, "Failed to parse the checkov policy template")

					return reconcile.Result{}, err
				}
				secret.Data[terraformv1alpha1.CheckovJobTemplateConfigMapKey] = config
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
func (c *Controller) ensureTerraformPlan(configuration *terraformv1alpha1.Configuration, state *state) controller.EnsureFunc {
	cond := controller.ConditionMgr(configuration, terraformv1alpha1.ConditionTerraformPlan, c.recorder)
	generation := fmt.Sprintf("%d", configuration.GetGeneration())

	return func(ctx context.Context) (reconcile.Result, error) {
		switch {
		// @note: ensure the retryable annotation is valid
		case configuration.HasRetryableAnnotation() && !configuration.IsRetryableValid():
			cond.Failed(nil, "Retryable annotation is invalid, must be a valid unix timestamp")

			return reconcile.Result{}, controller.ErrIgnore

		// @note: if the configuration is marked for retry and the last reconcile was before us - we can retry
		// the configuration
		case configuration.HasRetryableAnnotation() && configuration.IsRetryable():
			log.WithFields(log.Fields{
				"name":      configuration.Name,
				"namespace": configuration.Namespace,
			}).Info("retrying the configuration")

		// @note: the last plan failed for this generation - we do not run it again
		case cond.GetCondition().IsFailed(configuration.GetGeneration()):
			return reconcile.Result{}, controller.ErrIgnore

		case cond.GetCondition().IsComplete(configuration.GetGeneration()):
			if !configuration.Spec.EnableDriftDetection || configuration.GetAnnotations()[terraformv1alpha1.DriftAnnotation] == "" {
				// @note: this is effectively checking the status of plan condition - if the condition is True
				// for the given generation we can say the plan has already been run and can move on
				return reconcile.Result{}, nil
			}
		}

		// @step: should we save the terraform state
		saveState := c.HasBackendTemplate()
		if state.provider != nil && state.provider.HasBackendTemplate() {
			saveState = true
		}

		// @step: lets build the options to render the job
		options := jobs.Options{
			AdditionalJobAnnotations: state.provider.JobAnnotations(),
			AdditionalJobSecrets:     state.additionalJobSecrets,
			AdditionalJobLabels: utils.MergeStringMaps(
				// thats a lot of labels!
				c.ControllerJobLabels,
				state.provider.JobLabels(),
				configuration.GetLabels(),
				map[string]string{
					terraformv1alpha1.DriftAnnotation:      configuration.GetAnnotations()[terraformv1alpha1.DriftAnnotation],
					terraformv1alpha1.RetryAnnotation:      configuration.GetAnnotations()[terraformv1alpha1.RetryAnnotation],
					terraformv1alpha1.JobTemplateHashLabel: state.jobTemplateHash,
				}),
			BackoffLimit:       c.BackoffLimit,
			EnableInfraCosts:   c.EnableInfracosts,
			ExecutorImage:      c.ExecutorImage,
			ExecutorSecrets:    c.ExecutorSecrets,
			InfracostsImage:    c.InfracostsImage,
			InfracostsSecret:   c.InfracostsSecretName,
			Namespace:          c.ControllerNamespace,
			PolicyConstraint:   state.checkovConstraint,
			PolicyImage:        c.PolicyImage,
			SaveTerraformState: saveState,
			Template:           state.jobTemplate,
			TerraformImage:     GetTerraformImage(configuration, c.TerraformImage),
		}

		// @step: use the options to generate the job
		runner, err := jobs.New(configuration, state.provider).NewTerraformPlan(options)
		if err != nil {
			cond.Failed(err, "Failed to create the terraform plan job")

			return reconcile.Result{}, err
		}

		// @step: search for any current jobs
		job, found := filters.Jobs(state.jobs).
			WithGeneration(generation).
			WithLabel(terraformv1alpha1.DriftAnnotation, configuration.GetAnnotations()[terraformv1alpha1.DriftAnnotation]).
			WithLabel(terraformv1alpha1.RetryAnnotation, configuration.GetAnnotations()[terraformv1alpha1.RetryAnnotation]).
			WithLabel(terraformv1alpha1.JobTemplateHashLabel, state.jobTemplateHash).
			WithName(configuration.GetName()).
			WithNamespace(configuration.GetNamespace()).
			WithStage(terraformv1alpha1.StageTerraformPlan).
			WithUID(string(configuration.GetUID())).
			Latest()

		if !found {
			// @step: if auto approval is not enabled we should annotate the configuration with the need to approve.
			if !configuration.Spec.EnableAutoApproval && !configuration.NeedsApproval() {

				original := configuration.DeepCopy()
				if configuration.Annotations == nil {
					configuration.Annotations = map[string]string{}
				}
				configuration.Annotations[terraformv1alpha1.ApplyAnnotation] = "false"

				if err := c.cc.Patch(ctx, configuration, client.MergeFrom(original)); err != nil {
					cond.Failed(err, "Failed to create or update the terraform configuration")

					return reconcile.Result{}, err
				}

				// @step: if the configuration is part of a managed plan, we should update the cloudresource
				// to reflect the need to approve
				if configuration.IsManaged() {
					cloudresource := &terraformv1alpha1.CloudResource{}
					cloudresource.Namespace = configuration.Namespace
					cloudresource.Name = configuration.GetLabels()[terraformv1alpha1.CloudResourceNameLabel]

					if found, err := kubernetes.GetIfExists(ctx, c.cc, cloudresource); err != nil {
						cond.Failed(err, "Failed to retrieve the cloudresource: %q which this configuration is part of", cloudresource.Name)

						return reconcile.Result{}, err
					} else if !found {
						cond.Failed(err, "CloudResource: %q which this configuration is part of is missing", cloudresource.Name)

						return reconcile.Result{RequeueAfter: 5 * time.Minute}, nil
					}
					original := cloudresource.DeepCopy()
					cloudresource.Annotations[terraformv1alpha1.ApplyAnnotation] = "false"

					if err := c.cc.Patch(ctx, cloudresource, client.MergeFrom(original)); err != nil {
						cond.Failed(err, "Failed to create or update the cloudresource: %q", cloudresource.Name)

						return reconcile.Result{}, err
					}
				}

				return controller.RequeueImmediate, nil
			}

			if c.EnableWatchers {
				// @step: create a watch job in the configuration namespace to allow the user to witness
				// the terraform output
				if err := c.CreateWatcher(ctx, configuration, terraformv1alpha1.StageTerraformPlan); err != nil {
					cond.Failed(err, "Failed to create the terraform plan watcher")

					return reconcile.Result{}, err
				}
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

			return c.ensureErrorDetection(configuration, job, state)(ctx)

		case jobs.IsActive(job):
			cond.InProgress("Terraform plan is running")
		}

		return reconcile.Result{RequeueAfter: 10 * time.Second}, nil
	}
}

// ensureTerraformPlanSecret is responsible for ensuring the plan ran successfully
func (c *Controller) ensureTerraformPlanSecret(configuration *terraformv1alpha1.Configuration, state *state) controller.EnsureFunc {
	cond := controller.ConditionMgr(configuration, corev1alpha1.ConditionReady, c.recorder)

	return func(ctx context.Context) (reconcile.Result, error) {
		secret := &v1.Secret{}
		secret.Name = configuration.GetTerraformPlanJSONSecretName()
		secret.Namespace = c.ControllerNamespace

		found, err := kubernetes.GetIfExists(ctx, c.cc, secret)
		if err != nil {
			cond.Failed(err, "Failed to get terraform plan secret (%s/%s)", c.ControllerNamespace, secret.Name)

			return reconcile.Result{}, err
		}
		if !found {
			cond.Failed(nil, "Terraform plan secret (%s/%s) not found", c.ControllerNamespace, secret.Name)

			return reconcile.Result{}, controller.ErrIgnore
		}
		state.tfplan = secret
		return reconcile.Result{}, nil
	}
}

// ensureCostStatus is responsible for updating the cost status post a plan
func (c *Controller) ensureCostStatus(configuration *terraformv1alpha1.Configuration) controller.EnsureFunc {
	cond := controller.ConditionMgr(configuration, corev1alpha1.ConditionReady, c.recorder)
	labels := []string{configuration.GetNamespace(), configuration.GetName()}

	return func(ctx context.Context) (reconcile.Result, error) {
		switch {
		case !c.EnableInfracosts:
			configuration.Status.Costs = &terraformv1alpha1.CostStatus{Monthly: "Not Enabled"}

			return reconcile.Result{}, nil
		}

		secret := &v1.Secret{}
		secret.Namespace = c.ControllerNamespace
		secret.Name = configuration.GetTerraformCostSecretName()

		found, err := kubernetes.GetIfExists(ctx, c.cc, secret)
		if err != nil {
			cond.Failed(err, "Failed to get the terraform costs secret")

			return reconcile.Result{}, err
		}
		if !found {
			configuration.Status.Costs = &terraformv1alpha1.CostStatus{Enabled: false}

			return reconcile.Result{}, nil
		}

		input := secret.Data["costs.json"]
		if len(input) == 0 {
			return reconcile.Result{}, nil
		}

		// @step: parse the cost report json
		values := map[string]float64{
			"totalMonthlyCost": 0,
			"totalHourlyCost":  0,
		}
		for key := range values {
			value := gjson.GetBytes(input, key)
			if !value.Exists() {
				cond.ActionRequired("Cost report does not include the %s value", key)

				return reconcile.Result{}, controller.ErrIgnore
			}

			cost, err := strconv.ParseFloat(value.String(), 64)
			if err != nil {
				cond.ActionRequired("Cost report contains an include value: %q for item: %s", value.String(), key)

				return reconcile.Result{}, controller.ErrIgnore
			}
			values[key] = cost
		}

		configuration.Status.Costs = &terraformv1alpha1.CostStatus{
			Enabled: true,
			Hourly:  fmt.Sprintf("$%v", values["totalHourlyCost"]),
			Monthly: fmt.Sprintf("$%v", values["totalMonthlyCost"]),
		}

		// @step: update the prometheus metrics
		monthlyCostMetric.WithLabelValues(labels...).Set(values["totalMonthlyCost"])
		hourlyCostMetric.WithLabelValues(labels...).Set(values["totalHourlyCost"])

		// @step: copy the infracost report into the configuration namespace
		copied := &v1.Secret{}
		copied.Namespace = configuration.GetNamespace()
		copied.Name = configuration.GetTerraformCostSecretName()
		copied.Labels = map[string]string{
			terraformv1alpha1.ConfigurationNameLabel: configuration.GetName(),
			terraformv1alpha1.ConfigurationUIDLabel:  string(configuration.GetUID()),
		}
		copied.OwnerReferences = []metav1.OwnerReference{
			{
				APIVersion: terraformv1alpha1.SchemeGroupVersion.String(),
				Kind:       terraformv1alpha1.ConfigurationKind,
				Name:       configuration.GetName(),
				UID:        configuration.GetUID(),
			},
		}
		copied.Data = secret.Data

		if err := kubernetes.CreateOrForceUpdate(ctx, c.cc, copied); err != nil {
			cond.Failed(err, "Failed to create or update the terraform costs secret")

			return reconcile.Result{}, err
		}

		return reconcile.Result{}, nil
	}
}

// ensurePolicyStatus is responsible for checking the checkov results and refusing to continue if failed
func (c *Controller) ensurePolicyStatus(configuration *terraformv1alpha1.Configuration, state *state) controller.EnsureFunc {
	cond := controller.ConditionMgr(configuration, terraformv1alpha1.ConditionTerraformPolicy, c.recorder)
	key := "results_json.json"

	return func(ctx context.Context) (reconcile.Result, error) {
		switch {
		case state.checkovConstraint == nil:
			cond.Success("Security policy is not configured")

			return reconcile.Result{}, nil
		}

		// @step: retrieve the uploaded scan
		secret := &v1.Secret{}
		secret.Namespace = c.ControllerNamespace
		secret.Name = configuration.GetTerraformPolicySecretName()

		found, err := kubernetes.GetIfExists(ctx, c.cc, secret)
		if err != nil {
			cond.Failed(err, "Failed to retrieve the secret containing the checkov scan")

			return reconcile.Result{}, err
		}
		if !found {
			cond.Warning("Failed to find the secret: (%s/%s) containing checkov scan", c.ControllerNamespace, configuration.GetTerraformPolicySecretName())

			return reconcile.Result{RequeueAfter: 10 * time.Minute}, nil
		}

		var failed gjson.Result

		// @step: retrieve summary from the report
		if gjson.GetBytes(secret.Data[key], "summary").Exists() {
			failed = gjson.GetBytes(secret.Data[key], "summary.failed")
			if !failed.Exists() {
				cond.Failed(errors.New("missing report"), "Security report does not contain a summary of finding, please contact platform administrator")

				return reconcile.Result{}, controller.ErrIgnore
			}

			if failed.Type != gjson.Number {
				cond.Failed(errors.New("invalid resport"), "Security report failed summary is not numerical as expected, please contact platform administrator")

				return reconcile.Result{}, controller.ErrIgnore
			}
		} else {
			for _, x := range []string{"passed", "failed"} {
				if !gjson.GetBytes(secret.Data[key], x).Exists() {
					cond.Failed(errors.New("invalid policy report"), "Security report is missing %s field", x)

					return reconcile.Result{}, controller.ErrIgnore
				}
				if gjson.GetBytes(secret.Data[key], "zero").Int() != 0 {
					cond.Failed(errors.New("invalid policy report"), "Security report is field %s is non-zero", x)

					return reconcile.Result{}, controller.ErrIgnore
				}
			}
		}

		// @step: copy the report into the configuration namespace
		copied := &v1.Secret{}
		copied.Namespace = configuration.GetNamespace()
		copied.Name = configuration.GetTerraformPolicySecretName()
		copied.Labels = map[string]string{
			terraformv1alpha1.ConfigurationNameLabel: configuration.GetName(),
			terraformv1alpha1.ConfigurationUIDLabel:  string(configuration.GetUID()),
		}
		copied.OwnerReferences = []metav1.OwnerReference{
			{
				APIVersion: terraformv1alpha1.SchemeGroupVersion.String(),
				Kind:       terraformv1alpha1.ConfigurationKind,
				Name:       configuration.GetName(),
				UID:        configuration.GetUID(),
			},
		}
		copied.Data = secret.Data

		if err := kubernetes.CreateOrForceUpdate(ctx, c.cc, copied); err != nil {
			cond.Failed(err, "Failed to create or update the terraform policy secret")

			return reconcile.Result{}, err
		}

		if failed.Int() > 0 {
			cond.ActionRequired("Configuration has failed security policy, refusing to continue")

			return reconcile.Result{}, controller.ErrIgnore
		}

		cond.Success("Passed security checks")

		return reconcile.Result{}, nil
	}
}

// ensureDriftDetection is responsible for checking for drift in the terraform state
func (c *Controller) ensureDriftDetection(configuration *terraformv1alpha1.Configuration, state *state) controller.EnsureFunc {
	cond := controller.ConditionMgr(configuration, corev1alpha1.ConditionReady, c.recorder)

	return func(ctx context.Context) (reconcile.Result, error) {
		if configuration.Spec.EnableDriftDetection &&
			configuration.GetAnnotations()[terraformv1alpha1.DriftAnnotation] != configuration.Status.DriftTimestamp {
			// @note: everytime we run a drift we update the timestamp on the status, this is used to ensure we don't
			// try and rerun the drift. We should remove the annotation from the configuration but that has issues as it
			// updates the resourceVersion which make updating the status conflict.
			configuration.Status.DriftTimestamp = configuration.GetAnnotations()[terraformv1alpha1.DriftAnnotation]
		}

		tfplan, err := terraform.DecodePlan(state.tfplan.Data[terraformv1alpha1.TerraformPlanJSONSecretKey])
		if err != nil {
			cond.Failed(err, "Failed to decode the terraform plan")

			return reconcile.Result{}, err
		}

		// @step: check for changes in the plan
		state.hasDrift = tfplan.NeedsApply()

		return reconcile.Result{}, nil
	}
}

// ensureTerraformApply is responsible for ensuring the terraform apply is running or run
func (c *Controller) ensureTerraformApply(configuration *terraformv1alpha1.Configuration, state *state) controller.EnsureFunc {
	cond := controller.ConditionMgr(configuration, terraformv1alpha1.ConditionTerraformApply, c.recorder)
	generation := fmt.Sprintf("%d", configuration.GetGeneration())

	readyCond := controller.ConditionMgr(configuration, corev1alpha1.ConditionReady, c.recorder)

	return func(ctx context.Context) (reconcile.Result, error) {
		switch {
		case !state.hasDrift:
			// There is nothing to apply, we can skip it.
			return reconcile.Result{}, nil
		case configuration.NeedsApproval() && !configuration.Spec.EnableAutoApproval:
			cond.ActionRequired("Waiting for terraform apply annotation to be set to true")
			// update the ready condition to reflect the new state
			readyCond.InProgress("Waiting for changes to be approved")
			// update the resource status
			configuration.Status.ResourceStatus = terraformv1alpha1.ResourcesOutOfSync

			return reconcile.Result{}, controller.ErrIgnore
		}

		tfplan, err := terraform.DecodePlan(state.tfplan.Data[terraformv1alpha1.TerraformPlanJSONSecretKey])
		if err != nil {
			cond.Failed(err, "Failed to decode the terraform plan")

			return reconcile.Result{}, err
		}

		// @step: check if we need to save the terraform state
		saveState := c.HasBackendTemplate()
		if state.provider != nil && state.provider.HasBackendTemplate() {
			saveState = true
		}

		// @step: create the terraform job
		runner, err := jobs.New(configuration, state.provider).NewTerraformApply(jobs.Options{
			AdditionalJobAnnotations: state.provider.JobAnnotations(),
			AdditionalJobSecrets:     state.additionalJobSecrets,
			AdditionalJobLabels: utils.MergeStringMaps(
				c.ControllerJobLabels,
				state.provider.JobLabels(),
				configuration.GetLabels(),
				map[string]string{
					terraformv1alpha1.JobPlanIDLabel:       tfplan.ID(),
					terraformv1alpha1.JobTemplateHashLabel: state.jobTemplateHash,
				},
			),
			BackoffLimit:       c.BackoffLimit,
			EnableInfraCosts:   c.EnableInfracosts,
			ExecutorImage:      c.ExecutorImage,
			ExecutorSecrets:    c.ExecutorSecrets,
			InfracostsImage:    c.InfracostsImage,
			InfracostsSecret:   c.InfracostsSecretName,
			Namespace:          c.ControllerNamespace,
			SaveTerraformState: saveState,
			Template:           state.jobTemplate,
			TerraformImage:     GetTerraformImage(configuration, c.TerraformImage),
		})
		if err != nil {
			cond.Failed(err, "Failed to create the terraform apply job")

			return reconcile.Result{}, err
		}

		// @step: find the job which is implementing this stage if any
		job, found := filters.Jobs(state.jobs).
			WithGeneration(generation).
			WithLabel(terraformv1alpha1.JobPlanIDLabel, tfplan.ID()).
			WithLabel(terraformv1alpha1.JobTemplateHashLabel, state.jobTemplateHash).
			WithNamespace(configuration.GetNamespace()).
			WithName(configuration.GetName()).
			WithStage(terraformv1alpha1.StageTerraformApply).
			WithUID(string(configuration.GetUID())).
			Latest()

		// @step: we can requeue or move on depending on the status
		if !found {
			configuration.Status.ResourceStatus = terraformv1alpha1.ResourcesOutOfSync

			if c.EnableWatchers {
				if err := c.CreateWatcher(ctx, configuration, terraformv1alpha1.StageTerraformApply); err != nil {
					cond.Failed(err, "Failed to create the terraform apply watcher")

					return reconcile.Result{}, err
				}
			}

			// @step: create the job for terraform apply
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
			configuration.Status.ResourceStatus = terraformv1alpha1.ResourcesInSync

			cond.Success("Terraform apply is complete")
			return reconcile.Result{}, nil

		case jobs.IsFailed(job):
			cond.Failed(nil, "Terraform apply has failed")

			return c.ensureErrorDetection(configuration, job, state)(ctx)

		case jobs.IsActive(job):
			cond.InProgress("Terraform apply in progress")
		}

		return reconcile.Result{RequeueAfter: 5 * time.Second}, nil
	}
}

// ensureConnectionSecret is responsible for ensuring the jobs ran successfully
func (c *Controller) ensureConnectionSecret(configuration *terraformv1alpha1.Configuration, state *state) controller.EnsureFunc {
	cond := controller.ConditionMgr(configuration, corev1alpha1.ConditionReady, c.recorder)

	return func(ctx context.Context) (reconcile.Result, error) {
		secret := &v1.Secret{}
		secret.Name = configuration.GetTerraformStateSecretName()
		secret.Namespace = c.ControllerNamespace

		found, err := kubernetes.GetIfExists(ctx, c.cc, secret)
		if err != nil {
			cond.Failed(err, "Failed to get terraform state secret (%s/%s)", c.ControllerNamespace, secret.Name)

			return reconcile.Result{}, err
		}
		if !found {
			cond.Failed(nil, "Terraform state secret (%s/%s) not found", c.ControllerNamespace, secret.Name)

			return reconcile.Result{}, controller.ErrIgnore
		}
		state.tfstate = secret

		if configuration.Spec.WriteConnectionSecretToRef != nil {
			// @step: decode the terraform state (essentially just returning the uncompressed content)
			state, err := terraform.DecodeState(secret.Data[terraformv1alpha1.TerraformStateSecretKey])
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
					terraformv1alpha1.ConfigurationNameLabel:      configuration.Name,
					terraformv1alpha1.ConfigurationNamespaceLabel: configuration.Namespace,
					terraformv1alpha1.ConfigurationUIDLabel:       string(configuration.GetUID()),
				}
				secret.Data = make(map[string][]byte)

				mapping, err := configuration.Spec.WriteConnectionSecretToRef.KeysMap()
				if err != nil {
					cond.Failed(err, "Failed to parse the configuration connection secrets mapping")

					return reconcile.Result{}, err
				}

				for k, v := range state.Outputs {
					if !configuration.Spec.WriteConnectionSecretToRef.HasKeys() {
						secret.Data[strings.ToUpper(k)] = []byte(v.String())

						continue
					}

					// @step: we check if the mapping exists and if so, add the mapping as expected
					for key, value := range mapping {
						if k == key {
							secret.Data[strings.ToUpper(value)] = []byte(v.String())
						}
					}
				}

				// @step: create the terraform secret
				if err := kubernetes.CreateOrForceUpdate(ctx, c.cc, secret); err != nil {
					cond.Failed(err, "Failed to create the terraform state secret")

					return reconcile.Result{}, err
				}
			}
		}

		return reconcile.Result{}, nil
	}
}

// ensureTerraformStatus is responsible for updating the configuration status
func (c *Controller) ensureTerraformStatus(configuration *terraformv1alpha1.Configuration, state *state) controller.EnsureFunc {
	cond := controller.ConditionMgr(configuration, corev1alpha1.ConditionReady, c.recorder)

	return func(ctx context.Context) (reconcile.Result, error) {
		tfstate, err := terraform.DecodeState(state.tfstate.Data[terraformv1alpha1.TerraformStateSecretKey])
		if err != nil {
			cond.Failed(err, "Failed to decode the terraform state")

			return reconcile.Result{}, err
		}
		configuration.Status.Resources = ptr.To(tfstate.CountResources())
		configuration.Status.TerraformVersion = tfstate.TerraformVersion

		switch configuration.Status.ResourceStatus {
		case terraformv1alpha1.ResourcesInSync:
			inSyncMetric.WithLabelValues(configuration.Namespace, configuration.Name).Set(1)
		case terraformv1alpha1.ResourcesOutOfSync:
			inSyncMetric.WithLabelValues(configuration.Namespace, configuration.Name).Set(0)
		}
		cond.Success("Resource ready")

		return reconcile.Result{}, nil
	}
}
