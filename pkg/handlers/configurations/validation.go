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

	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	terraformv1alpha1 "github.com/appvia/terranetes-controller/pkg/apis/terraform/v1alpha1"
	"github.com/appvia/terranetes-controller/pkg/utils/kubernetes"
	"github.com/appvia/terranetes-controller/pkg/utils/policies"
)

type validator struct {
	cc client.Client
	// enableVersions indicates the terraform version can be changed
	enableVersions bool
}

// NewValidator is validation handler
func NewValidator(cc client.Client, versioning bool) admission.CustomValidator {
	return &validator{cc: cc, enableVersions: versioning}
}

// ValidateCreate is called when a new resource is created
func (v *validator) ValidateCreate(ctx context.Context, obj runtime.Object) (admission.Warnings, error) {
	o, ok := obj.(*terraformv1alpha1.Configuration)
	if !ok {
		return admission.Warnings{}, fmt.Errorf("expected a Configuration, but got: %T", obj)
	}

	return admission.Warnings{}, v.validate(ctx, nil, o)
}

// ValidateUpdate is called when a resource is being updated
func (v *validator) ValidateUpdate(ctx context.Context, oldObj, newObj runtime.Object) (admission.Warnings, error) {
	var before, after *terraformv1alpha1.Configuration

	if newObj != nil {
		after = newObj.(*terraformv1alpha1.Configuration)
	}
	if oldObj != nil {
		before = oldObj.(*terraformv1alpha1.Configuration)
	}

	return admission.Warnings{}, v.validate(ctx, before, after)
}

// ValidateDelete is called when a resource is being deleted
func (v *validator) ValidateDelete(ctx context.Context, obj runtime.Object) (admission.Warnings, error) {
	return admission.Warnings{}, nil
}

// validate is called to ensure the configuration is valid and incline with current policies
func (v *validator) validate(ctx context.Context, before, configuration *terraformv1alpha1.Configuration) error {
	creating := before == nil

	// @step: let us check the provider
	switch {
	case configuration.Spec.ProviderRef == nil:
		return errors.New("spec.providerRef is required")
	case configuration.Spec.Module == "":
		return errors.New("spec.module is required")
	}

	if configuration.Spec.Plan != nil {
		if err := configuration.Spec.Plan.IsValid(); err != nil {
			return err
		}
	}
	if configuration.Spec.Auth != nil {
		if configuration.Spec.Auth.Name == "" {
			return errors.New("spec.auth.name is required")
		}
	}
	if err := configuration.Spec.ProviderRef.IsValid(); err != nil {
		return err
	}

	// @step: perform some checks which are dependent on if the resource is being created or updated
	switch creating {
	case true:
		if configuration.Spec.TerraformVersion != "" && !v.enableVersions {
			return errors.New("spec.terraformVersion changes have been disabled")
		}

	default:
		if !v.enableVersions {
			switch {
			case configuration.Spec.TerraformVersion == "":
				break
			case before.Spec.TerraformVersion == "" && configuration.Spec.TerraformVersion != "":
				return errors.New("spec.terraformVersion has been disabled and cannot be changed")
			case before.Spec.TerraformVersion != configuration.Spec.TerraformVersion:
				return errors.New("spec.terraformVersion has been disabled, version cannot be changed only removed")
			}
		}
	}

	// @step: check the configuration secret
	if configuration.Spec.WriteConnectionSecretToRef != nil {
		if err := configuration.Spec.WriteConnectionSecretToRef.IsValid(); err != nil {
			return err
		}
	}
	// @step: check the configuration valud froms are valid
	if err := configuration.Spec.ValueFrom.IsValid(); err != nil {
		return err
	}

	// @step: grab the namespace of the configuration
	namespace := &v1.Namespace{}
	namespace.Name = configuration.Namespace
	found, err := kubernetes.GetIfExists(ctx, v.cc, namespace)
	if err != nil {
		return err
	}
	if !found {
		return errors.New("configuration namespace was not found")
	}

	// @step: check the configuration is permitted to use the provider
	if err := validateProvider(ctx, v.cc, configuration, namespace); err != nil {
		return err
	}

	list := &terraformv1alpha1.PolicyList{}
	if err := v.cc.List(ctx, list); err != nil {
		return err
	}
	// @step: validate the configuration against all module constraints
	if len(list.Items) > 0 {
		if err := validateModuleConstriants(configuration, list, namespace); err != nil {
			return err
		}
	}

	return nil
}

// validateProvider is called to ensure the configuration is valid and inline with current provider policy
func validateProvider(ctx context.Context, cc client.Client, configuration *terraformv1alpha1.Configuration, namespace *v1.Namespace) error {
	provider := &terraformv1alpha1.Provider{}
	provider.Name = configuration.Spec.ProviderRef.Name

	found, err := kubernetes.GetIfExists(ctx, cc, provider)
	if err != nil {
		return err
	}
	if !found || provider.Spec.Selector == nil {
		return nil
	}

	matched, err := kubernetes.IsSelectorMatch(*provider.Spec.Selector, configuration.GetLabels(), namespace.GetLabels())
	if err != nil {
		return err
	}
	if !matched {
		return errors.New("configuration has been denied by the provider policy")
	}

	return nil
}

// validateModuleConstriants evaluates the module constraints and ensure the configuration passes all policies
func validateModuleConstriants(
	configuration *terraformv1alpha1.Configuration,
	list *terraformv1alpha1.PolicyList,
	namespace *v1.Namespace) error {

	var filtered []terraformv1alpha1.Policy

	// @step: first we build a list of policies which apply this configuration.
	for _, x := range policies.FindModuleConstraints(list) {
		if x.Spec.Constraints.Modules.Selector != nil {
			matched, err := kubernetes.IsSelectorMatch(*x.Spec.Constraints.Modules.Selector,
				configuration.GetLabels(),
				namespace.GetLabels(),
			)
			if err != nil {
				return err
			} else if !matched {
				continue
			}
		}
		filtered = append(filtered, x)
	}

	if len(filtered) == 0 {
		return nil
	}

	// @step: we then iterate the allowed list; at least one of the policies must be satisfied
	for _, x := range filtered {
		if found, err := x.Spec.Constraints.Modules.Matches(configuration.Spec.Module); err != nil {
			return fmt.Errorf("failed to compile the policy: %s, error: %w", x.Name, err)
		} else if found {
			return nil
		}
	}

	return fmt.Errorf("spec.module: source has been denied by module policy, contact an administrator")
}
