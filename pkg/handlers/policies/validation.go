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

	terraformv1alphav1 "github.com/appvia/terraform-controller/pkg/apis/terraform/v1alpha1"
	"github.com/appvia/terraform-controller/pkg/utils"
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
	return v.ValidateUpdate(ctx, nil, obj)
}

// ValidateUpdate is called when a resource is being updated
func (v *validator) ValidateUpdate(ctx context.Context, oldObj, newObj runtime.Object) error {
	o := newObj.(*terraformv1alphav1.Policy)

	if o.Spec.Constraints != nil {
		if o.Spec.Constraints.Modules != nil {
			if o.Spec.Constraints.Modules.Selector != nil {
				if o.Spec.Constraints.Modules.Selector.Namespace != nil {
					if _, err := metav1.LabelSelectorAsSelector(o.Spec.Constraints.Modules.Selector.Namespace); err != nil {
						return fmt.Errorf("spec.constraints.modules.selector.namespace is invalid, %v", err)
					}
				}

				if o.Spec.Constraints.Modules.Selector.Resource != nil {
					if _, err := metav1.LabelSelectorAsSelector(o.Spec.Constraints.Modules.Selector.Resource); err != nil {
						return fmt.Errorf("spec.constraints.modules.selector.resource is invalid, %v", err)
					}
				}
			}

			// @step: ensure the regexes are valid
			for i, x := range o.Spec.Constraints.Modules.Allowed {
				if _, err := regexp.Compile(x); err != nil {
					return fmt.Errorf("spec.constraints.modules.allowed[%d] is invalid, %v", i, err)
				}
			}
		}

		if o.Spec.Constraints.Checkov != nil {
			if o.Spec.Constraints.Checkov.Selector != nil {

				if o.Spec.Constraints.Checkov.Selector.Namespace != nil {
					if _, err := metav1.LabelSelectorAsSelector(o.Spec.Constraints.Checkov.Selector.Namespace); err != nil {
						return fmt.Errorf("spec.constraints.checkov.selector.namespace is invalid, %v", err)
					}
				}

				if o.Spec.Constraints.Checkov.Selector.Resource != nil {
					if _, err := metav1.LabelSelectorAsSelector(o.Spec.Constraints.Checkov.Selector.Resource); err != nil {
						return fmt.Errorf("spec.constraints.checkov.selector.resource is invalid, %v", err)
					}
				}
			}

			if len(o.Spec.Constraints.Checkov.Checks) > 0 && len(o.Spec.Constraints.Checkov.SkipChecks) > 0 {
				if utils.ContainsList(o.Spec.Constraints.Checkov.Checks, o.Spec.Constraints.Checkov.SkipChecks) {
					return errors.New("spec.constraints.policy.skipChecks cannot contain checks from spec.constraints.policy.checks")
				}
			}
		}
	}

	return nil
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
