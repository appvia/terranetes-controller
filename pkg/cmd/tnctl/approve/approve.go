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

	"sigs.k8s.io/controller-runtime/pkg/client"

	terraformv1alpha1 "github.com/appvia/terranetes-controller/pkg/apis/terraform/v1alpha1"
	"github.com/appvia/terranetes-controller/pkg/cmd"
	"github.com/appvia/terranetes-controller/pkg/utils/kubernetes"
	"github.com/spf13/cobra"
)

// Command represents the available get command options
type Command struct {
	cmd.Factory
	// Names is the name of the resource we are describing
	Names []string
	// Namespace is the namespace of the resource
	Namespace string
	// Kind is the kind of resource
	Kind string
}

var longDescription = `
Used to approve a terraform configuration and permit the
configuration to move into the apply stage. This command
effectively changes the terraform.appvia.io/apply annotation
from 'false' to 'true'.

Approve one or more configurations
$ tnctl approve configuration NAME

Approve one or more cloudresource
$ tnctl approve cloudresource NAME
`

// NewCommand creates and returns the command
func NewCommand(factory cmd.Factory) *cobra.Command {
	c := &cobra.Command{
		Use:   "approve KIND",
		Long:  longDescription,
		Short: "Approves either a configuration or cloudresource",
	}
	c.SetErr(factory.GetStreams().ErrOut)
	c.SetOut(factory.GetStreams().Out)
	c.SetIn(factory.GetStreams().In)

	c.AddCommand(
		NewApproveConfigurationCommand(factory),
		NewApproveCloudResourceCommand(factory),
	)

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

	for _, name := range o.Names {
		var resource client.Object
		switch o.Kind {
		case terraformv1alpha1.ConfigurationKind:
			resource = &terraformv1alpha1.Configuration{}
		default:
			resource = &terraformv1alpha1.CloudResource{}
		}
		resource.SetNamespace(o.Namespace)
		resource.SetName(name)

		found, err := kubernetes.GetIfExists(ctx, cc, resource)
		if err != nil {
			return err
		}
		if !found {
			return fmt.Errorf("resource %s not found", resource.GetName())
		}

		original := resource.DeepCopyObject()

		// @step: update the resource if required
		switch {
		case resource.GetAnnotations() == nil:
			continue
		case resource.GetAnnotations()[terraformv1alpha1.ApplyAnnotation] == "":
			continue
		case resource.GetAnnotations()[terraformv1alpha1.ApplyAnnotation] == "true":
			continue
		}
		resource.GetAnnotations()[terraformv1alpha1.ApplyAnnotation] = "true"

		if err := cc.Patch(ctx, resource, client.MergeFrom(original.(client.Object))); err != nil {
			return err
		}

		switch {
		case o.Kind == terraformv1alpha1.ConfigurationKind:
			o.Println("%s Configuration %s has been approved", cmd.IconGood, resource.GetName())
		default:
			o.Println("%s CloudResource %s has been approved", cmd.IconGood, resource.GetName())
		}
	}

	return nil
}
