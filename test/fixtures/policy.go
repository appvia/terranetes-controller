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

package fixtures

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	terraformv1alphav1 "github.com/appvia/terranetes-controller/pkg/apis/terraform/v1alpha1"
)

// NewPolicy returns an empty policy
func NewPolicy(name string) *terraformv1alphav1.Policy {
	return &terraformv1alphav1.Policy{
		ObjectMeta: metav1.ObjectMeta{Name: name},
		Spec:       terraformv1alphav1.PolicySpec{},
	}
}

// NewMatchAllPolicyConstraint returns a policy which matches all configurations
func NewMatchAllPolicyConstraint(name string) *terraformv1alphav1.Policy {
	p := NewPolicy(name)
	p.Spec.Constraints = &terraformv1alphav1.Constraints{}
	p.Spec.Constraints.Checkov = &terraformv1alphav1.PolicyConstraint{}

	return p
}
