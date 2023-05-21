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
	"k8s.io/apimachinery/pkg/runtime"

	terraformv1alpha1 "github.com/appvia/terranetes-controller/pkg/apis/terraform/v1alpha1"
)

// NewTerranettesContext returns a new Context
func NewTerranettesContext(name string) *terraformv1alpha1.Context {
	c := terraformv1alpha1.NewContext(name)
	c.Spec.Variables = map[string]runtime.RawExtension{
		"vpc_id": {
			Raw: []byte(`{"description": "netwrk", "value": "vpc-123456"}`),
		},
		"public_subnets": {
			Raw: []byte(`{"description": "netwrk", "value": ["subnet-123456", "subnet-123456"]}`),
		},
	}

	return c
}
