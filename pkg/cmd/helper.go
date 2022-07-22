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

	"github.com/spf13/cobra"
	v1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	terraformv1alphav1 "github.com/appvia/terranetes-controller/pkg/apis/terraform/v1alpha1"
)

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

		list := &terraformv1alphav1.ConfigurationList{}
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
