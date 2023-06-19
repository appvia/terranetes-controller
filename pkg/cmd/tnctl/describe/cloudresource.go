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

package describe

import (
	"context"
	"errors"
	"strings"

	"github.com/spf13/cobra"

	terraformv1alpha1 "github.com/appvia/terranetes-controller/pkg/apis/terraform/v1alpha1"
	"github.com/appvia/terranetes-controller/pkg/cmd"
	"github.com/appvia/terranetes-controller/pkg/utils/kubernetes"
)

// CloudResourceCommand describes a cloud resource
type CloudResourceCommand struct {
	cmd.Factory
	// Name is the name of the resource we are describing
	Name string
	// Namespace is the namespace of the resource
	Namespace string
	// ShowPassedChecks is a flag to show passed checks
	ShowPassedChecks bool
}

// NewDescribeCloudResourceCommand returns a new instance of the get command
func NewDescribeCloudResourceCommand(factory cmd.Factory) *cobra.Command {
	o := &CloudResourceCommand{Factory: factory}

	c := &cobra.Command{
		Use:     "cloudresource [OPTIONS] NAME",
		Args:    cobra.MaximumNArgs(1),
		Short:   "Used to describe the current state of the resources",
		Long:    strings.TrimPrefix(longDescription, "\n"),
		PreRunE: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			o.Name = args[0]

			return o.Run(cmd.Context())
		},
		ValidArgsFunction: cmd.AutoCompleteCloudresources(o.Factory),
	}

	flags := c.Flags()
	flags.BoolVar(&o.ShowPassedChecks, "show-passed-checks", true, "Indicates we should show passed checks")
	flags.StringVarP(&o.Namespace, "namespace", "n", "default", "Namespace of the resource/s")

	cmd.RegisterFlagCompletionFunc(c, "namespace", cmd.AutoCompleteNamespaces(factory))

	return c
}

// Run is called to run the command
func (o *CloudResourceCommand) Run(ctx context.Context) error {
	cloudresource := &terraformv1alpha1.CloudResource{}
	cloudresource.Namespace = o.Namespace
	cloudresource.Name = o.Name

	// @step: get a client to the cluster
	cc, err := o.GetClient()
	if err != nil {
		return err
	}

	// @step: retrieve the cloud resource
	if found, err := kubernetes.GetIfExists(ctx, cc, cloudresource); err != nil {
		return err
	} else if !found {
		return errors.New("the cloud resource does not exist")
	}

	if cloudresource.Status.ConfigurationName == "" {
		return errors.New("cloudresource hasn't provisioned the configuration yet")
	}

	return (&Command{
		Factory:          o.Factory,
		ShowPassedChecks: o.ShowPassedChecks,
		Namespace:        o.Namespace,
		Name:             cloudresource.Status.ConfigurationName,
	}).Run(ctx)
}
