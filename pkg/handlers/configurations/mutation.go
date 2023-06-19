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

package configurations

import (
	"context"
	"errors"
	"fmt"
	"strings"

	jsonpatch "github.com/evanphx/json-patch"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	terraformv1alpha1 "github.com/appvia/terranetes-controller/pkg/apis/terraform/v1alpha1"
	"github.com/appvia/terranetes-controller/pkg/utils"
	"github.com/appvia/terranetes-controller/pkg/utils/kubernetes"
)

type mutator struct {
	cc client.Client
}

// NewMutator returns a mutation handler
func NewMutator(cc client.Client) admission.CustomDefaulter {
	return &mutator{cc: cc}
}

// Default implements the mutation handler
func (m *mutator) Default(ctx context.Context, obj runtime.Object) error {
	o, ok := obj.(*terraformv1alpha1.Configuration)
	if !ok {
		return fmt.Errorf("expected terraform configuration, not %T", obj)
	}

	// @step: we need to check if the configuration has a provider reference
	if err := m.mutateOnProviderDefault(ctx, o); err != nil {
		return err
	}

	// @step: add labels if configuration is using a plan
	m.mutateOnPlan(o)

	// @step: retrieve a list of all policies
	list := &terraformv1alpha1.PolicyList{}
	if err := m.cc.List(ctx, list); err != nil {
		return fmt.Errorf("failed to list policies: %w", err)
	}
	// @step: inject any default variables from a policy
	if err := m.mutateOnPolicyDefaults(ctx, list, o); err != nil {
		return err
	}

	return nil
}

// mutateOnPlan is used to inject any default labels on the configuration
func (m *mutator) mutateOnPlan(o *terraformv1alpha1.Configuration) {
	if o.Spec.Plan == nil {
		return
	}
	if o.Labels == nil {
		o.Labels = make(map[string]string)
	}

	o.Labels = utils.MergeStringMaps(o.Labels, map[string]string{
		terraformv1alpha1.ConfigurationPlanLabel:       o.Spec.Plan.Name,
		terraformv1alpha1.ConfigurationRevisionVersion: o.Spec.Plan.Revision,
	})
}

// mutateOnProviderDefault is used to fill in a provider default if required
func (m *mutator) mutateOnProviderDefault(ctx context.Context, o *terraformv1alpha1.Configuration) error {
	switch {
	case o.Spec.ProviderRef != nil:
		return nil
	}

	// @step: retrieve a list of all providers
	list := &terraformv1alpha1.ProviderList{}
	if err := m.cc.List(ctx, list); err != nil {
		return fmt.Errorf("failed to list providers: %w", err)
	}

	var provider *terraformv1alpha1.Provider

	// @step: ensure only one provider is default as default
	var count int
	for i := 0; i < len(list.Items); i++ {
		switch {
		case list.Items[i].Annotations == nil:
			continue
		case list.Items[i].Annotations[terraformv1alpha1.DefaultProviderAnnotation] == "true":
			count++
			provider = &list.Items[i]
		}
	}
	if count == 0 {
		return nil
	}
	if count > 1 {
		return errors.New("only one provider can be default, please contact your administrator")
	}

	o.Spec.ProviderRef = &terraformv1alpha1.ProviderReference{
		Name: provider.Name,
	}

	return nil
}

// mutateOnPolicyDefaults is called to validate the module policy enforced
func (m *mutator) mutateOnPolicyDefaults(ctx context.Context, list *terraformv1alpha1.PolicyList, o *terraformv1alpha1.Configuration) error {
	if len(list.Items) == 0 {
		return nil
	}

	// @step: we need to retrieve the namespace of the configuration
	namespace := &v1.Namespace{}
	namespace.Name = o.Namespace

	if found, err := kubernetes.GetIfExists(ctx, m.cc, namespace); err != nil {
		return fmt.Errorf("failed to get namespace: %w", err)
	} else if !found {
		return fmt.Errorf("failed to find namespace %s", o.Namespace)
	}

	var names []string

	// @step: iterate over the policies and update the configuration if required
	for _, policy := range list.Items {
		for _, x := range policy.Spec.Defaults {
			var filtered bool

			if !x.HasVariables() {
				continue
			}

			// @step: if the policy a wildcard and applies to all configurations
			if !x.HasSelectors() {
				filtered = true
			} else {
				// @step: does the policy match the configuration
				matched, err := isMatch(x.Selector, o, namespace)
				switch {
				case err != nil:
					return fmt.Errorf("failed to match selector: %w", err)
				case matched:
					filtered = true
				}
			}
			// did we have a match?
			if !filtered {
				continue
			}

			names = append(names, policy.Name)

			// @step: create a patch for the configuration
			patch, err := jsonpatch.CreateMergePatch([]byte(`{}`), x.Variables.Raw)
			if err != nil {
				return fmt.Errorf("failed to create merge patch: %w", err)
			}
			if !o.Spec.HasVariables() {
				o.Spec.Variables = &runtime.RawExtension{Raw: patch}
				continue
			}

			modified, err := jsonpatch.MergePatch(o.Spec.Variables.Raw, patch)
			if err != nil {
				return fmt.Errorf("failed to merge patch: %w", err)
			}
			o.Spec.Variables.Raw = modified
		}
	}

	if len(names) > 0 {
		if o.Annotations == nil {
			o.Annotations = make(map[string]string)
		}
		o.Annotations[terraformv1alpha1.DefaultVariablesAnnotation] = strings.Join(names, ",")
	}

	return nil
}

// isMatch returns if the selector matches the policy
func isMatch(
	selector terraformv1alpha1.DefaultVariablesSelector,
	configuration *terraformv1alpha1.Configuration,
	namespace client.Object,
) (bool, error) {

	switch {
	case len(selector.Modules) > 0 && selector.Namespace != nil:
		a, err := selector.IsLabelsMatch(namespace)
		if err != nil {
			return false, fmt.Errorf("failed to match label selector: %w", err)
		}
		b, err := selector.IsModulesMatch(configuration)
		if err != nil {
			return false, fmt.Errorf("failed to match module selector: %w", err)
		}

		return a && b, nil

	case len(selector.Modules) > 0:
		return selector.IsModulesMatch(configuration)

	case selector.Namespace != nil:
		return selector.IsLabelsMatch(namespace)
	}

	return false, nil
}
