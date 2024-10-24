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

// Help maps enumeration values to their corresponding help descriptions. These
// descriptions should contain just the description but without any "foo\t" enum
// value prefix. The reason is that enumflag will automatically register the
// correct (erm, “complete”) completion text. Please note that it isn't
// necessary to supply any help texts in order to register enum flag completion.
type Help[E constraints.Integer] map[E]string

// Completor tells cobra how to complete a flag. See also cobra's [dynamic flag
// completion] documentation.
//
// [dynamic flag completion]: https://github.com/spf13/cobra/blob/main/shell_completions.md#specify-dynamic-flag-completion
type Completor func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective)
