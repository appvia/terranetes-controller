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

// ErrorDetection defines an error and potential causes for it.
type ErrorDetection struct {
	// Regex is the string we are looking for
	Regex string
	// Message is cause of the error
	Message string
}

var (
	// Detectors is the error detection pattern
	Detectors = map[string][]ErrorDetection{
		"aws": {
			{
				Regex:   "operation error STS: GetCallerIdentity",
				Message: "AWS Credentials in provider has been missconfigured, contact platform administrator",
			},
		},
		"google":  {},
		"azurerm": {},
		"*": {
			{
				Regex:   "error validating provider credentials",
				Message: "Provider credentials are missconfigured, please contact the platform administrator",
			},
		},
	}
)
