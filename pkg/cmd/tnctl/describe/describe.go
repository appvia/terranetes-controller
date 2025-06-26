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
	"sigs.k8s.io/controller-runtime/pkg/client"

	terraformv1alpha1 "github.com/appvia/terranetes-controller/pkg/apis/terraform/v1alpha1"
	"github.com/appvia/terranetes-controller/pkg/cmd"
	"github.com/appvia/terranetes-controller/pkg/cmd/tnctl/describe/assets"
	"github.com/appvia/terranetes-controller/pkg/utils/kubernetes"
	"github.com/appvia/terranetes-controller/pkg/utils/template"
)

// Command represents the available get command options
type Command struct {
	cmd.Factory
	// Name is the name of the resource we are describing
	Name string
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

Describe a configuration in a namespace
$ tnctl describe configuration -n apps NAME

Describe a cloudresource in a namespace
$ tnctl describe cloudresource -n apps NAME
`

// NewCommand returns a new instance of the get command
func NewCommand(factory cmd.Factory) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "describe KIND",
		Short: "Used to describe the current state of the configuration",
		Long:  strings.TrimPrefix(longDescription, "\n"),
	}

	cmd.AddCommand(
		NewDescribeCloudResourceCommand(factory),
		NewDescribeConfigurationCommand(factory),
	)

	return cmd
}

// Run is called to execute the get command
func (o *Command) Run(ctx context.Context) error {
	switch {
	case o.Namespace == "":
		return fmt.Errorf("namespace is required")
	case o.Name == "":
		return fmt.Errorf("name is required")
	}

	// @step: get a client to the cluster
	cc, err := o.GetClient()
	if err != nil {
		return err
	}

	// @step: retrieve the configuration
	configuration := &terraformv1alpha1.Configuration{}
	configuration.Namespace = o.Namespace
	configuration.Name = o.Name

	if found, err := kubernetes.GetIfExists(ctx, cc, configuration); err != nil {
		return err
	} else if !found {
		return fmt.Errorf("no configurations found in namespace %q", o.Namespace)
	}

	// @step: retrieve a list of all secrets related to the builds
	secrets := &v1.SecretList{}
	if err = cc.List(context.Background(), secrets,
		client.InNamespace(o.Namespace),
		client.MatchingLabels(map[string]string{
			terraformv1alpha1.ConfigurationNameLabel: configuration.Name,
			terraformv1alpha1.ConfigurationUIDLabel:  string(configuration.GetUID()),
		}),
	); err != nil {
		return err
	}

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

	annotations := configuration.GetAnnotations()
	if annotations != nil {
		delete(annotations, "kubectl.kubernetes.io/last-applied-configuration")
		configuration.SetAnnotations(annotations)
	}

	data := map[string]interface{}{
		"EnablePassedPolicy": o.ShowPassedChecks,
		"Name":               configuration.GetName(),
		"Namespace":          configuration.GetNamespace(),
		"Object":             configuration,
	}

	// @step: check if the configuration has a policy report
	if report, found := findPolicyReport(configuration); found {
		data["Policy"] = report
	}
	// @step: check if we have a cost report
	if report, found := findCostReport(configuration); found {
		data["Cost"] = report["projects"].([]interface{})[0]
	}

	x, err := template.New(string(assets.MustAsset("describe.yaml.tpl")), data)
	if err != nil {
		return err
	}
	o.Println("%s", x)

	return nil
}
