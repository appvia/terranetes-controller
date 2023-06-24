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

package get

import (
	"fmt"

	"github.com/spf13/cobra"

	terraformv1alpha1 "github.com/appvia/terranetes-controller/pkg/apis/terraform/v1alpha1"
	"github.com/appvia/terranetes-controller/pkg/cmd"
)

// NewCommand creates and returns a new command
func NewCommand(factory cmd.Factory) *cobra.Command {
	c := &cobra.Command{
		Use:   "get KIND",
		Short: "Used to retrieve a list of resources",
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmd.Help()
		},
	}

	for _, resource := range []string{
		"cloudresources",
		"configurations",
		"contexts",
		"policies",
		"plans",
		"revisions",
	} {
		c.AddCommand(
			NewGetResourceCommand(factory,
				fmt.Sprintf("%s.%s", resource, terraformv1alpha1.SchemeGroupVersion.Group),
			),
		)
	}

	return c
}
