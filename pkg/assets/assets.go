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

package assets

import (
	"embed"
	"fmt"
)

//go:embed *.yaml.tpl
var policyFS embed.FS

// AssetNames will get all the policy assets
func AssetNames() []string {
	var assets []string
	// read current dir
	d, err := policyFS.ReadDir(".")
	if err != nil {
		panic(fmt.Errorf("something wrong - this should have been tested first"))
	}
	for _, f := range d {
		assets = append(assets, f.Name())
	}
	return assets
}

// Asset will get a policy file asset or an error if it doesn't exist
func Asset(name string) ([]byte, error) {
	return policyFS.ReadFile(name)
}

// MustAsset will return a single file name asset or panic
func MustAsset(name string) []byte {
	// normally for cross-OS we would use filepath lib, but due to
	// embed.FS path handling: "The path separator is a forward slash,
	// even on Windows systems" (more in https://pkg.go.dev/embed)
	// We always must use path with embed! (even on Windows)
	content, err := Asset(name)
	if err != nil {
		panic(fmt.Errorf("embedded asset does not exist %s - %w", name, err))
	}
	return content
}
