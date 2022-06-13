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

func TestToHCL(t *testing.T) {
	cases := []struct {
		Data     interface{}
		Expected string
	}{
		{
			Data:     map[string]interface{}{},
			Expected: "\n",
		},
		{
			Data:     map[string]interface{}{"features": map[string]interface{}{}},
			Expected: "features {}\n",
		},
		{
			Data: map[string]interface{}{"features": map[string]interface{}{
				"hello": "world",
				"test":  []string{"a", "b", "c"},
			}},
			Expected: "features {\n  hello = \"world\"\n\n  test = [\n    \"a\",\n    \"b\",\n    \"c\",\n  ]\n}\n",
		},
	}
	for _, c := range cases {
		m, err := ToHCL(c.Data)
		assert.NoError(t, err)
		assert.Equal(t, string(c.Expected), string(m))
	}
}

func TestGetTxtFunc(t *testing.T) {
	m := GetTxtFunc()
	assert.NotNil(t, m)
}

func TestTemplate(t *testing.T) {
	tmpl := `{{ .Hello }} {{ .World }}`
	m, err := Template(tmpl, map[string]interface{}{
		"Hello": "Hello", "World": "World",
	})
	assert.NoError(t, err)
	assert.NotEmpty(t, m)
	assert.Equal(t, "Hello World", string(m))
}

func TestTemplateInvalid(t *testing.T) {
	tmpl := `{{ .Hello }} {{ .World }`

	m, err := Template(tmpl, map[string]interface{}{"Hello": "Hello"})
	assert.Error(t, err)
	assert.Empty(t, m)
}
