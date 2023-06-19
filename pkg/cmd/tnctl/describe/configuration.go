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

package describe

import (
	"strings"

	"github.com/appvia/terranetes-controller/pkg/cmd"
	"github.com/spf13/cobra"
)

// NewDescribeConfigurationCommand returns a new instance of the get command
func NewDescribeConfigurationCommand(factory cmd.Factory) *cobra.Command {
	o := &Command{Factory: factory}

	c := &cobra.Command{
		Use:     "configuration [OPTIONS] NAME",
		Args:    cobra.MaximumNArgs(1),
		Short:   "Used to describe the current state of the resources",
		Long:    strings.TrimPrefix(longDescription, "\n"),
		PreRunE: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			o.Name = args[0]

			return o.Run(cmd.Context())
		},
		ValidArgsFunction: cmd.AutoCompleteConfigurations(o.Factory),
	}

	flags := c.Flags()
	flags.BoolVar(&o.ShowPassedChecks, "show-passed-checks", true, "Indicates we should show passed checks")
	flags.StringVarP(&o.Namespace, "namespace", "n", "default", "Namespace of the resource/s")

	cmd.RegisterFlagCompletionFunc(c, "namespace", cmd.AutoCompleteNamespaces(factory))

	return c
}
