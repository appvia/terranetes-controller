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

package create

// Input defines an input to the cloud resource, we use this internally
// to the command to pass options around
type Input struct {
	// Context is an optional name of the context the input comes from
	Context string `json:"context"`
	// Default is the default value of the input
	Default interface{} `json:"default"`
	// Description is the description of the input
	Description string `json:"description"`
	// Key is the key of the input
	Key string `json:"key"`
	// Required is a flag to indicate if the input is required
	Required bool `json:"required"`
	// Type is the type of the input
	Type string `json:"type"`
}
