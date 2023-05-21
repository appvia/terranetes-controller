/*
 * Copyright (C) 2023  Appvia Ltd <info@appvia.io>
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

package preload

import (
	"encoding/json"
	"io"
)

// NewData returns a new Data instance
func NewData() Data {
	data := make(map[string]*Entry)

	return data
}

// Add adds a new entry to the data
func (d *Data) Add(name string, entry Entry) {
	(*d)[name] = &entry
}

// Get returns an entry from the data
func (d *Data) Get(name string) *Entry {
	return (*d)[name]
}

// Keys returns a list of keys in the data
func (d *Data) Keys() []string {
	var list []string

	for k := range *d {
		list = append(list, k)
	}

	return list
}

// MarshalTo marshals the data to JSON
func (d *Data) MarshalTo(w io.Writer) error {
	return json.NewEncoder(w).Encode(d)
}

// Marshal marshals the data to JSON
func (e *Entry) Marshal() ([]byte, error) {
	return json.Marshal(e)
}
