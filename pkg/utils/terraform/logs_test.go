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
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestFindChangesInLogsNoChanges(t *testing.T) {
	logs := `
There is nothing
here to see
  Your infrastructure matches the configuration.
 Plan nope, not here
`
	found, err := FindChangesInLogs(strings.NewReader(logs))
	assert.NoError(t, err)
	assert.False(t, found)
}

func TestFindChangesInLogs(t *testing.T) {
	logs := `
There is nothing
 Plan nope, not here
`
	found, err := FindChangesInLogs(strings.NewReader(logs))
	assert.NoError(t, err)
	assert.True(t, found)
}
