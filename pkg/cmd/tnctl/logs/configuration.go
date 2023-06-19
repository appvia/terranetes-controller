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

package logs

import (
	"time"

	"github.com/spf13/cobra"

	"github.com/appvia/terranetes-controller/pkg/cmd"
)

// NewConfigurationLogsCommand returns a new instance of the get command
func NewConfigurationLogsCommand(factory cmd.Factory) *cobra.Command {
	o := &Command{Factory: factory}

	c := &cobra.Command{
		Use:     "configuration NAME [OPTIONS]",
		Short:   "Displays the latest logs for the given resource",
		Long:    longDescription,
		PreRunE: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			o.Name = args[0]

			return o.Run(cmd.Context())
		},
		ValidArgsFunction: cmd.AutoCompleteConfigurations(factory),
	}
	c.SetIn(o.GetStreams().In)
	c.SetErr(o.GetStreams().ErrOut)
	c.SetOut(o.GetStreams().Out)

	flags := c.Flags()
	flags.BoolVarP(&o.Follow, "follow", "f", false, "Indicates we should follow the logs")
	flags.DurationVar(&o.WaitInterval, "timeout", 3*time.Second, "Indicates how long we should wait for logs to be available")
	flags.StringVar(&o.Stage, "stage", "", "Select the stage to show logs for, else defaults to the current state")
	flags.StringVarP(&o.Namespace, "namespace", "n", "default", "The namespace of the resource")

	cmd.RegisterFlagCompletionFunc(c, "namespace", cmd.AutoCompleteNamespaces(factory))
	cmd.RegisterFlagCompletionFunc(c, "stage", cmd.AutoCompletionStages())

	return c
}
