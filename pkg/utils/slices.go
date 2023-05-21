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

import "sort"

// Contains checks a list has a value in it
func Contains(v string, l []string) bool {
	for _, x := range l {
		if v == x {
			return true
		}
	}

	return false
}

// ContainsList checks a list has a value in it
func ContainsList(v []string, l []string) bool {
	for _, x := range v {
		if Contains(x, l) {
			return true
		}
	}

	return false
}

// Sorted returns a sorted list of values
func Sorted(slice []string) []string {
	sorted := make([]string, len(slice))
	copy(sorted, slice)
	sort.Strings(sorted)

	return sorted
}

// Unique returns a list of unique values
func Unique(slice []string) []string {
	var list []string

	for _, item := range slice {
		if !Contains(item, list) {
			list = append(list, item)
		}
	}

	return list
}
