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

package cloudresources

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	corev1alpha1 "github.com/appvia/terranetes-controller/pkg/apis/core/v1alpha1"
	terraformv1alpha1 "github.com/appvia/terranetes-controller/pkg/apis/terraform/v1alpha1"
	"github.com/appvia/terranetes-controller/pkg/utils"
	"github.com/appvia/terranetes-controller/pkg/utils/kubernetes"
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
	o, ok := obj.(*terraformv1alpha1.CloudResource)
	if !ok {
		return fmt.Errorf("expected a %s, but got: %T", terraformv1alpha1.CloudResourceKind, obj)
	}

	return validate(ctx, v.cc, o)
}

// ValidateUpdate is called when a resource is being updated
func (v *validator) ValidateUpdate(ctx context.Context, oldObj, newObj runtime.Object) error {
	var before *terraformv1alpha1.CloudResource

	if newObj != nil {
		o, ok := newObj.(*terraformv1alpha1.CloudResource)
		if !ok {
			return fmt.Errorf("expected a %s, but got: %T", terraformv1alpha1.CloudResourceKind, newObj)
		}
		before = o
	}

	return validate(ctx, v.cc, before)
}

// ValidateDelete is called when a resource is being deleted
func (v *validator) ValidateDelete(ctx context.Context, obj runtime.Object) error {
	return nil
}

// validate is responsible for validating the configuration plan
func validate(ctx context.Context, cc client.Client, o *terraformv1alpha1.CloudResource) error {
	var values map[string]interface{}

	if err := o.Spec.Plan.IsValid(); err != nil {
		return err
	}
	if o.Spec.ProviderRef != nil {
		if err := o.Spec.ProviderRef.IsValid(); err != nil {
			return err
		}
	}
	if o.Spec.Auth != nil && o.Spec.Auth.Name == "" {
		return errors.New("spec.auth.name is required")
	}
	if o.Spec.WriteConnectionSecretToRef != nil {
		if err := o.Spec.WriteConnectionSecretToRef.IsValid(); err != nil {
			return err
		}
	}
	if o.Spec.HasValueFrom() {
		if err := o.Spec.ValueFrom.IsValid(); err != nil {
			return err
		}
	}

	// @step: lets check the inputs are valid
	plan := &terraformv1alpha1.Plan{}
	plan.Name = o.Spec.Plan.Name

	if found, err := kubernetes.GetIfExists(ctx, cc, plan); err != nil {
		return fmt.Errorf("spec.plan.name failed to retrieve plan %s: %w", o.Spec.Plan, err)
	} else if !found {
		return errors.New("spec.plan.name does not exist")
	}

	// @step: ensure the plan has a condition of ready
	switch {
	case
		!plan.Status.HasCondition(corev1alpha1.ConditionReady),
		plan.Status.GetCondition(corev1alpha1.ConditionReady).Status != metav1.ConditionTrue:

		return errors.New("spec.plan.name is not is a ready state")
	}

	// @step: lets which revision to should use
	revision, found := plan.GetRevision(o.Spec.Plan.Revision)
	if !found {
		return fmt.Errorf("spec.plan.revision: %s does not exist in plan", o.Spec.Plan.Revision)
	}

	rv := &terraformv1alpha1.Revision{}
	rv.Name = revision.Name

	// @step: now we need to the configuration of the revision
	if found, err := kubernetes.GetIfExists(ctx, cc, rv); err != nil {
		return fmt.Errorf("spec.plan.revision: failed to retrieve resource %w", err)
	} else if !found {
		return errors.New("spec.plan.revision: does not exist")
	}

	// @step: if we don't have a provider on the cloudresource OR the revision we need
	// to fail
	if o.Spec.ProviderRef == nil && rv.Spec.Configuration.ProviderRef == nil {
		return errors.New("spec.providerRef is required")
	}

	// @step: lets checks if the variables already defined on the cloudresource
	// are permitted by the revision
	permitted := rv.ListOfInputs()

	// @step: lets decode cloudresource variables
	if o.Spec.HasVariables() {
		values = make(map[string]interface{})

		if err := json.NewDecoder(bytes.NewReader(o.Spec.Variables.Raw)).Decode(&values); err != nil {
			return fmt.Errorf("failed to decode variables: %w", err)
		}
	}

	if o.Spec.HasVariables() {
		// @step: then we need iterate the variables and check if they are permitted
		for key := range values {
			if !utils.Contains(key, permitted) {
				return fmt.Errorf("spec.variables.%s is not permitted by revision: %s", key, o.Spec.Plan.Revision)
			}
		}
	} else {
		o.Spec.Variables = &runtime.RawExtension{Raw: []byte("{}")}
	}

	// @step: we need to check the only variables added are permitted by the plan
	for i, x := range o.Spec.ValueFrom {
		if !utils.Contains(x.Name, permitted) {
			return fmt.Errorf("spec.valueFrom[%d].%s input is not permitted by revision: %s",
				i, x.Name, o.Spec.Plan.Revision)
		}
	}

	// @step: now we need to ensure any variables defined in the revision are present
	for _, input := range rv.Spec.Inputs {
		if pointer.BoolDeref(input.Required, false) && input.Default == nil {
			var found bool

			if o.Spec.HasVariables() {
				if _, ok := values[input.Key]; ok {
					found = true
				}
			}
			for _, x := range o.Spec.ValueFrom {
				if x.Name == input.Key {
					found = true
				}
			}

			if !found {
				return fmt.Errorf("spec.variables.%s is required variable for revision: %s", input.Key, o.Spec.Plan.Revision)
			}
		}
	}

	return nil
}
