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

package main

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSantizeSource(t *testing.T) {
	/*
		These will sanitize to the gitconfig urls in such a way. Note, subpaths are ignored

		[url "https://GIT_PASSWORD@github.com/appvia/terranetes-controller"]
			insteadOf = https://github.com/appvia/terranetes-controller
		[url "https://GIT_USERNAME:GIT_PASSWORD@github.com/appvia/terranetes-controller"]
			insteadOf = https://github.com/appvia/terranetes-controller
	*/

	cases := []struct {
		Location    string
		Source      string
		Destination string
		ExpectError bool
		Environment map[string]string
	}{
		{
			Location:    "https://github.com/appvia/terranetes-controller.git",
			Source:      "https://user:pass@github.com/appvia/terranetes-controller.git",
			Destination: "https://github.com/appvia/terranetes-controller.git",
			Environment: map[string]string{
				"GIT_USERNAME": "user",
				"GIT_PASSWORD": "pass",
			},
		},
		{
			Location:    "https://github.com/appvia/terranetes-controller.git//module/test",
			Source:      "https://user:pass@github.com/appvia/terranetes-controller.git",
			Destination: "https://github.com/appvia/terranetes-controller.git",
			Environment: map[string]string{
				"GIT_USERNAME": "user",
				"GIT_PASSWORD": "pass",
			},
		},
		{
			Location:    "git::https://github.com/appvia/terranetes-controller.git//module/test",
			Source:      "https://user:pass@github.com/appvia/terranetes-controller.git",
			Destination: "https://github.com/appvia/terranetes-controller.git",
			Environment: map[string]string{
				"GIT_USERNAME": "user",
				"GIT_PASSWORD": "pass",
			},
		},
		{
			Location:    "git::https://github.com/appvia/terranetes-controller.git",
			Source:      "https://token@github.com/appvia/terranetes-controller.git",
			Destination: "https://github.com/appvia/terranetes-controller.git",
			Environment: map[string]string{
				"GIT_PASSWORD": "token",
			},
		},
		{
			Location:    "https://github.com/appvia/terranetes-controller.git",
			Source:      "https://token@github.com/appvia/terranetes-controller.git",
			Destination: "https://github.com/appvia/terranetes-controller.git",
			Environment: map[string]string{
				"GIT_PASSWORD": "token",
			},
		},
		{
			Location:    "https://github.com/appvia/terranetes-controller.git//module/source",
			Source:      "https://token@github.com/appvia/terranetes-controller.git",
			Destination: "https://github.com/appvia/terranetes-controller.git",
			Environment: map[string]string{
				"GIT_PASSWORD": "token",
			},
		},
		{
			Location:    "git::https://github.com/appvia/terranetes-controller.git//module/source",
			Source:      "https://token@github.com/appvia/terranetes-controller.git",
			Destination: "https://github.com/appvia/terranetes-controller.git",
			Environment: map[string]string{
				"GIT_PASSWORD": "token",
			},
		},
		{
			Location:    "git::https://dev.azure.com/gambol99/terranetes-controller/_git/e2e//module/submodule",
			Source:      "https://token@dev.azure.com/gambol99/terranetes-controller/_git/e2e",
			Destination: "https://dev.azure.com/gambol99/terranetes-controller/_git/e2e",
			Environment: map[string]string{
				"GIT_PASSWORD": "token",
			},
		},
		{
			Location:    "git::https://dev.azure.com/gambol99/terranetes-controller/_git/e2e//module/submodule",
			Source:      "https://user:token@dev.azure.com/gambol99/terranetes-controller/_git/e2e",
			Destination: "https://dev.azure.com/gambol99/terranetes-controller/_git/e2e",
			Environment: map[string]string{
				"GIT_USERNAME": "user",
				"GIT_PASSWORD": "token",
			},
		},
	}
	for i, c := range cases {
		os.Unsetenv("GIT_PASSORD")
		os.Unsetenv("GIT_USERNAME")
		for k, v := range c.Environment {
			assert.NoError(t, os.Setenv(k, v))
		}

		source, destination, err := sanitizeSource(c.Location)
		if c.ExpectError {
			assert.Error(t, err, "case %d, expected an error", i)
		} else {
			assert.NoError(t, err, "case %d, expected no error", i)
		}
		assert.Equal(t, c.Source, source, "case %d, expected source to match", i)
		assert.Equal(t, c.Destination, destination, "case %d, expected destination to match", i)
	}
}
