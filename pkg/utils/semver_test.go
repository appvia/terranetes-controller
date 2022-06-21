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

func TestSortSemverVersions(t *testing.T) {
	versions := []string{"1.0.0", "1.0.2", "0.0.2"}

	sorted, err := SortSemverVersions(versions)
	assert.NoError(t, err)
	assert.Equal(t, []string{"0.0.2", "1.0.0", "1.0.2"}, sorted)
}

func TestSortSemverVersionsWithTags(t *testing.T) {
	versions := []string{"v1.0.0", "v1.0.2", "v0.0.2"}

	sorted, err := SortSemverVersions(versions)
	assert.NoError(t, err)
	assert.Equal(t, []string{"v0.0.2", "v1.0.0", "v1.0.2"}, sorted)
}

func TestSortSemverVersionsBad(t *testing.T) {
	versions := []string{"1.0.0", "1.0.2", "latest"}

	sorted, err := SortSemverVersions(versions)
	assert.Error(t, err)
	assert.Empty(t, sorted)
}
