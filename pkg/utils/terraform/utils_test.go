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

package terraform

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"k8s.io/apimachinery/pkg/runtime"

	terraformv1alphav1 "github.com/appvia/terranetes-controller/pkg/apis/terraform/v1alpha1"
)

func TestNewTerraformProvider(t *testing.T) {
	cases := []struct {
		Provider *terraformv1alphav1.Provider
		Expected string
	}{
		{
			Provider: &terraformv1alphav1.Provider{Spec: terraformv1alphav1.ProviderSpec{
				Provider:      terraformv1alphav1.AWSProviderType,
				Configuration: nil,
			}},
			Expected: "provider \"aws\" {\n}\n",
		},
		{
			Provider: &terraformv1alphav1.Provider{Spec: terraformv1alphav1.ProviderSpec{
				Provider:      terraformv1alphav1.AzureProviderType,
				Configuration: nil,
			}},
			Expected: "provider \"azurerm\" {\n  \n  features {}\n  \n}\n",
		},
		{
			Provider: &terraformv1alphav1.Provider{Spec: terraformv1alphav1.ProviderSpec{
				Provider:      terraformv1alphav1.AzureProviderType,
				Configuration: &runtime.RawExtension{Raw: []byte("{\"features\": {\"hello\": \"world\"}}")},
			}},
			Expected: "provider \"azurerm\" {\n  \n  features {\n    hello = \"world\"\n  }\n  \n}\n",
		},
		{
			Provider: &terraformv1alphav1.Provider{Spec: terraformv1alphav1.ProviderSpec{
				Provider:      terraformv1alphav1.AzureProviderType,
				Configuration: &runtime.RawExtension{Raw: []byte("{\"features\": \"hello\"}}")},
			}},
			Expected: "provider \"azurerm\" {\n  \n  features = \"hello\"\n  \n}\n",
		},
	}

	for _, c := range cases {
		var raw []byte
		if c.Provider.Spec.Configuration != nil {
			raw = c.Provider.Spec.Configuration.Raw
		}
		x, err := NewTerraformProvider(string(c.Provider.Spec.Provider), raw)
		assert.NoError(t, err)
		assert.Equal(t, string(c.Expected), string(x))
	}
}
