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

package convert

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/AlecAivazis/survey/v2"
	"github.com/fatih/color"
	"github.com/spf13/cobra"
	"github.com/tidwall/sjson"
	"k8s.io/apimachinery/pkg/runtime"

	terraformv1alpha1 "github.com/appvia/terranetes-controller/pkg/apis/terraform/v1alpha1"
	"github.com/appvia/terranetes-controller/pkg/cmd"
	"github.com/appvia/terranetes-controller/pkg/utils"
	"github.com/appvia/terranetes-controller/pkg/utils/kubernetes"
)

// RevisionCommand are the options for the command
type RevisionCommand struct {
	cmd.Factory
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
	// File is the path to the file to containing the revision
	File string
	// Revision the revision we are converting
	Revision *terraformv1alpha1.Revision
	// Contexts is a list of contexts from the cluster
	Contexts *terraformv1alpha1.ContextList
	// Policies is a list of policies from the cluster
	Policies *terraformv1alpha1.PolicyList
	// Providers is a collection of providers in the cluster
	Providers *terraformv1alpha1.ProviderList
	// configuration is the generated configuration
	configuration *terraformv1alpha1.Configuration
}

// NewRevisionCommand creates and returns a new command
func NewRevisionCommand(factory cmd.Factory) *cobra.Command {
	o := &RevisionCommand{Factory: factory}

	ccm := &cobra.Command{
		Use:   "revision [OPTIONS] [NAME|--file PATH]",
		Short: "Used to convert revision back to terraform",
		Long:  strings.TrimPrefix(longDescription, "\n"),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) > 0 {
				o.Name = args[0]
			}

			return o.Run(cmd.Context())
		},
	}
	ccm.SetErr(o.GetStreams().ErrOut)
	ccm.SetIn(o.GetStreams().In)
	ccm.SetOut(o.GetStreams().Out)

	flags := ccm.Flags()
	flags.BoolVar(&o.IncludeCheckov, "include-checkov", true, "Include checkov in the output")
	flags.BoolVar(&o.IncludeProvider, "include-provider", true, "Include provider in the output")
	flags.StringVarP(&o.Directory, "path", "p", ".", "The path to write the files to")
	flags.StringVarP(&o.File, "file", "f", "", "The path to the file containing the revision")
	flags.StringVarP(&o.Namespace, "namespace", "n", "", "The namespace of the revision")

	cmd.RegisterFlagCompletionFunc(ccm, "namespace", cmd.AutoCompleteNamespaces(factory))

	return ccm
}

// Run implements the command
func (o *RevisionCommand) Run(ctx context.Context) error {
	switch {
	case o.File != "":
		break
	case o.Name != "" && o.Namespace != "":
		break
	case o.Revision != nil:
		break
	default:
		return errors.New("either file or name and namespace must be provided")
	}

	// @step: retrieve the revision from file or kubernetes
	if err := o.retrieveRevision(ctx); err != nil {
		return err
	}
	// @step: retrieve the inputs
	if err := o.retrieveInputs(ctx); err != nil {
		return err
	}
	// @step: we first convert to a Configuration - and pass on to the configuration
	// convert command to handle the rest
	if err := o.renderConfiguration(ctx); err != nil {
		return err
	}
	// @step: render a cloud resource file
	if err := o.renderCloudResource(ctx); err != nil {
		return err
	}

	return (&ConfigurationCommand{
		Factory:          o.Factory,
		Configuration:    o.configuration,
		Contexts:         o.Contexts,
		Directory:        o.Directory,
		IncludeCheckov:   o.IncludeCheckov,
		IncludeProvider:  o.IncludeProvider,
		IncludeTerraform: true,
		Policies:         o.Policies,
		Providers:        o.Providers,
	}).Run(ctx)
}

// renderCloudResource is responsible for rendering the cloud resource
func (o *RevisionCommand) renderCloudResource(_ context.Context) error {
	cloudresource := terraformv1alpha1.NewCloudResource("", "")
	cloudresource.Name = o.Revision.Name
	cloudresource.Spec.Plan.Name = o.Revision.Spec.Plan.Name
	cloudresource.Spec.Plan.Revision = o.Revision.Spec.Plan.Revision
	cloudresource.Spec.ProviderRef = o.Revision.Spec.Configuration.ProviderRef

	values := make(map[string]interface{})
	for _, v := range o.Revision.Spec.Inputs {
		if v.Default == nil {
			continue
		}
		value, found, err := o.Revision.Spec.GetInputDefaultValue(v.Key)
		if err != nil {
			return err
		}
		if !found {
			continue
		}
		values[v.Key] = value
	}

	if len(values) > 0 {
		wr := &bytes.Buffer{}
		if err := json.NewEncoder(wr).Encode(values); err != nil {
			return err
		}
		cloudresource.Spec.Variables = &runtime.RawExtension{}
		cloudresource.Spec.Variables.Raw = wr.Bytes()
	}

	// @step: render the cloud resource
	filename := filepath.Join(o.Directory, "cloudresource.yaml")
	if err := utils.WriteYAML(filename, cloudresource); err != nil {
		return err
	}

	return nil
}

// retrieveInputs retrieves the inputs if the revision has user defined inputs
func (o *RevisionCommand) retrieveInputs(_ context.Context) error {
	if len(o.Revision.Spec.Inputs) == 0 {
		return nil
	}

	// @step: check if there are any complex types in the inputs as prompting for these
	// via the CLI will be horrid; it better to ask them to edit and fill in
	for i, input := range o.Revision.Spec.Inputs {
		switch {
		case input.Required == nil:
			continue
		case input.Default != nil:
			continue
		case input.Type == nil:
			continue
		}

		switch *input.Type {
		case "string", "number", "bool":
			break
		case "":
			return fmt.Errorf("spec.inputs[%d].type is required (if a complex type and no default set, use an editor to define)", i)
		default:
			return fmt.Errorf("spec.inputs[%d] is a complex type, please use an editor to specify a default", i)
		}
	}

	for i, input := range o.Revision.Spec.Inputs {
		switch {
		case input.Required == nil:
			continue
		case input.Default != nil:
			continue
		case input.Type == nil:
			continue
		case !utils.Contains(*input.Type, []string{"string", "number", "bool"}):
			continue
		}

		// @step: else we need to ask the user
		var selected string
		if err := survey.AskOne(&survey.Input{
			Message: fmt.Sprintf("Input %s is a mandatory input, what should it's value be?", color.YellowString(input.Key)),
			Help:    input.Description,
		}, &selected, survey.WithValidator(survey.Required), survey.WithKeepFilter(false)); err != nil {
			return err
		}

		switch *input.Type {
		case "bool":
			o.Revision.Spec.Inputs[i].Default = &runtime.RawExtension{
				Raw: []byte(fmt.Sprintf(`{"value":%T}`, selected)),
			}

		case "number":
			number, err := strconv.ParseInt(selected, 10, 64)
			if err != nil {
				return err
			}
			o.Revision.Spec.Inputs[i].Default = &runtime.RawExtension{
				Raw: []byte(fmt.Sprintf(`{"value":%d}`, number)),
			}

		case "string":
			o.Revision.Spec.Inputs[i].Default = &runtime.RawExtension{
				Raw: []byte(`{"value": "` + selected + `"}`),
			}
		}
	}

	return nil
}

// renderConfiguration renders the configuration from the revision
func (o *RevisionCommand) renderConfiguration(_ context.Context) error {
	// @step: we first convert to a Configuration - and pass on to the configuration
	// convert command to handle the rest
	configuration := &terraformv1alpha1.Configuration{}
	configuration.Name = o.Revision.Name
	configuration.Namespace = "default"
	configuration.Annotations = o.Revision.Annotations
	configuration.Labels = o.Revision.Labels
	configuration.Spec = o.Revision.Spec.Configuration
	o.configuration = configuration

	// @step: add this inputs if any
	if o.Revision.Spec.Inputs == nil {
		return nil
	}

	for _, input := range o.Revision.Spec.Inputs {
		if input.Default == nil {
			continue
		}

		value, found, err := o.Revision.Spec.GetInputDefaultValue(input.Key)
		if err != nil {
			return err
		}
		if !found {
			continue
		}

		configuration.Spec.Variables.Raw, err = sjson.SetBytes(configuration.Spec.Variables.Raw, input.Key, value)
		if err != nil {
			return err
		}
	}

	return nil
}

// retrieveRevision retrieves the revision from the file or kubernetes
func (o *RevisionCommand) retrieveRevision(ctx context.Context) error {
	switch {
	case o.Revision != nil:
		return nil
	case o.File != "":
		o.Revision = &terraformv1alpha1.Revision{}

		return utils.LoadYAML(o.File, o.Revision)
	}

	o.Revision = &terraformv1alpha1.Revision{}
	o.Revision.Namespace = o.Namespace
	o.Revision.Name = o.Name

	cc, err := o.GetClient()
	if err != nil {
		return err
	}

	if found, err := kubernetes.GetIfExists(ctx, cc, o.Revision); err != nil {
		return err
	} else if !found {
		return errors.New("revision not found")
	}

	return nil
}
