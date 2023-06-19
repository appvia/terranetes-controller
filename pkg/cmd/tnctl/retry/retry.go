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

package retry

import (
	"context"
	"fmt"
	"time"

	"github.com/spf13/cobra"
	"sigs.k8s.io/controller-runtime/pkg/client"

	terraformv1alpha1 "github.com/appvia/terranetes-controller/pkg/apis/terraform/v1alpha1"
	"github.com/appvia/terranetes-controller/pkg/cmd"
	"github.com/appvia/terranetes-controller/pkg/cmd/tnctl/logs"
	"github.com/appvia/terranetes-controller/pkg/utils/kubernetes"
)

var longUsage = `
By default a Configuration is only run on a change to the specification. Its
useful however to be able to restart the process without changing the
spec - i.e. the credentials were incorrect and out-of-band error occurred or
so forth.

This command will restart the process by tagging the configuration with a
annotation. By default the restarted process will be watched for logs.

Restart the Configuration:
$ tnctl retry NAME

Restart the Configuration but do not watch the logs:
$ tnctl retry NAME --watch=false
`

// Command returns the cobra command
type Command struct {
	cmd.Factory
	// Name is the name of the configuration
	Name string
	// Namespace is the namespace the configuration resides in
	Namespace string
	// Kind is the kind of configuration
	Kind string
	// WatchLogs indicates we should watch the logs after restarting the configuration
	WatchLogs bool
}

// NewCommand creates and returns the command
func NewCommand(factory cmd.Factory) *cobra.Command {
	c := &cobra.Command{
		Use:   "retry KIND",
		Long:  longUsage,
		Short: "Attempts to trigger the resource to retry",
	}
	c.SetErr(factory.GetStreams().ErrOut)
	c.SetOut(factory.GetStreams().Out)
	c.SetIn(factory.GetStreams().In)

	c.AddCommand(
		NewRetryCloudResourceCommand(factory),
		NewRetryConfigurationCommand(factory),
	)

	return c
}

// Run implements the command
func (o *Command) Run(ctx context.Context) error {
	cc, err := o.GetClient()
	if err != nil {
		return err
	}

	var resource client.Object

	// @step: retrieve the configuration
	switch o.Kind {
	case terraformv1alpha1.CloudResourceKind:
		resource = &terraformv1alpha1.CloudResource{}

	default:
		resource = &terraformv1alpha1.Configuration{}
	}

	resource.SetName(o.Name)
	resource.SetNamespace(o.Namespace)

	if found, err := kubernetes.GetIfExists(ctx, cc, resource); err != nil {
		return err
	} else if !found {
		return fmt.Errorf("resource (%s/%s) does not exist", o.Namespace, o.Name)
	}

	original := resource.DeepCopyObject()

	// @step: update the retry annotation
	if resource.GetAnnotations() == nil {
		resource.SetAnnotations(map[string]string{})
	}
	resource.GetAnnotations()[terraformv1alpha1.RetryAnnotation] = fmt.Sprintf("%d", time.Now().Unix())

	// @step: update the resource
	if err := cc.Patch(ctx, resource, client.MergeFrom(original.(client.Object))); err != nil {
		return err
	}
	o.Println("%s Resource %q has been marked for retry", cmd.IconGood, o.Name)

	if o.WatchLogs {
		return nil
	}

	if o.Kind == terraformv1alpha1.CloudResourceKind {
		return (&logs.CloudResourceLogsCommand{
			Factory:      o.Factory,
			Follow:       true,
			Name:         o.Name,
			Namespace:    o.Namespace,
			WaitInterval: 3 * time.Second,
		}).Run(ctx)
	}

	return (&logs.Command{
		Factory:      o.Factory,
		Follow:       true,
		Name:         o.Name,
		Namespace:    o.Namespace,
		WaitInterval: 3 * time.Second,
	}).Run(ctx)
}
