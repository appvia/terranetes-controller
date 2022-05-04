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
