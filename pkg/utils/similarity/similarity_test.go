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

package similarity

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCloseness(t *testing.T) {
	h := Closeness("This is a sentence about a kubernetes controller", []string{
		"This have nothing to do with anything",
		"This is related to kubernetes, but not anything else",
		"This is entirely about kubernetes and controllers",
	}, Filter{})
	assert.NotEmpty(t, h.Scores)
}

func TestClosenessWithMin(t *testing.T) {
	h := Closeness("This is a sentence about a kubernetes controller", []string{
		"This have nothing to do with anything",
		"This is related to kubernetes, but not anything else",
		"This is entirely about kubernetes and controllers",
		"This is a sentence about a kubernetes controllers",
	}, Filter{
		Min: 0.9,
	})
	assert.NotEmpty(t, h.Scores)
	assert.Equal(t, 1, len(h.Scores))
}

func TestClosenessWithTopN(t *testing.T) {
	h := Closeness("This is a sentence about a kubernetes controller", []string{
		"This have nothing to do with anything",
		"This is related to kubernetes, but not anything else",
		"This is entirely about kubernetes and controllers",
		"This is a sentence about a kubernetes controllers",
		"This is a about a kubernetes controllers",
	}, Filter{
		TopN: 2,
		Min:  0.5,
	})
	assert.NotEmpty(t, h.Scores)
	assert.Equal(t, 2, len(h.Scores))
}

func TestClosest(t *testing.T) {
	cases := []struct {
		Sentence string
		List     []string
		Expected string
	}{
		{
			Sentence: "This the network vpc id the cluster is associated",
			List: []string{
				"This is not a match",
				"The network vpc id which the cluster resides in",
				"The network associated to the cluster",
			},
			Expected: "The network vpc id which the cluster resides in",
		},
		{
			Sentence: "The EKS oidc provider arn",
			List: []string{
				"The ARN of the OIDC issuer URL of the EKS cluster",
				"The ID of the VPC where the EKS cluster is deployed",
				"The OIDC issuer URL of the EKS cluster",
			},
			Expected: "The ARN of the OIDC issuer URL of the EKS cluster",
		},
		{
			Sentence: "The subnet where the cluster is deployed",
			List: []string{
				"The subnets associated to the EKS cluster definition, where nodegroup",
				"Subnets associated to the EKS cluster definition",
			},
			Expected: "The subnets associated to the EKS cluster definition, where nodegroup",
		},
	}
	for _, c := range cases {
		sentence := Closest(c.Sentence, c.List)
		assert.Equal(t, c.Expected, sentence)
	}
}
