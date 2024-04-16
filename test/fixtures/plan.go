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

package fixtures

import (
	"bytes"
	"compress/gzip"
	"fmt"

	v1 "k8s.io/api/core/v1"

	terraformv1alpha1 "github.com/appvia/terranetes-controller/pkg/apis/terraform/v1alpha1"
	"github.com/appvia/terranetes-controller/pkg/controller"
)

// NewConfigurationPlan returns a new ConfigurationPlan
func NewConfigurationPlan(name string) *terraformv1alpha1.Plan {
	plan := terraformv1alpha1.NewPlan(name)
	controller.EnsureConditionsRegistered(terraformv1alpha1.DefaultPlanConditions, plan)

	return plan
}

var tfplanJSON = `
{
	"format_version": "1.2",
	"terraform_version": "1.5.7",
	"resource_changes": [
		{
			"address": "aws_cloudfront_distribution.this",
			"mode": "managed",
			"type": "aws_cloudfront_distribution",
			"name": "this",
			"provider_name": "registry.terraform.io/hashicorp/aws",
			"change": {
				"actions": [
					"%[1]s"
				]
			}
		}
	],
	"output_changes": {
		"distribution_arn": {
			"actions": [
				"%[1]s"
			]
		}
	},
	"timestamp": "%[2]s"
}
`
var TFPlanID = "2024-04-15T19-47-35Z"

// NewTerraformPlanWithDiff returns a fake plan that requires apply
func NewTerraformPlanWithDiff(configuration *terraformv1alpha1.Configuration, namespace string) *v1.Secret {
	return newTerraformPlanStateAction(configuration, namespace, "update")
}

// NewTerraformPlanNoop returns a fake plan that requires apply
func NewTerraformPlanNoop(configuration *terraformv1alpha1.Configuration, namespace string) *v1.Secret {
	return newTerraformPlanStateAction(configuration, namespace, "no-op")
}

// NewTerraformPlan returns a fake plan
func newTerraformPlanStateAction(configuration *terraformv1alpha1.Configuration, namespace string, action string) *v1.Secret {
	encoded := &bytes.Buffer{}

	w := gzip.NewWriter(encoded)
	//nolint:errcheck
	w.Write([]byte(fmt.Sprintf(tfplanJSON, action, TFPlanID)))
	w.Close()

	secret := &v1.Secret{}
	secret.Name = configuration.GetTerraformPlanJSONSecretName()
	secret.Namespace = namespace
	secret.Data = map[string][]byte{
		terraformv1alpha1.TerraformPlanJSONSecretKey: encoded.Bytes(),
	}

	return secret
}
