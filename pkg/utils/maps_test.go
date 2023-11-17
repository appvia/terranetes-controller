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

func TestMergeMapsBothNil(t *testing.T) {
	m := MergeStringMaps(nil, nil)
	assert.Equal(t, map[string]string{}, m)
}

func TestMergeMapsBNil(t *testing.T) {
	a := map[string]string{"a": "b"}

	m := MergeStringMaps(a, nil)
	assert.Equal(t, a, m)
}

func TestMergeMapMany(t *testing.T) {
	a := map[string]string{"a": "b"}
	b := map[string]string{"c": "d"}
	c := map[string]string{"a": "z"}

	m := MergeStringMaps(a, b, c)
	assert.Equal(t, map[string]string{"a": "z", "c": "d"}, m)
}

func TestMergeMapsNil(t *testing.T) {
	b := map[string]string{"a": "b"}

	l := MergeStringMaps(nil, b)

	assert.Equal(t, map[string]string{"a": "b"}, l)
}

func TestMergeMapsChange(t *testing.T) {
	a := map[string]string{"a": "b"}
	b := map[string]string{"a": "changed"}

	l := MergeStringMaps(a, b)

	assert.Equal(t, map[string]string{"a": "changed"}, l)
}
