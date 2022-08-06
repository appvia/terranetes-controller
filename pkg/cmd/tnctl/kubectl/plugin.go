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

package kubectl

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/appvia/terranetes-controller/pkg/cmd"
	"github.com/appvia/terranetes-controller/pkg/utils"
)

var longPluginHelp = `
This command is used to integrate the tnctl command as a kubectl
plugin. It effectively generates a series of shortcuts that are
called from kubectl. You need to ensure the scripts this command
generates are included your $PATH, long with the location of the
tnctl command.

# Create the kubectl plugins (defaults to ${HOME}/bin)
$ tnctl kubectl plugin

# Place the plugins scripts in another directory
$ tnctl kubectl plugin -d ${GOPATH}/bin
`

var pluginTemplate = `#!/bin/env sh

tnctl {{ .Command }} $@
`

// PluginCommand returns the cobra command for the "build" sub-command.
type PluginCommand struct {
	cmd.Factory
	// Directory is the location to place the kubectl plugins
	Directory string
}

// NewPluginCommand returns a new instance of the get command
func NewPluginCommand(factory cmd.Factory) *cobra.Command {
	o := &PluginCommand{Factory: factory}

	c := &cobra.Command{
		Use:   "plugin [OPTIONS]",
		Short: "Generates the kubectl plugin integration",
		Long:  longPluginHelp,
		RunE: func(cmd *cobra.Command, args []string) error {
			return o.Run(cmd.Context())
		},
	}

	flags := c.Flags()
	flags.StringVarP(&o.Directory, "directory", "d", filepath.Join(os.Getenv("HOME"), "bin"), "Directory to place the kubectl plugins shortcuts")

	return c
}

// Run implements the command action
func (o *PluginCommand) Run(ctx context.Context) error {
	if o.Directory == "" {
		return errors.New("no directory specified for kubectl plugins")
	}

	commands := []string{
		"approve",
		"config",
		"describe",
		"logs",
		"search",
	}

	// @step: ensure the directory exists
	if err := os.MkdirAll(o.Directory, 0755); err != nil {
		return err
	}

	for _, name := range commands {
		path := filepath.Join(o.Directory, fmt.Sprintf("kubectl-tnctl-%s", name))

		content, err := utils.Template(pluginTemplate, map[string]interface{}{"Command": name})
		if err != nil {
			return err
		}
		if err := os.WriteFile(path, []byte(content), 0755); err != nil {
			return err
		}
	}

	return nil
}
