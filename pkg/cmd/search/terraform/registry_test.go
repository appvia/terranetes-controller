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

package terraform

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNew(t *testing.T) {
	c, err := New("https://registry.terraform.io")
	assert.NoError(t, err)
	assert.NotNil(t, c)
}

func TestInvalidNamespace(t *testing.T) {
	c, err := New("https://registry.terraform.io/appvia")
	assert.Error(t, err)
	assert.Nil(t, c)
}

func TestValidNamespace(t *testing.T) {
	c, err := New("https://registry.terraform.io/namespaces/appvia")
	assert.NoError(t, err)
	assert.NotNil(t, c)
}

func TestIsHandle(t *testing.T) {
	cases := []struct {
		Source   string
		Expected bool
	}{
		{
			Source: "",
		},
		{
			Source: "appvia/terraform",
		},
		{
			Source:   "terraform://appvia/terraform",
			Expected: true,
		},
		{
			Source:   "https://registry.terraform.io",
			Expected: true,
		},
	}
	for _, c := range cases {
		assert.Equal(t, c.Expected, IsHandle(c.Source))
	}
}
