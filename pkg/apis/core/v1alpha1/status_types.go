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

// Status is the status of a thing
import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// Status is the status of a thing
type Status string

const (
	// DeletingStatus indicates we are deleting the resource
	DeletingStatus Status = "Deleting"
	// DeleteErrorStatus indicates an error has occurred while attempting to delete the resource
	DeleteErrorStatus Status = "DeleteError"
	// DeletedStatus indicates a deleted entity
	DeletedStatus Status = "Deleted"
	// DeleteFailedStatus indicates that deleting the entity failed
	DeleteFailedStatus Status = "DeleteFailed"
	// ErrorStatus indicates that a recoverable error happened
	ErrorStatus Status = "Error"
	// PendingStatus indicate we are waiting
	PendingStatus Status = "Pending"
	// SuccessStatus is a successful resource
	SuccessStatus Status = "Success"
	// FailureStatus indicates the resource has failed for one or more reasons
	FailureStatus Status = "Failure"
	// WarningStatus indicates are warning
	WarningStatus Status = "Warning"
	// Unknown is an unknown status
	Unknown Status = "Unknown"
	// EmptyStatus indicates an empty status
	EmptyStatus Status = ""
	// CreatingStatus indicate we are creating a resource
	CreatingStatus Status = "Creating"
	// UpdatingStatus indicate we are creating a resource
	UpdatingStatus Status = "Updating"
	// ActionRequiredStatus indicates that user action is required to remediate the current state
	// of a resource, e.g. a spec value is wrong or some external action needs to be taken
	ActionRequiredStatus Status = "ActionRequired"
)

// IsSuccess returns true if the status is a success
func (s Status) IsSuccess() bool {
	return s == SuccessStatus
}

// IsFailed returns true if the status is a failure
func (s Status) IsFailed() bool {
	return s == FailureStatus || s == DeleteFailedStatus
}

// IsError returns true if the status is an error
func (s Status) IsError() bool {
	return s.IsFailed() || s == ErrorStatus || s == DeleteErrorStatus
}

// IsDeleting returns true is the resource is being deleted
func (s Status) IsDeleting() bool {
	return s == DeletingStatus || s == DeletedStatus || s == DeleteFailedStatus || s == DeleteErrorStatus
}

// OneOf returns true if the status is one of the provided statuses
func (s Status) OneOf(statuses ...Status) bool {
	for _, status := range statuses {
		if status == s {
			return true
		}
	}
	return false
}

// CommonStatus is the common status for a resource
// +k8s:openapi-gen=true
type CommonStatus struct {
	// Conditions represents the observations of the resource's current state.
	// +kubebuilder:validation:Type=array
	// +patchMergeKey=type
	// +patchStrategy=merge
	// +listType=map
	// +listMapKey=type
	// +kubebuilder:validation:Optional
	Conditions Conditions `json:"conditions,omitempty"`
	// LastReconcile describes the generation and time of the last reconciliation
	// +kubebuilder:validation:Optional
	LastReconcile *LastReconcileStatus `json:"lastReconcile,omitempty"`
	// LastSuccess descibes the generation and time of the last reconciliation which resulted in
	// a Success status
	// +kubebuilder:validation:Optional
	LastSuccess *LastReconcileStatus `json:"lastSuccess,omitempty"`
}

// LastReconcileStatus is the status of the last reconciliation
// +k8s:openapi-gen=true
type LastReconcileStatus struct {
	// Time is the last time the resource was reconciled
	// +kubebuilder:validation:Optional
	Time metav1.Time `json:"time"`
	// Generation is the generation reconciled on the last reconciliation
	// +kubebuilder:validation:Optional
	Generation int64 `json:"generation"`
}

// GetCommonStatus returns the standard Wayfinder common status information for the resource
func (s *CommonStatus) GetCommonStatus() *CommonStatus {
	return s
}

// CommonStatusAware is implemented by any Wayfinder resource which has the standard Wayfinder common status
// implementation
// +kubebuilder:object:generate=false
type CommonStatusAware interface {
	client.Object
	GetCommonStatus() *CommonStatus
}
