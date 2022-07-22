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

package fixtures

import (
	"bytes"
	"compress/gzip"

	v1 "k8s.io/api/core/v1"

	terraformv1alphav1 "github.com/appvia/terranetes-controller/pkg/apis/terraform/v1alpha1"
)

var state = `
{
	"terraform_version": "1.1.9",
	"resources": [
		{
			"mode": "managed",
			"type" : "aws_instance",
			"instances": [
				{
					"id": "i-0f9c9f9c9f9c9f9c9",
					"name": "foo"
				}
			]
		}
	],
	"outputs": {
		"test_output": {
			"value": "test",
			"type": "string"
		}
	}
}
`

var fakeCostsReport = `
{
	"totalHourlyCost": "0.01",
  "totalMonthlyCost": "100.00"
}
`

// NewCostsSecret returns a fake costs secret
func NewCostsSecret(namespace, name string) *v1.Secret {
	secret := &v1.Secret{}
	secret.Name = name
	secret.Namespace = namespace
	secret.Data = map[string][]byte{"INFRACOST_API_KEY": []byte("api-key")}

	return secret
}

// NewCostsReport returns a secret used to mock a cost report for a configuration
func NewCostsReport(configuration *terraformv1alphav1.Configuration) *v1.Secret {
	secret := &v1.Secret{}
	secret.Name = configuration.GetTerraformCostSecretName()
	secret.Data = map[string][]byte{"costs.json": []byte(fakeCostsReport)}

	return secret
}

// NewTerraformState returns a fake state
func NewTerraformState(configuration *terraformv1alphav1.Configuration) *v1.Secret {
	encoded := &bytes.Buffer{}

	w := gzip.NewWriter(encoded)
	//nolint:errcheck
	w.Write([]byte(state))
	w.Close()

	secret := &v1.Secret{}
	secret.Name = configuration.GetTerraformStateSecretName()
	secret.Data = map[string][]byte{
		terraformv1alphav1.TerraformStateSecretKey: encoded.Bytes(),
	}

	return secret
}
