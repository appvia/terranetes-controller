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

package controller

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	corev1alpha1 "github.com/appvia/terranetes-controller/pkg/apis/core/v1alpha1"
)

// EnsureConditionsRegistered is responsible for ensuring all the conditions are registered
func EnsureConditionsRegistered(conditions []corev1alpha1.ConditionSpec, resource Object) {
	status := resource.GetCommonStatus()

	if status.LastReconcile == nil {
		status.LastReconcile = &corev1alpha1.LastReconcileStatus{}
	}
	if status.LastSuccess == nil {
		status.LastSuccess = &corev1alpha1.LastReconcileStatus{}
	}
	if status.Conditions == nil {
		status.Conditions = make([]corev1alpha1.Condition, 0)
	}

	for _, x := range conditions {
		if status.GetCondition(x.Type) == nil {
			condition := corev1alpha1.Condition{
				Name:   x.Name,
				Reason: corev1alpha1.ReasonNotDetermined,
				Status: x.DefaultStatus,
				Type:   x.Type,
			}
			if condition.Status == "" {
				condition.Status = metav1.ConditionFalse
			}
			if condition.LastTransitionTime.IsZero() {
				condition.LastTransitionTime = metav1.Now()
			}

			status.Conditions = append(status.Conditions, condition)
		}
	}
}
