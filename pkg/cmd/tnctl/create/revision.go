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

package create

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/AlecAivazis/survey/v2"
	"github.com/fatih/color"
	"github.com/hashicorp/terraform-config-inspect/tfconfig"
	"github.com/spf13/cobra"
	"k8s.io/utils/ptr"

	terraformv1alpha1 "github.com/appvia/terranetes-controller/pkg/apis/terraform/v1alpha1"
	"github.com/appvia/terranetes-controller/pkg/cmd"
	"github.com/appvia/terranetes-controller/pkg/cmd/tnctl/create/assets"
	"github.com/appvia/terranetes-controller/pkg/utils"
	"github.com/appvia/terranetes-controller/pkg/utils/template"
	"github.com/appvia/terranetes-controller/pkg/version"
)

// RevisionCommand are the options for the command
type RevisionCommand struct {
	cmd.Factory
	// Name is the name of the revision
	Name string
	// Description is a description of the revision
	Description string
	// EnableDefaultVariables indicates if we should enable the default variables
	EnableDefaultVariables bool
	// Contexts is a list of contexts from the cluster
	Contexts *terraformv1alpha1.ContextList
	// Policies is a list of policies from the cluster
	Policies *terraformv1alpha1.PolicyList
	// Plans is a collection of plans already in the cluster
	Plans *terraformv1alpha1.PlanList
	// Revisions is a collection of revisions already in the cluster
	Revisions *terraformv1alpha1.RevisionList
	// Providers is a collection of providers in the cluster
	Providers *terraformv1alpha1.ProviderList
	// Inputs is a list of inputs for the revision
	Inputs []Input
	// Variables are the module variables
	Variables map[string]interface{}
	// ValueFrom is a list of value froms
	ValueFrom []terraformv1alpha1.ValueFromSource
	// Output are the outputs from the module
	Outputs []string
	// Module is the module to create the revision from
	Module string
	// Revision is the version of the revision
	Revision string
	// File is where to save the revision
	File string
	// Provider is the name of the provider to use
	Provider string
  // DeleteDownload indicates we should retain the download
	DeleteDownload bool
}

var revisionCommandDesc = `
Create a terranetes revision from a terraform module. The command will
retrieve the module code if required, parse the attributes from the code
and build a plan.

Create a terranetes revision from the current directory
$ tnctl create revision .

Create a terranetes revision from a terraform module in a git repository
$ tnctl create revision -n test.01 -m https://examples.com/terraform-module.git?ref=v1.0.0
`

// NewRevisionCommand creates a new command
func NewRevisionCommand(factory cmd.Factory) *cobra.Command {
	o := &RevisionCommand{Factory: factory, Variables: make(map[string]interface{})}

	c := &cobra.Command{
		Use:   "revision [OPTIONS] MODULE",
		Args:  cobra.MinimumNArgs(1),
		Short: "Used to create terranetes revision from a terraform module",
		Long:  revisionCommandDesc,
		RunE: func(cmd *cobra.Command, args []string) error {
			o.Module = args[0]

			return o.Run(cmd.Context())
		},
	}
	c.SetErr(o.GetStreams().ErrOut)
	c.SetIn(o.GetStreams().In)
	c.SetOut(o.GetStreams().Out)

	flags := c.Flags()
	flags.BoolVar(&o.EnableDefaultVariables, "enable-default-variables", true, "Indicates if include variables which have defaults from the terraform module")
	flags.BoolVar(&o.DeleteDownload, "delete-download", true, "Indicates if we should delete the download after the command has completed")
	flags.StringVar(&o.Description, "description", "", "A human readable description of the revision and what is provides")
	flags.StringVarP(&o.Name, "name", "n", "", "This name of the revision")
	flags.StringVarP(&o.Revision, "revision", "r", "", "The semvar version of this revision")
	flags.StringVarP(&o.File, "file", "f", "", "The path to save the revision to")
	flags.StringVar(&o.Provider, "provider", "aws", "The name of the terranetes provider to use")

	_ = c.MarkFlagRequired("file")

	return c
}

// Run runs the command
func (o *RevisionCommand) Run(ctx context.Context) (err error) {
	// @step: download the terraform module
	path, delete, err := o.Download(ctx, o.Module)
	if err != nil {
		return err
	}
	defer func() {
		if delete {
			retain := err

			if o.DeleteDownload {
				err = os.RemoveAll(path)
			}
			if retain != nil {
				err = retain
			}
		}
	}()
	o.Println("%s Successfully downloaded module to: %s", cmd.IconGood, path)

	// @step: we need to parse the terraform code
	module, diag := tfconfig.LoadModule(path)
	if diag.HasErrors() {
		return fmt.Errorf("failed to load terraform module: %w", diag.Err())
	}

	if err := o.retrieveConfiguration(ctx); err != nil {
		return err
	}
	// @step: we need to ask or guess the plan name
	if err := o.retrievePlan(); err != nil {
		return err
	}
	// @step: we need to ask or guess the revision name
	if err := o.retrieveRevision(); err != nil {
		return err
	}
	// @step: retrieve the inputs
	if err := o.retrieveInputs(module); err != nil {
		return err
	}
	// @step: retrieve the outputs
	if err := o.retrieveOutputs(module); err != nil {
		return err
	}

	// @step: generate the revision
	tpl := assets.MustAsset("tnctl.revision.yaml.tpl")
	generated, err := template.NewWithBytes(tpl, map[string]interface{}{
		"Annotations": map[string]string{
			"terranetes.appvia.io/tnctl.version": version.Version,
		},
		"Labels": map[string]string{},
		"Configuration": map[string]interface{}{
			"ChangeLog": "",
			"Module":    o.Module,
			"Outputs":   o.Outputs,
			"Provider":  o.Provider,
			"ValueFrom": o.ValueFrom,
			"Variables": o.Variables,
		},
		"Inputs": o.Inputs,
		"Plan": map[string]interface{}{
			"Name":        o.Name,
			"Description": o.Description,
			"Revision":    o.Revision,
		},
	})
	if err != nil {
		return err
	}

	return o.renderRevision(generated)
}

// renderRevision is used to render the revision
func (o *RevisionCommand) renderRevision(revision []byte) error {
	if o.File == "" {
		o.Println("%s", revision)

		return nil
	}

	// @step: open and write the revision
	wr, err := os.OpenFile(o.File, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0600)
	if err != nil {
		return err
	}
	if _, err := wr.Write(revision); err != nil {
		return err
	}
	o.Println("%s Successfully written revision to: %s", cmd.IconGood, o.File)

	return nil
}

// retrieveOutputs is used to retrieve the outputs from the module
func (o *RevisionCommand) retrieveOutputs(module *tfconfig.Module) error {
	switch {
	case len(o.Outputs) > 0:
		return nil
	case len(module.Outputs) == 0:
		return nil
	}

	var suggestions []string
	var length int

	for _, x := range module.Outputs {
		if len(x.Name) > length {
			length = len(x.Name)
		}
	}
	format := fmt.Sprintf(`%%-%ds %%s`, (length + 5))

	for _, x := range module.Outputs {
		suggestions = append(suggestions, fmt.Sprintf(format, x.Name, utils.MaxChars(x.Description, 60)))
	}

	// @step: ask the user which outputs should exposed
	var selected []string
	if err := survey.AskOne(&survey.MultiSelect{
		Message:  "What outputs should be extract into the secret?",
		Options:  suggestions,
		PageSize: 20,
	}, &selected, survey.WithKeepFilter(false)); err != nil {
		return err
	}
	for _, x := range selected {
		o.Outputs = append(o.Outputs, strings.TrimSpace(strings.Split(x, " ")[0]))
	}

	return nil
}

// retrieveRevision helps the user retrieve the revision name
func (o *RevisionCommand) retrieveRevision() error {
	if o.Revision != "" {
		return nil
	}

	if o.Plans == nil {
		o.Plans = &terraformv1alpha1.PlanList{}
	}

	plan, found := o.Plans.GetItem(o.Name)
	if !found {
		if err := survey.AskOne(&survey.Input{
			Message: fmt.Sprintf("What is the version of this %s (in semver format)?", color.YellowString("revision")),
			Help:    "Revisions must have a version, cloud resource reference both the plan and the version",
			Default: "v0.0.1",
		}, &o.Revision); err != nil {
			return err
		}

		return nil
	}
	o.Revision = "REVISION"

	// @step: increment the from the last revision
	if version, err := utils.GetVersionIncrement(plan.Status.Latest.Revision); err == nil {
		o.Revision = version
	}

	return nil
}

// retrieveInputs helps the user retrieve the inputs
func (o *RevisionCommand) retrieveInputs(module *tfconfig.Module) error {
	var length int

	// @step: if the inputs is not empty, we skip asking the user
	if len(o.Inputs) > 0 {
		return nil
	}

	// @step: calculate the max variable size - just of spacing
	for _, x := range module.Variables {
		if len(x.Name) > length {
			length = len(x.Name)
		}
	}
	format := fmt.Sprintf(`%%-%ds (%%s) %%s`, (length + 5))

	var required, optional []string

	// @step: else we need to ask the user for the inputs
	for _, x := range module.Variables {
		if x.Required && x.Default == nil {
			required = append(required, fmt.Sprintf(format, x.Name,
				color.RedString("Required"),
				utils.MaxChars(x.Description, 60),
			))
		} else {
			optional = append(optional, fmt.Sprintf(format, x.Name,
				color.YellowString("Optional"),
				utils.MaxChars(x.Description, 60),
			))
		}
	}

	// @step: if we have nothing to select from, we can skip
	if len(required) == 0 && len(optional) == 0 {
		return nil
	}

	var selected []string
	if err := survey.AskOne(&survey.MultiSelect{
		Message:  "What variables should be exposed to the developers?",
		Help:     "Choose the variables you want to expose to the developers",
		Options:  append(required, (utils.Sorted(optional))...),
		PageSize: 15,
		Default:  required,
	}, &selected, survey.WithKeepFilter(false)); err != nil {
		return err
	}

	isVariableSelected := func(name string) bool {
		for _, x := range selected {
			if name == strings.Split(x, " ")[0] {
				return true
			}
		}

		return false
	}

	// @step: inject the inputs
	for _, variable := range module.Variables {

		switch {
		case !isVariableSelected(variable.Name):
			if o.EnableDefaultVariables {
				if variable.Default != nil {
					if m, ok := variable.Default.(map[string]interface{}); ok {
						if len(m) == 0 {
							continue
						}
					}
					o.Variables[variable.Name] = variable.Default
				}
			}

		default:
			// @step: check if we have any suggestions from the contexts
			input, found := SuggestContextualInput(variable.Description, o.Contexts, 0.2)
			if !found {
				o.Inputs = append(o.Inputs, Input{
					Default: map[string]interface{}{
						"example": variable.Type,
						"value":   variable.Default,
					},
					Description: variable.Description,
					Key:         variable.Name,
					Required:    variable.Required,
					Type:        variable.Type,
				})
			} else {
				o.ValueFrom = append(o.ValueFrom, terraformv1alpha1.ValueFromSource{
					Context: ptr.To(input.Context),
					Key:     input.Key,
					Name:    variable.Name,
				})
			}
		}
	}

	return nil
}

// retrievePlan is used to retrieve the name of the plan
func (o *RevisionCommand) retrievePlan() error {
	switch {
	case o.Name != "":
		return nil

	case o.Plans == nil:
		if err := survey.AskOne(&survey.Input{
			Message: fmt.Sprintf("What is the name of the %s this revision will be part of?", color.YellowString("plan")),
			Help:    "Revisions are grouped by plan names, i.e. mysql-database, redis-cluster and so on",
			Default: "my-plan",
		}, &o.Name); err != nil {
			return nil
		}

		return nil
	}
	if len(o.Plans.Items) == 0 {
		if err := survey.AskOne(&survey.Input{
			Message: fmt.Sprintf("Enter the name of the %s this revision will be part of?", color.YellowString("plan")),
			Help:    "Revisions are grouped by plan names, i.e. mysql-database, redis-cluster and so on",
			Default: "my-plan",
		}, &o.Name); err != nil {
			return nil
		}

		return nil
	}

	// @step: else we have plans already in the cluster, lets try and use them
	var list []string
	for _, x := range o.Plans.Items {
		list = append(list, x.Name)
	}

	// @step: we an produce a list from the current plans
	if err := survey.AskOne(&survey.Select{
		Message: fmt.Sprintf("The cluster already contains plans, will the %s will be part of?",
			color.YellowString("revision"),
		),
		Options: append(list, "None of these..."),
	}, &o.Name); err != nil {
		return nil
	}

	// @step: if no name is defined we need to prompt the user
	if !utils.Contains(o.Name, list) {
		if err := survey.AskOne(&survey.Input{
			Message: fmt.Sprintf("Enter the name of the %s this revision will be part of?", color.YellowString("plan")),
			Help:    "Revisions are grouped by plan names, i.e. mysql-database, redis-cluster and so on",
			Default: "my-plan",
		}, &o.Name); err != nil {

			return nil
		}
	}

	if o.Name == "" {
		return errors.New("you must provide a name for the plan")
	}

	return nil
}

// retrieveConfiguration is responsible for retrieving such as policies, contexts, plans etc from
// the current kubeconfig
func (o *RevisionCommand) retrieveConfiguration(ctx context.Context) error {
	switch {
	case o.Contexts == nil:
	case o.Plans == nil:
	case o.Policies == nil:
	case o.Providers == nil:
	case o.Revisions == nil:
	default:
		return nil
	}

	var answer bool

	if err := survey.AskOne(&survey.Confirm{
		Message: fmt.Sprintf("Can we use the %s to retrieve terranetes configuration?", color.YellowString("current kubeconfig")),
		Help:    "We will interrogate the cluster, retrieving providers, contexts and policies",
		Default: true,
	}, &answer); err != nil {
		return err
	}
	if !answer {
		return nil
	}

	// @step: retrieve a client
	cc, err := o.GetClient()
	if err != nil {
		return fmt.Errorf("failed to create client on current kubeconfig: %w", err)
	}

	// @step: retrieve the various items
	if o.Contexts == nil {
		o.Contexts = &terraformv1alpha1.ContextList{}
		if err := cc.List(ctx, o.Contexts); err != nil {
			return fmt.Errorf("failed to retrieves contexts from cluster: %w", err)
		}
	}
	if o.Policies == nil {
		o.Policies = &terraformv1alpha1.PolicyList{}
		if err := cc.List(ctx, o.Policies); err != nil {
			return fmt.Errorf("failed to retrieves policies from cluster: %w", err)
		}
	}
	if o.Providers == nil {
		o.Providers = &terraformv1alpha1.ProviderList{}
		if err := cc.List(ctx, o.Providers); err != nil {
			return fmt.Errorf("failed to retrieves providers from cluster: %w", err)
		}
	}
	if o.Revisions == nil {
		o.Revisions = &terraformv1alpha1.RevisionList{}
		if err := cc.List(ctx, o.Revisions); err != nil {
			return fmt.Errorf("failed to retrieves revisions from cluster: %w", err)
		}
	}
	if o.Plans == nil {
		o.Plans = &terraformv1alpha1.PlanList{}
		if err := cc.List(ctx, o.Plans); err != nil {
			return fmt.Errorf("failed to retrieves plans from cluster: %w", err)
		}
	}

	return nil
}

// Download is responsible for downloading the modules
func (o *RevisionCommand) Download(ctx context.Context, module string) (string, bool, error) {
	var path string
	var err error

	switch o.Module {
	case "":
		return "", false, errors.New("module is required")

	case ".":
		path, err = os.Getwd()
		if err != nil {
			return "", false, fmt.Errorf("failed to get current directory: %w", err)
		}

		return path, false, nil

	default:
		path = utils.TempDirName()
		if err := utils.Download(ctx, o.Module, path); err != nil {
			return "", false, err
		}
	}

	return path, true, nil
}
