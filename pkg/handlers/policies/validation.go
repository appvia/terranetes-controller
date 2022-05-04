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
	"fmt"
	"strings"

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
	return nil
}

// ValidateUpdate is called when a resource is being updated
func (v *validator) ValidateUpdate(ctx context.Context, oldObj, newObj runtime.Object) error {
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
