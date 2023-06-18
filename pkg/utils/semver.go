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

import (
	"sort"

	"github.com/Masterminds/semver"
)

// LatestSemverVersion returns the latest semver version from a list of versions.
func LatestSemverVersion(versions []string) (string, error) {
	list, err := SortSemverVersions(versions)
	if err != nil {
		return "", err
	}

	return list[len(list)-1], nil
}

// SortSemverVersions sorts a list of semver versions in ascending order.
func SortSemverVersions(versions []string) ([]string, error) {
	vs := make([]*semver.Version, len(versions))

	for i, r := range versions {
		v, err := semver.NewVersion(r)
		if err != nil {
			return nil, err
		}
		vs[i] = v
	}
	sort.Sort(semver.Collection(vs))

	var list []string
	for i := 0; i < len(vs); i++ {
		list = append(list, vs[i].Original())
	}

	return list, nil
}
