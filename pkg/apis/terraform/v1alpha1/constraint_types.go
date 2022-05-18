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

package v1alpha1

import (
	"regexp"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// Constraints defined a collection of constraints which can be applied against
// the terraform configurations
type Constraints struct {
	// Modules is a list of regexes which are permitted as module sources
	// +kubebuilder:validation:Optional
	Modules *ModuleConstraint `json:"modules,omitempty"`
	// Checkov provides a definition to enforce checkov policies on the terraform
	// configurations
	// +kubebuilder:validation:Optional
	Checkov *PolicyConstraint `json:"checkov,omitempty"`
}

// ModuleConstraint provides a collection of constraints on modules
type ModuleConstraint struct {
	// Allowed is a list of regexes which are permitted as module sources
	// +kubebuilder:validation:Optional
	Allowed []string `json:"allowed,omitempty"`
}

// Selector defines the definition for a selector on configuration labels
// of the namespace the resource resides
type Selector struct {
	// Namespace provides the ability to filter on the namespace
	// +kubebuilder:validation:Optional
	Namespace *metav1.LabelSelector `json:"namespace,omitempty"`
	// Resource provides the ability to filter on the resource labels
	// +kubebuilder:validation:Optional
	Resource *metav1.LabelSelector `json:"resource,omitempty"`
}

// PolicyConstraint defines the checkov policies the configurations must comply with
type PolicyConstraint struct {
	// Checks is a list of checks which should be applied against the configuration
	// Please see https://www.checkov.io/5.Policy%20Index/terraform.html
	// +kubebuilder:validation:Optional
	Checks []string `json:"checks,omitempty"`
	// Selector is the selector on the namespace or labels on the configuration. Note, defining
	// no selector dictates the policy should apply to all
	// +kubebuilder:validation:Optional
	Selector *Selector `json:"selector,omitempty"`
	// SkipChecks is a collection of checks which need to be skipped
	// +kubebuilder:validation:Optional
	SkipChecks []string `json:"skipChecks,omitempty"`
}

// ExternalCheck defines the definition for an external check - this comprises of the
// source and any optional secret
type ExternalCheck struct {
	// Name provides a arbitrary name to the checks - note, this name is used as the directory
	// name when we source the code
	// +kubebuilder:validation:Required
	Name string `json:"name,omitempty"`
	// URL is the source external checks - this is usualluy a git repository. The notation
	// for this is https://github.com/hashicorp/go-getter
	// +kubebuilder:validation:Required
	URL string `json:"url,omitempty"`
	// SecretRef is reference to secret which contains environment variables used by the source
	// command to retrieve the code. This could be cloud credentials, ssh keys, git username
	// and password etc
	// +kubebuilder:validation:Optional
	SecretRef *v1.SecretReference `json:"secretRef,omitempty"`
}

// Matches returns true if the module matches the regex
func (m *ModuleConstraint) Matches(module string) (bool, error) {
	for _, m := range m.Allowed {
		re, err := regexp.Compile(m)
		if err != nil {
			return false, err
		}

		if re.MatchString(module) {
			return true, nil
		}
	}

	return false, nil
}
