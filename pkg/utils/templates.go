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
	"fmt"
	"strings"

	"github.com/Masterminds/sprig/v3"
)

// GetTxtFunc returns a defaults list of methods for text templating
func GetTxtFunc() map[string]any {
	funcs := sprig.TxtFuncMap()
	funcs["prefix"] = Prefix

	return funcs
}

// Prefix is a hack to get prefixes to fix
func Prefix(prefix string, e interface{}) string {
	w := &strings.Builder{}

	list, ok := e.([]string)
	if !ok {
		return ""
	}

	for _, line := range list {
		for _, x := range strings.Split(line, ", ") {
			w.WriteString(fmt.Sprintf("%s %s ", prefix, x))
		}
	}

	return w.String()
}
