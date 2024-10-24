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
	"strings"

	"github.com/spf13/cobra"
	"golang.org/x/exp/constraints"
	"golang.org/x/exp/slices"
)

// enumSlice represents a slice of enumeration values that can be retrieved,
// set, and stringified.
type enumSlice[E constraints.Integer] struct {
	v     *[]E
	merge bool // replace the complete slice or merge values?
}

// Get returns the slice enum values.
func (s *enumSlice[E]) Get() any { return *s.v }

// Set or merge one or more values of the new scalar enum value corresponding to
// the passed textual representation, using the additionally specified
// text-to-value mapping. If the specified textual representation doesn't match
// any of the defined ones, an error is returned instead and the value isn't
// changed. The first call to Set will always clear any previous default value.
// All subsequent calls to Set will merge the specified enum values with the
// current enum values.
func (s *enumSlice[E]) Set(val string, names enumMapper[E]) error {
	// First parse and convert the textual enum values into their
	// program-internal codes.
	ids := strings.Split(val, ",")
	enumvals := make([]E, 0, len(ids)) // ...educated guess
	for _, id := range ids {
		enumval, err := names.ValueOf(id)
		if err != nil {
			return err
		}
		enumvals = append(enumvals, enumval)
	}
	if !s.merge {
		// Replace any existing default enum value set on first Set().
		*s.v = enumvals
		s.merge = true // ...and next time: merge.
		return nil
	}
	// Later, merge with the existing enum values.
	for _, enumval := range enumvals {
		if slices.Index(*s.v, enumval) >= 0 {
			continue
		}
		*s.v = append(*s.v, enumval)
	}
	return nil
}

// String returns the textual representation of the slice enum value, using the
// specified text-to-value mapping.
func (s *enumSlice[E]) String(names enumMapper[E]) string {
	n := make([]string, 0, len(*s.v))
	for _, enumval := range *s.v {
		if enumnames := names.Lookup(enumval); len(enumnames) > 0 {
			n = append(n, enumnames[0])
			continue
		}
		n = append(n, unknown)
	}
	return "[" + strings.Join(n, ",") + "]"
}

// NewCompletor returns a cobra Completor that completes enum flag values.
func (s *enumSlice[E]) NewCompletor(enums EnumIdentifiers[E], help Help[E]) Completor {
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
	return func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		prefix := ""
		completes := []string{}
		if lastComma := strings.LastIndex(toComplete, ","); lastComma >= 0 {
			prefix = toComplete[:lastComma+1] // ...Prof J. won't ever like this variable name
			completes = strings.Split(prefix, ",")
			completes = completes[:len(completes)-1] // remove last empty element
		}
		filteredCompletions := make([]string, 0, len(completions))
		for _, completion := range completions {
			if slices.Contains(completes, strings.Split(completion, "\t")[0]) {
				continue
			}
			filteredCompletions = append(filteredCompletions, prefix+completion)
		}
		return filteredCompletions, cobra.ShellCompDirectiveNoFileComp | cobra.ShellCompDirectiveNoSpace
	}
}
