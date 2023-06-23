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

package v1alpha1

import (
	"bytes"
	"fmt"
	"strconv"
	"time"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"

	corev1alpha1 "github.com/appvia/terranetes-controller/pkg/apis/core/v1alpha1"
)

// CloudResourceKind is the kind for a CloudResource
const CloudResourceKind = "CloudResource"

const (
	// CloudResourceNameLabel is the label used to identify the cloud resource the
	// configuration belongs to
	CloudResourceNameLabel = "terraform.appvia.io/cloud-resource-name"
	// CloudResourcePlanNameLabel is the name of the plan the cloud resource is associated with
	CloudResourcePlanNameLabel = RevisionPlanNameLabel
	// CloudResourceRevisionLabel is the revision version of the cloud resource is
	// associated with
	CloudResourceRevisionLabel = RevisionLabel
	// CloudResourceRevisionNameLabel is the revision name of the cloud resource is
	// associated with
	CloudResourceRevisionNameLabel = RevisionNameLabel
)

// NewCloudResource returns an empty configuration
func NewCloudResource(namespace, name string) *CloudResource {
	return &CloudResource{
		TypeMeta: metav1.TypeMeta{
			Kind:       CloudResourceKind,
			APIVersion: SchemeGroupVersion.String(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      name,
		},
	}
}

// CloudResourceGVK is the GVK for a CloudResource
var CloudResourceGVK = schema.GroupVersionKind{
	Group:   GroupVersion.Group,
	Version: GroupVersion.Version,
	Kind:    CloudResourceKind,
}

// CloudResourceSpec defines the desired state of a terraform
// +k8s:openapi-gen=true
type CloudResourceSpec struct {
	// Auth is used to configure any options required when the source of the terraform
	// module is private or requires credentials to retrieve. This could be SSH keys or git
	// user/pass or AWS credentials for an s3 bucket.
	// +kubebuilder:validation:Optional
	Auth *v1.SecretReference `json:"auth,omitempty"`
	// EnableAutoApproval when enabled indicates the configuration does not need to be
	// manually approved. On a change to the configuration, the controller will automatically
	// approve the configuration. Note it still needs to adhere to any checks or policies.
	// +kubebuilder:validation:Optional
	EnableAutoApproval bool `json:"enableAutoApproval,omitempty"`
	// EnableDriftDetection when enabled run periodic reconciliation configurations looking
	// for any drift between the expected and current state. If any drift is detected the
	// status is changed and a kubernetes event raised.
	EnableDriftDetection bool `json:"enableDriftDetection,omitempty"`
	// Plan is the reference to the plan which this cloud resource is associated with. This
	// field is required, and needs both the name and version the plan revision to use
	// +kubebuilder:validation:Required
	Plan PlanReference `json:"plan"`
	// ProviderRef is the reference to the provider which should be used to execute this
	// configuration.
	// +kubebuilder:validation:Optional
	ProviderRef *ProviderReference `json:"providerRef,omitempty"`
	// WriteConnectionSecretToRef is the name for a secret. On execution of the terraform module
	// any module outputs are written to this secret. The outputs are automatically uppercased
	// and ready to be consumed as environment variables.
	// +kubebuilder:validation:Optional
	// WriteConnectionSecretRef is the secret where the terraform outputs will be written.
	// +kubebuilder:validation:Required
	WriteConnectionSecretToRef *WriteConnectionSecret `json:"writeConnectionSecretToRef,omitempty"`
	// Variables provides the inputs for the terraform module itself. These are passed to the
	// terraform executor and used to execute the plan, apply and destroy phases.
	// +kubebuilder:validation:Optional
	// +kubebuilder:pruning:PreserveUnknownFields
	Variables *runtime.RawExtension `json:"variables,omitempty"`
	// ValueFromSource is a collection of value from sources, where the source of the value
	// is taken from a secret
	// +kubebuilder:validation:Optional
	ValueFrom ValueFromList `json:"valueFrom,omitempty"`
	// TerraformVersion provides the ability to override the default terraform version. Before
	// changing this field its best to consult with platform administrator. As the
	// value of this field is used to change the tag of the terraform container image.
	// +kubebuilder:validation:Optional
	TerraformVersion string `json:"terraformVersion,omitempty"`
}

// HasVariables returns true if the configuration has variables
func (c *CloudResourceSpec) HasVariables() bool {
	switch {
	case c.Variables == nil:
		return false
	case c.Variables.Raw == nil, len(c.Variables.Raw) <= 0:
		return false
	case bytes.Equal(c.Variables.Raw, []byte("{}")):
		return false
	}

	return true
}

// HasValueFrom returns true if the configuration has variables
func (c *CloudResourceSpec) HasValueFrom() bool {
	return len(c.ValueFrom) > 0
}

// +kubebuilder:webhook:name=cloudresources.terraform.appvia.io,mutating=false,path=/validate/terraform.appvia.io/cloudresources,verbs=create;update,groups="terraform.appvia.io",resources=cloudresources,versions=v1alpha1,failurePolicy=fail,sideEffects=None,admissionReviewVersions=v1
// +kubebuilder:webhook:name=cloudresources.terraform.appvia.io,mutating=true,path=/mutate/terraform.appvia.io/cloudresources,verbs=create;update,groups="terraform.appvia.io",resources=cloudresources,versions=v1alpha1,failurePolicy=fail,sideEffects=None,admissionReviewVersions=v1

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// CloudResource is the schema for terraform definitions in terraform controller
// +k8s:openapi-gen=true
// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:path=cloudresources,scope=Namespaced,categories={terraform}
// +kubebuilder:printcolumn:name="Plan",type="string",JSONPath=".spec.plan.name"
// +kubebuilder:printcolumn:name="Revision",type="string",JSONPath=".spec.plan.revision"
// +kubebuilder:printcolumn:name="Secret",type="string",JSONPath=".spec.writeConnectionSecretToRef.name"
// +kubebuilder:printcolumn:name="Configuration",type="string",JSONPath=".status.configurationName"
// +kubebuilder:printcolumn:name="Estimated",type="string",JSONPath=".status.costs.monthly"
// +kubebuilder:printcolumn:name="Update",type="string",JSONPath=".status.updateAvailable"
// +kubebuilder:printcolumn:name="Synchronized",type="string",JSONPath=".status.resourceStatus"
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"
type CloudResource struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   CloudResourceSpec   `json:"spec,omitempty"`
	Status CloudResourceStatus `json:"status,omitempty"`
}

// CloudResourceRevisionStatus defines the observed state of CloudResource
type CloudResourceRevisionStatus struct {
	corev1alpha1.CommonStatus `json:",inline"`
	// Revision is the revision number of the configuration
	// +kubebuilder:validation:Optional
	Revision string `json:"revision,omitempty"`
}

// CloudResourceStatus defines the observed state of a terraform
// +k8s:openapi-gen=true
type CloudResourceStatus struct {
	corev1alpha1.CommonStatus `json:",inline"`
	// ConfigurationName is the of the configuration this cloudresource is managing on behalf of
	// +kubebuilder:validation:Optional
	ConfigurationName string `json:"configurationName,omitempty"`
	// Configuration is the state taken from the underlying configuration
	// +kubebuilder:validation:Optional
	ConfigurationStatus ConfigurationStatus `json:"configurationStatus,omitempty"`
	// Costs is the predicted costs of this configuration. Note this field is only populated
	// when the integration has been configured by the administrator.
	// +kubebuilder:validation:Optional
	Costs *CostStatus `json:"costs,omitempty"`
	// Resources is the number of managed cloud resources which are currently under management.
	// This field is taken from the terraform state itself.
	// +kubebuilder:validation:Optional
	Resources int `json:"resources,omitempty"`
	// ResourceStatus indicates the status of the resources and if the resources are insync with the
	// configuration
	// +kubebuilder:validation:Optional
	ResourceStatus ResourceStatus `json:"resourceStatus,omitempty"`
	// UpdateAvailable indicates if there is a new version of the plan available
	// +kubebuilder:validation:Optional
	UpdateAvailable string `json:"updateAvailable,omitempty"`
}

// GetNamespacedName returns the namespaced resource type
func (c *CloudResource) GetNamespacedName() types.NamespacedName {
	return types.NamespacedName{
		Namespace: c.Namespace,
		Name:      c.Name,
	}
}

// GetCommonStatus returns the common status
func (c *CloudResource) GetCommonStatus() *corev1alpha1.CommonStatus {
	return &c.Status.CommonStatus
}

// HasRetryableAnnotation returns true if the configuration has the retryable annotation
func (c *CloudResource) HasRetryableAnnotation() bool {
	if c.Annotations == nil {
		return false
	}
	_, found := c.Annotations[RetryAnnotation]

	return found
}

// IsRetryableValid returns true if the retryable annotation is valid
func (c *CloudResource) IsRetryableValid() bool {
	if c.Annotations == nil {
		return false
	}
	retryable, found := c.Annotations[RetryAnnotation]
	if !found {
		return false
	}
	_, err := strconv.ParseInt(retryable, 10, 64)

	return err == nil
}

// IsRetryable returns true if the configuration is in a state where it can be retried
func (c *CloudResource) IsRetryable() bool {
	switch {
	case c.Annotations == nil:
		return false
	case c.Annotations[RetryAnnotation] == "":
		return false
	case c.Status.LastReconcile == nil:
		return false
	case c.Status.LastReconcile.Time.IsZero():
		return false
	}

	// @step: we need to parse the unix timestamp
	timestamp, err := strconv.ParseInt(c.Annotations[RetryAnnotation], 10, 64)
	if err != nil {
		return false
	}
	tm := time.Unix(timestamp, 0)

	return tm.After(c.Status.LastReconcile.Time.Time)
}

// HasApproval returns true if the configuration has an approval
func (c *CloudResource) HasApproval() bool {
	return c.GetAnnotations()[ApplyAnnotation] == "true"
}

// NeedsApproval returns true if the configuration needs approval
func (c *CloudResource) NeedsApproval() bool {
	return c.GetAnnotations()[ApplyAnnotation] == "false"
}

// GetTerraformConfigSecretName returns the name of the configuration secret
func (c *CloudResource) GetTerraformConfigSecretName() string {
	return fmt.Sprintf("config-%s", string(c.GetUID()))
}

// GetTerraformStateSecretName returns the name of the secret holding the terraform state
func (c *CloudResource) GetTerraformStateSecretName() string {
	return fmt.Sprintf("tfstate-default-%s", string(c.GetUID()))
}

// GetTerraformPolicySecretName returns the name of the secret holding the terraform state
func (c *CloudResource) GetTerraformPolicySecretName() string {
	return fmt.Sprintf("policy-%s", string(c.GetUID()))
}

// GetTerraformCostSecretName returns the name which should be used for the costs report
func (c *CloudResource) GetTerraformCostSecretName() string {
	return fmt.Sprintf("costs-%s", string(c.GetUID()))
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// CloudResourceList contains a list of cloudresources
type CloudResourceList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []CloudResource `json:"items"`
}
