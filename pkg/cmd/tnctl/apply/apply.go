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

package apply

import (
	"fmt"
	"os/exec"

	"github.com/spf13/cobra"

	"github.com/appvia/terranetes-controller/pkg/cmd"
)

// Command are the options for the command
type Command struct {
	cmd.Factory
	// File is the path to the file to apply
	File []string
	// Namespace is the namespace to apply the resources to
	Namespace string
	// DryRun is the dry run strategy
	DryRun string
	// Force is the force flag
	Force bool
}

// NewCommand creates and returns a new command
func NewCommand(factory cmd.Factory) *cobra.Command {
	o := &Command{Factory: factory}

	c := &cobra.Command{
		Use:   "apply [OPTIONS] [-f PATH...]",
		Short: "Used to apply from a file or stdin",
		RunE: func(cmd *cobra.Command, args []string) error {
			options := []string{"apply"}

			for _, file := range o.File {
				options = append(options, "-f", file)
			}
			if o.Force {
				options = append(options, "--force")
			}
			if o.Namespace != "" {
				options = append(options, fmt.Sprintf("--namespace=%s", o.Namespace))
			}
			if o.DryRun != "" {
				options = append(options, "--dry-run "+o.DryRun)
			}

			exe := exec.CommandContext(cmd.Context(), "kubectl", options...)
			exe.Stdout = factory.GetStreams().Out
			exe.Stderr = factory.GetStreams().ErrOut
			exe.Stdin = factory.GetStreams().In

			return exe.Run()
		},
	}

	flags := c.Flags()
	flags.StringSliceVarP(&o.File, "file", "f", []string{}, "Path to file to apply")
	flags.StringVarP(&o.Namespace, "namespace", "n", "", "Namespace the resources will applied to")
	flags.StringVar(&o.DryRun, "dry-run", "", `Must be "none", "server", or "client". If client strategy, only print the object that would be sent, without sending it`)
	flags.BoolVarP(&o.Force, "force", "", false, "If true, immediately remove resources from API and bypass graceful deletion")

	return c
}
