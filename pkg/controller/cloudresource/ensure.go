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
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"time"

	log "github.com/sirupsen/logrus"
	"github.com/tidwall/sjson"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	corev1alpha1 "github.com/appvia/terranetes-controller/pkg/apis/core/v1alpha1"
	terraformv1alpha1 "github.com/appvia/terranetes-controller/pkg/apis/terraform/v1alpha1"
	"github.com/appvia/terranetes-controller/pkg/controller"
	"github.com/appvia/terranetes-controller/pkg/utils"
	"github.com/appvia/terranetes-controller/pkg/utils/kubernetes"
)

// ensurePlanExists ensures that the plan exists
func (c *Controller) ensurePlanExists(cloudresource *terraformv1alpha1.CloudResource, state *state) controller.EnsureFunc {
	cond := controller.ConditionMgr(cloudresource, corev1alpha1.ConditionReady, c.recorder)
	cc := c.cc

	return func(ctx context.Context) (reconcile.Result, error) {
		plan := &terraformv1alpha1.Plan{}
		plan.Name = cloudresource.Spec.Plan.Name

		found, err := kubernetes.GetIfExists(ctx, cc, plan)
		if err != nil {
			cond.Failed(err, "Failed to retrieve the cloud resource plan")

			return reconcile.Result{}, err
		}
		if !found {
			cond.ActionRequired("Cloud resource plan %q does not exist", cloudresource.Spec.Plan.Name)

			return reconcile.Result{RequeueAfter: 5 * time.Minute}, nil
		}

		state.plan = plan

		return reconcile.Result{}, nil
	}
}

// ensureRevisionExists is responsible for ensuring that the cloud resource revision exists
func (c *Controller) ensureRevisionExists(cloudresource *terraformv1alpha1.CloudResource, state *state) controller.EnsureFunc {
	cond := controller.ConditionMgr(cloudresource, corev1alpha1.ConditionReady, c.recorder)

	return func(ctx context.Context) (reconcile.Result, error) {
		plan := state.plan

		reference, found := plan.GetRevision(cloudresource.Spec.Plan.Revision)
		if !found {
			cond.ActionRequired("Revision: %q does not exist in plan (spec.plan.revision)", cloudresource.Spec.Plan.Revision)

			return reconcile.Result{RequeueAfter: 5 * time.Minute}, nil
		}

		revision := &terraformv1alpha1.Revision{}
		revision.Name = reference.Name

		found, err := kubernetes.GetIfExists(ctx, c.cc, revision)
		if err != nil {
			cond.Failed(err, "Failed to retrieve the cloud resource revision")

			return reconcile.Result{}, err
		}
		if !found {
			cond.ActionRequired("Revision: %q does not exist or has been removed", cloudresource.Spec.Plan.Revision)

			return reconcile.Result{RequeueAfter: 5 * time.Minute}, nil
		}

		state.revision = revision

		return reconcile.Result{}, nil
	}
}

// ensureConfigurationExists is responsible for ensuring the configuration is provisioned
func (c *Controller) ensureConfigurationExists(cloudresource *terraformv1alpha1.CloudResource, state *state) controller.EnsureFunc {
	cond := controller.ConditionMgr(cloudresource, corev1alpha1.ConditionReady, c.recorder)
	cc := c.cc
	logger := log.WithFields(log.Fields{
		"name":      cloudresource.Name,
		"namespace": cloudresource.Namespace,
	})

	return func(ctx context.Context) (reconcile.Result, error) {
		var current *terraformv1alpha1.Configuration
		revision := state.revision

		list := &terraformv1alpha1.ConfigurationList{}
		err := c.cc.List(ctx, list,
			client.InNamespace(cloudresource.Namespace),
			client.MatchingLabels(map[string]string{
				terraformv1alpha1.CloudResourceNameLabel: cloudresource.Name,
			},
			))
		if err != nil {
			cond.Failed(err, "Failed to retrieve the cloud resource configuration")

			return reconcile.Result{}, err
		}
		switch len(list.Items) {
		case 0:
			break
		case 1:
			current = &terraformv1alpha1.Configuration{}
			current.Name = list.Items[0].Name
			current.Namespace = list.Items[0].Namespace

			found, err := kubernetes.GetIfExists(ctx, cc, current)
			if err != nil {
				cond.Failed(err, "Failed to retrieve the cloud resource configuration")

				return reconcile.Result{}, err
			}
			if !found {
				cond.ActionRequired("Cloud resource configuration does not exist")

				return reconcile.Result{RequeueAfter: 1 * time.Minute}, nil
			}

		default:
			cond.ActionRequired("Multiple configurations found for cloud resource")

			return reconcile.Result{RequeueAfter: 5 * time.Minute}, nil
		}

		// @step: we need to provision the configuration
		configuration := terraformv1alpha1.NewConfiguration(cloudresource.Namespace, "")
		configuration.Annotations = cloudresource.Annotations
		configuration.Labels = map[string]string{
			terraformv1alpha1.CloudResourceNameLabel:         cloudresource.Name,
			terraformv1alpha1.CloudResourcePlanNameLabel:     revision.Spec.Plan.Name,
			terraformv1alpha1.CloudResourceRevisionLabel:     revision.Spec.Plan.Revision,
			terraformv1alpha1.CloudResourceRevisionNameLabel: revision.Name,
		}
		configuration.OwnerReferences = []metav1.OwnerReference{
			{
				APIVersion: terraformv1alpha1.SchemeGroupVersion.String(),
				Kind:       terraformv1alpha1.CloudResourceKind,
				Name:       cloudresource.Name,
				UID:        cloudresource.UID,
			},
		}

		configuration.Spec.Module = revision.Spec.Configuration.Module
		configuration.Spec.EnableAutoApproval = cloudresource.Spec.EnableAutoApproval
		configuration.Spec.EnableDriftDetection = cloudresource.Spec.EnableDriftDetection
		configuration.Spec.Plan = &terraformv1alpha1.PlanReference{
			Name:     cloudresource.Spec.Plan.Name,
			Revision: cloudresource.Spec.Plan.Revision,
		}
		configuration.Spec.TerraformVersion = cloudresource.Spec.TerraformVersion
		configuration.Spec.ValueFrom = append(revision.Spec.Configuration.ValueFrom, cloudresource.Spec.ValueFrom...)
		configuration.Spec.Variables = revision.Spec.Configuration.Variables

		// @step: set the write connection secret if we have one
		configuration.Spec.WriteConnectionSecretToRef = revision.Spec.Configuration.WriteConnectionSecretToRef
		if cloudresource.Spec.WriteConnectionSecretToRef != nil {
			configuration.Spec.WriteConnectionSecretToRef = cloudresource.Spec.WriteConnectionSecretToRef
		}

		// @step: set the provider configuration
		configuration.Spec.ProviderRef = revision.Spec.Configuration.ProviderRef
		if cloudresource.Spec.ProviderRef != nil {
			configuration.Spec.ProviderRef = cloudresource.Spec.ProviderRef
		}

		// @step: copy in the values from the inputs which have a value
		for _, input := range revision.Spec.Inputs {
			if input.Default == nil || len(input.Default.Raw) == 0 {
				continue
			}

			value := make(map[string]interface{})
			if err := json.NewDecoder(bytes.NewBuffer(input.Default.Raw)).Decode(&value); err != nil {
				cond.ActionRequired("Failed to decode the input %q default value from revision", input.Key)

				return reconcile.Result{}, controller.ErrIgnore
			}
			if configuration.Spec.Variables == nil {
				configuration.Spec.Variables = &runtime.RawExtension{}
			}

			configuration.Spec.Variables.Raw, err = sjson.SetBytes(configuration.Spec.Variables.Raw, input.Key, value["value"])
			if err != nil {
				cond.ActionRequired("Failed to set the input %q default value from revision", input.Key)

				return reconcile.Result{}, controller.ErrIgnore
			}
		}

		// @step: copy the provider reference if not defined
		if cloudresource.Spec.ProviderRef == nil && revision.Spec.Configuration.ProviderRef != nil {
			configuration.Spec.ProviderRef = revision.Spec.Configuration.ProviderRef
		}

		// @step: set the variables from the cloud resource
		if cloudresource.Spec.HasVariables() {
			if configuration.Spec.Variables == nil {
				configuration.Spec.Variables = &runtime.RawExtension{}
			}

			values := make(map[string]interface{})
			if err := json.NewDecoder(bytes.NewBuffer(cloudresource.Spec.Variables.Raw)).Decode(&values); err != nil {
				cond.Failed(err, "Failed to decode the configuration spec.variables in cloud resource")

				return reconcile.Result{}, err
			}
			for k, v := range values {
				configuration.Spec.Variables.Raw, err = sjson.SetBytes(configuration.Spec.Variables.Raw, k, v)
				if err != nil {
					cond.Failed(err, "Failed to set the variable: %s in cloud resource", k)

					return reconcile.Result{}, err
				}
			}
		}

		// @step: if we have an current we are performing a patch
		if current != nil {
			original := current.DeepCopy()

			current.Spec = configuration.Spec
			current.Labels = utils.MergeStringMaps(current.Labels, configuration.Labels)
			current.Annotations = utils.MergeStringMaps(current.Annotations, configuration.Annotations)
			current.OwnerReferences = configuration.OwnerReferences

			resourceVersion := current.ResourceVersion
			if err := cc.Patch(ctx, current, client.MergeFrom(original)); err != nil {
				cond.Failed(err, "Failed to patch the cloud resource configuration")

				return reconcile.Result{}, err
			}
			if resourceVersion != configuration.ResourceVersion {
				log.Debug("cloud resource configuration has been patched", "resource_version", configuration.ResourceVersion)

				c.recorder.Event(cloudresource, v1.EventTypeNormal, "ConfigurationUpdated", "Updated the cloud resource configuration")
			}

			// @step: update the state with the configuration
			state.configuration = current

			cond := controller.ConditionMgr(cloudresource, terraformv1alpha1.ConditionConfigurationReady, c.recorder)
			cond.Success("Configuration has been updated")

			return reconcile.Result{}, nil
		}

		// @step: when first creating, we use a generated name
		configuration.GenerateName = fmt.Sprintf("%s-", cloudresource.Name)

		if err := cc.Create(ctx, configuration); err != nil {
			cond.Failed(err, "Failed to create the cloud resource configuration")

			return reconcile.Result{}, err
		}

		logger.Info("cloud resource configuration has been created", "name", configuration.Name)

		// @step: update the state with the configuration
		state.configuration = configuration

		cond := controller.ConditionMgr(cloudresource, terraformv1alpha1.ConditionConfigurationReady, c.recorder)
		cond.Success("Provisioned the Configuration")

		// @step: we need to update the status
		cloudresource.Status.ConfigurationName = configuration.Name

		return reconcile.Result{}, nil
	}
}

// ensureConfigurationStatus is responsible for ensuring the status of the cloudresource and
// the configuration are in sync
func (c *Controller) ensureConfigurationStatus(cloudresource *terraformv1alpha1.CloudResource, state *state) controller.EnsureFunc {
	logger := log.WithFields(log.Fields{
		"name":      cloudresource.Name,
		"namespace": cloudresource.Namespace,
	})

	return func(ctx context.Context) (reconcile.Result, error) {
		logger.WithFields(log.Fields{
			"conditions": len(state.configuration.Status.Conditions),
		}).Debug("ensuring the configuration status")

		// @step: ensure all the conditions from the configuration are copied on the cloud resource
		for _, x := range state.configuration.Status.Conditions {
			condition := cloudresource.Status.GetCondition(x.Type)
			if condition == nil {
				logger.WithFields(log.Fields{
					"conditions": x.Type,
				}).Warn("condition is missing from the configuration status")

				continue
			}

			if x.Type == corev1alpha1.ConditionReady {
				condition = cloudresource.Status.GetCondition(terraformv1alpha1.ConditionConfigurationStatus)
			}
			condition.Detail = x.Detail
			condition.LastTransitionTime = x.LastTransitionTime
			condition.Name = x.Name
			condition.Message = x.Message
			condition.ObservedGeneration = x.ObservedGeneration
			condition.Reason = x.Reason
			condition.Status = x.Status
		}

		// @step: copy other fields
		cloudresource.Status.ConfigurationName = state.configuration.Name
		cloudresource.Status.Costs = state.configuration.Status.Costs
		cloudresource.Status.ResourceStatus = state.configuration.Status.ResourceStatus
		cloudresource.Status.Resources = state.configuration.Status.Resources

		return reconcile.Result{}, nil
	}
}
