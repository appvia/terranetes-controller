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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"

	terraformv1alpha1 "github.com/appvia/terranetes-controller/pkg/apis/terraform/v1alpha1"
)

func TestNewKubernetesBackend(t *testing.T) {
	cases := []struct {
		Options  BackendOptions
		Expected string
	}{
		{
			Options: BackendOptions{
				Configuration: &terraformv1alpha1.Configuration{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test",
						Namespace: "default",
					},
				},
				Namespace: "default",
				Suffix:    "test",
				Template:  KubernetesBackendTemplate,
			},
			Expected: `
terraform {
	backend "kubernetes" {
		in_cluster_config = true
		namespace         = "default"
		labels            = {
			"terraform.appvia.io/configuration" = "test"
			"terraform.appvia.io/configuration-uid" = ""
			"terraform.appvia.io/generation" = "0"
			"terraform.appvia.io/namespace" = "default"
		}
		secret_suffix     = "test"
	}
}
`},
	}
	for _, c := range cases {
		generated, err := NewKubernetesBackend(c.Options)
		assert.NoError(t, err)
		assert.Equal(t, string(c.Expected), string(generated))
	}
}

func TestNewTerraformProvider(t *testing.T) {
	azureConfig := `
		{
		  "use_oidc": true,
		  "storage_use_azuread": true,
		  "subscription_id": "injected",
		  "tenant_id": "injected",
		  "client_id": "injected",
		  "oi*dc_token_file_path": "/var/run/secrets/azure/tokens/azure-identity-token"
		}`

	cases := []struct {
		Provider *terraformv1alpha1.Provider
		Expected string
	}{
		{
			Provider: &terraformv1alpha1.Provider{Spec: terraformv1alpha1.ProviderSpec{
				Provider:      terraformv1alpha1.AWSProviderType,
				Configuration: nil,
			}},
			//Expected: "provider \"aws\" {\n}\n",
			Expected: "{\n  \"provider\": {\n    \"aws\": {}\n  }\n",
		},
		{
			Provider: &terraformv1alpha1.Provider{Spec: terraformv1alpha1.ProviderSpec{
				Provider:      terraformv1alpha1.AzureProviderType,
				Configuration: nil,
			}},
			Expected: "{\n  \"provider\": {\n    \"azurerm\": {\n      \"features\": {}\n    }\n  }\n}\n",
		},
		{
			Provider: &terraformv1alpha1.Provider{Spec: terraformv1alpha1.ProviderSpec{
				Provider:      terraformv1alpha1.AzureProviderType,
				Configuration: &runtime.RawExtension{Raw: []byte("{\"features\": {\"hello\": \"world\"}}")},
			}},
			Expected: "{\n  \"provider\": {\n    \"azurerm\": {\n      \"features\": {\n        \"hello\": \"world\"\n      }\n    }\n  }\n}\n",
		},
		{
			Provider: &terraformv1alpha1.Provider{Spec: terraformv1alpha1.ProviderSpec{
				Provider:      terraformv1alpha1.AzureProviderType,
				Configuration: &runtime.RawExtension{Raw: []byte("{\"features\": \"hello\"}}")},
			}},
			Expected: "{\n  \"provider\": {\n    \"azurerm\": {\n      \"features\": \"hello\"\n    }\n  }\n}\n",
		},
		{
			Provider: &terraformv1alpha1.Provider{Spec: terraformv1alpha1.ProviderSpec{
				Provider:      terraformv1alpha1.AzureProviderType,
				Configuration: &runtime.RawExtension{Raw: []byte(azureConfig)},
			}},
			Expected: "{\n  \"provider\": {\n    \"azurerm\": {\n      \"client_id\": \"injected\",\n      \"features\": {},\n      \"oi*dc_token_file_path\": \"/var/run/secrets/azure/tokens/azure-identity-token\",\n      \"storage_use_azuread\": true,\n      \"subscription_id\": \"injected\",\n      \"tenant_id\": \"injected\",\n      \"use_oidc\": true\n    }\n  }\n}\n",
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
