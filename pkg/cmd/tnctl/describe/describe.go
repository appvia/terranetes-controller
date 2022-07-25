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

package describe

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/spf13/cobra"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/controller-runtime/pkg/client"

	terraformv1alphav1 "github.com/appvia/terranetes-controller/pkg/apis/terraform/v1alpha1"
	"github.com/appvia/terranetes-controller/pkg/cmd"
	"github.com/appvia/terranetes-controller/pkg/cmd/tnctl/describe/assets"
	"github.com/appvia/terranetes-controller/pkg/utils"
)

// Command represents the available get command options
type Command struct {
	cmd.Factory
	// Names is the name of the resource we are describing
	Names []string
	// Namespace is the namespace of the resource
	Namespace string
	// ShowPassedChecks is a flag to show passed checks
	ShowPassedChecks bool
}

var longDescription = `
Retrieves the definition and current state of one or more of the
terraform configurations, displaying in a human friendly format.
The command also extracts any integration details which have been
produced by infracosts or checkov scans.

Describe all configurations in a namespace
$ tnctl describe -n apps

Describe a single configuration called 'test'
$ tnctl describe -n apps test
`

// NewCommand returns a new instance of the get command
func NewCommand(factory cmd.Factory) *cobra.Command {
	options := &Command{Factory: factory}

	c := &cobra.Command{
		Use:   "describe [NAME...]",
		Args:  cobra.MaximumNArgs(1),
		Short: "Used to describe the current state of the configuration",
		Long:  strings.TrimPrefix(longDescription, "\n"),
		RunE: func(cmd *cobra.Command, args []string) error {
			options.Names = args

			return options.Run(cmd.Context())
		},
		ValidArgsFunction: cmd.AutoCompleteConfigurations(options.Factory),
	}

	flags := c.Flags()
	flags.BoolVar(&options.ShowPassedChecks, "show-passed-checks", true, "Indicates we should show passed checks")
	flags.StringVarP(&options.Namespace, "namespace", "n", "", "Namespace of the resource/s")

	cmd.RegisterFlagCompletionFunc(c, "namespace", cmd.AutoCompleteNamespaces(factory))

	return c
}

// Run is called to execute the get command
func (o *Command) Run(ctx context.Context) error {
	if o.Namespace == "" {
		return fmt.Errorf("namespace is required")
	}
	if len(o.Names) == 0 {
		o.Names = append(o.Names, "*")
	}

	cc, err := o.GetClient()
	if err != nil {
		return err
	}

	list := &unstructured.UnstructuredList{}
	list.SetGroupVersionKind(terraformv1alphav1.ConfigurationGVK)
	err = cc.List(context.Background(), list, client.InNamespace(o.Namespace))
	if err != nil {
		return err
	}
	if len(list.Items) == 0 {
		o.Println("No terraform configurations found in namespace %q", o.Namespace)

		return nil
	}

	resourceFilter := func(resource client.Object) bool {
		switch {
		case utils.Contains("*", o.Names):
			return true
		case utils.Contains(resource.GetName(), o.Names):
			return true
		}
		return false
	}

	// @step: retrieve all pods related to builds in the namespace
	pods := &v1.PodList{}
	_ = cc.List(context.Background(), pods,
		client.HasLabels([]string{terraformv1alphav1.ConfigurationNameLabel}),
		client.InNamespace(o.Namespace),
	)

	// @step: retrieve a list of all secrets related to the builds
	secrets := &v1.SecretList{}
	_ = cc.List(context.Background(), secrets,
		client.HasLabels([]string{terraformv1alphav1.ConfigurationNameLabel}),
		client.InNamespace(o.Namespace),
	)

	findSecret := func(name, key string) (map[string]interface{}, bool) {
		if len(secrets.Items) == 0 {
			return nil, false
		}
		for _, secret := range secrets.Items {
			switch {
			case secret.GetName() != name:
				continue
			case secret.Data[key] == nil:
				continue
			}

			values := make(map[string]interface{})
			if err := json.Unmarshal(secret.Data[key], &values); err == nil {
				return values, true
			}
		}

		return nil, false
	}

	findPolicyReport := func(resource client.Object) (map[string]interface{}, bool) {
		name := fmt.Sprintf("policy-%v", resource.GetUID())
		key := "results_json.json"

		return findSecret(name, key)
	}

	findCostReport := func(resource client.Object) (map[string]interface{}, bool) {
		name := fmt.Sprintf("costs-%v", resource.GetUID())
		key := "costs.json"

		return findSecret(name, key)
	}

	for _, resource := range list.Items {
		if !resourceFilter(&resource) {
			continue
		}

		annotations := resource.GetAnnotations()
		if annotations != nil {
			delete(annotations, "kubectl.kubernetes.io/last-applied-configuration")
			resource.SetAnnotations(annotations)
		}

		data := map[string]interface{}{
			"EnablePassedPolicy": o.ShowPassedChecks,
			"Name":               resource.GetName(),
			"Namespace":          resource.GetNamespace(),
			"Object":             resource.Object,
		}

		// @step: check if the configuration has a policy report
		if report, found := findPolicyReport(&resource); found {
			data["Policy"] = report
		}
		// @step: check if we have a cost report
		if report, found := findCostReport(&resource); found {
			data["Cost"] = report["projects"].([]interface{})[0]
		}

		x, err := utils.Template(string(assets.MustAsset("describe.yaml.tpl")), data)
		if err != nil {
			return err
		}
		o.Println("%s", x)
	}

	return nil
}
