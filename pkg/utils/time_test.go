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
	"time"

	"github.com/stretchr/testify/assert"
)

func TestHumanDuration(t *testing.T) {
	cases := []struct {
		Duration time.Duration
		Expected string
	}{
		{
			Duration: 1 * time.Second,
			Expected: "1s",
		},
		{
			Duration: 150 * time.Millisecond,
			Expected: "0s",
		},
		{
			Duration: 2500 * time.Millisecond,
			Expected: "2s",
		},
	}
	for _, x := range cases {
		assert.Equal(t, x.Expected, HumanDuration(x.Duration))
	}
}
