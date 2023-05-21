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
	"errors"
	"fmt"

	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	terraformv1alpha1 "github.com/appvia/terranetes-controller/pkg/apis/terraform/v1alpha1"
)

type validator struct {
	cc client.Client
	// jobNamespace is the namespace where static credentials should be provision.
	jobNamespace string
}

// NewValidator is validation handler
func NewValidator(cc client.Client, namespace string) admission.CustomValidator {
	return &validator{cc: cc, jobNamespace: namespace}
}

// ValidateCreate is called when a new resource is created
func (v *validator) ValidateCreate(ctx context.Context, obj runtime.Object) error {
	return v.validate(ctx, obj.(*terraformv1alpha1.Provider))
}

// ValidateUpdate is called when a resource is being updated
func (v *validator) ValidateUpdate(ctx context.Context, oldObj, newObj runtime.Object) error {
	return v.validate(ctx, newObj.(*terraformv1alpha1.Provider))
}

// ValidateDelete is called when a resource is being deleted
func (v *validator) ValidateDelete(ctx context.Context, obj runtime.Object) error {
	return nil
}

// validate handles the generic validation of a provider
func (v *validator) validate(ctx context.Context, provider *terraformv1alpha1.Provider) error {
	switch provider.Spec.Source {
	case terraformv1alpha1.SourceSecret:
		switch {
		case provider.Spec.SecretRef == nil:
			return errors.New("spec.secretRef: secret is required when source is secret")
		case provider.Spec.SecretRef.Name == "":
			return errors.New("spec.secretRef.name: name is required when source is secret")
		case provider.Spec.SecretRef.Namespace == "":
			return errors.New("spec.secretRef.namespace: namespace is required when source is secret")
		case provider.Spec.SecretRef.Namespace != v.jobNamespace:
			return errors.New("spec.secretRef.namespace: must be in same namespace as the controller")
		}

	case terraformv1alpha1.SourceInjected:
		switch {
		case provider.Spec.ServiceAccount == nil:
			return errors.New("spec.serviceAccount: serviceAccount is required when source is injected")
		case *provider.Spec.ServiceAccount == "":
			return errors.New("spec.serviceAccount: serviceAccount is required when source is injected")
		}

	default:
		return fmt.Errorf("spec.source: %s is not supported", provider.Spec.Source)
	}

	// @step: are we trying to set provider as a default provider
	annotations := provider.GetAnnotations()
	if annotations != nil && annotations[terraformv1alpha1.DefaultProviderAnnotation] == "true" {

		list := &terraformv1alpha1.ProviderList{}
		if err := v.cc.List(ctx, list); err != nil {
			return fmt.Errorf("failed to list providers: %w", err)
		}

		for i := 0; i < len(list.Items); i++ {
			if list.Items[i].Name == provider.Name {
				continue
			}
			if list.Items[i].GetAnnotations()[terraformv1alpha1.DefaultProviderAnnotation] == "true" {
				return errors.New("only one provider can be set as the default provider")
			}
		}
	}

	// @step: validate any preloading configuration
	if provider.Spec.Preload != nil {
		switch {
		case provider.Spec.Preload.Context == "":
			return errors.New("spec.preload.context: is required")
		case provider.Spec.Preload.Cluster == "":
			return errors.New("spec.preload.cluster: is required")
		case provider.Spec.Preload.Region == "":
			return errors.New("spec.preload.region: is required")
		}
	}

	return nil
}
