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

	"github.com/appvia/terranetes-controller/pkg/cmd"
	"github.com/appvia/terranetes-controller/pkg/utils"
)

// RemoveSourceCommand are the options for the command
type RemoveSourceCommand struct {
	cmd.Factory
	// Source is the source to add
	Source string
}

// NewRemoveSourceCommand creates and returns the command
func NewRemoveSourceCommand(factory cmd.Factory) *cobra.Command {
	o := &RemoveSourceCommand{Factory: factory}

	c := &cobra.Command{
		Use:     "remove SOURCE",
		Aliases: []string{"rm"},
		Args:    cobra.ExactArgs(1),
		Short:   "Removes a terraform module source to the configuration",
		RunE: func(cmd *cobra.Command, args []string) error {
			o.Source = args[0]

			return o.Run(cmd.Context())
		},
	}

	return c
}

// Run runs the command
func (o *RemoveSourceCommand) Run(ctx context.Context) error {
	config, _, err := o.GetConfig()
	if err != nil {
		return err
	}

	if utils.Contains(o.Source, config.Sources) {
		var updated []string

		for _, source := range config.Sources {
			if source != o.Source {
				updated = append(updated, source)
			}
		}
		config.Sources = updated

		if err := o.SaveConfig(config); err != nil {
			return err
		}
	}

	o.Println("%s Successfully saved configuration", cmd.IconGood)

	return nil
}
