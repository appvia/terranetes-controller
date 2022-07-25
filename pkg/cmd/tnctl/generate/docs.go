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

package generate

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/cobra/doc"

	"github.com/appvia/terranetes-controller/pkg/cmd"
)

// DocsCommand contains the command options
type DocsCommand struct {
	cmd.Factory
	// Directory is the path to save the markdown
	Directory string
	// Root is the root command
	Root *cobra.Command
}

const (
	fmTemplate = `---
title: "%s"
---
`
)

// NewDocsCommand creates and returns a new command
func NewDocsCommand(factory cmd.Factory) *cobra.Command {
	o := &DocsCommand{Factory: factory}

	c := &cobra.Command{
		Use:    "docs",
		Short:  "Generates the CLI documentation",
		Hidden: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			o.Root = cmd.Root()
			o.Root.DisableAutoGenTag = true

			return o.Run(cmd.Context())
		},
	}

	flags := c.Flags()
	flags.StringVar(&o.Directory, "directory", "generated", "The directory to save the markdown")

	return c
}

// Run executes the command
func (o *DocsCommand) Run(ctx context.Context) error {
	if err := os.MkdirAll(o.Directory, os.FileMode(0775)); err != nil {
		return err
	}

	return doc.GenMarkdownTreeCustom(o.Root, o.Directory,
		func(filename string) string {
			name := filepath.Base(filename)
			base := strings.TrimSuffix(name, filepath.Ext(name))

			return fmt.Sprintf(fmTemplate, strings.Replace(base, "_", " ", -1))
		},
		func(name string) string {
			base := strings.TrimSuffix(name, filepath.Ext(name))

			return fmt.Sprintf("../%s", strings.ToLower(base))
		},
	)
}
