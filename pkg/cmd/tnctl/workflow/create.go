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

package workflow

import (
	"context"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/AlecAivazis/survey/v2"
	"github.com/spf13/cobra"

	"github.com/appvia/terraform-controller/pkg/cmd"
	"github.com/appvia/terraform-controller/pkg/cmd/tnctl/workflow/assets"
	"github.com/appvia/terraform-controller/pkg/utils"
)

var longModuleHelp = `
Can be used to generate an opinionate pipeline for terraform modules.
The module command will generate a Github actions pipeline, integrating
linting, validating and security checks.

# Generate a workflow for module
$ tnctl workflow create PATH
`

// ModuleCommand defines the command line options for the command
type ModuleCommand struct {
	cmd.Factory
	// Branch is the default branch to use
	Branch string `survey:"branch"`
	// DryRun indicates we should print to screen rather than create the workflow
	DryRun bool
	// EnsurePolicyLinting is used to ensure the policy linting is enabled
	EnsurePolicyLinting bool `survey:"ensure_policy_linting"`
	// EnsureNameLinting indicates we should ensure the naming conventions of the repository
	EnsureNameLinting bool `survey:"ensure_name_linting"`
	// EnsureCommitLinting indicates we should ensure the commit message via a linter
	EnsureCommitLinting bool `survey:"ensure_commit_linting"`
	// PolicySource is the name the repository containing policies
	PolicySource string `survey:"policy_source"`
	// Provider is the provider to use i.e github actions etc
	Provider string `survey:"provider"`
	// Source is the directory to create the workflow in
	Source string `survey:"source"`
	// TerraformVersion is the version of terraform to use
	TerraformVersion string `survey:"terraform_version"`
	// Force is used to bypass the confirmation prompt
	Force bool
}

// NamingConvention is the naming convention for a repository^terraform-\\w-\\w$
var NamingConvention = regexp.MustCompile(`^terraform\-[\w]+\-[\w]+`)

// NewCreateCommand returns a new instance of the command
func NewCreateCommand(factory cmd.Factory) *cobra.Command {
	options := &ModuleCommand{Factory: factory}

	c := &cobra.Command{
		Use:   "create [PATH] [options]",
		Short: "Generates a workflow used to lint, validate and publish the module",
		Long:  strings.TrimSuffix(longModuleHelp, "\n"),
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			options.Source = args[0]

			return options.Run(cmd.Context())
		},
	}

	flags := c.Flags()
	flags.BoolVar(&options.DryRun, "dry-run", false, "Print the workflow to screen rather than create it")
	flags.BoolVar(&options.EnsureNameLinting, "ensure-naming-linting", true, "Ensure the naming conventions of the repository")
	flags.BoolVar(&options.EnsurePolicyLinting, "ensure-policy-linting", true, "Ensure the policy checks are enabled")
	flags.BoolVar(&options.Force, "force", false, "Indicates we always overwrite any existing workflows")
	flags.StringVar(&options.Branch, "branch", "master", "Default branch to use i.e. master or main")
	flags.StringVar(&options.Provider, "provider", "github", "The provider to use i.e github action, circleci, etc")
	flags.StringVar(&options.TerraformVersion, "terraform-version", "latest", "The version of terraform to use")

	cmd.RegisterFlagCompletionFunc(c, "branch", cmd.AutoCompleteWithList([]string{"master", "main"}))
	cmd.RegisterFlagCompletionFunc(c, "provider", cmd.AutoCompleteWithList([]string{"github"}))

	return c
}

// Run is called to execute the command
func (o *ModuleCommand) Run(ctx context.Context) error {
	repository := filepath.Base(o.Source)
	switch {
	case o.Source == "":
		return cmd.ErrMissingArgument("source")

	case o.Provider == "":
		return cmd.ErrMissingArgument("provider")

	case o.Source != "." && o.EnsureNameLinting && !NamingConvention.MatchString(repository):
		return fmt.Errorf("repository %q must conform to the naming convention: terraform-PROVIDER-NAME", repository)
	}

	// @step: ensure the source directory exists
	if found, err := utils.DirExists(o.Source); err != nil {
		return err
	} else if !found {
		if err := os.MkdirAll(o.Source, 0755); err != nil {
			return err
		}
		o.Println("%s Created the repository directory: %q", cmd.IconGood, o.Source)
	}

	// @step: prompt the user for questions
	if err := o.askQuestions(); err != nil {
		return err
	}

	for _, x := range assets.RecursiveAssetNames(o.Provider) {
		template, err := assets.Asset(x)
		if err != nil {
			return fmt.Errorf("failed to load template %q: %s", x, err)
		}

		content, err := utils.Template(string(template), map[string]interface{}{
			"Directory":        o.Source,
			"EnsureCommitLint": o.EnsureCommitLinting,
			"EnsurePolicyLint": o.EnsurePolicyLinting,
			"PolicySource":     o.PolicySource,
			"PolicyVersion":    "latest",
			"Provider":         o.Provider,
			"Terraform": map[string]interface{}{
				"Version": o.TerraformVersion,
			},
		})
		if err != nil {
			return fmt.Errorf("failed to template %s: %s", x, err)
		}
		if o.DryRun {
			o.Println("---")
			o.Println("%s", content)

			continue
		}
		trimmed := strings.TrimSuffix(filepath.Base(x), ".tpl")

		// @step: create the path
		paths := []string{o.Source}
		switch o.Provider {
		case "github":
			paths = append(paths, []string{".github", "workflows"}...)
		}
		filename := path.Join(append(paths, trimmed)...)

		if err := o.saveFile(ctx, filename, content); err != nil {
			return fmt.Errorf("failed to save file %q: %s", filename, err)
		}
	}

	// @step: generate the local content
	for _, x := range assets.RecursiveAssetNames("local") {
		template, err := assets.Asset(x)
		if err != nil {
			return fmt.Errorf("failed to load makefile: %s", err)
		}
		content, err := utils.Template(string(template), map[string]interface{}{
			"Directory": filepath.Base(o.Source),
		})
		if err != nil {
			return fmt.Errorf("failed to generate the %s: %s", x, err)
		}

		filename := path.Join(o.Source, strings.TrimSuffix(filepath.Base(x), ".tpl"))
		if err := o.saveFile(ctx, filename, content); err != nil {
			return fmt.Errorf("failed to save file %q: %s", filename, err)
		}
	}

	return nil
}

// saveFile is called to save the workflow to diskA
func (o *ModuleCommand) saveFile(_ context.Context, filename string, workflow []byte) error {
	if err := os.MkdirAll(filepath.Dir(filename), 0755); err != nil {
		return err
	}

	found, err := utils.FileExists(filename)
	if err != nil {
		return err
	}
	if found && !o.Force {
		if err := survey.AskOne(&survey.Confirm{
			Message: fmt.Sprintf("The file %q already exist, do you want to overwrite?", filename)}, &o.Force,
		); err != nil {
			return err
		}
		if !o.Force {
			o.Println("%s Skipping the update to %q", cmd.IconGood, filename)

			return nil
		}
	}
	if err := os.WriteFile(filename, []byte(workflow), 0644); err != nil {
		return err
	}
	o.Println("%s Saved file %q", cmd.IconGood, filename)

	return nil
}

// askQuestions is called to prompt the user for the options
func (o *ModuleCommand) askQuestions() error {
	questions := []*survey.Question{
		{
			Name: "ensure_policy_linting",
			Prompt: &survey.Confirm{
				Message: "Should we enable security checks on workflow?",
				Help:    "Enables Checkov security checks are enforced on the module",
				Default: true,
			},
		},
		{
			Name: "ensure_commit_linting",
			Prompt: &survey.Confirm{
				Message: "Should we enable commit message linting?",
				Help:    "Enforces all commit messages follow the convention of https://github.com/conventional-changelog/commitlint",
				Default: true,
			},
		},
	}
	if err := survey.Ask(questions, o); err != nil {
		return err
	}

	var confirm string
	err := survey.AskOne(&survey.Select{
		Message: "Does your organization has a central policy repository?",
		Help:    "Name of the central repository which contains your organizations security policies",
		Options: []string{"Yes", "No"},
		Default: "No"}, &confirm,
	)
	if err != nil {
		return err
	}

	switch confirm {
	case "Yes":
		err := survey.AskOne(&survey.Input{
			Message: "What is the name of the repository?",
			Help:    "Is the of the repository in your organization containing the security policies",
		}, &o.PolicySource)
		if err != nil {
			return err
		}
	}

	return nil
}
