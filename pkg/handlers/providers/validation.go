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

package providers

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
	return v.Validate(ctx, obj.(*terraformv1alphav1.Provider))
}

// ValidateUpdate is called when a resource is being updated
func (v *validator) ValidateUpdate(ctx context.Context, oldObj, newObj runtime.Object) error {
	return v.Validate(ctx, newObj.(*terraformv1alphav1.Provider))
}

// ValidateDelete is called when a resource is being deleted
func (v *validator) ValidateDelete(ctx context.Context, obj runtime.Object) error {
	return nil
}

// Validate handles the generic validation of a provider
func (v *validator) Validate(ctx context.Context, provider *terraformv1alphav1.Provider) error {
	switch provider.Spec.Provider {
	case terraformv1alphav1.AWSProviderType, terraformv1alphav1.GCPProviderType, terraformv1alphav1.AzureProviderType:
		break
	default:
		return fmt.Errorf("spec.provider: %s is not supported", provider.Spec.Provider)
	}

	switch provider.Spec.Source {
	case terraformv1alphav1.SourceSecret:
		switch {
		case provider.Spec.SecretRef == nil:
			return fmt.Errorf("spec.secretRef: secret is required when source is secret")
		case provider.Spec.SecretRef.Name == "":
			return fmt.Errorf("spec.secretRef.name: name is required when source is secret")
		case provider.Spec.SecretRef.Namespace == "":
			return fmt.Errorf("spec.secretRef.namespace: namespace is required when source is secret")
		}

	case terraformv1alphav1.SourceInjected:
		switch {
		case provider.Spec.ServiceAccount == nil:
			return fmt.Errorf("spec.serviceAccount: serviceAccount is required when source is injected")
		case *provider.Spec.ServiceAccount == "":
			return fmt.Errorf("spec.serviceAccount: serviceAccount is required when source is injected")
		}

	default:
		return fmt.Errorf("spec.source: %s is not supported", provider.Spec.Source)

	}

	return nil
}
