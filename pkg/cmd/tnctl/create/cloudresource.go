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

package create

import (
	"context"
	"errors"
	"fmt"

	"github.com/AlecAivazis/survey/v2"
	"github.com/fatih/color"
	"github.com/spf13/cobra"
	"sigs.k8s.io/controller-runtime/pkg/client"

	terraformv1alpha1 "github.com/appvia/terranetes-controller/pkg/apis/terraform/v1alpha1"
	"github.com/appvia/terranetes-controller/pkg/cmd"
	"github.com/appvia/terranetes-controller/pkg/utils"
)

// CloudResourceCommand are the options for the command
type CloudResourceCommand struct {
	cmd.Factory
	// Filename is the name of the file to write the cloud resource to
	Filename string
	// Plan is the name of the plan
	Plan string
	// Revision is the semvar version of the revision
	Revision string
	// Revisions is a list of revisions available
	Revisions *terraformv1alpha1.RevisionList
}

// NewCloudResourceCommand creates a new command
func NewCloudResourceCommand(factory cmd.Factory) *cobra.Command {
	o := &CloudResourceCommand{Factory: factory}

	c := &cobra.Command{
		Use:   "cloudresource [OPTIONS]",
		Short: "Used to create a cloud resource from a plan",
		Long:  revisionCommandDesc,
		RunE: func(cmd *cobra.Command, args []string) error {
			return o.Run(cmd.Context())
		},
	}
	c.SetErr(o.GetStreams().ErrOut)
	c.SetIn(o.GetStreams().In)
	c.SetOut(o.GetStreams().Out)

	flags := c.Flags()
	flags.StringVar(&o.Revision, "revision", "", "The semvar version of this revision")
	flags.StringVar(&o.Plan, "plan", "", "The name of the plan to use")
	flags.StringVarP(&o.Filename, "filename", "f", "", "The name of the file to write the cloud resource to")

	return c
}

// Run runs the command
func (o *CloudResourceCommand) Run(ctx context.Context) (err error) {
	// @step: we need to ask or guess the plan name
	if err := o.retrievePlan(ctx); err != nil {
		return err
	}
	// @step: we need to ask or guess the revision name
	if err := o.retrieveRevision(ctx); err != nil {
		return err
	}
	// @step: we need to render the cloud resource
	if err := o.renderCloudResource(); err != nil {
		return err
	}

	return nil
}

// renderCloudResource renders the cloud resource
func (o *CloudResourceCommand) renderCloudResource() error {
	findRevision := func() terraformv1alpha1.Revision {
		for _, x := range o.Revisions.Items {
			switch {
			case x.Spec.Plan.Name != o.Plan:
				continue
			case x.Spec.Plan.Revision != o.Revision:
				continue
			}

			return x
		}
		panic("revision not found")
	}
	revision := findRevision()

	// @step: convert a revision into a cloud resource
	cr, err := terraformv1alpha1.NewCloudResourceFromRevision(&revision)
	if err != nil {
		return err
	}

	// @step: write the cloud resource to the file or stdout
	if o.Filename == "" {
		return utils.WriteYAMLToWriter(o.GetStreams().Out, cr)
	}

	return utils.WriteYAML(o.Filename, cr)
}

// retrieveRevision retrieves the revision from the user or guesses it
func (o *CloudResourceCommand) retrieveRevision(ctx context.Context) error {
	// @step: if we have no revisions, we need to source them
	if o.Revisions == nil {
		cc, err := o.GetClient()
		if err != nil {
			return err
		}
		o.Revisions = &terraformv1alpha1.RevisionList{}

		if err := cc.List(ctx, o.Revisions,
			client.MatchingLabels(map[string]string{
				terraformv1alpha1.CloudResourcePlanNameLabel: o.Plan,
			})); err != nil {
			return err
		}
	}

	// @step: if we have no revisions, we cannot continue
	if len(o.Revisions.Items) == 0 {
		return errors.New("no revisions found to create a cloud resource")
	}

	// @step: get a list of the revisions
	var list []string
	for _, x := range o.Revisions.Items {
		list = append(list, x.Spec.Plan.Revision)
	}

	// @step: only ask if we have not been given a revision
	if o.Revision != "" {
		if !utils.Contains(o.Revision, list) {
			return fmt.Errorf("the revision %s does not exist", o.Revision)
		}

		return nil
	}

	// @step: ask the user to select a revision
	latest, err := utils.LatestSemverVersion(list)
	if err != nil {
		return err
	}

	if err := survey.AskOne(&survey.Select{
		Message: fmt.Sprintf("Which %s should we use from the plan?", color.YellowString("revision")),
		Help:    "Is the name of the cloudresource plan we should use",
		Options: list,
		Default: latest,
	}, &o.Revision); err != nil {
		return err
	}

	return nil
}

// retrievePlan retrieves the plan from the user or guesses it
func (o *CloudResourceCommand) retrievePlan(ctx context.Context) error {
	if o.Revisions == nil {
		cc, err := o.GetClient()
		if err != nil {
			return err
		}
		o.Revisions = &terraformv1alpha1.RevisionList{}

		if err := cc.List(ctx, o.Revisions); err != nil {
			return err
		}
	}

	// @step: if we have no plans, we cannot continue
	if len(o.Revisions.Items) == 0 {
		return errors.New("no plans found to create a cloud resource")
	}

	var list []string
	for _, x := range o.Revisions.Items {
		list = append(list, x.Spec.Plan.Name)
	}
	list = utils.Unique(list)

	// @step: only ask if we have not been given a plan
	if o.Plan == "" {
		if err := survey.AskOne(&survey.Select{
			Message: fmt.Sprintf("Enter the name of the %s this cloudresource will use?", color.YellowString("plan")),
			Options: list,
			Help:    "Is the name of the cloudresource plan we should use",
		}, &o.Plan); err != nil {
			return err
		}

		return nil
	}

	// @step: ensure the plan exists
	if !utils.Contains(o.Plan, list) {
		return fmt.Errorf("the plan %s does not exist", o.Plan)
	}

	return nil
}
