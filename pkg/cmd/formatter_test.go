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

package cmd

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewTextFormatter(t *testing.T) {
	assert.NotNil(t, NewTextFormatter())
}

func TestNewJSONFormatter(t *testing.T) {
	assert.NotNil(t, NewJSONFormatter())
}

func TestDefaultTextFormatterPrintf(t *testing.T) {
	buffer := &bytes.Buffer{}
	formatter := NewTextFormatter()

	formatter.Printf(buffer, "Hello, %s", "world")
	assert.Equal(t, "Hello, world", buffer.String())
}

func TestDefaultTextFormatterPrintln(t *testing.T) {
	buffer := &bytes.Buffer{}
	formatter := NewTextFormatter()

	formatter.Println(buffer, "Hello, world")
	assert.Equal(t, "Hello, world\n", buffer.String())
}

func TestDefaultJSONFormatterPrintf(t *testing.T) {
	buffer := &bytes.Buffer{}
	formatter := NewJSONFormatter()

	formatter.Printf(buffer, "Hello, %s", "world")
	assert.Equal(t, `{ "message": "Hello, world" }`, buffer.String())
}

func TestDefaultJSONFormatterPrintln(t *testing.T) {
	buffer := &bytes.Buffer{}
	formatter := NewJSONFormatter()

	formatter.Println(buffer, "Hello, world")
	assert.Equal(t, `{ "message": "Hello, world" }`, buffer.String())
}
