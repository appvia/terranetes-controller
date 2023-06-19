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

package logs

import (
	"context"
	"fmt"
	"time"

	"github.com/spf13/cobra"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	corev1alpha1 "github.com/appvia/terranetes-controller/pkg/apis/core/v1alpha1"
	terraformv1alpha1 "github.com/appvia/terranetes-controller/pkg/apis/terraform/v1alpha1"
	"github.com/appvia/terranetes-controller/pkg/cmd"
	"github.com/appvia/terranetes-controller/pkg/utils/kubernetes"
)

// CloudResourceLogsCommand defines the struct for running the command
type CloudResourceLogsCommand struct {
	cmd.Factory
	// Name is the name of the cloudresource
	Name string
	// Namespace is the namespace of the cloudresource
	Namespace string
	// Stage is the stage to show logs for
	Stage string
	// Follow indicates we should follow the logs
	Follow bool
	// WaitInterval is the interval to wait for the logs
	WaitInterval time.Duration
}

// NewCloudResourceLogsCommand returns a new instance of the get command
func NewCloudResourceLogsCommand(factory cmd.Factory) *cobra.Command {
	o := &CloudResourceLogsCommand{Factory: factory}

	c := &cobra.Command{
		Use:     "cloudresource NAME [OPTIONS]",
		Short:   "Displays the latest logs for the given resource",
		Long:    longDescription,
		PreRunE: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			o.Name = args[0]

			return o.Run(cmd.Context())
		},
		ValidArgsFunction: cmd.AutoCompleteCloudresources(factory),
	}
	c.SetIn(o.GetStreams().In)
	c.SetErr(o.GetStreams().ErrOut)
	c.SetOut(o.GetStreams().Out)

	flags := c.Flags()
	flags.BoolVarP(&o.Follow, "follow", "f", false, "Indicates we should follow the logs")
	flags.DurationVar(&o.WaitInterval, "timeout", 3*time.Second, "The interval to wait for the logs")
	flags.StringVarP(&o.Namespace, "namespace", "n", "default", "The namespace of the resource")
	flags.StringVar(&o.Stage, "stage", "", "Select the stage to show logs for, else defaults to the current resource state")

	cmd.RegisterFlagCompletionFunc(c, "namespace", cmd.AutoCompleteNamespaces(factory))
	cmd.RegisterFlagCompletionFunc(c, "stage", cmd.AutoCompletionStages())

	return c
}

// Run is called to implement the action
func (o *CloudResourceLogsCommand) Run(ctx context.Context) error {
	cc, err := o.GetClient()
	if err != nil {
		return err
	}

	// @step: get the cloudresource
	cloudresource := &terraformv1alpha1.CloudResource{}
	cloudresource.Namespace = o.Namespace
	cloudresource.Name = o.Name

	if found, err := kubernetes.GetIfExists(ctx, cc, cloudresource); err != nil {
		return err
	} else if !found {
		return fmt.Errorf("cloudresource (%s/%s) does not exist", cloudresource.Namespace, cloudresource.Name)
	}

	// @step: check the condition
	cond := cloudresource.Status.GetCondition(terraformv1alpha1.ConditionConfigurationReady)
	switch {
	case cond == nil:
		return fmt.Errorf("cloudresource (%s/%s) has no conditions yet", cloudresource.Namespace, cloudresource.Name)

	case cond.Status != metav1.ConditionTrue:
		switch cond.Reason {
		case corev1alpha1.ReasonError:
			return fmt.Errorf("cloudresource (%s/%s) failed to provison configuration: %s", cloudresource.Namespace, cloudresource.Name, cond.Message)
		}

		return fmt.Errorf("cloudresource (%s/%s) has no configuration yet", cloudresource.Namespace, cloudresource.Name)

	case cloudresource.Status.ConfigurationName != "":
		break
	}

	return (&Command{
		Factory:      o.Factory,
		Follow:       o.Follow,
		Name:         cloudresource.Status.ConfigurationName,
		Namespace:    o.Namespace,
		Stage:        o.Stage,
		WaitInterval: o.WaitInterval,
	}).Run(ctx)
}
