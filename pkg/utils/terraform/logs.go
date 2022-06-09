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

package terraform

import (
	"bufio"
	"io"
	"regexp"
)

var (
	changeNotice = regexp.MustCompile("Your infrastructure matches the configuration.")
)

// FindChangesInLogs is used to scan the logs for the terraform line which informs on changes
func FindChangesInLogs(in io.Reader) (bool, error) {
	scan := bufio.NewScanner(in)

	for scan.Scan() {
		if changeNotice.MatchString(scan.Text()) {
			return false, nil
		}
	}

	return true, nil
}
