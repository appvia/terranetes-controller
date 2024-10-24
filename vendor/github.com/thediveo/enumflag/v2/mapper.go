// Copyright 2022 Harald Albrecht.
//
// Licensed under the Apache License, Version 2.0 (the "License"); you may not
// use this file except in compliance with the License. You may obtain a copy
// of the License at
//
//    http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS, WITHOUT
// WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied. See the
// License for the specific language governing permissions and limitations
// under the License.

package enumflag

import (
	"fmt"
	"sort"
	"strings"

	"golang.org/x/exp/constraints"
	"golang.org/x/exp/slices"
)

// EnumIdentifiers maps enumeration values to their corresponding textual
// representations (~identifiers). This mapping is a one-to-many mapping in that
// the same enumeration value may have more than only one associated textual
// representation (indentifier). If more than one textual representation exists
// for the same enumeration value, then the first textual representation is
// considered to be the canonical one.
type EnumIdentifiers[E constraints.Integer] map[E][]string

// enumMapper is an optionally case insensitive map from enum values to their
// corresponding textual representations.
type enumMapper[E constraints.Integer] struct {
	m           EnumIdentifiers[E]
	sensitivity EnumCaseSensitivity
}

// newEnumMapper returns a new enumMapper for the given mapping and case
// sensitivity or insensitivity.
func newEnumMapper[E constraints.Integer](mapping EnumIdentifiers[E], sensitivity EnumCaseSensitivity) enumMapper[E] {
	return enumMapper[E]{
		m:           mapping,
		sensitivity: sensitivity,
	}
}

// Lookup returns the enum textual representations (identifiers) for the
// specified enum value, if any; otherwise, returns a zero string slice.
func (m enumMapper[E]) Lookup(enum E) (names []string) {
	return m.m[enum]
}

// ValueOf returns the enumeration value corresponding with the specified
// textual representation (identifier), or an error if no match is found.
func (m enumMapper[E]) ValueOf(name string) (E, error) {
	comparefn := func(s string) bool { return s == name }
	if m.sensitivity == EnumCaseInsensitive {
		name = strings.ToLower(name)
		comparefn = func(s string) bool { return strings.ToLower(s) == name }
	}
	// Try to find a matching enum value textual representation, and then take
	// its enumation value ("code").
	for enumval, ids := range m.m {
		if slices.IndexFunc(ids, comparefn) >= 0 {
			return enumval, nil
		}
	}
	// Oh no! An invalid textual enum value was specified, so let's generate
	// some useful error explaining which textual representations are valid.
	// We're ordering values by their canonical names in order to achieve a
	// stable error message.
	allids := []string{}
	for _, ids := range m.m {
		s := []string{}
		for _, id := range ids {
			s = append(s, "'"+id+"'")
		}
		allids = append(allids, strings.Join(s, "/"))
	}
	sort.Strings(allids)
	return 0, fmt.Errorf("must be %s", strings.Join(allids, ", "))
}

// Mapping returns the mapping of enum values to their names.
func (m enumMapper[E]) Mapping() EnumIdentifiers[E] {
	return m.m
}
