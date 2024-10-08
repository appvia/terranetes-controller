/*
 * Copyright (C) 2022 Appvia Ltd <info@appvia.io>
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

package version

import "fmt"

var (
	// Name is the name of the application
	Name = "terranetes-controller"
	// Version is the current version of the application
	Version = "0.0.1"
	// GitCommit is the current commit of the application
	GitCommit = "HEAD"
)

// GetVersion returns the current version of the application
func GetVersion() string {
	return fmt.Sprintf("%s (gitsha: %s)", Version, GitCommit)
}
