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

package controller

import (
	"fmt"
	"reflect"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	corev1alphav1 "github.com/appvia/terraform-controller/pkg/apis/core/v1alpha1"
)

// ConditionManager manages the condition of the resource
type ConditionManager struct {
	// condition is a reference to the condition
	condition *corev1alphav1.Condition
	// resource is the underlying resource
	resource client.Object
}

// ConditionMgr returns a manager for the condition
func ConditionMgr(resource corev1alphav1.CommonStatusAware, condition corev1alphav1.ConditionType) *ConditionManager {
	cond := resource.GetCommonStatus().GetCondition(condition)
	if cond == nil {
		cond = &corev1alphav1.Condition{}
	}

	return &ConditionManager{condition: cond, resource: resource}
}

// GetCondition returns the condition
func (c *ConditionManager) GetCondition() *corev1alphav1.Condition {
	return c.condition
}

// transition is a helper method used to update the transition time. The method takes of a copy of the current
// condition and then allows the handler to update. Before exiting it performs a comparison and if it's been update
// it updates the transitions time
func transition(cond *corev1alphav1.Condition, fn func()) {
	original := *cond
	fn()

	if !reflect.DeepEqual(original, *cond) {
		cond.LastTransitionTime = metav1.Now()
	}

}

// ActionRequired sets the condition to action required
func (c *ConditionManager) ActionRequired(message string, args ...interface{}) {
	transition(c.condition, func() {
		c.condition.ObservedGeneration = c.resource.GetGeneration()
		c.condition.Status = metav1.ConditionFalse
		c.condition.Reason = corev1alphav1.ReasonActionRequired
		c.condition.Message = fmt.Sprintf(message, args...)
		c.condition.Detail = ""
	})
}

// Warning sets the condition to successful
func (c *ConditionManager) Warning(message string, args ...interface{}) {
	transition(c.condition, func() {
		c.condition.ObservedGeneration = c.resource.GetGeneration()
		c.condition.Status = metav1.ConditionFalse
		c.condition.Reason = corev1alphav1.ReasonWarning
		c.condition.Message = fmt.Sprintf(message, args...)
		c.condition.Detail = ""
	})
}

// Success sets the condition to successful
func (c *ConditionManager) Success(message string, args ...interface{}) {
	transition(c.condition, func() {
		c.condition.ObservedGeneration = c.resource.GetGeneration()
		c.condition.Status = metav1.ConditionTrue
		c.condition.Reason = corev1alphav1.ReasonReady
		c.condition.Message = fmt.Sprintf(message, args...)
		c.condition.Detail = ""
	})
}

// Failed sets the condition to failed
func (c *ConditionManager) Failed(err error, message string, args ...interface{}) {
	transition(c.condition, func() {
		c.condition.ObservedGeneration = c.resource.GetGeneration()
		c.condition.Status = metav1.ConditionFalse
		c.condition.Reason = corev1alphav1.ReasonError
		c.condition.Message = fmt.Sprintf(message, args...)
		if err != nil {
			c.condition.Detail = err.Error()
		}
	})
}

// InProgress sets the condition to in progress
func (c *ConditionManager) InProgress(message string, args ...interface{}) {
	transition(c.condition, func() {
		c.condition.ObservedGeneration = c.resource.GetGeneration()
		c.condition.Status = metav1.ConditionFalse
		c.condition.Reason = corev1alphav1.ReasonInProgress
		c.condition.Message = fmt.Sprintf(message, args...)
		c.condition.Detail = ""
	})
}

// Deleting sets the condition to deleting
func (c *ConditionManager) Deleting(message string, args ...interface{}) {
	transition(c.condition, func() {
		c.condition.ObservedGeneration = c.resource.GetGeneration()
		c.condition.Status = metav1.ConditionFalse
		c.condition.Reason = corev1alphav1.ReasonDeleting
		c.condition.Message = fmt.Sprintf(message, args...)
		c.condition.Detail = ""
	})
}
