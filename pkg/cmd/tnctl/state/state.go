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

package state

import (
	"github.com/spf13/cobra"

	"github.com/appvia/terranetes-controller/pkg/cmd"
)

// Command returns the cobra command
type Command struct {
	cmd.Factory
}

var longDesc = `
When using the kubernetes backend to store the terraform state, this
command provides the ability to list, clean and match up state secrets
against the Configuration CRD which are using them.
`

// NewCommand returns a new instance of the command
func NewCommand(factory cmd.Factory) *cobra.Command {
	c := &cobra.Command{
		Use:   "state [COMMAND]",
		Long:  longDesc,
		Short: "Used to manage the Terraform Configuration state secrets",
	}

	c.AddCommand(
		NewListCommand(factory),
		NewCleanCommand(factory),
	)

	return c
}
