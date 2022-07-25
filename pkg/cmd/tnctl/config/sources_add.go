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
	"strings"

	"github.com/spf13/cobra"

	"github.com/appvia/terranetes-controller/pkg/cmd"
	"github.com/appvia/terranetes-controller/pkg/utils"
)

var addSourceLong = `
Sources are the URL locations for terraform modules. By default if
no sources are defined we use the public terraform registry. We currently
support aggregating modules from any terraform registry and Github.

Add a terraform registry to the source
$ tnctl config sources add https://registry.terraform.io

Add a Github organization or user to the source
$ tnctl config sources add github.com/appvia/terranetes-controller

Note, skipping the name github organization or user requires your GITHUB_TOKEN
is exported as the CLI will use this to authenticate to the github and
search any repositories you are a member, contributor or owner of.
`

// AddSourceCommand are the options for the command
type AddSourceCommand struct {
	cmd.Factory
	// Source is the source to add
	Source string
}

// NewAddSourceCommand creates and returns the command
func NewAddSourceCommand(factory cmd.Factory) *cobra.Command {
	o := &AddSourceCommand{Factory: factory}

	c := &cobra.Command{
		Use:   "add SOURCE",
		Args:  cobra.ExactArgs(1),
		Short: "Adds a terraform module source to the configuration",
		Long:  strings.TrimPrefix(addSourceLong, "\n"),
		RunE: func(cmd *cobra.Command, args []string) error {
			o.Source = args[0]

			return o.Run(cmd.Context())
		},
	}

	return c
}

// Run runs the command
func (o *AddSourceCommand) Run(ctx context.Context) error {
	config, _, err := o.GetConfig()
	if err != nil {
		return err
	}

	if utils.Contains(o.Source, config.Sources) {
		o.Println("%s Source already exists", cmd.IconGood)

		return nil
	}
	config.Sources = append(config.Sources, o.Source)

	if err := o.SaveConfig(config); err != nil {
		return err
	}
	o.Println("%s Successfully saved configuration", cmd.IconGood)

	return o.SaveConfig(config)
}
