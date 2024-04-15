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
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"

	corev1alpha1 "github.com/appvia/terranetes-controller/pkg/apis/core/v1alpha1"
)

// ConfigurationKind is the kind for a Configuration
const ConfigurationKind = "Configuration"

// NewConfiguration returns an empty configuration
func NewConfiguration(namespace, name string) *Configuration {
	return &Configuration{
		TypeMeta: metav1.TypeMeta{
			Kind:       ConfigurationKind,
			APIVersion: SchemeGroupVersion.String(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      name,
		},
	}
}

// ValueFromList is a list of value from sources
type ValueFromList []ValueFromSource

// HasContextReferences returns true if the configuration has context references
func (v *ValueFromList) HasContextReferences() bool {
	for _, x := range *v {
		if x.Context != nil {
			return true
		}
	}

	return false
}

// HasSecretReferences returns true if the configuration has secret references
func (v *ValueFromList) HasSecretReferences() bool {
	for _, x := range *v {
		if x.Secret != nil {
			return true
		}
	}

	return false
}

const (
	// ApplyAnnotation is the annotation used to mark a resource as a plan rather than apply
	ApplyAnnotation = "terraform.appvia.io/apply"
	// DriftAnnotation is the annotation used to mark a resource for drift detection
	DriftAnnotation = "terraform.appvia.io/drift"
	// ReconcileAnnotation is the label used control reconciliation
	ReconcileAnnotation = "terraform.appvia.io/reconcile"
	// RetryAnnotation is the annotation used to mark a resource for retry
	RetryAnnotation = "terraform.appvia.io/retry"
	// OrphanAnnotation is the label used to orphan a configuration
	OrphanAnnotation = "terraform.appvia.io/orphan"
	// VersionAnnotation is the label used to hold the version
	VersionAnnotation = "terraform.appvia.io/version"
)

const (
	// TerraformStateSecretKey is the key used by the terraform state secret
	TerraformStateSecretKey = "tfstate"

	// TerraformPlanJSONSecretKey is the key used by the terraform plan JSON secret
	TerraformPlanJSONSecretKey = "plan.json"
)

const (
	// CheckovJobTemplateConfigMapKey is the key name for the job template in the configmap
	CheckovJobTemplateConfigMapKey = "checkov.yaml"
	// TerraformBackendSecretKey is the key name for the terraform backend in the secret
	TerraformBackendSecretKey = "backend.tf"
	// TerraformVariablesConfigMapKey is the key name for the terraform variables in the configmap
	TerraformVariablesConfigMapKey = "variables.tfvars.json"
	// TerraformProviderConfigMapKey is the key name for the terraform variables in the configmap
	TerraformProviderConfigMapKey = "provider.tf"
	// TerraformJobTemplateConfigMapKey is the key name for the job template in the configmap
	TerraformJobTemplateConfigMapKey = "job.yaml"
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
	// ConfigurationPlanLabel is the label which contains the plan name for a configuration
	ConfigurationPlanLabel = RevisionPlanNameLabel
	// ConfigurationRevisionLabelName is the name of the revision being used
	ConfigurationRevisionLabelName = RevisionLabel
	// ConfigurationRevisionVersion is the version of the revision
	ConfigurationRevisionVersion = "terranetes.appvia.io/revision-version"
)

const (
	// JobPlanIDLabel is a label used on the apply Job and holds the timestamp
	// of when the plan was generated.
	JobPlanIDLabel = "terraform.appvia.io/plan-id"
	// JobTemplateHashLabel is the label used to hold a hash of the current Job
	// template, this allows re-running the plan Jobs when the template changes.
	JobTemplateHashLabel = "terraform.appvia.io/template-hash"
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
	// Name is the name of the provider which contains the credentials to use for this
	// configuration.
	// +kubebuilder:validation:Required
	Name string `json:"name"`
	// Namespace is the namespace of the provider itself.
	// +kubebuilder:validation:Optional
	// +kubebuilder:deprecatedversion:warning="namespace is a deprecated field for provider references"
	Namespace string `json:"namespace,omitempty"`
}

// IsValid returns an error if the provider is invalid
func (p *ProviderReference) IsValid() error {
	if p.Name == "" {
		return errors.New("spec.providerRef.name is required")
	}

	return nil
}

// WriteConnectionSecret defines the options around the secret produced by the terraform code
type WriteConnectionSecret struct {
	// Name is the of the secret where you want to the terraform output to be written. The terraform outputs
	// will be written to the secret as a key value pair. All are uppercased can read to be consumed by the
	// workload.
	// +kubebuilder:validation:Required
	Name string `json:"name"`
	// Keys is a collection of name used to filter the terraform output. By default all keys from the
	// output of the terraform state are written to the connection secret. Here we can define exactly
	// which keys we want from that output.
	// +kubebuilder:validation:Optional
	Keys []string `json:"keys,omitempty"`
}

// IsValid checks if the write connection secret is valid
func (w *WriteConnectionSecret) IsValid() error {
	if w.Name == "" {
		return errors.New("spec.writeConnectionSecretToRef.name is required")
	}

	for i, key := range w.Keys {
		if strings.Contains(key, ":") && len(strings.Split(key, ":")) != 2 {
			return fmt.Errorf("spec.writeConnectionSecretToRef.keys[%d] contains invalid key: %s, should be KEY:NEWNAME", i, key)
		}
	}

	return nil
}

// HasKeys returns true if the keys are not empty
func (w *WriteConnectionSecret) HasKeys() bool {
	return len(w.Keys) > 0
}

// AddKey adds a key to the list
func (w *WriteConnectionSecret) AddKey(key, override string) {
	name := key
	if override != "" {
		name = fmt.Sprintf("%s:%s", key, strings.ToUpper(override))
	}

	// @step: ensure we have no duplicates
	var list []string
	for _, x := range w.Keys {
		if !strings.HasPrefix(x, key+":") && x != key {
			list = append(list, x)
		}
	}
	list = append(list, name)

	w.Keys = list
}

// KeysMap returns the map of keys to name
func (w *WriteConnectionSecret) KeysMap() (map[string]string, error) {
	if !w.HasKeys() {
		return nil, nil
	}

	keys := make(map[string]string)

	for _, x := range w.Keys {
		switch {
		case strings.Contains(x, ":"):
			items := strings.Split(x, ":")
			if len(items) != 2 {
				return nil, fmt.Errorf("invalid key format %s, should be KEY:NEWNAME", x)
			}
			keys[items[0]] = items[1]

		default:
			keys[x] = x
		}
	}

	return keys, nil
}

// ValueFromSource defines a value which is taken from a secret
type ValueFromSource struct {
	// Context is the context is the name of the terraform context where the
	// value should be retrieved from
	Context *string `json:"context,omitempty"`
	// Optional indicates the secret can be optional, i.e if the secret does not exist, or the key is
	// not contained in the secret, we ignore the error
	// +kubebuilder:validation:Optional
	Optional bool `json:"optional,omitempty"`
	// Key is the key in the secret which we should used for the value
	// +kubebuilder:validation:Required
	Key string `json:"key"`
	// Name is the name which we use when injecting the value into the terraform code
	// i.e. the secret may contain data.DB_HOST but you call this database_hostname. Note,
	// for backwards compatiability if no name is provided, we using the key at the name
	Name string `json:"name,omitempty"`
	// Secret is the name of the secret in the configuration namespace
	// +kubebuilder:validation:Optional
	Secret *string `json:"secret,omitempty"`
}

// IsValid checks if all the value from are valid, else returns an error
func (v *ValueFromSource) IsValid(path string) error {
	switch {
	case v.Context == nil && v.Secret == nil:
		return fmt.Errorf("%s requires either context or secret", path)

	case v.Context != nil && v.Secret != nil:
		return fmt.Errorf("%s requires either context or secret, not both", path)
	}

	if v.Secret != nil {
		switch {
		case v.Name == "":
			return fmt.Errorf("%s requires a name to be set", path)
		}
	}
	if v.Context != nil {
		switch {
		case v.Key == "":
			return fmt.Errorf("%s requires a key", path)
		}
	}

	return nil
}

// IsValid checks the value from source is valid, else returns an error
func (v *ValueFromList) IsValid() error {
	for i, x := range *v {
		if err := x.IsValid(fmt.Sprintf("spec.valueFrom[%d]", i)); err != nil {
			return err
		}
	}

	return nil
}

// GetName returns the name or the key if not set
func (v *ValueFromSource) GetName() string {
	if len(v.Name) == 0 {
		return v.Key
	}

	return v.Name
}

// PlanReference are the fields related to a configuration plan
type PlanReference struct {
	// Name is the name of the plan this configuration is associated with
	// +kubebuilder:validation:Required
	Name string `json:"name"`
	// Revision is the revision of the plan this configuration is associated with
	// +kubebuilder:validation:Required
	Revision string `json:"revision"`
}

// IsValid returns an error if the plan reference is not valid
func (p *PlanReference) IsValid() error {
	switch {
	case p.Name == "":
		return errors.New("spec.plan.name is required")
	case p.Revision == "":
		return errors.New("spec.plan.revision is required")
	}

	return nil
}

// ConfigurationSpec defines the desired state of a terraform
// +k8s:openapi-gen=true
type ConfigurationSpec struct {
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
	// Module is the URL to the source of the terraform module. The format of the URL is
	// a direct implementation of terraform's module reference. Please see the following
	// repository for more details https://github.com/hashicorp/go-getter
	// +kubebuilder:validation:Required
	Module string `json:"module"`
	// Plan is an optional reference to a plan this configuration is associated with. If
	// not set and a policy exists to enforce a plan, the configuration will be rejected.
	// +kubebuilder:validation:Optional
	Plan *PlanReference `json:"plan,omitempty"`
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

// +kubebuilder:webhook:name=configurations.terraform.appvia.io,mutating=false,path=/validate/terraform.appvia.io/configurations,verbs=create;update,groups="terraform.appvia.io",resources=configurations,versions=v1alpha1,failurePolicy=fail,sideEffects=None,admissionReviewVersions=v1
// +kubebuilder:webhook:name=configurations.terraform.appvia.io,mutating=true,path=/mutate/terraform.appvia.io/configurations,verbs=create;update,groups="terraform.appvia.io",resources=configurations,versions=v1alpha1,failurePolicy=fail,sideEffects=None,admissionReviewVersions=v1

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// Configuration is the schema for terraform definitions in terraform controller
// +k8s:openapi-gen=true
// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Module",type="string",JSONPath=".spec.module"
// +kubebuilder:printcolumn:name="Plan",type="string",JSONPath=".spec.plan.name",priority=1
// +kubebuilder:printcolumn:name="Secret",type="string",JSONPath=".spec.writeConnectionSecretToRef.name"
// +kubebuilder:printcolumn:name="Drift Detection",type="boolean",JSONPath=".spec.enableDriftDetection"
// +kubebuilder:printcolumn:name="Estimated",type="string",JSONPath=".status.costs.monthly"
// +kubebuilder:printcolumn:name="Synchronized",type="string",JSONPath=".status.resourceStatus"
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"
type Configuration struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ConfigurationSpec   `json:"spec,omitempty"`
	Status ConfigurationStatus `json:"status,omitempty"`
}

// CostStatus defines the cost status of a configuration
type CostStatus struct {
	// Enabled indicates if the cost integration was enabled when this configuration was last
	// executed.
	// +kubebuilder:validation:Optional
	Enabled bool `json:"enabled,omitempty"`
	// Hourly is the hourly estimated cost of the configuration
	// +kubebuilder:validation:Optional
	Hourly string `json:"hourly,omitempty"`
	// Monthly is the monthly estimated cost of the configuration
	// +kubebuilder:validation:Optional
	Monthly string `json:"monthly,omitempty"`
}

// ResourceStatus is the status of the resources
type ResourceStatus string

const (
	// ResourcesInSync is the status when the configuration is in sync
	ResourcesInSync ResourceStatus = "InSync"
	// ResourcesOutOfSync is the status when the configuration is out of sync
	ResourcesOutOfSync ResourceStatus = "OutOfSync"
	// DestroyingResources is the status when the configuration is being destroyed
	DestroyingResources ResourceStatus = "Deleting"
	// DestroyingResourcesFailed is the status when the configuration is being destroyed and failed
	DestroyingResourcesFailed ResourceStatus = "DeletionFailed"
	// UnknownResourceStatus is the status when the configuration is unknown
	UnknownResourceStatus ResourceStatus = ""
)

// ConfigurationRevisionStatus defines the observed state of Configuration
type ConfigurationRevisionStatus struct {
	// Revision is the revision number of the configuration
	// +kubebuilder:validation:Optional
	Revision string `json:"revision,omitempty"`
}

// ConfigurationStatus defines the observed state of a terraform
// +k8s:openapi-gen=true
type ConfigurationStatus struct {
	corev1alpha1.CommonStatus `json:",inline"`
	// Costs is the predicted costs of this configuration. Note this field is only populated
	// when the integration has been configured by the administrator.
	// +kubebuilder:validation:Optional
	Costs *CostStatus `json:"costs,omitempty"`
	// DriftTimestamp is the timestamp of the last drift detection
	// +kubebuilder:validation:Optional
	DriftTimestamp string `json:"driftTimestamp,omitempty"`
	// Resources is the number of managed cloud resources which are currently under management.
	// This field is taken from the terraform state itself.
	// +kubebuilder:validation:Optional
	Resources *int `json:"resources,omitempty"`
	// ResourceStatus indicates the status of the resources and if the resources are insync with the
	// configuration
	ResourceStatus ResourceStatus `json:"resourceStatus,omitempty"`
	// TerraformVersion is the version of terraform which was last used to run this
	// configuration
	// +kubebuilder:validation:Optional
	TerraformVersion string `json:"terraformVersion,omitempty"`
}

// GetNamespacedName returns the namespaced resource type
func (c *Configuration) GetNamespacedName() types.NamespacedName {
	return types.NamespacedName{
		Namespace: c.Namespace,
		Name:      c.Name,
	}
}

// GetVariables returns the variables for the configuration
func (c *ConfigurationSpec) GetVariables() (map[string]interface{}, error) {
	if !c.HasVariables() {
		return map[string]interface{}{}, nil
	}

	values := make(map[string]interface{})
	if err := json.NewDecoder(bytes.NewReader(c.Variables.Raw)).Decode(&values); err != nil {
		return nil, err
	}

	return values, nil
}

// HasValueFrom returns true if the configuration has a value from
func (c *ConfigurationSpec) HasValueFrom() bool {
	return len(c.ValueFrom) > 0
}

// HasVariables returns true if the configuration has variables
func (c *ConfigurationSpec) HasVariables() bool {
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

// HasRetryableAnnotation returns true if the configuration has the retryable annotation
func (c *Configuration) HasRetryableAnnotation() bool {
	if c.Annotations == nil {
		return false
	}
	_, found := c.Annotations[RetryAnnotation]

	return found
}

// IsRetryableValid returns true if the retryable annotation is valid
func (c *Configuration) IsRetryableValid() bool {
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
func (c *Configuration) IsRetryable() bool {
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
func (c *Configuration) HasApproval() bool {
	return c.GetAnnotations()[ApplyAnnotation] == "true"
}

// NeedsApproval returns true if the configuration needs approval
func (c *Configuration) NeedsApproval() bool {
	return c.GetAnnotations()[ApplyAnnotation] == "false"
}

// IsManaged returns true if the configuration is managed
func (c *Configuration) IsManaged() bool {
	switch {
	case c.Spec.Plan == nil:
		return false
	case len(c.OwnerReferences) == 0:
		return false
	case c.GetLabels()[CloudResourceNameLabel] == "":
		return false
	}

	return true
}

// GetTerraformConfigSecretName returns the name of the configuration secret
func (c *Configuration) GetTerraformConfigSecretName() string {
	return fmt.Sprintf("config-%s", string(c.GetUID()))
}

// GetTerraformStateSecretName returns the name of the secret holding the terraform state
func (c *Configuration) GetTerraformStateSecretName() string {
	return fmt.Sprintf("tfstate-default-%s", string(c.GetUID()))
}

// GetTerraformPolicySecretName returns the name of the secret holding the terraform state
func (c *Configuration) GetTerraformPolicySecretName() string {
	return fmt.Sprintf("policy-%s", string(c.GetUID()))
}

// GetTerraformCostSecretName returns the name which should be used for the costs report
func (c *Configuration) GetTerraformCostSecretName() string {
	return fmt.Sprintf("costs-%s", string(c.GetUID()))
}

// GetTerraformPlanOutSecretName returns the name of the secret holding the
// terraform plan binary output
func (c *Configuration) GetTerraformPlanOutSecretName() string {
	return fmt.Sprintf("tfplan-out-%s", string(c.GetUID()))
}

// GetTerraformPlanJSONSecretName returns the name of the secret holding the
// terraform plan JSON
func (c *Configuration) GetTerraformPlanJSONSecretName() string {
	return fmt.Sprintf("tfplan-json-%s", string(c.GetUID()))
}

// GetCommonStatus returns the common status
func (c *Configuration) GetCommonStatus() *corev1alpha1.CommonStatus {
	return &c.Status.CommonStatus
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// ConfigurationList contains a list of configurations
type ConfigurationList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Configuration `json:"items"`
}
