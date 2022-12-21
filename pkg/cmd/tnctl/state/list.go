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
	"context"
	"time"

	"github.com/spf13/cobra"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/duration"
	"sigs.k8s.io/controller-runtime/pkg/client"

	terraformv1alphav1 "github.com/appvia/terranetes-controller/pkg/apis/terraform/v1alpha1"
	"github.com/appvia/terranetes-controller/pkg/cmd"
)

// ListCommand is the options for the list command
type ListCommand struct {
	cmd.Factory
	// ControllerNamespace is the namespace the controller is running in
	ControllerNamespace string
	// Namespace is the namespace to list the configurations, defaults to all
	Namespace string
}

// NewListCommand creates and returns a new list command
func NewListCommand(factory cmd.Factory) *cobra.Command {
	o := &ListCommand{Factory: factory}

	c := &cobra.Command{
		Use:     "list [OPTIONS]",
		Aliases: []string{"ls"},
		Short:   "Listing all the configurations in the cluster and the current state",
		RunE: func(cmd *cobra.Command, args []string) error {
			return o.Run(cmd.Context())
		},
	}

	flags := c.Flags()
	flags.StringVar(&o.ControllerNamespace, "controller-namespace", "terraform-system", "The namespace the controller is running in")
	flags.StringVar(&o.Namespace, "namespace", "", "The namespace to list the configurations, defaults to all")

	return c
}

// Run implements the command
func (o *ListCommand) Run(ctx context.Context) error {
	// @step: retrieve a kubernetes client
	cc, err := o.GetClient()
	if err != nil {
		return err
	}

	// @step: retrieve the list of configurations
	list := &terraformv1alphav1.ConfigurationList{}
	if err := cc.List(ctx, list, client.InNamespace(o.Namespace)); err != nil {
		return err
	}
	if len(list.Items) == 0 {
		o.Println("No configurations found")

		return nil
	}

	// @step: retrieve all the secrets within the controller namespace
	secrets := &v1.SecretList{}
	if err := cc.List(ctx, secrets, client.InNamespace(o.ControllerNamespace)); err != nil {
		return err
	}

	// @step: lets build the rows
	var data [][]string
	for _, x := range list.Items {
		row := []string{x.Name}
		for _, prefix := range SecretPrefixes {
			if name, found := findSecretByPrefix(prefix, string(x.GetUID()), secrets); found {
				row = append(row, name)
			} else {
				row = append(row, "None")
			}
		}
		row = append(row, duration.HumanDuration(time.Since(x.GetCreationTimestamp().Time)))

		data = append(data, row)
	}

	tw := cmd.NewTableWriter(o.Stdout())
	tw.SetHeader([]string{
		"Configuration",
		"State",
		"Config",
		"Policy",
		"Cost",
		"Age",
	})
	tw.AppendBulk(data)
	tw.Render()

	return nil
}
