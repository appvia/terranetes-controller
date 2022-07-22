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

package approve

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/spf13/cobra"
	"sigs.k8s.io/controller-runtime/pkg/client"

	terraformv1alphav1 "github.com/appvia/terranetes-controller/pkg/apis/terraform/v1alpha1"
	"github.com/appvia/terranetes-controller/pkg/cmd"
	"github.com/appvia/terranetes-controller/pkg/utils/kubernetes"
)

// Command represents the available get command options
type Command struct {
	cmd.Factory
	// Names is the name of the resource we are describing
	Names []string
	// Namespace is the namespace of the resource
	Namespace string
}

var longDescription = `
Used to approve a terraform configuration and permit the
configuration to move into the apply stage. This command
effectively changes the terraform.appvia.io/apply annotation
from 'false' to 'true'.

Approve one or more configurations
$ tnctl approve NAME
`

// NewCommand returns a new instance of the get command
func NewCommand(factory cmd.Factory) *cobra.Command {
	options := &Command{Factory: factory}

	c := &cobra.Command{
		Use:   "approve NAME",
		Short: "Approves a terraform configuration for release",
		Args:  cobra.MinimumNArgs(1),
		Long:  strings.TrimPrefix(longDescription, "\n"),
		RunE: func(cmd *cobra.Command, args []string) error {
			options.Names = args

			return options.Run(cmd.Context())
		},
		ValidArgsFunction: cmd.AutoCompleteConfigurations(options.Factory),
	}

	flags := c.Flags()
	flags.StringVarP(&options.Namespace, "namespace", "n", "default", "Namespace of the resource/s")

	cmd.RegisterFlagCompletionFunc(c, "namespace", cmd.AutoCompleteNamespaces(factory))

	return c
}

// Run is called to execute the get command
func (o *Command) Run(ctx context.Context) error {
	switch {
	case o.Namespace == "":
		return errors.New("namespace is required")

	case len(o.Names) == 0:
		return errors.New("name is required")
	}

	cc, err := o.GetClient()
	if err != nil {
		return err
	}

	for _, resource := range o.Names {
		configuration := &terraformv1alphav1.Configuration{}
		configuration.Namespace = o.Namespace
		configuration.Name = resource

		found, err := kubernetes.GetIfExists(ctx, cc, configuration)
		if err != nil {
			return err
		}
		if !found {
			return fmt.Errorf("configuration %s not found", resource)
		}

		original := configuration.DeepCopy()

		// @step: update the configuration if required
		switch {
		case configuration.Annotations == nil:
			continue
		case configuration.Annotations[terraformv1alphav1.ApplyAnnotation] == "":
			continue
		case configuration.Annotations[terraformv1alphav1.ApplyAnnotation] == "true":
			continue
		}
		configuration.Annotations[terraformv1alphav1.ApplyAnnotation] = "true"

		if err := cc.Patch(ctx, configuration, client.MergeFrom(original)); err != nil {
			return err
		}
		o.Println("%s Configuration %s has been approved", cmd.IconGood, resource)
	}

	return nil
}
