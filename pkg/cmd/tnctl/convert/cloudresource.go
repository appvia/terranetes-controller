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

package convert

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/spf13/cobra"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	corev1alpha1 "github.com/appvia/terranetes-controller/pkg/apis/core/v1alpha1"
	terraformv1alpha1 "github.com/appvia/terranetes-controller/pkg/apis/terraform/v1alpha1"
	"github.com/appvia/terranetes-controller/pkg/cmd"
	"github.com/appvia/terranetes-controller/pkg/utils/kubernetes"
)

// CloudResourceCommand are the options for the command
type CloudResourceCommand struct {
	cmd.Factory
	// Name is the name of the resource
	Name string
	// Namespace is the namespace of the resource
	Namespace string
}

// NewCloudResourceCommand creates and returns a new command
func NewCloudResourceCommand(factory cmd.Factory) *cobra.Command {
	o := &CloudResourceCommand{Factory: factory}

	cmd := &cobra.Command{
		Use:     "cloudresource [OPTIONS] NAME",
		Short:   "Used to convert cloudresource back to terraform",
		Long:    strings.TrimPrefix(longDescription, "\n"),
		PreRunE: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			o.Name = args[0]

			return o.Run(cmd.Context())
		},
	}
	cmd.SetErr(o.GetStreams().ErrOut)
	cmd.SetIn(o.GetStreams().In)
	cmd.SetOut(o.GetStreams().Out)

	flags := cmd.Flags()
	flags.StringVarP(&o.Namespace, "namespace", "n", "default", "Namespace of the resource")

	return cmd
}

// Run runs the command
func (o *CloudResourceCommand) Run(ctx context.Context) error {
	switch {
	case o.Name == "":
		return errors.New("name is required")
	case o.Namespace == "":
		return errors.New("namespace is required")
	}

	// @step: retrieve the client
	cc, err := o.GetClient()
	if err != nil {
		return err
	}

	cloudresource := &terraformv1alpha1.CloudResource{}
	cloudresource.Namespace = o.Namespace
	cloudresource.Name = o.Name

	found, err := kubernetes.GetIfExists(ctx, cc, cloudresource)
	if err != nil {
		return err
	} else if !found {
		return fmt.Errorf("cloudresource (%s/%s) does not exist", o.Namespace, o.Name)
	}

	cond := cloudresource.Status.GetCondition(terraformv1alpha1.ConditionConfigurationReady)
	switch {
	case cond == nil:
		return fmt.Errorf("cloudresource (%s/%s) is not ready", o.Namespace, o.Name)
	case cond.Reason == corev1alpha1.ReasonError:
		return fmt.Errorf("cloudresource (%s/%s) has failed: %s", o.Namespace, o.Name, cond.Message)
	case cond.Status != metav1.ConditionTrue:
		return fmt.Errorf("cloudresource (%s/%s) is not ready", o.Namespace, o.Name)
	}

	configuration := cloudresource.Status.ConfigurationName
	if configuration == "" {
		return fmt.Errorf("cloudresource (%s/%s) does not have a configuration", o.Namespace, o.Name)
	}

	return (&ConfigurationCommand{
		Factory:   o.Factory,
		Name:      configuration,
		Namespace: o.Namespace,
	}).Run(ctx)
}
