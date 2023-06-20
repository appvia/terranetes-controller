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

package template

import (
	"bytes"
	"text/template"

	"github.com/Masterminds/sprig/v3"
	"sigs.k8s.io/yaml"
)

// ToYaml converts a map to yaml
func ToYaml(data interface{}) (string, error) {
	encoded, err := yaml.Marshal(data)
	if err != nil {
		return "", err
	}

	return string(encoded), nil
}

// GetTxtFunc returns a defaults list of methods for text templating
func GetTxtFunc() map[string]any {
	return sprig.TxtFuncMap()
}

// NewWithFuncs renders a template with custom methods
func NewWithFuncs(tpl string, methods template.FuncMap, params interface{}) ([]byte, error) {
	funcs := GetTxtFunc()

	for key, method := range methods {
		funcs[key] = method
	}
	funcs["toYaml"] = ToYaml

	tm, err := template.New("main").Funcs(funcs).Parse(tpl)
	if err != nil {
		return nil, err
	}

	b := &bytes.Buffer{}
	if err := tm.Execute(b, params); err != nil {
		return nil, err
	}

	return b.Bytes(), nil
}

// New is called to render a template
func New(tpl string, data interface{}) ([]byte, error) {
	return NewWithFuncs(tpl, nil, data)
}

// NewWithBytes is called to render a template
func NewWithBytes(tpl []byte, data interface{}) ([]byte, error) {
	return NewWithFuncs(string(tpl), nil, data)
}
