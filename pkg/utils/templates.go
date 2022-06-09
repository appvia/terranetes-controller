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
	"bytes"
	"text/template"

	"github.com/Masterminds/sprig/v3"
)

// GetTxtFunc returns a defaults list of methods for text templating
func GetTxtFunc() map[string]any {
	return sprig.TxtFuncMap()
}

// Template is called to render a template
func Template(main string, data interface{}) ([]byte, error) {
	tpl, err := template.New("main").Funcs(GetTxtFunc()).Parse(main)
	if err != nil {
		return nil, err
	}

	b := &bytes.Buffer{}
	if err := tpl.Execute(b, data); err != nil {
		return nil, err
	}

	return b.Bytes(), nil
}
