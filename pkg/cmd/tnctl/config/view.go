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

package config

import (
	"context"

	"github.com/spf13/cobra"
	"sigs.k8s.io/yaml"

	"github.com/appvia/terranetes-controller/pkg/cmd"
)

// ViewCommand are the options for the command
type ViewCommand struct {
	cmd.Factory
}

// NewViewCommand creates and returns the command
func NewViewCommand(factory cmd.Factory) *cobra.Command {
	o := &ViewCommand{Factory: factory}

	c := &cobra.Command{
		Use:   "view",
		Short: "Shows the current configuration of the CLI",
		RunE: func(cmd *cobra.Command, args []string) error {
			return o.Run(cmd.Context())
		},
	}

	return c
}

// Run runs the command
func (o *ViewCommand) Run(ctx context.Context) error {
	config, found, err := o.GetConfig()
	if err != nil {
		return err
	}
	if !found {
		o.Println("No configuration found")

		return nil
	}

	encoded, err := yaml.Marshal(&config)
	if err != nil {
		return err
	}
	o.Println("%s", encoded)

	return nil
}
