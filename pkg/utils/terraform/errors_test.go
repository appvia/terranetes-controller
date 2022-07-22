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
	"regexp"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestErrorDetection(t *testing.T) {
	for cloud, list := range Detectors {
		for i, detector := range list {
			assert.NoError(t, regexp.Compile(detector.Regexp), "failed to compile %s[%d]: %s", cloud, i, detector.Regexp)
			assert.NotEmpty(t, detector.Message)
		}
	}
}
