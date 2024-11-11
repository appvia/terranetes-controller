// Copyright 2020, 2022 Harald Albrecht.
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

	"github.com/spf13/cobra"
	"golang.org/x/exp/constraints"
)

// Flag represents a CLI (enumeration) flag which can take on only a single
// enumeration value out of a fixed set of enumeration values. Applications
// using the enumflag package might want to “derive” their enumeration flags
// from Flag for documentation purposes; for instance:
//
//	type MyFoo enumflag.Flag
//
// However, applications don't need to base their own enum types on Flag. The
// only requirement for user-defined enumeration flags is that they must be
// (“somewhat”) compatible with the Flag type, or more precise: user-defined
// enumerations must satisfy [constraints.Integer].
type Flag uint

// EnumCaseSensitivity specifies whether the textual representations of enum
// values are considered to be case sensitive, or not.
type EnumCaseSensitivity bool

// Controls whether the textual representations for enum values are case
// sensitive, or not.
const (
	EnumCaseInsensitive EnumCaseSensitivity = false
	EnumCaseSensitive   EnumCaseSensitivity = true
)

// EnumFlagValue wraps a user-defined enum type value satisfying
// [constraints.Integer] or [][constraints.Integer]. It implements the
// [github.com/spf13/pflag.Value] interface, so the user-defined enum type value
// can directly be used with the fine pflag drop-in package for Golang CLI
// flags.
type EnumFlagValue[E constraints.Integer] struct {
	value    enumValue[E]  // enum value of a user-defined enum scalar or slice type.
	enumtype string        // user-friendly name of the user-defined enum type.
	names    enumMapper[E] // enum value names.
}

// enumValue supports getting, setting, and stringifying an scalar or slice enum
// enumValue.
//
// Do I smell preemptive interfacing here...? Now watch the magic of “cleanest
// code”: by just moving the interface type from the source file with the struct
// types to the source file with the consumer we achieve immediate Go
// perfectness! Strike!
type enumValue[E constraints.Integer] interface {
	Get() any
	Set(val string, names enumMapper[E]) error
	String(names enumMapper[E]) string
	NewCompletor(enums EnumIdentifiers[E], help Help[E]) Completor
}

// New wraps a given enum variable (satisfying [constraints.Integer]) so that it
// can be used as a flag Value with [github.com/spf13/pflag.Var] and
// [github.com/spf13/pflag.VarP]. In case no default enum value should be set
// and therefore no default shown in [spf13/cobra], use [NewWithoutDefault]
// instead.
//
// [spf13/cobra]: https://github.com/spf13/cobra
func New[E constraints.Integer](flag *E, typename string, mapping EnumIdentifiers[E], sensitivity EnumCaseSensitivity) *EnumFlagValue[E] {
	return new("New", flag, typename, mapping, sensitivity, false)
}

// NewWithoutDefault wraps a given enum variable (satisfying
// [constraints.Integer]) so that it can be used as a flag Value with
// [github.com/spf13/pflag.Var] and [github.com/spf13/pflag.VarP]. Please note
// that the zero enum value must not be mapped and thus not be assigned to any
// enum value textual representation.
//
// [spf13/cobra] won't show any default value in its help for CLI enum flags
// created with NewWithoutDefault.
//
// [spf13/cobra]: https://github.com/spf13/cobra
func NewWithoutDefault[E constraints.Integer](flag *E, typename string, mapping EnumIdentifiers[E], sensitivity EnumCaseSensitivity) *EnumFlagValue[E] {
	return new("NewWithoutDefault", flag, typename, mapping, sensitivity, true)
}

// new returns a new enum variable to be used with pflag.Var and pflag.VarP.
func new[E constraints.Integer](ctor string, flag *E, typename string, mapping EnumIdentifiers[E], sensitivity EnumCaseSensitivity, nodefault bool) *EnumFlagValue[E] {
	if flag == nil {
		panic(fmt.Sprintf("%s requires flag to be a non-nil pointer to an enum value satisfying constraints.Integer", ctor))
	}
	if mapping == nil {
		panic(fmt.Sprintf("%s requires mapping not to be nil", ctor))
	}
	return &EnumFlagValue[E]{
		value:    &enumScalar[E]{v: flag, nodefault: nodefault},
		enumtype: typename,
		names:    newEnumMapper(mapping, sensitivity),
	}
}

// NewSlice wraps a given enum slice variable (satisfying [constraints.Integer])
// so that it can be used as a flag Value with [github.com/spf13/pflag.Var] and
// [github.com/spf13/pflag.VarP].
func NewSlice[E constraints.Integer](flag *[]E, typename string, mapping EnumIdentifiers[E], sensitivity EnumCaseSensitivity) *EnumFlagValue[E] {
	if flag == nil {
		panic("NewSlice requires flag to be a non-nil pointer to an enum value slice satisfying []constraints.Integer")
	}
	if mapping == nil {
		panic("NewSlice requires mapping not to be nil")
	}
	return &EnumFlagValue[E]{
		value:    &enumSlice[E]{v: flag},
		enumtype: typename,
		names:    newEnumMapper(mapping, sensitivity),
	}
}

// Set sets the enum flag to the specified enum value. If the specified value
// isn't a valid enum value, then the enum flag won't be set and an error is
// returned instead.
func (e *EnumFlagValue[E]) Set(val string) error {
	return e.value.Set(val, e.names)
}

// String returns the textual representation of an enumeration (flag) value. In
// case multiple textual representations (~identifiers) exist for the same
// enumeration value, then only the first textual representation is returned,
// which is considered to be the canonical one.
func (e *EnumFlagValue[E]) String() string { return e.value.String(e.names) }

// Type returns the name of the flag value type. The type name is used in error
// messages.
func (e *EnumFlagValue[E]) Type() string { return e.enumtype }

// Get returns the current enum value for convenience. Please note that the enum
// value is either scalar or slice, depending on how the enum flag was created.
func (e *EnumFlagValue[E]) Get() any { return e.value.Get() }

// RegisterCompletion registers completions for the specified (flag) name, with
// optional help texts.
func (e *EnumFlagValue[E]) RegisterCompletion(cmd *cobra.Command, name string, help Help[E]) error {
	return cmd.RegisterFlagCompletionFunc(
		name, e.value.NewCompletor(e.names.Mapping(), help))
}
