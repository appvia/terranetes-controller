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

	terraformv1alpha1 "github.com/appvia/terranetes-controller/pkg/apis/terraform/v1alpha1"
	"github.com/appvia/terranetes-controller/pkg/cmd"
	"github.com/appvia/terranetes-controller/pkg/utils/kubernetes"
	"github.com/appvia/terranetes-controller/pkg/utils/terraform"
)

// ConfigurationCommand are the options for the command
type ConfigurationCommand struct {
	cmd.Factory
	// File is the location of the file containing the configuration
	File string
	// Name is the name of the resource
	Name string
	// Namespace is the namespace of the resource
	Namespace string
}

var longDescription = `
Provides the abiliy to convert configurations and cloudresources back
into terraform modules.

Convert a configuration in the cluster into a terraform module:
$ tnctl convert configuration -n my-namespace my-configuration

Convert a configuration file into a terraform module:
$ tnctl convert configuration -f my-configuration.yaml

Convert a cloudresource in the cluster into a terraform module:
$ tnctl convert cloudresource -n my-namespace my-cloudresource
`

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
	c := &cobra.Command{
		Use:     "configuration [OPTIONS] [NAME|-f FILE]",
		Aliases: []string{"config"},
		Args:    cobra.ExactArgs(1),
		Short:   "Converts configuration back to a terraform module",
		RunE: func(cmd *cobra.Command, args []string) error {
			return o.Run(cmd.Context())
		},
		ValidArgsFunction: cmd.AutoCompleteConfigurations(factory),
	}
	c.SetErr(o.GetStreams().ErrOut)
	c.SetIn(o.GetStreams().In)
	c.SetOut(o.GetStreams().Out)

	flags := c.Flags()
	flags.StringVar(&o.Name, "name", "", "Name of the resource")
	flags.StringVarP(&o.Namespace, "namespace", "n", "default", "Namespace of the resource")
	flags.StringVarP(&o.File, "file", "f", "", "Path to the configuration file")

	cmd.RegisterFlagCompletionFunc(c, "namespace", cmd.AutoCompleteNamespaces(factory))

	return c
}

// Run is the entry point for the command
func (o *ConfigurationCommand) Run(ctx context.Context) error {
	switch {
	case o.File != "":
		break
	case o.Name != "" && o.Namespace != "":
		break
	default:
		return errors.New("either file or name and namespace must be provided")
	}

	// @step: retrieve the configuration
	configuration := &terraformv1alpha1.Configuration{}
	switch {
	case o.File != "":
		content, err := os.ReadFile(o.File)
		if err != nil {
			return fmt.Errorf("failed to read configuration file: %w", err)
		}

		if err := yaml.Unmarshal(content, configuration); err != nil {
			return err
		}

	default:
		cc, err := o.GetClient()
		if err != nil {
			return err
		}

		configuration.Namespace = o.Namespace
		configuration.Name = o.Name

		if found, err := kubernetes.GetIfExists(ctx, cc, configuration); err != nil {
			return err
		} else if !found {
			return fmt.Errorf("configuration (%s/%s) does not exist", o.Namespace, o.Name)
		}
	}

	// @step: ensure we have a valid configuration
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
	if configuration.Spec.HasVariables() {
		variables = make(map[string]interface{})

		err := json.NewDecoder(bytes.NewReader(configuration.Spec.Variables.Raw)).Decode(&variables)
		if err != nil {
			return fmt.Errorf("failed to parse spec.variables: %w", err)
		}
	}

	tmpl, err := terraform.Template(moduleTemplate, map[string]interface{}{
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
