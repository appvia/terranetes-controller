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
	"testing"

	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	terraformv1alphav1 "github.com/appvia/terranetes-controller/pkg/apis/terraform/v1alpha1"
)

func TestIsSelectorMatch(t *testing.T) {
	cases := []struct {
		ResourceLabels map[string]string
		NamespaceLabel map[string]string
		Selector       terraformv1alphav1.Selector
		Expect         bool
	}{
		{
			Expect: true,
		},
		{
			Expect: true,
			Selector: terraformv1alphav1.Selector{
				Namespace: &metav1.LabelSelector{
					MatchLabels: map[string]string{"is": "there"},
				},
			},
			NamespaceLabel: map[string]string{"is": "there"},
		},
		{
			Expect: false,
			Selector: terraformv1alphav1.Selector{
				Namespace: &metav1.LabelSelector{
					MatchLabels: map[string]string{"not": "there"},
				},
			},
			NamespaceLabel: map[string]string{"is": "there"},
		},
		{
			Expect: false,
			Selector: terraformv1alphav1.Selector{
				Namespace: &metav1.LabelSelector{
					MatchExpressions: []metav1.LabelSelectorRequirement{
						{
							Key:      "not_there",
							Operator: metav1.LabelSelectorOpExists,
						},
					},
				},
			},
			NamespaceLabel: map[string]string{"is": "there"},
		},
		{
			Expect: true,
			Selector: terraformv1alphav1.Selector{
				Namespace: &metav1.LabelSelector{
					MatchExpressions: []metav1.LabelSelectorRequirement{
						{
							Key:      "is",
							Operator: metav1.LabelSelectorOpExists,
						},
					},
				},
			},
			NamespaceLabel: map[string]string{"is": "there"},
		},
		{
			Expect: false,
			Selector: terraformv1alphav1.Selector{
				Resource: &metav1.LabelSelector{
					MatchExpressions: []metav1.LabelSelectorRequirement{
						{
							Key:      "is",
							Operator: metav1.LabelSelectorOpExists,
						},
					},
				},
			},
			ResourceLabels: map[string]string{"not": "there"},
		},
		{
			Expect: true,
			Selector: terraformv1alphav1.Selector{
				Resource: &metav1.LabelSelector{
					MatchExpressions: []metav1.LabelSelectorRequirement{
						{
							Key:      "is",
							Operator: metav1.LabelSelectorOpExists,
						},
					},
				},
			},
			ResourceLabels: map[string]string{"is": "there"},
		},
	}

	for _, c := range cases {
		match, err := IsSelectorMatch(c.Selector, c.ResourceLabels, c.NamespaceLabel)
		assert.NoError(t, err)
		assert.Equal(t, c.Expect, match)
	}
}

func TestIsLabelSelectorMatch(t *testing.T) {
	cases := []struct {
		Labels   map[string]string
		Selector metav1.LabelSelector
		Expect   bool
	}{
		{
			Labels:   map[string]string{"is": "empty"},
			Selector: metav1.LabelSelector{},
			Expect:   false,
		},
		{
			Labels: map[string]string{"is": "there"},
			Selector: metav1.LabelSelector{
				MatchLabels: map[string]string{"is": "there"},
			},
			Expect: true,
		},
		{
			Labels: map[string]string{"is": "there"},
			Selector: metav1.LabelSelector{
				MatchExpressions: []metav1.LabelSelectorRequirement{
					{
						Key:      "is",
						Operator: metav1.LabelSelectorOpExists,
					},
				},
			},
			Expect: true,
		},
		{
			Labels: map[string]string{"is": "there"},
			Selector: metav1.LabelSelector{
				MatchExpressions: []metav1.LabelSelectorRequirement{
					{
						Key:      "is",
						Operator: metav1.LabelSelectorOpIn,
						Values:   []string{"there"},
					},
				},
			},
			Expect: true,
		},
	}

	for _, c := range cases {
		match, err := IsLabelSelectorMatch(c.Labels, c.Selector)
		assert.NoError(t, err)
		assert.Equal(t, c.Expect, match)
	}
}
