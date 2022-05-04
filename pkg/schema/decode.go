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

package schema

import (
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/yaml"
)

// DecodeYAML decodes the yaml resource into a runtime.Object
func DecodeYAML(data []byte) (client.Object, error) {
	jsonData, err := yaml.YAMLToJSON(data)
	if err != nil {
		return nil, err
	}

	return DecodeJSON(jsonData)
}

// DecodeJSON decodes the json into a runtime.Object
func DecodeJSON(data []byte) (client.Object, error) {
	obj, _, err := GetCodecFactory().UniversalDeserializer().Decode(data, nil, nil)
	if err != nil && !runtime.IsNotRegisteredError(err) {
		return nil, err
	}
	if obj != nil {
		return obj.(client.Object), nil
	}

	u := &unstructured.Unstructured{}
	if err := u.UnmarshalJSON(data); err != nil {
		return nil, err
	}

	return u, nil
}
