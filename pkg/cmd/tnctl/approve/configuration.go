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

package approve

import (
	"github.com/spf13/cobra"

	terraformv1alpha1 "github.com/appvia/terranetes-controller/pkg/apis/terraform/v1alpha1"
	"github.com/appvia/terranetes-controller/pkg/cmd"
)

// NewApproveConfigurationCommand creates and returns the command
func NewApproveConfigurationCommand(factory cmd.Factory) *cobra.Command {
	o := &Command{Factory: factory}

	c := &cobra.Command{
		Use:   "configuration [OPTIONS] NAME",
		Long:  longDescription,
		Short: "Approves a configuration for release",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			o.Names = args
			o.Kind = terraformv1alpha1.ConfigurationKind

			return o.Run(cmd.Context())
		},
		ValidArgsFunction: cmd.AutoCompleteConfigurations(factory),
	}

	flags := c.Flags()
	flags.StringVarP(&o.Namespace, "namespace", "n", "default", "Namespace of the resource/s")

	cmd.RegisterFlagCompletionFunc(c, "namespace", cmd.AutoCompleteNamespaces(factory))

	return c
}
