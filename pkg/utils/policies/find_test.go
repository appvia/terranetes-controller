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

package policies

import (
	"testing"

	"github.com/stretchr/testify/assert"

	terraformv1alpha1 "github.com/appvia/terranetes-controller/pkg/apis/terraform/v1alpha1"
	"github.com/appvia/terranetes-controller/test/fixtures"
)

func TestFindModuleConstraintsEmpty(t *testing.T) {
	list := &terraformv1alpha1.PolicyList{}

	filtered := FindModuleConstraints(list)
	assert.Empty(t, filtered)
}

func TestFindModuleConstraintsNone(t *testing.T) {
	list := &terraformv1alpha1.PolicyList{
		Items: []terraformv1alpha1.Policy{
			*fixtures.NewMatchAllPolicyConstraint("test1"),
		},
	}

	filtered := FindModuleConstraints(list)
	assert.Len(t, filtered, 0)
}

func TestFindModuleConstraints(t *testing.T) {
	list := &terraformv1alpha1.PolicyList{
		Items: []terraformv1alpha1.Policy{
			*fixtures.NewMatchAllPolicyConstraint("test1"),
			*fixtures.NewMatchAllModuleConstraint("test2"),
		},
	}

	filtered := FindModuleConstraints(list)
	assert.Len(t, filtered, 1)
}

func TestFindSecurityPolicyConstraintsEmpty(t *testing.T) {
	list := &terraformv1alpha1.PolicyList{}

	filtered := FindSecurityPolicyConstraints(list)
	assert.Empty(t, filtered)
}

func TestFindSecurityPolicyConstraintsNone(t *testing.T) {
	list := &terraformv1alpha1.PolicyList{
		Items: []terraformv1alpha1.Policy{
			*fixtures.NewMatchAllModuleConstraint("test1"),
		},
	}

	filtered := FindSecurityPolicyConstraints(list)
	assert.Len(t, filtered, 0)
}

func TestFindSecurityPolicyConstraints(t *testing.T) {
	list := &terraformv1alpha1.PolicyList{
		Items: []terraformv1alpha1.Policy{
			*fixtures.NewMatchAllModuleConstraint("test1"),
			*fixtures.NewMatchAllPolicyConstraint("test2"),
		},
	}

	filtered := FindSecurityPolicyConstraints(list)
	assert.Len(t, filtered, 1)
}
