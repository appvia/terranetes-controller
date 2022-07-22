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

package configuration

import (
	"testing"

	"github.com/stretchr/testify/assert"

	terraformv1alphav1 "github.com/appvia/terranetes-controller/pkg/apis/terraform/v1alpha1"
)

func TestGetTerraformImage(t *testing.T) {
	cases := []struct {
		Default  string
		Override string
		Expected string
	}{
		{
			Default:  "hashicorp/terraform:1.1.9",
			Expected: "hashicorp/terraform:1.1.9",
		},
		{
			Default:  "hashicorp/terraform:1.1.9",
			Override: "override",
			Expected: "hashicorp/terraform:override",
		},
	}

	for _, c := range cases {
		config := &terraformv1alphav1.Configuration{}
		config.Spec.TerraformVersion = c.Override

		assert.Equal(t, c.Expected, GetTerraformImage(config, c.Default))
	}
}
