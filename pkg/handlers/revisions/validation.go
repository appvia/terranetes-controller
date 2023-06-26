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

package revisions

import (
	"context"
	"fmt"
	"reflect"
	"strings"

	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	"github.com/Masterminds/semver"
	"github.com/tidwall/gjson"

	terraformv1alpha1 "github.com/appvia/terranetes-controller/pkg/apis/terraform/v1alpha1"
)

type validator struct {
	cc client.Client
	// EnableUpdateProtection is a flag to enable revision protection, meaning the
	// revision cannot be updated once in use
	EnableUpdateProtection bool
}

// NewValidator is validation handler
func NewValidator(cc client.Client, updateProtection bool) admission.CustomValidator {
	return &validator{cc: cc, EnableUpdateProtection: updateProtection}
}

// ValidateCreate is called when a new resource is created
func (v *validator) ValidateCreate(ctx context.Context, obj runtime.Object) error {
	o, ok := obj.(*terraformv1alpha1.Revision)
	if !ok {
		return fmt.Errorf("expected a Revision but got a %T", obj)
	}

	return v.validate(ctx, nil, o)
}

// ValidateUpdate is called when a resource is being updated
func (v *validator) ValidateUpdate(ctx context.Context, oldObj, newObj runtime.Object) error {
	after, ok := newObj.(*terraformv1alpha1.Revision)
	if !ok {
		return fmt.Errorf("expected a Revision but got a %T", newObj)
	}
	before, ok := oldObj.(*terraformv1alpha1.Revision)
	if !ok {
		return fmt.Errorf("expected a Revision but got a %T", oldObj)
	}

	return v.validate(ctx, before, after)
}

// ValidateDelete is called when a resource is being deleted
func (v *validator) ValidateDelete(ctx context.Context, obj runtime.Object) error {
	return nil
}

// validate is called when a resource is being created or updated
// nolint:gocyclo
func (v *validator) validate(ctx context.Context, before, revision *terraformv1alpha1.Revision) error {
	updating := before != nil

	switch {
	case revision.Spec.Plan.Name == "":
		return fmt.Errorf("spec.plan.name is required")
	case revision.Spec.Plan.Description == "":
		return fmt.Errorf("spec.plan.description is required")
	case revision.Spec.Plan.Revision == "":
		return fmt.Errorf("spec.plan.version is required")
	}

	// @check the version is semvar
	if _, err := semver.NewVersion(revision.Spec.Plan.Revision); err != nil {
		return fmt.Errorf("spec.plan.version is not a valid semver")
	}

	// @step: check the dependencies
	for i, x := range revision.Spec.Dependencies {
		switch {
		case x.Context == nil && x.Provider == nil && x.Terranetes == nil:
			return fmt.Errorf("spec.plan.dependencies[%d] is missing a context, provider or terranetes", i)

		case x.Context != nil:
			if x.Context.Name == "" {
				return fmt.Errorf("spec.plan.dependencies[%d].context.name is required", i)
			}

		case x.Provider != nil:
			if x.Provider.Cloud == "" {
				return fmt.Errorf("spec.plan.dependencies[%d].provider.cloud is required", i)
			}

		case x.Terranetes != nil:
			if x.Terranetes.Version == "" {
				return fmt.Errorf("spec.plan.dependencies[%d].terranetes.version is required", i)
			}
		}
	}

	// @step: check the revision inputs
	for i, x := range revision.Spec.Inputs {
		switch {
		case x.Description == "":
			return fmt.Errorf("spec.plan.inputs[%d].description is required", i)

		case x.Key == "":
			return fmt.Errorf("spec.plan.inputs[%d].key is required", i)

		case x.Default != nil:
			if !gjson.ParseBytes(x.Default.Raw).Get("value").Exists() {
				return fmt.Errorf("spec.plan.inputs[%d].default.value is required", i)
			}
		}
	}

	// @step: you cannot create revisions with the same plan and version
	list := &terraformv1alpha1.RevisionList{}
	if err := v.cc.List(ctx, list); err != nil {
		return err
	}
	var existing []string

	for _, x := range list.Items {
		if x.Name == revision.Name {
			continue
		}
		if x.Spec.Plan.Name == revision.Spec.Plan.Name && x.Spec.Plan.Revision == revision.Spec.Plan.Revision {
			existing = append(existing, x.Name)
		}
	}
	if len(existing) > 0 {
		return fmt.Errorf("spec.plan.revision same version already exists on revision/s: %v", strings.Join(existing, ","))
	}

	// @step: if update protection is enabled, we need to ensure there isn't any current
	// cloud resource using the revision
	if updating && v.EnableUpdateProtection {
		switch {
		// if we have the annotation: true we skip the check
		case revision.GetAnnotations()[terraformv1alpha1.RevisionSkipUpdateProtectionAnnotation] == "true":
			break
		// if nothing has changed in the spec, we allow it
		case reflect.DeepEqual(before.Spec, revision.Spec):
			break

		default:
			list := &terraformv1alpha1.CloudResourceList{}
			if err := v.cc.List(ctx, list,
				client.MatchingLabels(map[string]string{
					terraformv1alpha1.CloudResourcePlanNameLabel: revision.Spec.Plan.Name,
					terraformv1alpha1.CloudResourceRevisionLabel: revision.Spec.Plan.Revision,
				},
				)); err != nil {
				return fmt.Errorf("failed to retrieved list of cloud resources: %w", err)
			}

			if len(list.Items) > 0 {
				return fmt.Errorf("in use by cloudresource/s, update denied (use %s to override)", terraformv1alpha1.RevisionSkipUpdateProtectionAnnotation)
			}
		}
	}

	return nil
}
