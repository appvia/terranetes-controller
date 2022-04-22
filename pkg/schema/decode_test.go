/*
 * Copyright (C) 2022  Rohith Jayawardene <gambol99@gmail.com>
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

package schema

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDecodeJSON(t *testing.T) {
	namespace := `
{
	"apiVersion": "v1",
	"kind": "Namespace",
	"metadata": {
		"name": "test"
	}
}
`
	obj, err := DecodeJSON([]byte(namespace))
	assert.NoError(t, err)
	assert.NotNil(t, obj)
}

func TestDecodeJSONFromSchema(t *testing.T) {
	crd := `
{
	"apiVersion": "terraform.appvia.io/v1alpha1",
	"kind": "Configuration",
	"metadata": {
		"name": "test"
	},
	"spec": {}
}
`
	obj, err := DecodeJSON([]byte(crd))
	assert.NoError(t, err)
	assert.NotNil(t, obj)
}

func TestDecodeYAML(t *testing.T) {
	namespace := `
apiVersion: v1
kind: Namespace
metadata:
  name: test
`
	obj, err := DecodeYAML([]byte(namespace))
	assert.NoError(t, err)
	assert.NotNil(t, obj)
}

func TestDecodeYAMLFromSchema(t *testing.T) {
	crd := `
apiVersion: terraform.appvia.io/v1alpha1
kind: Configuration
metadata:
  name: test
spec: {}
`
	obj, err := DecodeYAML([]byte(crd))
	assert.NoError(t, err)
	assert.NotNil(t, obj)
}
