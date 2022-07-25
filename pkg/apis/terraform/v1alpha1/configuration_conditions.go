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
	corev1alphav1 "github.com/appvia/terranetes-controller/pkg/apis/core/v1alpha1"
)

const (
	// ConditionProviderReady indicate the status of the provider
	ConditionProviderReady corev1alphav1.ConditionType = "ProviderReady"
	// ConditionTerraformPlan indicates the status of the terraform plan
	ConditionTerraformPlan corev1alphav1.ConditionType = "TerraformPlan"
	// ConditionTerraformPolicy indicates the status of the terraform apply
	ConditionTerraformPolicy corev1alphav1.ConditionType = "SecurityPolicy"
	// ConditionTerraformApply indicates the status of the terraform apply
	ConditionTerraformApply corev1alphav1.ConditionType = "TerraformApply"
)

// DefaultConfigurationConditions are the default conditions for all configurations
var DefaultConfigurationConditions = []corev1alphav1.ConditionSpec{
	{Type: ConditionProviderReady, Name: "Provider ready"},
	{Type: ConditionTerraformPlan, Name: "Terraform Plan"},
	{Type: ConditionTerraformPolicy, Name: "Security Policy"},
	{Type: ConditionTerraformApply, Name: "Terraform Apply"},
	{Type: corev1alphav1.ConditionReady, Name: "Ready"},
}
