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

package convert

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"sigs.k8s.io/yaml"

	terraformv1alphav1 "github.com/appvia/terranetes-controller/pkg/apis/terraform/v1alpha1"
	"github.com/appvia/terranetes-controller/pkg/cmd"
	"github.com/appvia/terranetes-controller/pkg/utils"
)

// ConfigurationCommand are the options for the command
type ConfigurationCommand struct {
	cmd.Factory
	// Path is the location of the file containing the configuration
	Path string
}

// moduleName is the template we use to generate the terraform code
var moduleTemplate = `module "main" {
  source = "{{ .source }}"
  {{- if .variables }}
  {{ toHCL .variables | nindent 2 }}
  {{- end }}
}
`

// NewConfigurationCommand creates a new command
func NewConfigurationCommand(factory cmd.Factory) *cobra.Command {
	o := &ConfigurationCommand{
		Factory: factory,
	}
	cmd := &cobra.Command{
		Use:     "configuration PATH",
		Aliases: []string{"config"},
		Args:    cobra.ExactArgs(1),
		Short:   "Convert configuration yaml into a terraform module",
		RunE: func(cmd *cobra.Command, args []string) error {
			o.Path = args[0]

			return o.Run(cmd.Context())
		},
	}

	return cmd
}

// Run is the entry point for the command
func (o *ConfigurationCommand) Run(ctx context.Context) error {
	content, err := os.ReadFile(o.Path)
	if err != nil {
		return fmt.Errorf("failed to read configuration file: %w", err)
	}

	configuration := &terraformv1alphav1.Configuration{}
	if err := yaml.Unmarshal(content, configuration); err != nil {
		return err
	}

	switch {
	case configuration.Spec.Module == "":
		return errors.New("spec.module name is required")
	}

	// @step: filter and fix up the source
	source := configuration.Spec.Module
	switch {
	case strings.Contains(source, "github.com") && strings.HasPrefix(source, "https://"):
		source = strings.TrimPrefix(source, "https://")
	}

	// @step: parse the variables if defined into a map
	var variables map[string]interface{}
	if configuration.HasVariables() {
		variables = make(map[string]interface{})

		err := json.NewDecoder(bytes.NewReader(configuration.Spec.Variables.Raw)).Decode(&variables)
		if err != nil {
			return fmt.Errorf("failed to parse spec.variables: %w", err)
		}
	}

	tmpl, err := utils.Template(moduleTemplate, map[string]interface{}{
		"source":    source,
		"variables": variables,
	})
	if err != nil {
		return err
	}

	scanner := bufio.NewScanner(bytes.NewReader(tmpl))
	for scanner.Scan() {
		line := scanner.Text()
		switch {
		case strings.HasPrefix(line, "  source = "):
			o.Println(line + "\n")
		case line == "  ":
			break
		default:
			o.Println(line)
		}
	}

	return nil
}
