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
	"errors"
	"fmt"
	"io"

	terraformv1alpha1 "github.com/appvia/terranetes-controller/pkg/apis/terraform/v1alpha1"
	"github.com/appvia/terranetes-controller/pkg/utils"
	"github.com/appvia/terranetes-controller/pkg/utils/template"
)

// TerraformStateOutputsKey is the key for the terraform state outputs
const TerraformStateOutputsKey = "outputs"

// checkovPolicyTemplate is the default template used to produce a checkov configuration
var (
	// CheckovPolicyTemplate is the default template used to produce a checkov configuration
	CheckovPolicyTemplate = `framework:
  - {{ default .Framework "terraform_plan" }}
soft-fail: true
compact: true
{{- if .Policy.Checks }}
check:
	{{- range $check := .Policy.Checks }}
  - {{ $check }}
  {{- end }}
{{- end }}
{{- if .Policy.SkipChecks }}
skip-check:
  {{- range .Policy.SkipChecks }}
  - {{ . }}
  {{- end }}
{{- end }}
{{- if .Policy.External }}
external-checks-dir:
  {{- range .Policy.External }}
  - /run/policy/{{ .Name }}
  {{- end }}
{{- end }}`
)

// KubernetesBackendTemplate is responsible for creating the kubernetes backend terraform configuration
var KubernetesBackendTemplate = `
terraform {
	backend "kubernetes" {
		in_cluster_config = true
		namespace         = "{{ .controller.namespace }}"
		{{- if .controller.labels }}
		labels            = {
			{{- range $key, $value := .controller.labels }}
			"{{ $key }}" = "{{ $value }}"
			{{- end }}
		}
		{{- end }}
		secret_suffix     = "{{ .controller.suffix }}"
	}
}
`

// providerTF is a template for a terraform provider
var providerTF = `{
	"provider": {
    "{{ .provider }}": { 
			{{- if .configuration }}
			{{ toJson .configuration }}
      {{- end }}
		}
	}
}`

// Decode returns a Reader that will decode a gzip byte stream
func Decode(data []byte) (io.Reader, error) {
	return gzip.NewReader(bytes.NewReader(data))
}

// DecodeState decodes the terraform state outputs
func DecodeState(in []byte) (*State, error) {
	decoded, err := Decode(in)
	if err != nil {
		return nil, err
	}

	state := &State{}
	if err := json.NewDecoder(decoded).Decode(state); err != nil {
		return nil, err
	}

	return state, nil
}

// DecodePlan decodes the terraform plan outputs
func DecodePlan(in []byte) (*Plan, error) {
	decoded, err := Decode(in)
	if err != nil {
		return nil, err
	}

	plan := &Plan{}
	if err := json.NewDecoder(decoded).Decode(plan); err != nil {
		return nil, err
	}

	return plan, nil
}

// NewCheckovPolicy generates a checkov policy from the configuration
func NewCheckovPolicy(data map[string]interface{}) ([]byte, error) {
	switch {
	case data == nil:
		return nil, errors.New("no data provided")
	case data["Policy"] == nil:
		return nil, errors.New("no policy provided")
	}

	return template.New(CheckovPolicyTemplate, data)
}

// NewTerraformProvider generates a terraform provider configuration
func NewTerraformProvider(provider string, configuration []byte) ([]byte, error) {
	// @step: azure requires the configuration for features
	switch terraformv1alpha1.ProviderType(provider) {
	case terraformv1alpha1.AzureProviderType:
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

	if terraformv1alpha1.ProviderType(provider) == terraformv1alpha1.AzureProviderType {
		if config["features"] == nil {
			config["features"] = make(map[string]interface{})
		}
	}

	rendered, err := Template(providerTF, map[string]interface{}{
		"configuration": config,
		"provider":      provider,
	})
	if err != nil {
		return nil, err
	}

	return utils.PrettyJSON(rendered), nil
}

// BackendOptions are the options used to generate the backend
type BackendOptions struct {
	// Configuration is a reference to the terraform configuration
	Configuration *terraformv1alpha1.Configuration
	// Namespace is a reference to the controller namespace
	Namespace string
	// Suffix is an expexted suffix for the terraform state
	Suffix string
	// Template is the golang template to use to generate the backend content
	Template string
}

// NewKubernetesBackend creates a new kubernetes backend
func NewKubernetesBackend(options BackendOptions) ([]byte, error) {
	params := map[string]interface{}{
		"controller": map[string]interface{}{
			"namespace": options.Namespace,
			"labels": map[string]string{
				terraformv1alpha1.ConfigurationGenerationLabel: fmt.Sprintf("%d", options.Configuration.GetGeneration()),
				terraformv1alpha1.ConfigurationNameLabel:       options.Configuration.Name,
				terraformv1alpha1.ConfigurationNamespaceLabel:  options.Configuration.Namespace,
				terraformv1alpha1.ConfigurationUIDLabel:        string(options.Configuration.GetUID()),
			},
			"suffix": options.Suffix,
		},
		"configuration": options.Configuration,
		"name":          options.Configuration.Name,
		"namespace":     options.Configuration.Namespace,
	}

	return Template(options.Template, params)
}
