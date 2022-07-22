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

package utils

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"

	terraformv1alphav1 "github.com/appvia/terranetes-controller/pkg/apis/terraform/v1alpha1"
)

// IsSelectorMatch is used to check if the resource matches the selectors.
func IsSelectorMatch(
	selector terraformv1alphav1.Selector,
	resourceLabels, namespaceLabels map[string]string) (bool, error) {

	if selector.Namespace == nil && selector.Resource == nil {
		return true, nil
	}

	if selector.Namespace != nil {
		match, err := IsLabelSelectorMatch(namespaceLabels, *selector.Namespace)
		if err != nil {
			return false, err
		}
		if !match {

			return false, nil
		}
	}

	if selector.Resource != nil {
		return IsLabelSelectorMatch(resourceLabels, *selector.Resource)
	}

	return true, nil
}

// IsLabelSelectorMatch is used to check if the selectors matches the labels.
func IsLabelSelectorMatch(source map[string]string, selector metav1.LabelSelector) (bool, error) {
	matcher, err := metav1.LabelSelectorAsSelector(&selector)
	if err != nil {
		return false, err
	}
	if matcher.Empty() {
		return false, nil
	}

	return matcher.Matches(labels.Set(source)), nil
}
