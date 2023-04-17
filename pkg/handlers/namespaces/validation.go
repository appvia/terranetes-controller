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

package namespaces

import (
	"context"
	"fmt"

	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	terraformv1alpha1 "github.com/appvia/terranetes-controller/pkg/apis/terraform/v1alpha1"
)

type validator struct {
	cc client.Client
	// EnableNamespaceProtection enables the namespace protection - i.e the namespace deletion
	// is blocked if there any Configurations in the namespace
	EnableNamespaceProtection bool
}

// NewValidator is validation handler
func NewValidator(cc client.Client, enabled bool) admission.CustomValidator {
	return &validator{cc: cc, EnableNamespaceProtection: enabled}
}

// ValidateDelete is called when a resource is being deleted
func (v *validator) ValidateDelete(ctx context.Context, obj runtime.Object) error {
	// @step: check if the namespace protection is enabled
	if !v.EnableNamespaceProtection {
		return nil
	}

	ns, ok := obj.(*v1.Namespace)
	if !ok {
		return fmt.Errorf("the object %T is not an expected v1.Namespace", obj)
	}

	// @step: check if there are any configurations in the namespace
	list := &terraformv1alpha1.ConfigurationList{}
	if err := v.cc.List(ctx, list, client.InNamespace(ns.Name)); err != nil {
		return err
	}
	if len(list.Items) > 0 {
		return fmt.Errorf("namespace %s is protected by terranetes, ensure configurations are deleted first", ns.Name)
	}

	return nil
}

// ValidateCreate is called when a new resource is created
func (v *validator) ValidateCreate(ctx context.Context, obj runtime.Object) error {
	return nil
}

// ValidateUpdate is called when a resource is being updated
func (v *validator) ValidateUpdate(ctx context.Context, oldObj, newObj runtime.Object) error {
	return nil
}
