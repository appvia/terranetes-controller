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

package cloudresources

import (
	"context"
	"errors"
	"fmt"

	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	terraformv1alpha1 "github.com/appvia/terranetes-controller/pkg/apis/terraform/v1alpha1"
	"github.com/appvia/terranetes-controller/pkg/utils"
	"github.com/appvia/terranetes-controller/pkg/utils/kubernetes"
)

type mutator struct {
	cc client.Client
}

// NewMutator returns a mutation handler
func NewMutator(cc client.Client) admission.CustomDefaulter {
	return &mutator{cc: cc}
}

// Default implements the mutation handler
func (m *mutator) Default(ctx context.Context, obj runtime.Object) error {
	o, ok := obj.(*terraformv1alpha1.CloudResource)
	if !ok {
		return fmt.Errorf("expected cloudresource, not %T", obj)
	}

	o.Labels = utils.MergeStringMaps(o.Labels, map[string]string{
		terraformv1alpha1.CloudResourcePlanNameLabel: o.Spec.Plan.Name,
		terraformv1alpha1.CloudResourceRevisionLabel: o.Spec.Plan.Revision,
	})

	// @step: ensure we have a provider
	if err := mutateOnProvider(ctx, m.cc, o); err != nil {
		return err
	}

	// @step: ensure we have a revision
	if err := mutateOnRevision(ctx, m.cc, o); err != nil {
		return err
	}

	return nil
}

// mutateOnRevision is responsible for adding a revision if not defined
func mutateOnRevision(ctx context.Context, cc client.Client, o *terraformv1alpha1.CloudResource) error {
	switch {
	case o.Spec.Plan.Revision != "":
		return nil
	}

	// @step: if no revision is defined we get the latest from the plan
	plan := &terraformv1alpha1.Plan{}
	plan.Name = o.Spec.Plan.Name

	found, err := kubernetes.GetIfExists(ctx, cc, plan)
	if err != nil {
		return err
	}
	if !found {
		return fmt.Errorf("spec.plan.name resource %q not found", o.Spec.Plan.Name)
	}
	if plan.Status.Latest.Revision == "" {
		return fmt.Errorf("spec.plan.name resource %q does not have a latest revision", o.Spec.Plan.Name)
	}

	o.Spec.Plan.Revision = plan.Status.Latest.Revision

	return nil
}

// mutateOnProvider is used to fill in a provider default if required
func mutateOnProvider(ctx context.Context, cc client.Client, o *terraformv1alpha1.CloudResource) error {
	switch {
	case o.Spec.ProviderRef != nil && o.Spec.ProviderRef.Name != "":
		return nil
	}

	// @step: retrieve a list of all providers
	list := &terraformv1alpha1.ProviderList{}
	if err := cc.List(ctx, list); err != nil {
		return err
	}

	var provider *terraformv1alpha1.Provider

	// @step: ensure only one provider is default as default
	var count int
	for i := 0; i < len(list.Items); i++ {
		switch {
		case list.Items[i].Annotations == nil:
			continue
		case list.Items[i].Annotations[terraformv1alpha1.DefaultProviderAnnotation] == "true":
			count++
			provider = &list.Items[i]
		}
	}
	if count == 0 {
		return nil
	}
	if count > 1 {
		return errors.New("only one provider can be default, please contact your administrator")
	}

	o.Spec.ProviderRef = &terraformv1alpha1.ProviderReference{
		Name: provider.Name,
	}

	return nil
}
