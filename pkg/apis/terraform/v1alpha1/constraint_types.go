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
	// Modules provides the ability to control the source for all terraform modules. Allowing
	// platform teams to control where the modules can be downloaded from.
	// +kubebuilder:validation:Optional
	Modules *ModuleConstraint `json:"modules,omitempty"`
	// Checkov provides the ability to enforce a set of security standards on all configurations.
	// These can be configured to target specific resources based on namespace and resource
	// labels
	// +kubebuilder:validation:Optional
	Checkov *PolicyConstraint `json:"checkov,omitempty"`
}

// ModuleConstraint provides a collection of constraints on modules
type ModuleConstraint struct {
	// Allowed is a collection of regexes which are applied to the source of the terraform
	// configuration. The configuration MUST match one or more of the regexes in order to
	// be allowed to run.
	// +kubebuilder:validation:Optional
	Allowed []string `json:"allowed,omitempty"`
	// Selector is the selector on the namespace or labels on the configuration. By leaving this
	// fields empty you can implicitedly selecting all configurations.
	// +kubebuilder:validation:Optional
	Selector *Selector `json:"selector,omitempty"`
}

// Selector defines the definition for a selector on configuration labels
// of the namespace the resource resides
type Selector struct {
	// Namespace is used to filter a configuration based on the namespace labels of
	// where it exists
	// +kubebuilder:validation:Optional
	Namespace *metav1.LabelSelector `json:"namespace,omitempty"`
	// Resource provides the ability to filter a configuration based on it's labels
	// +kubebuilder:validation:Optional
	Resource *metav1.LabelSelector `json:"resource,omitempty"`
}

// PolicyConstraint defines the checkov policies the configurations must comply with
type PolicyConstraint struct {
	// Checks is a list of checks which should be applied against the configuration. Note, an
	// empty list here implies checkov should run ALL checks.
	// Please see https://www.checkov.io/5.Policy%20Index/terraform.html
	// +kubebuilder:validation:Optional
	Checks []string `json:"checks,omitempty"`
	// Selector is the selector on the namespace or labels on the configuration. By leaving this
	// fields empty you can implicitedly selecting all configurations.
	// +kubebuilder:validation:Optional
	Selector *Selector `json:"selector,omitempty"`
	// SkipChecks is a collection of checkov checks which you can defined as skipped. The security
	// scan will ignore any failures on these checks.
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
