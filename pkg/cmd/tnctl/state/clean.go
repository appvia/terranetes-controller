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
	"bufio"
	"context"
	"strings"
	"time"

	"github.com/spf13/cobra"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/duration"
	"sigs.k8s.io/controller-runtime/pkg/client"

	terraformv1alpha1 "github.com/appvia/terranetes-controller/pkg/apis/terraform/v1alpha1"
	"github.com/appvia/terranetes-controller/pkg/cmd"
)

// CleanCommand is the options for the clean command
type CleanCommand struct {
	cmd.Factory
	// ControllerNamespace is the namespace the controller is running in
	ControllerNamespace string
	// Force will force the deletion of the state
	Force bool
}

var longCleanHelp = `
The clean command will clean any orphaned state, cost, config or policy secrets.
These are kubernetes secrets which are not associated with a configuration.

# Clean all orphaned secrets (you will be prompted to confirm)
$ tnctl state clean
`

// NewCleanCommand creates and returns a new clean command
func NewCleanCommand(factory cmd.Factory) *cobra.Command {
	o := &CleanCommand{Factory: factory}

	c := &cobra.Command{
		Use:   "clean [OPTIONS]",
		Long:  longCleanHelp,
		Short: "Cleans any orphaned state, cost, config or policy secrets",
		RunE: func(cmd *cobra.Command, args []string) error {
			return o.Run(cmd.Context())
		},
	}

	flags := c.Flags()
	flags.StringVar(&o.ControllerNamespace, "controller-namespace", "terraform-system", "The namespace the controller is running in")
	flags.BoolVar(&o.Force, "force", false, "Force the deletion of the secrets")

	return c
}

// Run implements the command
func (o *CleanCommand) Run(ctx context.Context) error {
	// @step: retrieve a kubernetes client
	cc, err := o.GetClient()
	if err != nil {
		return err
	}

	// @step: retrieve the list of configurations
	list := &terraformv1alpha1.ConfigurationList{}
	if err := cc.List(ctx, list); err != nil {
		return err
	}

	// @step: retrieve all the secrets within the controller namespace
	secrets := &v1.SecretList{}
	if err := cc.List(ctx, secrets, client.InNamespace(o.ControllerNamespace)); err != nil {
		return err
	}

	// findMatchingConfiguration return true if the uuid is found
	findMatchingConfiguration := func(uuid string, list *terraformv1alpha1.ConfigurationList) bool {
		for _, x := range list.Items {
			if string(x.GetUID()) == uuid {
				return true
			}
		}

		return false
	}

	// @step: iterate the secrets and remove any orphaned secrets
	var data [][]string
	for _, secret := range secrets.Items {
		if !ConfigurationSecretRegex.MatchString(secret.Name) {
			continue
		}
		uuid := ConfigurationSecretRegex.FindStringSubmatch(secret.Name)[2]

		if !findMatchingConfiguration(uuid, list) {
			row := []string{secret.Name}

			if secret.GetLabels()[terraformv1alpha1.ConfigurationNameLabel] != "" {
				row = append(row, secret.GetLabels()[terraformv1alpha1.ConfigurationNameLabel])
			} else {
				row = append(row, "Unknown")
			}
			if secret.GetLabels()[terraformv1alpha1.ConfigurationNamespaceLabel] != "" {
				row = append(row, secret.GetLabels()[terraformv1alpha1.ConfigurationNamespaceLabel])
			} else {
				row = append(row, "Unknown")
			}
			row = append(row, duration.HumanDuration(time.Since(secret.GetCreationTimestamp().Time)))
			data = append(data, row)
		}
	}

	if len(data) == 0 {
		o.Println("\nNo orphaned secrets found")

		return nil
	}

	tw := cmd.NewTableWriter(o.Stdout())
	tw.SetHeader([]string{"Name", "Configuration", "Namespace", "Age"})
	tw.AppendBulk(data)
	tw.Render()

	if !o.Force {
		o.Printf("\nYou have %d secrets orphaned which can be removed, do you wish to delete? (y/n) ", len(data))
		choice, err := bufio.NewReader(o.GetStreams().In).ReadString('\n')
		if err != nil {
			return err
		}
		if strings.ToLower(strings.TrimSpace(choice)) != "y" {
			o.Println("Skipped deleting orphaned secrets")

			return nil
		}
	}

	for _, row := range data {
		name := row[0]

		secret := &v1.Secret{}
		secret.Namespace = o.ControllerNamespace
		secret.Name = name

		if err := cc.Delete(ctx, secret); err != nil {
			return err
		}
	}

	return nil
}
