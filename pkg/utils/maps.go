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

// MergeStringMaps merges maps together
func MergeStringMaps(a, b map[string]string) map[string]string {
	m := make(map[string]string)

	switch {
	case a == nil && b == nil:
		return m
	case a == nil && b != nil:
		return b
	case b == nil && a != nil:
		return a
	}

	for _, x := range []map[string]string{a, b} {
		for k, v := range x {
			if k == "" && v == "" {
				continue
			}
			m[k] = v
		}
	}

	return m
}
