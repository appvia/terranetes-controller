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

package logs

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/spf13/cobra"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	corev1alphav1 "github.com/appvia/terranetes-controller/pkg/apis/core/v1alpha1"
	terraformv1alphav1 "github.com/appvia/terranetes-controller/pkg/apis/terraform/v1alpha1"
	"github.com/appvia/terranetes-controller/pkg/cmd"
	"github.com/appvia/terranetes-controller/pkg/utils"
	"github.com/appvia/terranetes-controller/pkg/utils/kubernetes"
)

var longLogsHelp = `
`

// Command represents the options
type Command struct {
	cmd.Factory
	// Name is the name of the resource
	Name string
	// Namespace is the namespace of the resource
	Namespace string
	// Follow indicates we should follow the logs
	Follow bool
	// Stage override the stage to look for
	Stage string
}

// NewCommand returns a new instance of the get command
func NewCommand(factory cmd.Factory) *cobra.Command {
	o := &Command{Factory: factory}

	c := &cobra.Command{
		Use:     "logs NAME [OPTIONS]",
		Short:   "Displays the logs from the current configuration",
		Long:    longLogsHelp,
		PreRunE: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			o.Name = args[0]

			return o.Run(cmd.Context())
		},
		ValidArgsFunction: cmd.AutoCompleteConfigurations(factory),
	}

	flags := c.Flags()
	flags.BoolVarP(&o.Follow, "follow", "f", false, "follow the logs")
	flags.StringVar(&o.Name, "name", "", "Is the name of the resource to create")
	flags.StringVarP(&o.Namespace, "namespace", "n", "", "The namespace of the resource")
	flags.StringVar(&o.Stage, "stage", "", "Selects the stage to show logs for, else defaults to the condition")

	cmd.RegisterFlagCompletionFunc(c, "namespace", cmd.AutoCompleteNamespaces(factory))
	cmd.RegisterFlagCompletionFunc(c, "stage", cmd.AutoCompleteWithList([]string{
		terraformv1alphav1.StageTerraformPlan,
		terraformv1alphav1.StageTerraformApply,
	}))

	return c
}

// Run executes the command
func (o *Command) Run(ctx context.Context) error {
	switch {
	case o.Name == "":
		return cmd.ErrMissingArgument("name")
	case o.Namespace == "":
		return cmd.ErrMissingArgument("namespace")
	case o.Stage != "" && !utils.Contains(o.Stage, []string{
		terraformv1alphav1.StageTerraformPlan,
		terraformv1alphav1.StageTerraformApply,
	}):
		return errors.New("invalid stage (must be one of: plan, apply)")
	}

	cc, err := o.GetClient()
	if err != nil {
		return err
	}

	configuration := &terraformv1alphav1.Configuration{}
	configuration.Name = o.Name
	configuration.Namespace = o.Namespace

	found, err := kubernetes.GetIfExists(ctx, cc, configuration)
	if err != nil {
		return err
	}
	if !found {
		return fmt.Errorf("resource %s not found", o.Name)
	}

	if o.Stage != "" {
		return o.showLogs(ctx, o.Stage, configuration)
	}

	condition := configuration.Status.GetCondition(terraformv1alphav1.ConditionTerraformApply)
	if condition.ObservedGeneration == configuration.GetGeneration() && condition.Reason != corev1alphav1.ReasonNotDetermined {
		if condition.Reason == corev1alphav1.ReasonActionRequired {
			if strings.Contains(condition.Message, "Waiting for terraform apply annotation") {
				return o.showLogs(ctx, terraformv1alphav1.StageTerraformPlan, configuration)
			}
		}

		return o.showLogs(ctx, terraformv1alphav1.StageTerraformApply, configuration)
	}

	condition = configuration.Status.GetCondition(terraformv1alphav1.ConditionTerraformPlan)
	if condition.ObservedGeneration == configuration.GetGeneration() {
		return o.showLogs(ctx, terraformv1alphav1.StageTerraformPlan, configuration)
	}

	return errors.New("neither plan or apply have been run for this configuration")
}

// showLogs is a helper function to show the logs for all the containers under a build
func (o *Command) showLogs(ctx context.Context, stage string, configuration *terraformv1alphav1.Configuration) error {
	cc, err := o.GetKubeClient()
	if err != nil {
		return err
	}

	labels := []string{
		terraformv1alphav1.ConfigurationGenerationLabel + "=" + fmt.Sprintf("%d", configuration.GetGeneration()),
		terraformv1alphav1.ConfigurationNameLabel + "=" + configuration.Name,
		terraformv1alphav1.ConfigurationStageLabel + "=" + stage,
		terraformv1alphav1.ConfigurationUIDLabel + "=" + string(configuration.UID),
	}

	// @step: find the pods associated to this configuration
	list, err := cc.CoreV1().Pods(configuration.Namespace).List(ctx, metav1.ListOptions{
		LabelSelector: strings.Join(labels, ","),
	})
	if err != nil {
		return err
	}
	if len(list.Items) == 0 {
		return fmt.Errorf("no pods found for configuration %s", configuration.Name)
	}

	// @step: find the latest
	pod := kubernetes.FindLatestPod(list)

	// @step: render the logs
	for _, container := range append(pod.Spec.InitContainers, pod.Spec.Containers...) {
		stream, err := cc.CoreV1().Pods(configuration.Namespace).GetLogs(pod.Name, &v1.PodLogOptions{
			Container: container.Name,
			Follow:    o.Follow,
		}).Stream(ctx)
		if err != nil {
			return err
		}
		if _, err := io.Copy(o.Stdout(), stream); err != nil {
			return err
		}
		if err := stream.Close(); err != nil {
			return err
		}
	}

	return nil
}
