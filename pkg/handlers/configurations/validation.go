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

	terraformv1alphav1 "github.com/appvia/terraform-controller/pkg/apis/terraform/v1alpha1"
	"github.com/appvia/terraform-controller/pkg/utils"
	"github.com/appvia/terraform-controller/pkg/utils/kubernetes"
)

type validator struct {
	cc client.Client
}

// NewValidator is validation handler
func NewValidator(cc client.Client) admission.CustomValidator {
	return &validator{cc: cc}
}

// ValidateCreate is called when a new resource is created
func (v *validator) ValidateCreate(ctx context.Context, obj runtime.Object) error {
	return v.validate(ctx, obj.(*terraformv1alphav1.Configuration))
}

// ValidateUpdate is called when a resource is being updated
func (v *validator) ValidateUpdate(ctx context.Context, oldObj, newObj runtime.Object) error {
	return v.validate(ctx, newObj.(*terraformv1alphav1.Configuration))
}

// ValidateDelete is called when a resource is being deleted
func (v *validator) ValidateDelete(ctx context.Context, obj runtime.Object) error {
	return nil
}

// validate is called to ensure the configuration is valid and incline with current policies
func (v *validator) validate(ctx context.Context, c *terraformv1alphav1.Configuration) error {
	// @step: let us check the provider
	switch {
	case c.Spec.ProviderRef == nil:
		return errors.New("no spec.providerRef is defined")
	case c.Spec.ProviderRef.Name == "":
		return errors.New("spec.providerRef.name is empty")
	case c.Spec.ProviderRef.Namespace == "":
		return errors.New("spec.providerRef.namespace is empty")
	}

	// @step: check the configuration is permitted to use the provider
	if err := validateProvider(ctx, v.cc, c); err != nil {
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
	if err := validateModuleConstriants(ctx, c, list); err != nil {
		return err
	}

	return nil
}

// validateProvider is called to ensure the configuration is valid and inline with current provider policy
func validateProvider(ctx context.Context, cc client.Client, configuration *terraformv1alphav1.Configuration) error {
	provider := &terraformv1alphav1.Provider{}
	provider.Namespace = configuration.Spec.ProviderRef.Namespace
	provider.Name = configuration.Spec.ProviderRef.Name

	found, err := kubernetes.GetIfExists(ctx, cc, provider)
	if err != nil {
		return err
	}
	if !found || provider.Spec.Selector == nil {
		return nil
	}

	// @step: grab the namespace of the configuration
	namespace := &v1.Namespace{}
	namespace.Name = configuration.Namespace
	found, err = kubernetes.GetIfExists(ctx, cc, namespace)
	if err != nil {
		return err
	}
	if !found {
		return errors.New("configuration namespace was not found")
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
func validateModuleConstriants(ctx context.Context, configuration *terraformv1alphav1.Configuration, list *terraformv1alphav1.PolicyList) error {
	var policies []terraformv1alphav1.Policy

	// @step: first we find all policies which contains module constraints
	for _, x := range list.Items {
		switch {
		case x.Spec.Constraints == nil, x.Spec.Constraints.Modules == nil:
			continue
		}
		policies = append(policies, x)
	}

	if len(policies) == 0 {
		return nil
	}

	// @step: iterate all the policies for a matchj
	for _, x := range policies {
		matches, err := x.Spec.Constraints.Modules.Matches(configuration.Spec.Module)
		if err != nil {
			return fmt.Errorf("failed to compile the policy: %s, error: %s", x.Name, err)
		}
		if matches {
			return nil
		}
	}

	return fmt.Errorf("the configuration has been denied by policy")
}
