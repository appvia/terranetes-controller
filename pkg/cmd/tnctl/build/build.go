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

package build

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/AlecAivazis/survey/v2"
	"github.com/fatih/color"
	"github.com/hashicorp/terraform-config-inspect/tfconfig"
	"github.com/spf13/cobra"
	"github.com/tidwall/sjson"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/yaml"

	terraformv1alphav1 "github.com/appvia/terranetes-controller/pkg/apis/terraform/v1alpha1"
	"github.com/appvia/terranetes-controller/pkg/cmd"
	"github.com/appvia/terranetes-controller/pkg/utils"
	"github.com/appvia/terranetes-controller/pkg/version"
)

var longBuildHelp = `
Build is used to automatically generate a compatible terraform
configuration from a given terraform module. The source for the
module can be a local directory, a git repository, s3 bucket or
so forth. As long as you have the credentials and required CLI
binaries installed.

Generate a terraform configuration a Github repository
$ tnctl build github.com/terraform-aws-modules/terraform-aws-vpc
`

// Command returns the cobra command for the "build" sub-command.
type Command struct {
	cmd.Factory
	// EnableAutoApproval indicates we automatically approve the configuration
	EnableAutoApproval bool
	// EnableDriftDetection indicates we should detect drift in the configuration
	EnableDriftDetection bool
	// EnableDefaults indicates we keep variables with defaults in the configuration
	EnableDefaults bool
	// EnableSensitive prompts the user to place the variables in secrets when the
	// variable is highlighted as 'sensitive' in the terraform module.
	EnableSensitive bool
	// Name is the name of the configuration
	Name string
	// Namespace is the namespace for the configuration
	Namespace string
	// NoDelete is a flag to indicate if the package should be deleted after the build
	NoDelete bool
	// Provider is the name of the provider to use for the configuration
	Provider string
	// Source is the source of the terraform module
	Source string
	// Secret is name of the secret to hold any sensitive variables
	Secret string
}

// NewCommand returns a new instance of the get command
func NewCommand(factory cmd.Factory) *cobra.Command {
	o := &Command{Factory: factory}

	c := &cobra.Command{
		Use:   "build SOURCE [OPTIONS]",
		Short: "Can be used to package up the terraform module for consumption",
		Long:  strings.TrimSuffix(longBuildHelp, "\n"),
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			o.Source = args[0]

			return o.Run(cmd.Context())
		},
	}

	flags := c.Flags()
	flags.StringVar(&o.Name, "name", "test", "The name of the configuration resource")
	flags.StringVar(&o.Namespace, "namespace", "default", "The namespace for the configuration")
	flags.BoolVar(&o.EnableAutoApproval, "enable-auto-approval", false, "Automatically approve the configuration")
	flags.BoolVar(&o.EnableDefaults, "enable-defaults", true, "Indicates any defaults with values from the terraform module are included")
	flags.BoolVar(&o.EnableDriftDetection, "enable-drift-detection", true, "Detect drift in the configuration")
	flags.BoolVar(&o.EnableSensitive, "enable-sensitive", true, "Indicates any sensitive variables from the module should be placed into secrets")
	flags.BoolVar(&o.NoDelete, "no-delete", false, "Indicates we do not delete the temporary directory")
	flags.StringVar(&o.Provider, "provider", "", "Name of the credentials provider to use")
	flags.StringVar(&o.Source, "source", ".", "The path to the terraform module")
	flags.StringVar(&o.Secret, "secret", "", "Name of the secret any outputs from the resource are kept")

	cmd.RegisterFlagCompletionFunc(c, "provider", cmd.AutoCompleteWithList([]string{"aws", "google", "azurerm", "vsphere"}))
	cmd.RegisterFlagCompletionFunc(c, "namespace", cmd.AutoCompleteNamespaces(factory))
	cmd.MarkHidden(c, "no-delete")

	return c
}

// Run implements the command action
func (o *Command) Run(ctx context.Context) error {
	switch {
	case o.Name == "":
		return cmd.ErrMissingArgument("name")
	case o.Source == "":
		return cmd.ErrMissingArgument("source")
	case o.Provider == "":
		o.Provider = "NEEDS_VALUE"
	}

	// @step: download the module
	destination := utils.TempDirName()
	source := o.Source

	if err := utils.Download(ctx, source, destination); err != nil {
		return err
	}

	o.Println("%s Successfully downloaded the terraform module: %s", cmd.IconGood, color.CyanString(o.Source))

	defer func() {
		if err := os.RemoveAll(destination); err != nil {
			o.Printf("failed to remove %s, %s\n", destination, err)
		}
	}()

	// @step: parse the process the module
	module, diag := tfconfig.LoadModule(destination)
	if diag.HasErrors() {
		return fmt.Errorf("failed to load terraform module: %s", diag.Err())
	}
	o.Println("%s Successfully parsed the terraform module", cmd.IconGood)

	configuration := terraformv1alphav1.NewConfiguration(o.Namespace, o.Name)
	configuration.Annotations = map[string]string{
		"terraform.appvia.io/source":  o.Source,
		"terraform.appvia.io/version": version.Version,
	}
	configuration.Spec.EnableAutoApproval = o.EnableAutoApproval
	configuration.Spec.EnableDriftDetection = o.EnableDriftDetection
	configuration.Spec.ProviderRef = &terraformv1alphav1.ProviderReference{Name: o.Provider}
	configuration.Spec.Module = source

	data := []byte(`{}`)

	var answer string

	// @step: lets convert the variables to json
	for _, variable := range module.Variables {
		switch {
		case variable.Default == nil && variable.Required:
			switch {
			case variable.Type == "string":
				if (variable.Sensitive || strings.Contains(variable.Name, "password")) && o.EnableSensitive {
					err := survey.AskOne(&survey.Input{
						Message: fmt.Sprintf("Input %q is sensitive and should be in a secret, what will be the secret name?", variable.Name),
						Help:    variable.Description,
					}, &answer)
					if err != nil {
						return nil
					}
					vf := terraformv1alphav1.ValueFromSource{}
					vf.Key = variable.Name
					vf.Secret = answer
					configuration.Spec.ValueFrom = append(configuration.Spec.ValueFrom, vf)

					continue
				}

				err := survey.AskOne(&survey.Input{
					Message: fmt.Sprintf("What should the value be for %q?", variable.Name),
					Help:    variable.Description,
				}, &answer, survey.WithKeepFilter(true))
				if err != nil {
					return err
				}
				variable.Default = &answer
			}

		case variable.Default == nil && !variable.Required:
			continue

		case variable.Default != nil && !o.EnableDefaults:
			continue
		}

		// @step: lets clean up any maps with nil values
		switch v := variable.Default.(type) {
		case map[string]interface{}:
			for key, value := range v {
				if value == nil {
					delete(variable.Default.(map[string]interface{}), key)
				}
			}
		}

		u, err := sjson.SetBytes(data, variable.Name, variable.Default)
		if err != nil {
			return err
		}
		data = u
	}

	// @step: convert the json to a map of values and wrap in an object
	variables := make(map[string]interface{})
	if err := json.NewDecoder(bytes.NewReader(data)).Decode(&variables); err != nil {
		return err
	}
	if len(variables) > 0 {
		unstruct := &unstructured.Unstructured{Object: variables}
		configuration.Spec.Variables = &runtime.RawExtension{
			Object: unstruct,
		}
	}

	// @step: ask about secrets if we have any outputs
	if len(module.Outputs) > 0 && o.Secret == "" {
		if err := survey.AskOne(&survey.Input{
			Message: "What name should the cloud resource secrets be called?",
			Help:    "This resource produces outputs which will be stored in the kubernetes secret",
			Default: o.Name,
		}, &o.Secret); err != nil {
			return err
		}
		configuration.Spec.WriteConnectionSecretToRef = &terraformv1alphav1.WriteConnectionSecret{
			Name: o.Secret,
		}
	}

	m := &bytes.Buffer{}
	if err := json.NewEncoder(m).Encode(&configuration); err != nil {
		return err
	}

	e, err := yaml.JSONToYAML(m.Bytes())
	if err != nil {
		return err
	}

	// @step: check if the configuration has any valueFrom
	if len(configuration.Spec.ValueFrom) > 0 {
		o.Println("%s Note, the configuration references %d secrets (spec.valueFrom[]) which must be created", len(configuration.Spec.ValueFrom))
	}

	o.Println("---")
	o.Println("%s", e)

	return nil
}
