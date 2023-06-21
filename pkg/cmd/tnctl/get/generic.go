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
	"os/exec"
	"strings"

	"github.com/gertd/go-pluralize"
	"github.com/spf13/cobra"

	"github.com/appvia/terranetes-controller/pkg/cmd"
)

var (
	cc = pluralize.NewClient()
)

// ConfigurationOptions provides information required to retrieve a cloud resource
type ConfigurationOptions struct {
	cmd.Factory
	// Name is the name of the cloud resource
	Name string
	// Namespace is the namespace of the cloud resource
	Namespace string
	// Output is the output format
	Output string
	// AllNamespaces is a flag to indicate whether to retrieve cloud resources from all namespaces
	AllNamespaces bool
}

// NewGetResourceCommand returns a new cobra command
func NewGetResourceCommand(factory cmd.Factory, resource string) *cobra.Command {
	o := &ConfigurationOptions{Factory: factory}
	singular := cc.Singular(strings.Split(resource, ".")[0])
	plural := cc.Plural(singular)

	cmd := &cobra.Command{
		Use:     fmt.Sprintf("%s [OPTIONS] [NAME]", singular),
		Aliases: []string{plural},
		Short:   fmt.Sprintf("Used to retrieve %s/s from the cluster", singular),
		RunE: func(cmd *cobra.Command, args []string) error {
			options := []string{
				"get",
				resource,
			}
			if o.Namespace != "" {
				options = append(options, "--namespace", o.Namespace)
			}
			if len(args) > 0 {
				options = append(options, args...)
			}
			if o.Output != "" {
				options = append(options, "--output", o.Output)
			}
			if o.AllNamespaces {
				options = append(options, "--all-namespaces")
			}

			c := exec.CommandContext(cmd.Context(), "kubectl", options...)
			c.Stdout = factory.GetStreams().Out
			c.Stderr = factory.GetStreams().ErrOut
			c.Stdin = factory.GetStreams().In

			return c.Run()
		},
	}

	flags := cmd.Flags()
	flags.StringVarP(&o.Namespace, "namespace", "n", "", "Namespace to retrieve the resource from")
	flags.StringVarP(&o.Output, "output", "o", "", "The output format. Supported formats are: json|yaml|wide")
	flags.BoolVarP(&o.AllNamespaces, "all-namespaces", "A", false, "Retrieve cloud resources from all namespaces")

	return cmd
}
