/*
 * Copyright (C) 2022 Rohith Jayawardene <gambol99@gmail.com>
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
	"fmt"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"

	corev1alphav1 "github.com/appvia/terraform-controller/pkg/apis/core/v1alpha1"
)

// ConfigurationKind is the kind for a Configuration
const ConfigurationKind = "Configuration"

const (
	// ApplyAnnotation is the annotation used to mark a resource as a plan rather than apply
	ApplyAnnotation = "terraform.appvia.io/apply"
	// OrphanAnnotation is the label used to orphan a configuration
	OrphanAnnotation = "terraform.appvia.io/orphan"
)

const (
	// TerraformBackendConfigMapKey is the key name for the terraform backend in the configmap
	TerraformBackendConfigMapKey = "backend.tf"
	// TerraformVariablesConfigMapKey is the key name for the terraform variables in the configmap
	TerraformVariablesConfigMapKey = "variables.tfvars.json"
	// TerraformProviderConfigMapKey is the key name for the terraform variables in the configmap
	TerraformProviderConfigMapKey = "provider.tf"
)

const (
	// ConfigurationGenerationLabel is the label used to identify a configuration generation
	ConfigurationGenerationLabel = "terraform.appvia.io/generation"
	// ConfigurationNameLabel is the label used to identify a configuration
	ConfigurationNameLabel = "terraform.appvia.io/configuration"
	// ConfigurationUIDLabel is the uid of the configuration
	ConfigurationUIDLabel = "terraform.appvia.io/configuration-uid"
	// ConfigurationNamespaceLabel is the label used to identify a configuration namespace
	ConfigurationNamespaceLabel = "terraform.appvia.io/namespace"
	// ConfigurationStageLabel is the label used to identify a configuration stage
	ConfigurationStageLabel = "terraform.appvia.io/stage"
)

const (
	// StageTerraformApply is the stage for a terraform apply
	StageTerraformApply = "apply"
	// StageTerraformDestroy is the stage for a terraform destroy
	StageTerraformDestroy = "destroy"
	// StageTerraformPlan is the stage for a terraform plan
	StageTerraformPlan = "plan"
	// StageTerraformVerify is the stage for a verify
	StageTerraformVerify = "verify"
)

// ConfigurationGVK is the GVK for a Configuration
var ConfigurationGVK = schema.GroupVersionKind{
	Group:   GroupVersion.Group,
	Version: GroupVersion.Version,
	Kind:    ConfigurationKind,
}

// ProviderReference is the reference to the provider which is used to create
// the configuration
type ProviderReference struct {
	// Name is the name of the provider
	Name string `json:"name,omitempty"`
	// Namespace is the namespace of the provider
	Namespace string `json:"namespace,omitempty"`
}

// AdditionalResource defines an additional resource to be created
type AdditionalResource struct {
	// Name is the name of the resource
	Name string `json:"name,omitempty"`
	// Type is the type of the resource
	Type string `json:"type,omitempty"`
	// Source is the source of the resource
	Source string `json:"source,omitempty"`
}

// ConfigurationSpec defines the desired state of a terraform
// +k8s:openapi-gen=true
type ConfigurationSpec struct {
	// EnableAutoApproval indicates the apply it automatically approved
	// +kubebuilder:validation:Optional
	EnableAutoApproval bool `json:"enableAutoApproval,omitempty"`
	// Module is the location of the module to use for the configuration
	// +kubebuilder:validation:Required
	Module string `json:"module"`
	// ProviderRef is the reference to the provider
	// +kubebuilder:validation:Required
	ProviderRef *ProviderReference `json:"providerRef"`
	// WriteConnectionSecretToRef is the name of the secret where the terraform
	// configuration is stored
	// +kubebuilder:validation:Optional
	WriteConnectionSecretToRef *v1.SecretReference `json:"writeConnectionSecretToRef,omitempty"`
	// Variables are the variables that are used to configure the terraform
	// +kubebuilder:validation:Optional
	// +kubebuilder:pruning:PreserveUnknownFields
	Variables *runtime.RawExtension `json:"variables,omitempty"`
}

// +kubebuilder:webhook:name=configurations.terraform.appvia.io,mutating=false,path=/validate/terraform.appvia.io/configurations,verbs=create;update,groups="terraform.appvia.io",resources=configurations,versions=v1alpha1,failurePolicy=fail,sideEffects=None,admissionReviewVersions=v1
// +kubebuilder:webhook:name=configurations.terraform.appvia.io,mutating=true,path=/mutate/terraform.appvia.io/configurations,verbs=create;update,groups="terraform.appvia.io",resources=configurations,versions=v1alpha1,failurePolicy=fail,sideEffects=None,admissionReviewVersions=v1

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// Configuration is the schema for terraform definitions in terraform controller
// +k8s:openapi-gen=true
// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Module",type="string",JSONPath=".spec.module"
// +kubebuilder:printcolumn:name="Secret",type="string",JSONPath=".spec.writeConnectionSecretToRef.name"
// +kubebuilder:printcolumn:name="Resources",type="string",JSONPath=".status.resources"
// +kubebuilder:printcolumn:name="Estimated",type="string",JSONPath=".status.costs.monthly"
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"
type Configuration struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ConfigurationSpec   `json:"spec,omitempty"`
	Status ConfigurationStatus `json:"status,omitempty"`
}

// CostStatus defines the cost status of a configuration
type CostStatus struct {
	// Enabled indicates if the cost is enabled
	// +kubebuilder:validation:Optional
	Enabled bool `json:"enabled,omitempty"`
	// Hourly is the hourly estimated cost of the configuration
	// +kubebuilder:validation:Optional
	Hourly string `json:"hourly,omitempty"`
	// Monthly is the monthly estimated cost of the configuration
	// +kubebuilder:validation:Optional
	Monthly string `json:"monthly,omitempty"`
}

// ConfigurationStatus defines the observed state of a terraform
// +k8s:openapi-gen=true
type ConfigurationStatus struct {
	corev1alphav1.CommonStatus `json:",inline"`
	// Costs is the cost status of the configuration is enabled via the controller
	// +kubebuilder:validation:Optional
	Costs *CostStatus `json:"costs,omitempty"`
	// Resources is the number of managed resources created by this configuration
	// +kubebuilder:validation:Optional
	Resources int `json:"resources,omitempty"`
}

// GetNamespacedName returns the namespaced resource type
func (c *Configuration) GetNamespacedName() types.NamespacedName {
	return types.NamespacedName{
		Namespace: c.Namespace,
		Name:      c.Name,
	}
}

// HasVariables returns true if the configuration has variables
func (c *Configuration) HasVariables() bool {
	switch {
	case c.Spec.Variables == nil:
		return false
	case c.Spec.Variables.Raw == nil, len(c.Spec.Variables.Raw) <= 0:
		return false
	case bytes.Compare(c.Spec.Variables.Raw, []byte("{}")) == 0:
		return false
	}

	return true
}

// HasApproval returns true if the configuration has an approval
func (c *Configuration) HasApproval() bool {
	return c.GetAnnotations()[ApplyAnnotation] == "true"
}

// NeedsApproval returns true if the configuration needs approval
func (c *Configuration) NeedsApproval() bool {
	return c.GetAnnotations()[ApplyAnnotation] == "false"
}

// GetTerraformStateSecretName returns the name of the secret holding the terraform state
func (c *Configuration) GetTerraformStateSecretName() string {
	return fmt.Sprintf("tfstate-default-%s", string(c.GetUID()))
}

// GetTerraformCostSecretName returns the name which should be used for the costs report
func (c *Configuration) GetTerraformCostSecretName() string {
	return fmt.Sprintf("costs-%s", string(c.GetUID()))
}

// GetCommonStatus returns the common status
func (c *Configuration) GetCommonStatus() *corev1alphav1.CommonStatus {
	return &c.Status.CommonStatus
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// ConfigurationList contains a list of configurations
type ConfigurationList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Configuration `json:"items"`
}
