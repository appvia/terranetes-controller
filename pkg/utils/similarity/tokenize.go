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
	"strings"

	"github.com/bbalet/stopwords"
	"github.com/gertd/go-pluralize"
)

var (
	plural = pluralize.NewClient()
)

// Tokenize extracts inforamtion from the input
func Tokenize(sentence string) []string {
	list := wordRe.FindAllString(
		stopwords.CleanString(sentence, "en", true), -1,
	)

	for i := 0; i < len(list); i++ {
		list[i] = plural.Singular(strings.ToLower(list[i]))
	}

	return list
}
