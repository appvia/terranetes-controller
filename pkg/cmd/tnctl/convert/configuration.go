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
	"path/filepath"
	"strings"

	"github.com/AlecAivazis/survey/v2"
	"github.com/spf13/cobra"
	"github.com/tidwall/sjson"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"

	terraformv1alpha1 "github.com/appvia/terranetes-controller/pkg/apis/terraform/v1alpha1"
	"github.com/appvia/terranetes-controller/pkg/cmd"
	"github.com/appvia/terranetes-controller/pkg/utils"
	"github.com/appvia/terranetes-controller/pkg/utils/kubernetes"
	"github.com/appvia/terranetes-controller/pkg/utils/policies"
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
	// IncludeProvider is whether to include the provider in the output
	IncludeProvider bool
	// IncludeCheckov is whether to include checkov in the output
	IncludeCheckov bool
	// IncludeTerraform is whether to include terraform in the output
	IncludeTerraform bool
	// Directory is the path to write the files to
	Directory string
	// Configuration is the configuration we are converting
	Configuration *terraformv1alpha1.Configuration
	// Contexts is a list of contexts from the cluster
	Contexts *terraformv1alpha1.ContextList
	// Policies is a list of policies from the cluster
	Policies *terraformv1alpha1.PolicyList
	// Providers is a collection of providers in the cluster
	Providers *terraformv1alpha1.ProviderList
}

var longDescription = `
Provides the ability to convert configurations and cloudresources back
into terraform modules.

Note, if you include --include-provider or --include-checkov, this
command will use the current kubeconfig context to retrieve the provider
and checkov policy from the cluster.

Convert a configuration in the cluster into a terraform module:
$ tnctl convert configuration -n my-namespace my-configuration

Convert a configuration file into a terraform module:
$ tnctl convert configuration -f my-configuration.yaml

Convert a cloudresource in the cluster into a terraform module:
$ tnctl convert cloudresource -n my-namespace my-cloudresource
`

// moduleName is the template we use to generate the terraform code
var moduleTemplate = `module "main" {
  source = "{{ .Source }}"
  {{- if .Variables }}
  {{ toHCL .Variables | nindent 2 }}
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
		Short:   "Converts configuration back to a terraform module",
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) > 0 {
				o.Name = args[0]
			}

			return o.Run(cmd.Context())
		},
		ValidArgsFunction: cmd.AutoCompleteConfigurations(factory),
	}
	c.SetErr(o.GetStreams().ErrOut)
	c.SetIn(o.GetStreams().In)
	c.SetOut(o.GetStreams().Out)

	flags := c.Flags()
	flags.BoolVar(&o.IncludeCheckov, "include-checkov", true, "Include checkov in the output")
	flags.BoolVar(&o.IncludeProvider, "include-provider", true, "Include provider in the output")
	flags.BoolVar(&o.IncludeTerraform, "include-terraform", true, "Include terraform in the output")
	flags.StringVarP(&o.Directory, "path", "p", ".", "The path to write the files to")
	flags.StringVarP(&o.File, "file", "f", "", "Path to the configuration file")
	flags.StringVarP(&o.Namespace, "namespace", "n", "default", "Namespace of the resource")

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
	case o.Configuration != nil:
		break
	default:
		return errors.New("either file or name and namespace must be provided")
	}

	// @step: retrieve the configuration
	if err := o.resolveConfiguration(ctx); err != nil {
		return err
	}
	// @step: render the configuration
	if err := o.renderConfiguration(); err != nil {
		return err
	}
	// @step: render the provider
	if err := o.renderProvider(ctx); err != nil {
		return err
	}
	// @step: render the checkov policy
	if err := o.renderPolicy(ctx); err != nil {
		return err
	}

	return nil
}

// renderPolicy renders the checkov policy
func (o *ConfigurationCommand) renderPolicy(ctx context.Context) error {
	configuration := o.Configuration

	switch {
	case o.File != "":
		o.Println("Skipping checkov policy as file was provided")

		return nil

	case !o.IncludeCheckov:
		return nil
	}

	// @step: retrieve a client
	cc, err := o.GetClient()
	if err != nil {
		return err
	}

	namespace := &v1.Namespace{}
	namespace.Name = configuration.Namespace

	if found, err := kubernetes.GetIfExists(ctx, cc, namespace); err != nil {
		return err
	} else if !found {
		return fmt.Errorf("namespace %q not found", configuration.Namespace)
	}

	// @step: retrieve a list of policies in the cluster
	if o.Policies == nil {
		o.Policies = &terraformv1alpha1.PolicyList{}

		if err := cc.List(ctx, o.Policies); err != nil {
			return err
		}
	}

	// @step: find the policy matching the configuration
	policy, err := policies.FindMatchingPolicy(ctx, configuration, namespace, o.Policies)
	if err != nil {
		return err
	}
	if policy == nil {
		return nil
	}

	// @step: render the policy
	generated, err := terraform.NewCheckovPolicy(map[string]interface{}{
		"Framework": "terraform",
		"Policy":    policy,
	})
	if err != nil {
		return err
	}

	// @step: write the policy to disk
	wr, err := os.OpenFile(filepath.Join(o.Directory, ".checkov.yml"), os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}

	if _, err := wr.Write(generated); err != nil {
		return err
	}

	return nil
}

// renderProvider retrieves the provider from the cluster and renders it
func (o *ConfigurationCommand) renderProvider(ctx context.Context) error {
	configuration := o.Configuration

	switch {
	case o.File != "":
		o.Println("Skipping provider as file was provided")

		return nil
	case !o.IncludeProvider:
		return nil

	case configuration.Spec.ProviderRef == nil:
		return nil
	}

	if o.Providers == nil {
		// @step: retrieve a client
		cc, err := o.GetClient()
		if err != nil {
			return err
		}

		// @step: retrieve a list of providers in the cluster
		o.Providers = &terraformv1alpha1.ProviderList{}
		if err := cc.List(ctx, o.Providers); err != nil {
			return err
		}
	}

	// @step: retrieve the provider
	provider, found := o.Providers.GetItem(configuration.Spec.ProviderRef.Name)
	if !found {
		return fmt.Errorf("provider: %q does not exist in cluster", provider.Name)
	}

	// @step: render the provider
	var config []byte
	if provider.Spec.Configuration != nil {
		config = provider.Spec.Configuration.Raw
	}

	template, err := terraform.NewTerraformProvider(provider.Name, config)
	if err != nil {
		return err
	}

	// @step: open the file for writing
	wr, err := os.OpenFile(filepath.Join(o.Directory, "provider.tf"), os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
	if err != nil {
		return err
	}

	// @step: write the provider to the file
	if _, err := wr.Write(template); err != nil {
		return err
	}

	return nil
}

// resolveConfiguration retrieves the configuration from cluster or file
// nolint:gocyclo
func (o *ConfigurationCommand) resolveConfiguration(ctx context.Context) error {
	if o.Configuration == nil {
		// @step: retrieve the configuration
		o.Configuration = &terraformv1alpha1.Configuration{}
		switch {
		case o.File != "":
			if err := utils.LoadYAML(o.File, o.Configuration); err != nil {
				return fmt.Errorf("failed to read configuration file: %w", err)
			}

		default:
			cc, err := o.GetClient()
			if err != nil {
				return err
			}

			o.Configuration.Namespace = o.Namespace
			o.Configuration.Name = o.Name

			if found, err := kubernetes.GetIfExists(ctx, cc, o.Configuration); err != nil {
				return err
			} else if !found {
				return fmt.Errorf("configuration (%s/%s) does not exist", o.Namespace, o.Name)
			}
		}
	}

	// @step: does the configuration have any secrets of contexts
	if len(o.Configuration.Spec.ValueFrom) == 0 {
		return nil
	}

	var answer bool

	if o.Configuration.Spec.Variables == nil {
		o.Configuration.Spec.Variables = &runtime.RawExtension{}
	}

	// @step: we need to prompt the user to ask if they want use to resolve them?
	if o.Configuration.Spec.ValueFrom.HasSecretReferences() {
		if err := survey.AskOne(&survey.Confirm{
			Message: "The resource contains references to Secrets, do you want to resolve them?",
		}, &answer); err != nil {
			return err
		}
		if !answer {
			cc, err := o.GetClient()
			if err != nil {
				return err
			}

			for i, x := range o.Configuration.Spec.ValueFrom {
				if x.Secret == nil {
					continue
				}
				secret := &v1.Secret{}
				secret.Name = *x.Secret
				secret.Namespace = o.Configuration.Namespace

				if found, err := kubernetes.GetIfExists(ctx, cc, secret); err != nil {
					return err
				} else if !found {
					return fmt.Errorf("spec.valueFrom[%d].secret: %q does not exist", i, secret.Name)
				}
				value, found := secret.Data[x.GetName()]
				if !found {
					return fmt.Errorf("spec.valueFrom[%d].key: data key %q does not exist in secret", i, x.Key)
				}
				o.Configuration.Spec.Variables.Raw, err = sjson.SetBytes(o.Configuration.Spec.Variables.Raw, x.Name, value)
				if err != nil {
					return err
				}
			}
		}
	}

	if o.Configuration.Spec.ValueFrom.HasContextReferences() {
		if err := survey.AskOne(&survey.Confirm{
			Message: "The resource contains references to Contexts, do you want to resolve them?",
			Default: true,
		}, &answer); err != nil {
			return err
		}
		if answer {
			if o.Contexts == nil {
				cc, err := o.GetClient()
				if err != nil {
					return err
				}

				o.Contexts = &terraformv1alpha1.ContextList{}
				if err := cc.List(ctx, o.Contexts); err != nil {
					return err
				}
			}
			for _, x := range o.Configuration.Spec.ValueFrom {
				if x.Context == nil {
					continue
				}
				txt, found := o.Contexts.GetItem(*x.Context)
				if !found {
					return fmt.Errorf("context: %q not found in cluster, unable to resolve", *x.Context)
				}
				value, found, err := txt.Spec.GetVariable(x.Key)
				if err != nil {
					return err
				}
				if !found {
					return fmt.Errorf("context: %q does not contain the key: %q", *x.Context, x.Key)
				}
				o.Configuration.Spec.Variables.Raw, err = sjson.SetBytes(o.Configuration.Spec.Variables.Raw, x.Name, value)
				if err != nil {
					return err
				}
			}
		}
	}

	return nil
}

// renderConfiguration retrieves the configuration and renders it
// nolint:errcheck
func (o *ConfigurationCommand) renderConfiguration() error {
	if !o.IncludeTerraform {
		return nil
	}

	configuration := o.Configuration

	var variables map[string]interface{}

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
	if configuration.Spec.HasVariables() {
		variables = make(map[string]interface{})

		err := json.NewDecoder(bytes.NewReader(configuration.Spec.Variables.Raw)).Decode(&variables)
		if err != nil {
			return fmt.Errorf("failed to parse spec.variables: %w", err)
		}
	}

	// @step: render the module
	tmpl, err := terraform.Template(moduleTemplate, map[string]interface{}{
		"Source":    source,
		"Variables": variables,
	})
	if err != nil {
		return err
	}

	// @step: open the file for writing
	wr, err := os.OpenFile(filepath.Join(o.Directory, "main.tf"), os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
	if err != nil {
		return err
	}

	scanner := bufio.NewScanner(bytes.NewReader(tmpl))
	for scanner.Scan() {
		line := scanner.Text()
		switch {
		case strings.HasPrefix(line, "  source = "):
			wr.WriteString(line + "\n\n")
		case line == "  ":
			break
		default:
			wr.WriteString(line + "\n")
		}
	}

	return nil
}
