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

package retry

import (
	"github.com/spf13/cobra"

	terraformv1alpha1 "github.com/appvia/terranetes-controller/pkg/apis/terraform/v1alpha1"
	"github.com/appvia/terranetes-controller/pkg/cmd"
)

// NewRetryCloudResourceCommand creates and returns the command
func NewRetryCloudResourceCommand(factory cmd.Factory) *cobra.Command {
	o := &Command{Factory: factory}

	c := &cobra.Command{
		Use:   "cloudresource [OPTIONS] NAME",
		Long:  longUsage,
		Short: "Attempts to restart a cloud resource",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			o.Name = args[0]
			o.Kind = terraformv1alpha1.CloudResourceKind

			return o.Run(cmd.Context())
		},
		ValidArgsFunction: cmd.AutoCompleteCloudresources(factory),
	}

	flags := c.Flags()
	flags.BoolVarP(&o.WatchLogs, "watch", "w", true, "Watch the logs after restarting the resource")
	flags.StringVarP(&o.Namespace, "namespace", "n", "default", "The namespace the resource resides")

	cmd.RegisterFlagCompletionFunc(c, "namespace", cmd.AutoCompleteNamespaces(factory))

	return c
}
