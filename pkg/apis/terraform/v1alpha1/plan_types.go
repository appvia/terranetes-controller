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
	"k8s.io/apimachinery/pkg/types"

	corev1alpha1 "github.com/appvia/terranetes-controller/pkg/apis/core/v1alpha1"
)

// PlanKind is the kind for a Plan
const PlanKind = "Plan"

// NewPlan creates a new Plan
func NewPlan(name string) *Plan {
	return &Plan{
		TypeMeta: metav1.TypeMeta{
			Kind:       PlanKind,
			APIVersion: SchemeGroupVersion.String(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
	}
}

// PlanRevision is a reference to a revision of a plan existing in the system
type PlanRevision struct {
	// Name is the name of the revision containing the configuration
	//+kubebuilder:validation:Required
	Name string `json:"name"`
	// Revision is the version of the revision
	//+kubebuilder:validation:Required
	Revision string `json:"version"`
}

// PlanSpec defines the desired state for a context
// +k8s:openapi-gen=true
type PlanSpec struct {
	// Revisions is a collection of revision associated with this plan
	Revisions []PlanRevision `json:"revisions,omitempty"`
}

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// Plan is the schema for the plan type
// +k8s:openapi-gen=true
// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:path=plans,scope=Cluster,categories={terraform}
// +kubebuilder:printcolumn:name="Latest",type="string",JSONPath=".status.latest.version"
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"
type Plan struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   PlanSpec   `json:"spec,omitempty"`
	Status PlanStatus `json:"status,omitempty"`
}

// ListRevisions returns a list of revisions
func (c *Plan) ListRevisions() []string {
	var revisions []string

	for _, r := range c.Spec.Revisions {
		revisions = append(revisions, r.Revision)
	}

	return revisions
}

// GetRevision returns the revision with the specified version
func (c *Plan) GetRevision(version string) (PlanRevision, bool) {
	for _, r := range c.Spec.Revisions {
		if r.Revision == version {
			return r, true
		}
	}

	return PlanRevision{}, false
}

// HasRevision returns true if the plan has the specified revision
func (c *Plan) HasRevision(version string) bool {
	for _, x := range c.Spec.Revisions {
		if x.Revision == version {
			return true
		}
	}

	return false
}

// RemoveRevision removes the specified revision from the plan
func (c *Plan) RemoveRevision(version string) {
	var revisions []PlanRevision

	for _, x := range c.Spec.Revisions {
		if x.Revision != version {
			revisions = append(revisions, x)
		}
	}

	c.Spec.Revisions = revisions
}

// PlanStatus defines the observed state of a terraform
// +k8s:openapi-gen=true
type PlanStatus struct {
	corev1alpha1.CommonStatus `json:",inline"`
	// Latest is the latest revision from this plan
	// +kubebuilder:validation:Optional
	Latest PlanRevision `json:"latest,omitempty"`
}

// GetCommonStatus returns the common status
func (c *Plan) GetCommonStatus() *corev1alpha1.CommonStatus {
	return &c.Status.CommonStatus
}

// GetNamespacedName returns the namespaced resource type
func (c *Plan) GetNamespacedName() types.NamespacedName {
	return types.NamespacedName{
		Namespace: c.Namespace,
		Name:      c.Name,
	}
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// PlanList contains a list of plans
type PlanList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Plan `json:"items"`
}
