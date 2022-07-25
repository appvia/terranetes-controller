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
	"strings"

	"github.com/spf13/cobra"

	"github.com/appvia/terranetes-controller/pkg/cmd"
)

// Command represents the available get command options
type Command struct {
	cmd.Factory
}

var longDescription = `
Workflow provide an out of the box solution to generating ci
pipelines for your terraform modules. The pipelines are coded
to enforce, linting, validation, documentation generation
and security scanning. Also when enabled the pipeline will
also include a release.

Generate a pipeline for a terraform module
$ tnctl workflow create PATH
`

// NewCommand returns a new instance of the get command
func NewCommand(factory cmd.Factory) *cobra.Command {
	c := &cobra.Command{
		Use:   "workflow COMMAND",
		Short: "Can be used to generate a skelton CI pipeline",
		Long:  strings.TrimPrefix(longDescription, "\n"),
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmd.Help()
		},
	}

	c.AddCommand(
		NewCreateCommand(factory),
	)

	return c
}
