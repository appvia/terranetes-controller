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
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
)

// StateProperty is a output property
type StateProperty struct {
	// Value is the value of the property
	Value interface{} `json:"value,omitempty"`
	// Type is the type of property
	Type string `json:"type,omitempty"`
}

// Resource represents a resource in the state
type Resource struct {
	// Mode is the mode of the resource
	Mode string `json:"mode,omitempty"`
	// Type is the type of the resource
	Type string `json:"type,omitempty"`
	// Instances a collection of the resource instances in the state
	Instances []map[string]interface{} `json:"instances,omitempty"`
}

// ToValue returns the value of the property
func (s *StateProperty) ToValue() (string, error) {
	if s.Value == nil {
		return "", errors.New("value is nil")
	}

	var value string

	switch v := s.Value.(type) {
	case string:
		value = v
	case int:
		value = strconv.Itoa(v)
	case float64:
		value = fmt.Sprint(v)
	case bool:
		value = strconv.FormatBool(v)
	default:
		valueJSON, err := json.Marshal(v)
		if err != nil {
			return "", fmt.Errorf("cloud not convert %v to string", v)
		}
		value = string(valueJSON)
	}

	return value, nil
}

// State is the state of the terraform
type State struct {
	// Outputs are the terraform outputs
	Outputs map[string]StateProperty `json:"outputs"`
	// Resources is a collection of resources in the state
	Resources []Resource `json:"resources,omitempty"`
	// TerraformVersion is the version of terraform used
	TerraformVersion string `json:"terraform_version,omitempty"`
}

// CountResources returns the number of managed resources from the state
func (s *State) CountResources() int {
	var count int

	for i := 0; i < len(s.Resources); i++ {
		if s.Resources[i].Mode == "managed" {
			count += len(s.Resources[i].Instances)
		}
	}

	return count
}

// HasOutputs returns true if the state has outputs
func (s *State) HasOutputs() bool {
	return len(s.Outputs) > 0
}
