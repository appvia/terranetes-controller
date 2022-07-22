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

package policies

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	terraformv1alphav1 "github.com/appvia/terranetes-controller/pkg/apis/terraform/v1alpha1"
	"github.com/appvia/terranetes-controller/pkg/utils"
)

type validator struct {
	cc client.Client
}

// NewValidator is validation handler
func NewValidator(cc client.Client) admission.CustomValidator {
	return &validator{cc: cc}
}

// ValidateDelete is called when a resource is being deleted
func (v *validator) ValidateDelete(ctx context.Context, obj runtime.Object) error {
	list := &terraformv1alphav1.ConfigurationList{}
	o := obj.(*terraformv1alphav1.Policy)

	err := v.cc.List(ctx, list, client.InNamespace(""))
	if err != nil {
		return err
	}

	if o.GetAnnotations()[terraformv1alphav1.SkipDefaultsValidationCheck] == "true" {
		return nil
	}

	var using []string

	for _, x := range list.Items {
		items := strings.Split(x.GetAnnotations()[terraformv1alphav1.DefaultVariablesAnnotation], ",")
		if len(items) == 0 {
			continue
		}

		if utils.Contains(o.Name, items) {
			using = append(using, fmt.Sprintf("%s/%s", x.Namespace, x.Name))
		}
	}

	if len(using) > 0 {
		return fmt.Errorf("policy in use by configurations: %s", strings.Join(using, ", "))
	}

	return nil
}

// ValidateCreate is called when a new resource is created
func (v *validator) ValidateCreate(ctx context.Context, obj runtime.Object) error {
	return v.ValidateUpdate(ctx, nil, obj)
}

// ValidateUpdate is called when a resource is being updated
func (v *validator) ValidateUpdate(ctx context.Context, oldObj, newObj runtime.Object) error {
	o := newObj.(*terraformv1alphav1.Policy)

	if err := validateCheckovConstraints(o); err != nil {
		return err
	}
	if err := validateModuleConstraint(o); err != nil {
		return err
	}

	return nil
}

// validateModuleConstraint ensures the constraints are valid
func validateModuleConstraint(policy *terraformv1alphav1.Policy) error {
	switch {
	case policy.Spec.Constraints == nil, policy.Spec.Constraints.Modules == nil:
		return nil
	}

	constraint := policy.Spec.Constraints.Modules

	if constraint.Selector != nil {
		if constraint.Selector.Namespace != nil {
			if _, err := metav1.LabelSelectorAsSelector(constraint.Selector.Namespace); err != nil {
				return fmt.Errorf("spec.constraints.modules.selector.namespace is invalid, %v", err)
			}
		}

		if constraint.Selector.Resource != nil {
			if _, err := metav1.LabelSelectorAsSelector(constraint.Selector.Resource); err != nil {
				return fmt.Errorf("spec.constraints.modules.selector.resource is invalid, %v", err)
			}
		}
	}

	// @step: ensure the regexes are valid
	for i, x := range constraint.Allowed {
		if _, err := regexp.Compile(x); err != nil {
			return fmt.Errorf("spec.constraints.modules.allowed[%d] is invalid, %v", i, err)
		}
	}

	return nil
}

// validateCheckovConstraints ensures the constraints are valid
func validateCheckovConstraints(policy *terraformv1alphav1.Policy) error {
	switch {
	case policy.Spec.Constraints == nil, policy.Spec.Constraints.Checkov == nil:
		return nil
	}

	constraint := policy.Spec.Constraints.Checkov

	if constraint.Selector != nil {
		if constraint.Selector.Namespace != nil {
			if _, err := metav1.LabelSelectorAsSelector(constraint.Selector.Namespace); err != nil {
				return fmt.Errorf("spec.constraints.checkov.selector.namespace is invalid, %v", err)
			}
		}

		if constraint.Selector.Resource != nil {
			if _, err := metav1.LabelSelectorAsSelector(constraint.Selector.Resource); err != nil {
				return fmt.Errorf("spec.constraints.checkov.selector.resource is invalid, %v", err)
			}
		}
	}

	if len(constraint.Checks) > 0 && len(constraint.SkipChecks) > 0 {
		if utils.ContainsList(constraint.Checks, constraint.SkipChecks) {
			return errors.New("spec.constraints.policy.skipChecks cannot contain checks from spec.constraints.policy.checks")
		}
	}

	for i, external := range constraint.External {
		switch {
		case external.Name == "":
			return fmt.Errorf("spec.constraints.checkov.external[%d].name cannot be empty", i)
		case external.URL == "":
			return fmt.Errorf("spec.constraints.checkov.external[%d].url cannot be empty", i)
		case external.SecretRef != nil && external.SecretRef.Name == "":
			return fmt.Errorf("spec.constraints.checkov.external[%d].secretRef.name cannot be empty", i)
		case external.SecretRef != nil && external.SecretRef.Namespace != "":
			return fmt.Errorf("spec.constraints.checkov.external[%d].secretRef.namespace should not be set", i)
		}
	}

	return nil
}
