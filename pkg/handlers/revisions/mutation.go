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

	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	terraformv1alpha1 "github.com/appvia/terranetes-controller/pkg/apis/terraform/v1alpha1"
	"github.com/appvia/terranetes-controller/pkg/utils"
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
	o, ok := obj.(*terraformv1alpha1.Revision)
	if !ok {
		return fmt.Errorf("expected Revision, not %T", obj)
	}

	o.Labels = utils.MergeStringMaps(o.Labels, map[string]string{
		terraformv1alpha1.RevisionPlanNameLabel: o.Spec.Plan.Name,
		terraformv1alpha1.RevisionNameLabel:     o.Spec.Plan.Revision,
	})

	return nil
}
