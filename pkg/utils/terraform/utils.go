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
	"bytes"
	"compress/gzip"
	"encoding/json"
	"io"
	"text/template"

	terraformv1alphav1 "github.com/appvia/terranetes-controller/pkg/apis/terraform/v1alpha1"
	"github.com/appvia/terranetes-controller/pkg/utils"
)

// TerraformStateOutputsKey is the key for the terraform state outputs
const TerraformStateOutputsKey = "outputs"

// backendTF is responsible for creating the kubernetes backend terraform configuration
var backendTF = `
terraform {
	backend "kubernetes" {
		in_cluster_config = true
		namespace         = "{{ .Namespace }}"
		secret_suffix     = "{{ .Suffix }}"
	}
}
`

// providerTF is a template for a terraform provider
var providerTF = `provider "{{ .Provider }}" {
{{- if .Configuration }}
  {{ toHCL .Configuration | nindent 2 }}
{{- end }}
}
`

// Decode decodes the terraform state returning the json output
func Decode(state []byte) ([]byte, error) {
	in, err := gzip.NewReader(bytes.NewReader(state))
	if err != nil {
		return nil, err
	}

	return io.ReadAll(in)
}

// DecodeState decodes the terraform state outputs
func DecodeState(in []byte) (*State, error) {
	decoded, err := Decode(in)
	if err != nil {
		return nil, err
	}

	state := &State{}
	if err := json.NewDecoder(bytes.NewReader(decoded)).Decode(state); err != nil {
		return nil, err
	}

	return state, nil
}

// NewTerraformProvider generates a terraform provider configuration
func NewTerraformProvider(provider string, configuration []byte) ([]byte, error) {
	// @step: azure requires the configuration for features
	switch terraformv1alphav1.ProviderType(provider) {
	case terraformv1alphav1.AzureProviderType:
		if len(configuration) == 0 {
			configuration = []byte(`{"features":{}}`)
		}
	}

	config := make(map[string]interface{})
	if len(configuration) > 0 {
		if err := json.NewDecoder(bytes.NewReader(configuration)).Decode(&config); err != nil {
			return nil, err
		}
	}

	return utils.Template(providerTF, map[string]interface{}{
		"Configuration": config,
		"Provider":      provider,
	})
}

// NewKubernetesBackend creates a new kubernetes backend
func NewKubernetesBackend(namespace, suffux string) ([]byte, error) {
	tmpl, err := template.New("main").Parse(backendTF)
	if err != nil {
		return nil, err
	}

	render := &bytes.Buffer{}
	if err := tmpl.Execute(render, map[string]string{
		"Namespace": namespace,
		"Suffix":    suffux,
	}); err != nil {
		return nil, err
	}

	return render.Bytes(), nil
}
