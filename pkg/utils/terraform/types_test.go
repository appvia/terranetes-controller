/*
 * Copyright (C) 2026  Appvia Ltd <info@appvia.io>
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

package terraform

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestOutputValueString(t *testing.T) {
	testCases := []struct {
		name     string
		output   *OutputValue
		expected string
	}{
		{
			name:     "nil value returns empty string",
			output:   &OutputValue{Value: nil},
			expected: "",
		},
		{
			name:     "string value",
			output:   &OutputValue{Value: "my-output-string"},
			expected: "my-output-string",
		},
		{
			name:     "integer value",
			output:   &OutputValue{Value: 12345},
			expected: "12345",
		},
		{
			name:     "boolean value",
			output:   &OutputValue{Value: true},
			expected: "true",
		},
		{
			name:     "map or complex value",
			output:   &OutputValue{Value: map[string]string{"foo": "bar"}},
			expected: "map[foo:bar]",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.expected, tc.output.String())
		})
	}
}

func TestStateCountResources(t *testing.T) {
	testCases := []struct {
		name     string
		state    *State
		expected int
	}{
		{
			name:     "empty resources",
			state:    &State{},
			expected: 0,
		},
		{
			name: "only managed resources",
			state: &State{
				Resources: []Resource{
					{
						Mode: "managed",
						Instances: []map[string]interface{}{
							{"index_key": 0},
							{"index_key": 1},
						},
					},
					{
						Mode: "managed",
						Instances: []map[string]interface{}{
							{"index_key": "a"},
						},
					},
				},
			},
			expected: 3,
		},
		{
			name: "mix of managed and data resources",
			state: &State{
				Resources: []Resource{
					{
						Mode: "data",
						Instances: []map[string]interface{}{
							{"id": "data_source_1"},
						},
					},
					{
						Mode: "managed",
						Instances: []map[string]interface{}{
							{"id": "res_1"},
						},
					},
					{
						Mode: "unknown",
						Instances: []map[string]interface{}{
							{"id": "other_1"},
						},
					},
				},
			},
			expected: 1,
		},
		{
			name: "managed resource with no instances",
			state: &State{
				Resources: []Resource{
					{
						Mode:      "managed",
						Instances: []map[string]interface{}{},
					},
				},
			},
			expected: 0,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.expected, tc.state.CountResources())
		})
	}
}

func TestStateHasOutputs(t *testing.T) {
	testCases := []struct {
		name     string
		state    *State
		expected bool
	}{
		{
			name:     "nil outputs map",
			state:    &State{Outputs: nil},
			expected: false,
		},
		{
			name:     "empty outputs map",
			state:    &State{Outputs: map[string]OutputValue{}},
			expected: false,
		},
		{
			name: "with outputs",
			state: &State{
				Outputs: map[string]OutputValue{
					"endpoint": {Value: "https://example.com"},
				},
			},
			expected: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.expected, tc.state.HasOutputs())
		})
	}
}

func TestPlanID(t *testing.T) {
	testCases := []struct {
		name      string
		timestamp string
		expected  string
	}{
		{
			name:      "already label safe",
			timestamp: "2026-09-24T10.00.00Z",
			expected:  "2026-09-24T10.00.00Z",
		},
		{
			name:      "contains colons and spaces",
			timestamp: "2026-09-24 10:00:00+00:00",
			expected:  "2026-09-24-10-00-00-00-00",
		},
		{
			name:      "special characters stripped to hyphens",
			timestamp: "plan@2026#v1/stage",
			expected:  "plan-2026-v1-stage",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			p := Plan{Timestamp: tc.timestamp}
			assert.Equal(t, tc.expected, p.ID())
		})
	}
}

func TestPlanNeedsApply(t *testing.T) {
	boolTrue := true
	boolFalse := false

	testCases := []struct {
		name     string
		plan     Plan
		expected bool
	}{
		{
			name: "Applyable field is explicitly true",
			plan: Plan{
				Applyable: &boolTrue,
				ResourceChanges: []ResourceChange{
					{Change: Change{Actions: []ChangeAction{TFActionNoOp}}},
				},
			},
			expected: true,
		},
		{
			name: "Applyable field is explicitly false",
			plan: Plan{
				Applyable: &boolFalse,
				ResourceChanges: []ResourceChange{
					{Change: Change{Actions: []ChangeAction{"create"}}},
				},
			},
			expected: false,
		},
		{
			name: "Applyable nil: empty plan",
			plan: Plan{
				Applyable: nil,
			},
			expected: false,
		},
		{
			name: "Applyable nil: only no-op resource changes",
			plan: Plan{
				Applyable: nil,
				ResourceChanges: []ResourceChange{
					{Change: Change{Actions: []ChangeAction{TFActionNoOp}}},
					{Change: Change{Actions: []ChangeAction{TFActionNoOp, TFActionNoOp}}},
				},
			},
			expected: false,
		},
		{
			name: "Applyable nil: resource change with create action",
			plan: Plan{
				Applyable: nil,
				ResourceChanges: []ResourceChange{
					{Change: Change{Actions: []ChangeAction{"create"}}},
				},
			},
			expected: true,
		},
		{
			name: "Applyable nil: resource change with update action",
			plan: Plan{
				Applyable: nil,
				ResourceChanges: []ResourceChange{
					{Change: Change{Actions: []ChangeAction{"update"}}},
				},
			},
			expected: true,
		},
		{
			name: "Applyable nil: resource change with delete action",
			plan: Plan{
				Applyable: nil,
				ResourceChanges: []ResourceChange{
					{Change: Change{Actions: []ChangeAction{"delete"}}},
				},
			},
			expected: true,
		},
		{
			name: "Applyable nil: only no-op output changes",
			plan: Plan{
				Applyable: nil,
				OutputChanges: map[string]OutputChange{
					"out": {Actions: []ChangeAction{TFActionNoOp}},
				},
			},
			expected: false,
		},
		{
			name: "Applyable nil: output change with non-noop action",
			plan: Plan{
				Applyable: nil,
				OutputChanges: map[string]OutputChange{
					"out": {Actions: []ChangeAction{"create"}},
				},
			},
			expected: true,
		},
		{
			name: "Applyable nil: resource no-op but output non-noop",
			plan: Plan{
				Applyable: nil,
				ResourceChanges: []ResourceChange{
					{Change: Change{Actions: []ChangeAction{TFActionNoOp}}},
				},
				OutputChanges: map[string]OutputChange{
					"out": {Actions: []ChangeAction{"update"}},
				},
			},
			expected: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.expected, tc.plan.NeedsApply())
		})
	}
}
