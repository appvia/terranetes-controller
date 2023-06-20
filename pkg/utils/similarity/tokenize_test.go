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

package similarity

import (
	"reflect"
	"testing"
)

func TestTokenize(t *testing.T) {
	cases := []struct {
		Sentence string
		Expected []string
	}{
		{
			Sentence: "This is a test",
			Expected: []string{"test"},
		},
		{
			Sentence: "This is a tests",
			Expected: []string{"test"},
		},
		{
			Sentence: "The clusters the networks is running in",
			Expected: []string{"cluster", "network", "running"},
		},
	}

	for i, c := range cases {
		tokens := Tokenize(c.Sentence)
		if !reflect.DeepEqual(tokens, c.Expected) {
			t.Errorf("case %d: expected %v, got %v", i, c.Expected, tokens)
		}
	}
}
