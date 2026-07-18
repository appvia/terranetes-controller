/*
 * Copyright (C) 2024  Appvia Ltd <info@appvia.io>
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

package main

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestIsValid(t *testing.T) {
	cases := []struct {
		Name        string
		Step        Step
		ExpectError bool
	}{
		{
			Name:        "no commands",
			Step:        Step{},
			ExpectError: true,
		},
		{
			Name: "valid step with command",
			Step: Step{
				Commands: []string{"echo hello"},
				Shell:    "/bin/sh",
			},
			ExpectError: false,
		},
		{
			Name: "upload file without namespace",
			Step: Step{
				Commands:   []string{"echo hello"},
				UploadFile: []string{"secret=/path/to/file"},
			},
			ExpectError: true,
		},
		{
			Name: "upload file with namespace",
			Step: Step{
				Commands:   []string{"echo hello"},
				UploadFile: []string{"secret=/path/to/file"},
				Namespace:  "default",
			},
			ExpectError: false,
		},
		{
			Name: "upload file with invalid format",
			Step: Step{
				Commands:   []string{"echo hello"},
				UploadFile: []string{"invalid-format"},
				Namespace:  "default",
			},
			ExpectError: true,
		},
		{
			Name: "upload-on-error without namespace",
			Step: Step{
				Commands:          []string{"echo hello"},
				UploadOnErrorFile: []string{"secret=/path/to/file"},
			},
			ExpectError: true,
		},
		{
			Name: "upload-on-error with namespace",
			Step: Step{
				Commands:          []string{"echo hello"},
				UploadOnErrorFile: []string{"secret=/path/to/file"},
				Namespace:         "default",
			},
			ExpectError: false,
		},
		{
			Name: "upload-on-error with invalid format",
			Step: Step{
				Commands:          []string{"echo hello"},
				UploadOnErrorFile: []string{"invalid-format"},
				Namespace:         "default",
			},
			ExpectError: true,
		},
		{
			Name: "upload-on-error with valid key=path format",
			Step: Step{
				Commands:          []string{"echo hello"},
				UploadOnErrorFile: []string{"my-secret=/run/tfstate"},
				Namespace:         "terraform-system",
			},
			ExpectError: false,
		},
	}

	for _, tc := range cases {
		t.Run(tc.Name, func(t *testing.T) {
			err := tc.Step.IsValid()
			if tc.ExpectError {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestUploadOnErrorKeyPairs(t *testing.T) {
	cases := []struct {
		Name     string
		Step     Step
		Expected map[string]string
	}{
		{
			Name:     "no upload-on-error files",
			Step:     Step{},
			Expected: nil,
		},
		{
			Name: "single upload-on-error file",
			Step: Step{
				UploadOnErrorFile: []string{"my-secret=/run/tfstate"},
			},
			Expected: map[string]string{
				"my-secret": "/run/tfstate",
			},
		},
		{
			Name: "multiple upload-on-error files",
			Step: Step{
				UploadOnErrorFile: []string{
					"state-secret=/run/tfstate",
					"plan-secret=/run/plan.out",
				},
			},
			Expected: map[string]string{
				"state-secret": "/run/tfstate",
				"plan-secret":  "/run/plan.out",
			},
		},
		{
			Name: "upload-on-error with invalid format is skipped",
			Step: Step{
				UploadOnErrorFile: []string{"invalid-format"},
			},
			Expected: map[string]string{},
		},
	}

	for _, tc := range cases {
		t.Run(tc.Name, func(t *testing.T) {
			result := tc.Step.UploadOnErrorKeyPairs()
			assert.Equal(t, tc.Expected, result)
		})
	}
}

func TestUploadKeyPairs(t *testing.T) {
	cases := []struct {
		Name     string
		Step     Step
		Expected map[string]string
	}{
		{
			Name:     "no upload files",
			Step:     Step{},
			Expected: nil,
		},
		{
			Name: "single upload file",
			Step: Step{
				UploadFile: []string{"my-secret=/run/tfstate"},
			},
			Expected: map[string]string{
				"my-secret": "/run/tfstate",
			},
		},
		{
			Name: "multiple upload files",
			Step: Step{
				UploadFile: []string{
					"state-secret=/run/tfstate",
					"plan-secret=/run/plan.out",
				},
			},
			Expected: map[string]string{
				"state-secret": "/run/tfstate",
				"plan-secret":  "/run/plan.out",
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.Name, func(t *testing.T) {
			result := tc.Step.UploadKeyPairs()
			assert.Equal(t, tc.Expected, result)
		})
	}
}
