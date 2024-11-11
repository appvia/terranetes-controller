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
	"github.com/spf13/cobra"
	"golang.org/x/exp/constraints"
)

// unknown is the textual representation of an unknown enum value, that is, when
// the enum value to name mapping doesn't have any idea about a particular enum
// value.
const unknown = "<unknown>"

// enumScalar represents a mutable, single enumeration value that can be
// retrieved, set, and stringified.
type enumScalar[E constraints.Integer] struct {
	v         *E
	nodefault bool // opts in to accepting a zero enum value as the "none"
}

// Get returns the scalar enum value.
func (s *enumScalar[E]) Get() any { return *s.v }

// Set the value to the new scalar enum value corresponding to the passed
// textual representation, using the additionally specified text-to-value
// mapping. If the specified textual representation doesn't match any of the
// defined ones, an error is returned instead and the value isn't changed.
func (s *enumScalar[E]) Set(val string, names enumMapper[E]) error {
	enumcode, err := names.ValueOf(val)
	if err != nil {
		return err
	}
	*s.v = enumcode
	return nil
}

// String returns the textual representation of the scalar enum value, using the
// specified text-to-value mapping.
//
// String will return "<unknown>" for undefined/unmapped enum values. If the
// enum flag has been created using [NewWithoutDefault], then an empty string is
// returned instead: in this case [spf13/cobra] will not show any default for
// the corresponding CLI flag.
//
// [spf13/cobra]: https://github.com/spf13/cobra
func (s *enumScalar[E]) String(names enumMapper[E]) string {
	if ids := names.Lookup(*s.v); len(ids) > 0 {
		return ids[0]
	}
	if *s.v == 0 && s.nodefault {
		return ""
	}
	return unknown
}

// NewCompletor returns a cobra Completor that completes enum flag values.
// Please note that shell completion hasn't the notion of case sensitivity or
// insensitivity, so we cannot take this into account but instead return all
// available enum value names in their original form.
func (s *enumScalar[E]) NewCompletor(enums EnumIdentifiers[E], help Help[E]) Completor {
	completions := []string{}
	for enumval, enumnames := range enums {
		helptext := ""
		if text, ok := help[enumval]; ok {
			helptext = "\t" + text
		}
		// complete not only the canonical enum value name, but also all other
		// (alias) names.
		for _, name := range enumnames {
			completions = append(completions, name+helptext)
		}
	}
	return func(*cobra.Command, []string, string) ([]string, cobra.ShellCompDirective) {
		return completions, cobra.ShellCompDirectiveNoFileComp
	}
}
