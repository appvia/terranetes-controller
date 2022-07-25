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
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/spf13/cobra"

	"github.com/appvia/terranetes-controller/pkg/cmd"
	"github.com/appvia/terranetes-controller/pkg/utils"
)

var longModuleHelp = `
Can be used to generate an opinionate pipeline for terraform modules.
The module command will generate a Github actions pipeline, integrating
linting, validating and security checks.

Generate a workflow for module
$ tnctl workflow create PATH

You can override the location of the template via the configuration
file ${HOME}/.tnctl/config.yaml (or TNCTL_CONFIG). Just add the
following

---
workflow: URL
`

// ModuleCommand defines the command line options for the command
type ModuleCommand struct {
	cmd.Factory
	// EnsureNameLinting is used to enable the linting of the module name
	EnsureNameLinting bool
	// Template is the repository to use for the template
	Template string
	// Source is the directory to create the workflow in
	Source string
}

// NamingConvention is the naming convention for a repository^terraform-\\w-\\w$
var NamingConvention = regexp.MustCompile(`^terraform\-[\w]+\-[\w]+`)

// NewCreateCommand returns a new instance of the command
func NewCreateCommand(factory cmd.Factory) *cobra.Command {
	options := &ModuleCommand{Factory: factory}

	c := &cobra.Command{
		Use:   "create PATH [OPTIONS]",
		Short: "Generates a workflow used to lint, validate and publish the module",
		Long:  strings.TrimSuffix(longModuleHelp, "\n"),
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			options.Source = args[0]

			return options.Run(cmd.Context())
		},
	}

	flags := c.Flags()
	flags.BoolVar(&options.EnsureNameLinting, "ensure-naming-linting", true, "Ensure the naming conventions of the repository")
	flags.StringVar(&options.Template, "template", "git::ssh://git@github.com/appvia/terranetes-workflows?ref=master", "Repository to use for the template")

	return c
}

// Run is called to execute the action
func (o *ModuleCommand) Run(ctx context.Context) error {
	config, found, err := o.GetConfig()
	if err != nil {
		return err
	}
	if found && config.Workflow != "" {
		o.Template = config.Workflow
	}

	switch {
	case o.Source == "":
		return cmd.ErrMissingArgument("source")

	case o.Source != "." && o.EnsureNameLinting && !NamingConvention.MatchString(filepath.Base(o.Source)):
		return fmt.Errorf("repository %q must conform to the naming convention: terraform-PROVIDER-NAME", filepath.Base(o.Source))
	}

	// @step: ensure the source directory exists
	found, err = utils.DirExists(o.Source)
	if err != nil {
		return err
	}
	if !found {
		if err := os.MkdirAll(o.Source, 0755); err != nil {
			return err
		}
		o.Println("%s Created the repository directory: %q", cmd.IconGood, o.Source)
	}

	// @step: download the template repository
	tmpdir := utils.TempDirName()
	if err := utils.Download(ctx, o.Template, tmpdir); err != nil {
		return fmt.Errorf("failed to download the template repository: %v", err)
	}
	defer func() {
		if err := os.RemoveAll(tmpdir); err != nil {
			o.Println("%s Failed to remove the temporary directory: %q", cmd.IconBad, tmpdir)
		}
	}()

	args := []string{
		"-r", "--exclude", ".git",
		tmpdir + "/", o.Source,
	}
	if err := exec.Command("rsync", args...).Run(); err != nil {
		return fmt.Errorf("failed to copy the template repository: %v", err)
	}

	o.Println("%s Successfully created the workflow in %s", cmd.IconGood, o.Source)

	return nil
}
