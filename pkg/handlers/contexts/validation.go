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

package contexts

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	terraformv1alpha1 "github.com/appvia/terranetes-controller/pkg/apis/terraform/v1alpha1"
)

type validator struct {
	cc client.Client
}

// NewValidator is validation handler
func NewValidator(cc client.Client) admission.CustomValidator {
	return &validator{cc: cc}
}

// ValidateCreate is called when a new resource is created
func (v *validator) ValidateCreate(ctx context.Context, obj runtime.Object) (admission.Warnings, error) {
	return admission.Warnings{}, v.validate(ctx, nil, obj.(*terraformv1alpha1.Context))
}

// ValidateUpdate is called when a resource is being updated
func (v *validator) ValidateUpdate(ctx context.Context, oldObj, newObj runtime.Object) (admission.Warnings, error) {
	var before, after *terraformv1alpha1.Context

	if newObj != nil {
		after = newObj.(*terraformv1alpha1.Context)
	}
	if oldObj != nil {
		before = oldObj.(*terraformv1alpha1.Context)
	}

	return admission.Warnings{}, v.validate(ctx, before, after)
}

// validate is called to ensure the configuration is valid and incline with current policies
func (v *validator) validate(_ context.Context, _, current *terraformv1alpha1.Context) error {
	if len(current.Spec.Variables) == 0 {
		return nil
	}

	for name, variable := range current.Spec.Variables {
		if len(variable.Raw) == 0 {
			return fmt.Errorf(`spec.variable["%s"] must have a value`, name)
		}

		var inputs map[string]interface{}
		if err := json.NewDecoder(bytes.NewReader(variable.Raw)).Decode(&inputs); err != nil {
			return fmt.Errorf(`spec.variable["%s"] invalid input`, name)
		}

		switch {
		case inputs[terraformv1alpha1.ContextDescription] == nil:
			return fmt.Errorf(`spec.variables["%s"].description is required`, name)

		case inputs[terraformv1alpha1.ContextValue] == nil:
			return fmt.Errorf(`spec.variables["%s"].value is required`, name)
		}
	}

	return nil
}

// ValidateDelete is called when a resource is being deleted
func (v *validator) ValidateDelete(ctx context.Context, obj runtime.Object) (admission.Warnings, error) {
	var warnings admission.Warnings
	current := obj.(*terraformv1alpha1.Context)

	if current.GetAnnotations()[terraformv1alpha1.OrphanAnnotation] == "true" {
		return warnings, nil
	}

	// @choice: unless the zero validation annotation is present we should not be
	// allowed to delete a context if it's being referenced
	list := &terraformv1alpha1.ConfigurationList{}

	if err := v.cc.List(ctx, list); err != nil {
		return warnings, err
	}
	if len(list.Items) == 0 {
		return warnings, nil
	}

	var inuse []string

	for i := 0; i < len(list.Items); i++ {
		for _, x := range list.Items[i].Spec.ValueFrom {
			if pointer.StringDeref(x.Context, "") == current.Name {
				inuse = append(inuse, list.Items[i].GetNamespacedName().String())
			}
		}
	}

	if len(inuse) > 0 {
		return warnings, fmt.Errorf("resource in use by configuration(s): %v", strings.Join(inuse, ", "))
	}

	return warnings, nil
}
