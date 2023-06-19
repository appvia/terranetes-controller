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

import corev1alpha1 "github.com/appvia/terranetes-controller/pkg/apis/core/v1alpha1"

const (
	// ConditionConfigurationReady indicate the status of the configuration
	ConditionConfigurationReady corev1alpha1.ConditionType = "ConfigurationReady"
	// ConditionConfigurationStatus indicate the status of the configuration
	ConditionConfigurationStatus corev1alpha1.ConditionType = "ConfigurationStatus"
)

// DefaultCloudResourceConditions are the default conditions for all cloud resources
var DefaultCloudResourceConditions = append(
	[]corev1alpha1.ConditionSpec{
		{Type: ConditionConfigurationReady, Name: "Configuration Ready"},
		{Type: ConditionConfigurationStatus, Name: "Configuration Status"},
	},
	DefaultConfigurationConditions...,
)
