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

package preload

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewData(t *testing.T) {
	assert.NotNil(t, NewData())
	assert.Len(t, NewData(), 0)
}

func TestAdd(t *testing.T) {
	d := NewData()
	d.Add("key", Entry{
		Description: "test",
		Value:       "value",
	})
	assert.Len(t, d, 1)

	v := d.Get("key")
	assert.Equal(t, "test", v.Description)
	assert.Equal(t, "value", v.Value)
}

func TestGet(t *testing.T) {
	d := NewData()
	d.Add("key", Entry{
		Description: "test",
		Value:       "value",
	})
	assert.Len(t, d, 1)

	v := d.Get("key")
	assert.Equal(t, "test", v.Description)
	assert.Equal(t, "value", v.Value)
}
