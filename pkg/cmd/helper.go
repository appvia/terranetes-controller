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

package cmd

import (
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"

	"github.com/olekukonko/tablewriter"
	"github.com/spf13/cobra"
	v1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	terraformv1alpha1 "github.com/appvia/terranetes-controller/pkg/apis/terraform/v1alpha1"
)

// NewTableWriter returns a default table writer
func NewTableWriter(out io.Writer) *tablewriter.Table {
	tw := tablewriter.NewWriter(out)
	tw.SetAutoWrapText(false)
	tw.SetAutoFormatHeaders(true)
	tw.SetHeaderAlignment(tablewriter.ALIGN_LEFT)
	tw.SetAlignment(tablewriter.ALIGN_LEFT)
	tw.SetCenterSeparator("")
	tw.SetColumnSeparator("")
	tw.SetRowSeparator("")
	tw.SetHeaderLine(false)
	tw.SetBorder(false)
	tw.SetTablePadding("\t")
	tw.SetNoWhiteSpace(true)

	return tw
}

// AutoCompletionFunc is a function that returns a list of completions
type AutoCompletionFunc func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective)

// MarkHidden marks a command as hidden and panics if an error is thrown
func MarkHidden(cmd *cobra.Command, name string) {
	if err := cmd.Flags().MarkHidden(name); err != nil {
		panic(errors.New("missing argument: " + name))
	}
}

// RegisterFlagCompletionFunc registers a completion function for a flag
func RegisterFlagCompletionFunc(cmd *cobra.Command, name string, fn AutoCompletionFunc) {
	if err := cmd.RegisterFlagCompletionFunc(name, fn); err != nil {
		panic(errors.New("missing argument: " + name))
	}
}

// AutoCompleteWithList registers a completion function for a flag
func AutoCompleteWithList(list []string) AutoCompletionFunc {
	return func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		return list, cobra.ShellCompDirectiveNoFileComp
	}
}

// AutoCompleteCloudResources registers a completion function for a flag
func AutoCompleteCloudResources(factory Factory) AutoCompletionFunc {
	return func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		namespace, _ := cmd.Flags().GetString("namespace")
		if namespace == "" {
			return nil, cobra.ShellCompDirectiveNoFileComp
		}

		cc, err := factory.GetClient()
		if err != nil {
			return nil, cobra.ShellCompDirectiveNoFileComp
		}

		list := &terraformv1alpha1.CloudResourceList{}
		if err := cc.List(context.Background(), list, client.InNamespace(namespace)); err != nil {
			return nil, cobra.ShellCompDirectiveNoFileComp
		}

		var resources []string
		for _, resource := range list.Items {
			resources = append(resources, resource.GetName())
		}

		return resources, cobra.ShellCompDirectiveNoFileComp
	}
}

// AutoCompleteConfigurations registers a completion function for a flag
func AutoCompleteConfigurations(factory Factory) AutoCompletionFunc {
	return func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		namespace, _ := cmd.Flags().GetString("namespace")
		if namespace == "" {
			return nil, cobra.ShellCompDirectiveNoFileComp
		}

		cc, err := factory.GetClient()
		if err != nil {
			return nil, cobra.ShellCompDirectiveNoFileComp
		}

		list := &terraformv1alpha1.ConfigurationList{}
		if err := cc.List(context.Background(), list, client.InNamespace(namespace)); err != nil {
			return nil, cobra.ShellCompDirectiveNoFileComp
		}

		var resources []string
		for _, resource := range list.Items {
			resources = append(resources, resource.GetName())
		}

		return resources, cobra.ShellCompDirectiveNoFileComp
	}
}

// AutoCompletionAvailableProviders returns a list of available providers in the cluster
func AutoCompletionAvailableProviders(factory Factory) AutoCompletionFunc {
	return func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		list, err := ListAvailableProviders(cmd.Context(), factory)
		if err != nil {
			return nil, cobra.ShellCompDirectiveNoFileComp
		}

		return list, cobra.ShellCompDirectiveNoFileComp
	}
}

// AutoCompletionStages returns a completion function for stage names
func AutoCompletionStages() AutoCompletionFunc {
	return AutoCompleteWithList([]string{
		terraformv1alpha1.StageTerraformApply,
		terraformv1alpha1.StageTerraformDestroy,
		terraformv1alpha1.StageTerraformPlan,
	})
}

// AutoCompleteNamespaces registers a completion function for a flag
func AutoCompleteNamespaces(factory Factory) AutoCompletionFunc {
	return func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		cc, err := factory.GetClient()
		if err != nil {
			return nil, cobra.ShellCompDirectiveNoFileComp
		}

		list := &v1.NamespaceList{}
		if err := cc.List(context.Background(), list); err != nil {
			return nil, cobra.ShellCompDirectiveNoFileComp
		}

		var namespaces []string
		for _, namespace := range list.Items {
			namespaces = append(namespaces, namespace.GetName())
		}

		return namespaces, cobra.ShellCompDirectiveNoFileComp
	}
}

// ConfigPath returns the path to the config file
func ConfigPath() string {
	if os.Getenv(ConfigPathEnvName) != "" {
		return os.Getenv(ConfigPathEnvName)
	}

	return filepath.Join(os.Getenv("HOME"), ".tnctl", "config.yaml")
}

// ListAvailableProviders returns a list of available providers in the cluster
func ListAvailableProviders(ctx context.Context, factory Factory) ([]string, error) {
	cc, err := factory.GetClient()
	if err != nil {
		return nil, err
	}

	list := &terraformv1alpha1.ProviderList{}
	if err := cc.List(ctx, list); err != nil {
		return nil, err
	}

	var providers []string
	for _, provider := range list.Items {
		providers = append(providers, string(provider.Name))
	}

	return providers, nil
}
