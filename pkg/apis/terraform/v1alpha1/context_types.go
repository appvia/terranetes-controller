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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"

	corev1alpha1 "github.com/appvia/terranetes-controller/pkg/apis/core/v1alpha1"
)

// ContextKind is the kind for a Context
const ContextKind = "Context"

const (
	// ContextDescription is the description field name
	ContextDescription = "description"
	// ContextValue is the value field name
	ContextValue = "value"
)

// NewContext creates a new Context
func NewContext(name string) *Context {
	return &Context{
		TypeMeta: metav1.TypeMeta{
			Kind:       ContextKind,
			APIVersion: SchemeGroupVersion.String(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
	}
}

// ContextSpec defines the desired state for a context
// +k8s:openapi-gen=true
type ContextSpec struct {
	// Variables is a list of variables which can be used globally by Context resources.
	// The structure of the variables is a map of key/value pairs, which MUST have both
	// a description and a value.
	// +kubebuilder:validation:Required
	Variables map[string]runtime.RawExtension `json:"variables"`
}

// GetVariableValue returns the string value of the a variable
func (c *ContextSpec) GetVariableValue(name string) ([]byte, bool) {
	if found := c.HasVariable(name); !found {
		return nil, false
	}

	return c.Variables[name].Raw, true
}

// HasVariables returns true if the context has variables defined
func (c *ContextSpec) HasVariables() bool {
	return len(c.Variables) > 0
}

// HasVariable returns true if the context has variables defined
func (c *ContextSpec) HasVariable(name string) bool {
	if !c.HasVariables() {
		return false
	}
	value, found := c.Variables[name]
	if !found {
		return false
	}

	return len(value.Raw) > 0
}

// +kubebuilder:webhook:name=contexts.terraform.appvia.io,mutating=false,path=/validate/terraform.appvia.io/contexts,verbs=create;delete;update,groups="terraform.appvia.io",resources=contexts,versions=v1alpha1,failurePolicy=fail,sideEffects=None,admissionReviewVersions=v1

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// Context is the schema for the context type
// +k8s:openapi-gen=true
// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:path=contexts,scope=Cluster,categories={terraform}
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"
type Context struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ContextSpec   `json:"spec,omitempty"`
	Status ContextStatus `json:"status,omitempty"`
}

// ContextStatus defines the observed state of a terraform
// +k8s:openapi-gen=true
type ContextStatus struct {
	corev1alpha1.CommonStatus `json:",inline"`
}

// GetNamespacedName returns the namespaced resource type
func (c *Context) GetNamespacedName() types.NamespacedName {
	return types.NamespacedName{
		Namespace: c.Namespace,
		Name:      c.Name,
	}
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// ContextList contains a list of contexts
type ContextList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Context `json:"items"`
}
