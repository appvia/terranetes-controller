/*
 * Copyright (C) 2022 Appvia Ltd <info@appvia.io>
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

package v1alpha1

import (
	"bytes"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"

	corev1alphav1 "github.com/appvia/terranetes-controller/pkg/apis/core/v1alpha1"
)

// ProviderKind is the kind for a Provider
const ProviderKind = "Provider"

// ProviderGVK is the GVK for a Provider
var ProviderGVK = schema.GroupVersionKind{
	Group:   GroupVersion.Group,
	Version: GroupVersion.Version,
	Kind:    ProviderKind,
}

// ProviderType is the type of cloud
type ProviderType string

const (
	// AliCloudProviderType is the Alibaba Cloud provider type
	AliCloudProviderType ProviderType = "alicloud"
	// AzureProviderType is the Azure provider type
	AzureProviderType ProviderType = "azurerm"
	// AzureCloudStackProviderType is the Azure Cloud Stack provider type
	AzureCloudStackProviderType ProviderType = "azurestack"
	// AWSProviderType is the AWS provider type
	AWSProviderType ProviderType = "aws"
	// AzureActiveDirectoryProviderType is the Azure Active Directory provider type
	AzureActiveDirectoryProviderType ProviderType = "azuread"
	// GCPProviderType is the GCP provider type
	GCPProviderType ProviderType = "google"
	// GoogleWorkpspaceProviderType is the Google Workspace provider type
	GoogleWorkpspaceProviderType ProviderType = "googleworkspace"
	// KubernetesProviderType is the Kubernetes provider type
	KubernetesProviderType ProviderType = "kubernetes"
	// VaultProviderType is the Vault provider type
	VaultProviderType ProviderType = "vault"
	// VSphereProviderType is the VSphere provider type
	VSphereProviderType ProviderType = "vsphere"
)

// SupportedProviderTypes returns the supported provider types
var SupportedProviderTypes = []ProviderType{
	AWSProviderType,
	AliCloudProviderType,
	AzureActiveDirectoryProviderType,
	AzureCloudStackProviderType,
	AzureProviderType,
	GCPProviderType,
	GoogleWorkpspaceProviderType,
	KubernetesProviderType,
	VSphereProviderType,
	VaultProviderType,
}

// IsSupportedProviderType returns true if the provider type is supported
func IsSupportedProviderType(providerType ProviderType) bool {
	for _, x := range SupportedProviderTypes {
		if x == providerType {
			return true
		}
	}

	return false
}

// SupportedProviderTypeList returns a list of supported provider types
func SupportedProviderTypeList() []string {
	var list []string

	for _, x := range SupportedProviderTypes {
		list = append(list, string(x))
	}

	return list
}

// SourceType is the type of source
type SourceType string

const (
	// SourceSecret is the source type for a secret
	SourceSecret = "secret"
	// SourceInjected indicates the source is pod identity
	SourceInjected = "injected"
)

// ProviderSpec defines the desired state of a provider
// +k8s:openapi-gen=true
type ProviderSpec struct {
	// Configuration is optional configuration to the provider. This is terraform provider specific.
	// +kubebuilder:validation:Optional
	// +kubebuilder:pruning:PreserveUnknownFields
	Configuration *runtime.RawExtension `json:"configuration,omitempty"`
	// ProviderType defines the cloud provider which is being used, currently supported providers are
	// aws, google or azurerm.
	// +kubebuilder:validation:Required
	Provider ProviderType `json:"provider"`
	// SecretRef is a reference to a kubernetes secret. This is required only when using the source: secret.
	// The secret should include the environment variables required to by the terraform provider.
	// +kubebuilder:validation:Optional
	SecretRef *v1.SecretReference `json:"secretRef,omitempty"`
	// Selector provider the ability to filter who can use this provider. If empty, all users
	// in the cluster is permitted to use the provider. Otherrise you can specify a selector
	// which can use namespace and resource labels
	// +kubebuilder:validation:Optional
	Selector *Selector `json:"selector,omitempty"`
	// ServiceAccount is the name of a service account to use when the provider source is 'injected'. The
	// service account should exist in the terraform controller namespace and be configure per cloud vendor
	// requirements for pod identity.
	// +kubebuilder:validation:Optional
	ServiceAccount *string `json:"serviceAccount,omitempty"`
	// Source defines the type of credentials the provider is wrapper, this could be wrapping a static secret
	// or using a managed identity. The currently supported values are secret and injected.
	// +kubebuilder:validation:Required
	Source SourceType `json:"source"`
	// Summary provides a human readable description of the provider
	// +kubebuilder:validation:Optional
	Summary string `json:"summary,omitempty"`
}

// HasConfiguration returns true if the provider has custom configuration
func (p *Provider) HasConfiguration() bool {
	switch {
	case p.Spec.Configuration == nil:
		return false
	case p.Spec.Configuration.Raw == nil, len(p.Spec.Configuration.Raw) <= 0:
		return false
	case bytes.Equal(p.Spec.Configuration.Raw, []byte("{}")):
		return false
	}

	return true
}

// GetConfiguration returns the provider configuration is any
func (p *Provider) GetConfiguration() []byte {
	if !p.HasConfiguration() {
		return nil
	}

	return p.Spec.Configuration.Raw
}

// +kubebuilder:webhook:name=providers.terraform.appvia.io,mutating=false,path=/validate/terraform.appvia.io/providers,verbs=create;update,groups="terraform.appvia.io",resources=providers,versions=v1alpha1,failurePolicy=fail,sideEffects=None,admissionReviewVersions=v1

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// Provider is the schema for provider definitions in terraform controller
// +k8s:openapi-gen=true
// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:path=providers,scope=Cluster,categories={terraform}
// +kubebuilder:printcolumn:name="Source",type="string",JSONPath=".spec.source"
// +kubebuilder:printcolumn:name="Provider",type="string",JSONPath=".spec.provider"
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"
type Provider struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ProviderSpec   `json:"spec,omitempty"`
	Status ProviderStatus `json:"status,omitempty"`
}

// GetNamespacedName returns the namespaced name type
func (p *Provider) GetNamespacedName() types.NamespacedName {
	return types.NamespacedName{Name: p.Name}
}

// ProviderStatus defines the observed state of a provider
// +k8s:openapi-gen=true
type ProviderStatus struct {
	corev1alphav1.CommonStatus `json:",inline"`
}

// GetCommonStatus returns the common status
func (p *Provider) GetCommonStatus() *corev1alphav1.CommonStatus {
	return &p.Status.CommonStatus
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// ProviderList contains a list of providers
type ProviderList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Provider `json:"items"`
}
