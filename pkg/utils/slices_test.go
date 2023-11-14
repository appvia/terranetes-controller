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
)

func TestToMap(t *testing.T) {
	m, err := ToMap([]string{"a=b", "c=d"})
	assert.NoError(t, err)
	assert.Equal(t, map[string]string{"a": "b", "c": "d"}, m)
}

func TestToMapError(t *testing.T) {
	m, err := ToMap([]string{"a=b", "c"})
	assert.Error(t, err)
	assert.Nil(t, m)
}

func TestMaxChars(t *testing.T) {
	v := MaxChars("hello", 3)
	assert.Equal(t, "hel", v)
}

func TestContainsPrefix(t *testing.T) {
	assert.True(t, ContainsPrefix("/tmp/revision", []string{"/", "."}))
	assert.True(t, ContainsPrefix(".", []string{"/", "."}))
	assert.False(t, ContainsPrefix("abc", []string{"def"}))
}

func TestContainsOK(t *testing.T) {
	list := []string{"a", "b", "c"}

	assert.True(t, Contains("a", list))
	assert.True(t, Contains("c", list))
	assert.True(t, Contains("b", list))
}

func TestContainsBad(t *testing.T) {
	list := []string{"a", "b", "c"}

	assert.False(t, Contains("d", list))
}

func TestContainsList(t *testing.T) {
	a := []string{"b"}
	b := []string{"a", "b", "c"}

	assert.True(t, ContainsList(a, b))
	assert.False(t, ContainsList([]string{"x"}, b))
}

func TestUnique(t *testing.T) {
	assert.Equal(t, []string{"a", "b", "c"}, Unique([]string{"a", "b", "b", "c"}))
}

func TestSorted(t *testing.T) {
	assert.Equal(t, []string{"a", "b", "c"}, Sorted([]string{"c", "a", "b"}))
}
