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

package tnctl

import (
	"flag"
	"os"
	"path/filepath"
	"strings"

	"github.com/fatih/color"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"

	"github.com/appvia/terranetes-controller/pkg/cmd"
	"github.com/appvia/terranetes-controller/pkg/cmd/tnctl/approve"
	"github.com/appvia/terranetes-controller/pkg/cmd/tnctl/build"
	"github.com/appvia/terranetes-controller/pkg/cmd/tnctl/config"
	"github.com/appvia/terranetes-controller/pkg/cmd/tnctl/describe"
	"github.com/appvia/terranetes-controller/pkg/cmd/tnctl/generate"
	"github.com/appvia/terranetes-controller/pkg/cmd/tnctl/logs"
	"github.com/appvia/terranetes-controller/pkg/cmd/tnctl/search"
	"github.com/appvia/terranetes-controller/pkg/cmd/tnctl/workflow"
	"github.com/appvia/terranetes-controller/pkg/version"
)

var longDescription = `
Provides a toolset for both the platform team and developers to work
seemlessly with the terranetes framework. The CLI can be used to view,
approve configurations, package up terraform modules for consumption and
permit developers to search for resources to consume.
`

// New creates and returns a new command
func New(factory cmd.Factory) *cobra.Command {
	command := &cobra.Command{
		Use:           "tnctl",
		Short:         "Terranetes CLI tool",
		Long:          strings.TrimPrefix(longDescription, "\n"),
		SilenceErrors: true,
		Version:       version.Version,
		RunE: func(cmd *cobra.Command, args []string) error {
			log.SetOutput(factory.GetStreams().Out)

			if v, _ := cmd.Flags().GetBool("verbose"); v {
				log.SetLevel(log.DebugLevel)
			}
			if v, _ := cmd.Flags().GetBool("no-color"); v {
				color.NoColor = true
			}

			return cmd.Help()
		},
	}

	command.AddCommand(
		config.NewCommand(factory),
		approve.NewCommand(factory),
		build.NewCommand(factory),
		search.NewCommand(factory),
		workflow.NewCommand(factory),
		describe.NewCommand(factory),
		generate.NewCommand(factory),
		logs.NewCommand(factory),
	)

	flags := command.PersistentFlags()
	flags.Bool("verbose", false, "Enable verbose logging")
	flags.String("config", filepath.Join(os.ExpandEnv("HOME"), ".tnctl.yaml"), "Path to the configuration file")
	flag.Bool("no-color", false, "Disable color output")

	return command
}
