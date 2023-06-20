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

package utils

import (
	"testing"

	"github.com/Masterminds/semver"
	"github.com/stretchr/testify/assert"
)

func TestVersionIncrement(t *testing.T) {
	version, err := GetVersionIncrement("bad")
	assert.Error(t, err)
	assert.Equal(t, "", version)
	assert.Equal(t, semver.ErrInvalidSemVer, err)

	version, err = GetVersionIncrement("v0.0.1")
	assert.NoError(t, err)
	assert.Equal(t, "0.0.2", version)
}
