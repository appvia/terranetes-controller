/*
 * Copyright (C) 2022  Rohith Jayawardene <gambol99@gmail.com>
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

	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	terraformv1alphav1 "github.com/appvia/terraform-controller/pkg/apis/terraform/v1alpha1"
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

func (v *validator) validate(ctx context.Context, c *terraformv1alphav1.Configuration) error {
	list := &terraformv1alphav1.PolicyList{}
	if err := v.cc.List(ctx, list); err != nil {
		return err
	}
	if len(list.Items) == 0 {
		return nil
	}

	for _, x := range list.Items {
		switch {
		case x.Spec.Constraints == nil:
			continue
		case x.Spec.Constraints.Modules == nil, len(x.Spec.Constraints.Modules.Allowed) == 0:
			continue
		}

		matches, err := x.Spec.Constraints.Modules.Matches(c.Spec.Module)
		if err != nil {
			return fmt.Errorf("failed to compile the policy: %s, error: %s", x.Name, err)
		}
		if matches {
			return nil
		}
	}

	return fmt.Errorf("the configuration has been denied by policy")
}

// ValidateDelete is called when a resource is being deleted
func (v *validator) ValidateDelete(ctx context.Context, obj runtime.Object) error {
	return nil
}
