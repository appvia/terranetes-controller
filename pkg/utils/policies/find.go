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

package policies

import (
	"context"
	"fmt"
	"strings"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"sigs.k8s.io/controller-runtime/pkg/client"

	terraformv1alphav1 "github.com/appvia/terranetes-controller/pkg/apis/terraform/v1alpha1"
	"github.com/appvia/terranetes-controller/pkg/utils/weights"
)

// FindMatchingPolicy is called to find a match of policy for a given configurations
func FindMatchingPolicy(
	ctx context.Context,
	configuration *terraformv1alphav1.Configuration,
	namespace client.Object,
	list *terraformv1alphav1.PolicyList) (*terraformv1alphav1.PolicyConstraint, error) {

	if len(list.Items) == 0 {
		return nil, nil
	}

	priority := weights.New()

	for i := 0; i < len(list.Items); i++ {
		weight := 0

		if list.Items[i].Spec.Constraints == nil || list.Items[i].Spec.Constraints.Checkov == nil {
			continue
		}

		if list.Items[i].Spec.Constraints.Checkov.Selector != nil {
			if list.Items[i].Spec.Constraints.Checkov.Selector.Namespace != nil {
				selector, err := metav1.LabelSelectorAsSelector(list.Items[i].Spec.Constraints.Checkov.Selector.Namespace)
				if err != nil {
					return nil, err
				}
				if !selector.Empty() && !selector.Matches(labels.Set(namespace.GetLabels())) {
					continue
				}
				weight += 10
			}

			// @step: if we have a resource selector lets check it
			if list.Items[i].Spec.Constraints.Checkov.Selector.Resource != nil {
				selector, err := metav1.LabelSelectorAsSelector(list.Items[i].Spec.Constraints.Checkov.Selector.Namespace)
				if err != nil {
					return nil, err
				}
				if !selector.Matches(labels.Set(configuration.GetLabels())) {
					continue
				}
				weight += 20
			}
		}
		// @note: we always get if a policy is defined (even without selector), but the weight is adjusted if their
		// is a selector/s defined
		priority.Add(&list.Items[i], weight)
	}

	if priority.Size() == 0 {
		return nil, nil
	}

	matches := priority.Highest()
	if len(matches) > 1 {
		return nil, fmt.Errorf("multiple policies match configuration: %s", strings.Join(priority.HighestNames(), ", "))
	}

	return matches[0].(*terraformv1alphav1.Policy).Spec.Constraints.Checkov, nil
}
