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

package state

import (
	"context"
	"fmt"
	"io"

	"github.com/spf13/cobra"
	v1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	terraformv1alpha1 "github.com/appvia/terranetes-controller/pkg/apis/terraform/v1alpha1"
	"github.com/appvia/terranetes-controller/pkg/cmd"
	"github.com/appvia/terranetes-controller/pkg/utils/terraform"
)

// GetCommand is the options for the list command
type GetCommand struct {
	cmd.Factory
	// ControllerNamespace is the namespace the controller is running in
	ControllerNamespace string
	// Name is the name of the configuration to retrieve
	Name string
	// Namespace is the namespace to list the configurations, defaults to all
	Namespace string
}

// NewGetCommand creates and returns a new get command
func NewGetCommand(factory cmd.Factory) *cobra.Command {
	o := &GetCommand{Factory: factory}

	c := &cobra.Command{
		Use:   "get NAME [OPTIONS]",
		Short: "Retrieves and displays the current state of a configuration",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			o.Name = args[0]

			return o.Run(cmd.Context())
		},
	}

	flags := c.Flags()
	flags.StringVar(&o.ControllerNamespace, "controller-namespace", "terraform-system", "The namespace the controller is running in")
	flags.StringVarP(&o.Namespace, "namespace", "n", "default", "The namespace to list the configurations")

	return c
}

// Run implements the command
func (o *GetCommand) Run(ctx context.Context) error {
	// @step: retrieve a kubernetes client
	cc, err := o.GetClient()
	if err != nil {
		return err
	}

	// @step: retrieve the secret for this configuration
	secrets := &v1.SecretList{}
	err = cc.List(ctx, secrets,
		client.InNamespace(o.ControllerNamespace),
		client.MatchingLabels(map[string]string{
			terraformv1alpha1.ConfigurationNameLabel:      o.Name,
			terraformv1alpha1.ConfigurationNamespaceLabel: o.Namespace,
			"tfstate": "true",
		}),
	)
	if err != nil {
		o.Println("Failed to retrieve the configuration secret", err)

		return err
	}
	if len(secrets.Items) == 0 {
		o.Println("No configuration terraform state found")

		return nil
	}

	state, found := secrets.Items[0].Data[terraformv1alpha1.TerraformStateSecretKey]
	if !found {
		return fmt.Errorf("no terraform state found in the secret: %s", secrets.Items[0].Name)
	}

	// @step: we need to decode the state secret
	rd, err := terraform.Decode(state)
	if err != nil {
		return fmt.Errorf("failed to decode the terraform state: %w", err)
	}

	// @step: read the state out of the reader
	out, err := io.ReadAll(rd)
	if err != nil {
		return fmt.Errorf("failed to read the terraform state: %w", err)
	}
	// @step: print the state in json format
	o.Println(string(out))

	return nil
}
