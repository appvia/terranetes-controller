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

package utils

import "github.com/Masterminds/semver"

// GetVersionIncrement returns either an error or the version increment
func GetVersionIncrement(version string) (string, error) {
	sem, err := semver.NewVersion(version)
	if err != nil {
		return "", err
	}
	updated := sem.IncPatch()

	return updated.String(), nil
}

// VersionLessThan returns either an error a bool indicating if the version is greater than the other
func VersionLessThan(version string, other string) (bool, error) {
	sem, err := semver.NewVersion(version)
	if err != nil {
		return false, err
	}
	otherSem, err := semver.NewVersion(other)
	if err != nil {
		return false, err
	}

	return sem.LessThan(otherSem), nil
}
