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
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestWriteYAML(t *testing.T) {
	values := map[string]interface{}{
		"foo":  "bar",
		"list": []interface{}{"item1", "item2"},
	}
	tempFile, err := os.CreateTemp(os.TempDir(), "test-write-yaml-*.yaml")
	assert.NoError(t, err)
	assert.NotEmpty(t, tempFile)
	defer os.Remove(tempFile.Name())

	err = WriteYAML(tempFile.Name(), values)
	assert.NoError(t, err)
}

func TestYAMLDocumentsOK(t *testing.T) {
	// Given
	var yaml = `
---
foo: bar
---
foo: bar
`
	var docs, err = YAMLDocuments(strings.NewReader(yaml))
	assert.NoError(t, err)
	assert.Equal(t, 2, len(docs))
}

func TestLoadYAMLFromReader(t *testing.T) {
	values := map[string]interface{}{}
	content := `---
foo: bar
list: [item1, item2]
`
	err := LoadYAMLFromReader(strings.NewReader(content), &values)
	assert.NoError(t, err)
	assert.Equal(t, "bar", values["foo"])
	assert.Equal(t, []interface{}{"item1", "item2"}, values["list"])
}

func TestLoadYAMLNoFile(t *testing.T) {
	values := map[string]interface{}{}
	err := LoadYAML("no-file.yaml", &values)
	assert.Error(t, err)
	assert.Equal(t, "file does not exist", err.Error())
}

func TestLoadYAMLOK(t *testing.T) {
	content := `---
foo: bar
list: [item1, item2]
`
	tempFile, err := os.CreateTemp(os.TempDir(), "test-load-yaml-*.yaml")
	assert.NoError(t, err)
	assert.NotEmpty(t, tempFile)

	n, err := tempFile.WriteString(content)
	assert.NoError(t, err)
	assert.Equal(t, len(content), n)
	defer os.Remove(tempFile.Name())

	values := map[string]interface{}{}
	err = LoadYAML(tempFile.Name(), &values)
	assert.NoError(t, err)
	assert.Equal(t, "bar", values["foo"])
	assert.Equal(t, []interface{}{"item1", "item2"}, values["list"])
}
