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

package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ConditionType defines a type of a condition in PascalCase or in foo.example.com/PascalCase
// ---
// Many .condition.type values are consistent across resources like Available, but because arbitrary
// conditions can be useful (see .node.status.conditions), the ability to deconflict is important.
// The regex it matches is (dns1123SubdomainFmt/)?(qualifiedNameFmt)
// +kubebuilder:validation:Pattern=`^([a-z0-9]([-a-z0-9]*[a-z0-9])?(\.[a-z0-9]([-a-z0-9]*[a-z0-9])?)*/)?(([A-Za-z0-9][-A-Za-z0-9_.]*)?[A-Za-z0-9])$`
// +kubebuilder:validation:MaxLength=316
type ConditionType string

const (
	// ConditionReady describes the overall status of the resource. All Wayfinder resources should
	// set ConditionReady
	ConditionReady ConditionType = "Ready"
)

const (
	// ReasonNotDetermined is the default reason when a condition's state has not yet been
	// determined by the controller
	ReasonNotDetermined = "NotDetermined"
	// ReasonWarning should be used as a reason whenever an unexpected error has caused the
	// condition to be in a non-desired state
	ReasonWarning = "Warning"
	// ReasonError should be used as a reason whenever an unexpected error has caused the
	// condition to be in a non-desired state
	ReasonError = "Error"
	// ReasonInProgress should be used as a reason whenever a condition status is caused
	// by an operation being in progress, e.g. deploying, upgrading, whatever.
	ReasonInProgress = "InProgress"
	// ReasonReady should be used as a reason whenever a condition status indicates that
	// some element is now ready for use and available
	ReasonReady = "Ready"
	// ReasonDisabled indicated the feature or options behind this condition is currently
	// disabled
	ReasonDisabled = "Disabled"
	// ReasonComplete should be used as a reason whenever a concrete process represented by a
	// condition is complete.
	ReasonComplete = "Complete"
	// ReasonActionRequired should be used as a reason whenever a condition is in the state it is
	// in due to needing some sort of user or administrator action to resolve it
	ReasonActionRequired = "ActionRequired"
	// ReasonDeleting should be used to indicate the thing represented by this condition is
	// currently in the process of being deleted
	ReasonDeleting = "Deleting"
	// ReasonErrorDeleting should be used as a reason whenever an unexpected error has caused the
	// condition to be in a non-desired state **while deleting**
	ReasonErrorDeleting = "ErrorDeleting"
	// ReasonDeleted should be used to indicate the thing represented by this condition has been
	// deleted
	ReasonDeleted = "Deleted"
)

// ConditionSpec describes the shape of a condition which will be populated onto the status
type ConditionSpec struct {
	// The PascalCase condition type, e.g. ServiceAvailable or InsufficientCapacity.
	// See ConditionType for the rules on condition types.
	Type ConditionType
	// Name is a human-readable name for this condition, used for UI and CLI reporting / explanation
	// If Name is empty, the Type will be used also as the Name.
	Name string
	// DefaultStatus is the default status - if unset, metav1.ConditionUnknown will be used.
	DefaultStatus metav1.ConditionStatus
}

// Condition is the current observed condition of some aspect of a resource
// +k8s:openapi-gen=true
type Condition struct {
	// The first several fields here follow the standard used in metav1.Condition:

	// Type of condition in CamelCase or in foo.example.com/CamelCase.
	// ---
	// Many .condition.type values are consistent across resources like Available, but because arbitrary conditions can be
	// useful (see .node.status.conditions), the ability to deconflict is important.
	// The regex it matches is (dns1123SubdomainFmt/)?(qualifiedNameFmt)
	// +required
	// +kubebuilder:validation:Required
	Type ConditionType `json:"type"`
	// Status of the condition, one of True, False, Unknown.
	// +required
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:Enum=True;False;Unknown
	Status metav1.ConditionStatus `json:"status"`
	// ObservedGeneration represents the .metadata.generation that the condition was set based upon.
	// For instance, if .metadata.generation is currently 12, but the .status.conditions[x].observedGeneration is 9, the condition is out of date
	// with respect to the current state of the instance.
	// +optional
	// +kubebuilder:validation:Minimum=0
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`
	// LastTransitionTime is the last time the condition transitioned from one status to another.
	// This should be when the underlying condition changed.  If that is not known, then using the time when the API field changed is acceptable.
	// +required
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:Type=string
	// +kubebuilder:validation:Format=date-time
	LastTransitionTime metav1.Time `json:"lastTransitionTime,omitempty"`
	// Reason contains a programmatic identifier indicating the reason for the condition's last transition.
	// Producers of specific condition types may define expected values and meanings for this field,
	// and whether the values are considered a guaranteed API.
	// The value should be a CamelCase string.
	// This field may not be empty.
	// +required
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MaxLength=1024
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:Pattern=`^[A-Za-z]([A-Za-z0-9_,:]*[A-Za-z0-9_])?$`
	Reason string `json:"reason"`
	// Message is a human readable message indicating details about the transition.
	// This may be an empty string.
	// +optional
	// +kubebuilder:validation:MaxLength=32768
	Message string `json:"message,omitempty"`
	// Name is a human-readable name for this condition.
	// +required
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinLength=1
	Name string `json:"name"`
	// Detail is any additional human-readable detail to understand this condition, for example,
	// the full underlying error which caused an issue
	// +optional
	Detail string `json:"detail,omitempty"`
}

// IsComplete returns true if the resource complete for a specific generation
func (c *Condition) IsComplete(generation int64) bool {
	return c.Status == metav1.ConditionTrue && c.ObservedGeneration == generation
}

// IsDeleting returns true if the condition is in status false and has a deleting/deleted reason
// (i.e. deleting, deleted or error deleting)
func (c *Condition) IsDeleting() bool {
	if c == nil {
		return false
	}
	return c.Status == metav1.ConditionFalse && (c.Reason == ReasonDeleting || c.Reason == ReasonDeleted || c.Reason == ReasonErrorDeleting)
}

// Conditions is a collection of condition
type Conditions []Condition

// GetCondition returns the current observed status of a specific element of this resource, or
// nil if the condition does not exist
func (s *CommonStatus) GetCondition(typ ConditionType) *Condition {
	for i := range s.Conditions {
		if s.Conditions[i].Type == typ {
			return &s.Conditions[i]
		}
	}
	return nil
}

// IsComplete returns true if the condition is complete for a specific generation
func (s *CommonStatus) IsComplete(condition ConditionType, generation int64) bool {
	cond := s.GetCommonStatus().GetCondition(condition)
	if cond == nil {
		return false
	}

	return cond.ObservedGeneration == generation && cond.Status == metav1.ConditionTrue
}

// InCondition returns true if the condition specified by typ is present and set to its true
// state (i.e. metav1.ConditionTrue for a normal condition or metav1.ConditionFalse for a negative
// polarity condition)
func (s *CommonStatus) InCondition(typ ConditionType) bool {
	c := s.GetCondition(typ)

	if c == nil {
		return false
	}

	return c.Status == metav1.ConditionTrue
}

// GetConditions returns the status of any sub-components of this resource
func (s *CommonStatus) GetConditions() Conditions {
	return s.Conditions
}
