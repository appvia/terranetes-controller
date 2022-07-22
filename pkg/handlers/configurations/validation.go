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

	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	terraformv1alphav1 "github.com/appvia/terranetes-controller/pkg/apis/terraform/v1alpha1"
	"github.com/appvia/terranetes-controller/pkg/utils"
	"github.com/appvia/terranetes-controller/pkg/utils/kubernetes"
)

type validator struct {
	cc client.Client
	// versioning indicates that configurations can be overridden
	versioning bool
}

// NewValidator is validation handler
func NewValidator(cc client.Client, versioning bool) admission.CustomValidator {
	return &validator{cc: cc, versioning: versioning}
}

// ValidateCreate is called when a new resource is created
func (v *validator) ValidateCreate(ctx context.Context, obj runtime.Object) error {
	return v.validate(ctx, nil, obj.(*terraformv1alphav1.Configuration))
}

// ValidateUpdate is called when a resource is being updated
func (v *validator) ValidateUpdate(ctx context.Context, oldObj, newObj runtime.Object) error {
	var before, after *terraformv1alphav1.Configuration

	if newObj != nil {
		after = newObj.(*terraformv1alphav1.Configuration)
	}
	if oldObj != nil {
		before = oldObj.(*terraformv1alphav1.Configuration)
	}

	return v.validate(ctx, before, after)
}

// ValidateDelete is called when a resource is being deleted
func (v *validator) ValidateDelete(ctx context.Context, obj runtime.Object) error {
	return nil
}

// validate is called to ensure the configuration is valid and incline with current policies
func (v *validator) validate(ctx context.Context, before, configuration *terraformv1alphav1.Configuration) error {
	creating := before == nil

	// @step: let us check the provider
	switch {
	case configuration.Spec.ProviderRef == nil:
		return errors.New("no spec.providerRef is defined")
	case configuration.Spec.ProviderRef.Name == "":
		return errors.New("spec.providerRef.name is empty")
	}

	// @step: perform some checks which are dependent on if the resource is being created or updated
	switch creating {
	case true:
		if configuration.Spec.TerraformVersion != "" && !v.versioning {
			return errors.New("spec.terraformVersion changes have been disabled")
		}

	default:
		if !v.versioning {
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
	if err := validateConnectionSecret(configuration); err != nil {
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

	list := &terraformv1alphav1.PolicyList{}
	if err := v.cc.List(ctx, list); err != nil {
		return err
	}
	if len(list.Items) == 0 {
		return nil
	}

	// @step: validate the configuration against all module constraints
	if err := validateModuleConstriants(configuration, list, namespace); err != nil {
		return err
	}

	return nil
}

// validateConnectionSecret checks if the secret is valid
func validateConnectionSecret(configuration *terraformv1alphav1.Configuration) error {
	switch {
	case configuration.Spec.WriteConnectionSecretToRef == nil:
		return nil
	case configuration.Spec.WriteConnectionSecretToRef.Name == "":
		return errors.New("spec.writeConnectionSecretToRef.name is empty")
	}

	for i, key := range configuration.Spec.WriteConnectionSecretToRef.Keys {
		if strings.Contains(key, ":") && len(strings.Split(key, ":")) != 2 {
			return fmt.Errorf("spec.writeConnectionSecretToRef.keys[%d] contains invalid key: %s, should be KEY:NEWNAME", i, key)
		}
	}

	return nil
}

// validateProvider is called to ensure the configuration is valid and inline with current provider policy
func validateProvider(ctx context.Context, cc client.Client, configuration *terraformv1alphav1.Configuration, namespace *v1.Namespace) error {
	provider := &terraformv1alphav1.Provider{}
	provider.Name = configuration.Spec.ProviderRef.Name

	found, err := kubernetes.GetIfExists(ctx, cc, provider)
	if err != nil {
		return err
	}
	if !found || provider.Spec.Selector == nil {
		return nil
	}

	matched, err := utils.IsSelectorMatch(*provider.Spec.Selector, configuration.GetLabels(), namespace.GetLabels())
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
	configuration *terraformv1alphav1.Configuration,
	policies *terraformv1alphav1.PolicyList,
	namespace *v1.Namespace) error {

	var list []terraformv1alphav1.Policy

	// @step: first we build a list of policies which apply this configuration.
	for _, x := range policies.Items {
		switch {
		case x.Spec.Constraints == nil, x.Spec.Constraints.Modules == nil:
			continue
		}
		if x.Spec.Constraints.Modules.Selector != nil {
			matched, err := utils.IsSelectorMatch(*x.Spec.Constraints.Modules.Selector, configuration.GetLabels(), namespace.GetLabels())
			if err != nil {
				return err
			} else if !matched {
				continue
			}
		}

		list = append(list, x)
	}

	if len(list) == 0 {
		return nil
	}

	// @step: we then iterate the allowed list; at least one of the policies must be satisfied
	for _, x := range list {
		if found, err := x.Spec.Constraints.Modules.Matches(configuration.Spec.Module); err != nil {
			return fmt.Errorf("failed to compile the policy: %s, error: %s", x.Name, err)
		} else if found {
			return nil
		}
	}

	return fmt.Errorf("configuration has been denied by policy")
}
