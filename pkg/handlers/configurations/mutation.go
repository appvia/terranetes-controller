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
	"fmt"
	"strings"

	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	jsonpatch "github.com/evanphx/json-patch"

	terraformv1alphav1 "github.com/appvia/terranetes-controller/pkg/apis/terraform/v1alpha1"
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
	o, ok := obj.(*terraformv1alphav1.Configuration)
	if !ok {
		return fmt.Errorf("expected terraform configuration, not %T", obj)
	}

	// @step: retrieve a list of all policies
	list := &terraformv1alphav1.PolicyList{}
	if err := m.cc.List(ctx, list); err != nil {
		return fmt.Errorf("failed to list policies: %w", err)
	}
	if len(list.Items) == 0 {
		return nil
	}

	if err := m.mutateOnDefaults(ctx, list, o); err != nil {
		return err
	}

	return nil
}

// mutateOnDefaults is called to validate the module policy enforced
func (m *mutator) mutateOnDefaults(ctx context.Context, list *terraformv1alphav1.PolicyList, o *terraformv1alphav1.Configuration) error {

	namespace := &v1.Namespace{}
	namespace.Name = o.Namespace
	found, err := kubernetes.GetIfExists(ctx, m.cc, namespace)
	if err != nil {
		return fmt.Errorf("failed to get namespace: %w", err)
	}
	if !found {
		return fmt.Errorf("failed to find namespace %s", o.Namespace)
	}

	var names []string

	// @step: iterate over the policies and update the configuration if required
	for _, policy := range list.Items {
		if len(policy.Spec.Defaults) == 0 {
			continue
		}

		for _, x := range policy.Spec.Defaults {
			match, err := isMatch(x.Selector, o, namespace)
			if err != nil {
				return fmt.Errorf("failed to match selector: %w", err)
			}
			if match {
				names = append(names, policy.Name)

				patch, err := jsonpatch.CreateMergePatch([]byte(`{}`), x.Variables.Raw)
				if err != nil {
					return fmt.Errorf("failed to create merge patch: %w", err)
				}
				if !o.HasVariables() {
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
	}

	if len(names) > 0 {
		if o.Annotations == nil {
			o.Annotations = make(map[string]string)
		}
		o.Annotations[terraformv1alphav1.DefaultVariablesAnnotation] = strings.Join(names, ",")
	}

	return nil
}

// isMatch returns if the selector matches the policy
func isMatch(
	selector terraformv1alphav1.DefaultVariablesSelector,
	configuration *terraformv1alphav1.Configuration,
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
