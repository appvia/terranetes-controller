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
	"github.com/rodaine/hclencoder"

	"github.com/appvia/terranetes-controller/pkg/utils/template"
)

// ToHCL converts the json to HCL format
func ToHCL(data interface{}) (string, error) {
	hcl, err := hclencoder.Encode(data)
	if err != nil {
		return "", err
	}

	return string(hcl), nil
}

// Template renders the content but includes the hcl method
func Template(main string, data interface{}) ([]byte, error) {
	custom := map[string]any{
		"toHCL": ToHCL,
	}

	return template.NewWithFuncs(main, custom, data)
}
