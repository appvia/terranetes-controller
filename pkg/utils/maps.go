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

// ListKeys return a list of keys from a map
func ListKeys(m map[string]any) []string {
	keys := make([]string, len(m))
	i := 0
	for k := range m {
		keys[i] = k
		i++
	}

	return keys
}

// MergeStringMaps merges maps together
func MergeStringMaps(list ...map[string]string) map[string]string {
	m := make(map[string]string)

	if len(list) == 0 {
		return m
	}

	for i := 0; i < len(list); i++ {
		for k, v := range list[i] {
			switch {
			case k == "" && v == "":
				continue
			}
			m[k] = v
		}
	}

	return m
}
