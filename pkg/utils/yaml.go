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

package utils

import (
	"bytes"
	"encoding/json"
	"io"
	"regexp"
	"strings"

	"sigs.k8s.io/yaml"
)

// DecodeYAML decodes the yaml resource into a runtime.Object
func DecodeYAML(data []byte, in interface{}) error {
	data, err := yaml.YAMLToJSON(data)
	if err != nil {
		return err
	}

	return DecodeJSON(data, in)
}

// DecodeJSON decodes the json into a runtime.Object
func DecodeJSON(data []byte, in interface{}) error {
	return json.NewDecoder(bytes.NewReader(data)).Decode(in)
}

// YAMLDocuments returns a collection of documents from the reader
func YAMLDocuments(reader io.Reader) ([]string, error) {
	content, err := io.ReadAll(reader)
	if err != nil {
		return nil, err
	}
	splitter := regexp.MustCompile("(?m)^---\n")

	var list []string

	for _, document := range splitter.Split(string(content), -1) {
		if strings.TrimSpace(document) == "" {
			continue
		}
		list = append(list, document)
	}

	return list, nil
}
