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

import "time"

type module struct {
	ID          string    `json:"id"`
	Owner       string    `json:"owner"`
	Namespace   string    `json:"namespace"`
	Name        string    `json:"name"`
	Version     string    `json:"version"`
	Provider    string    `json:"provider"`
	Description string    `json:"description"`
	Source      string    `json:"source"`
	Tag         string    `json:"tag"`
	PublishedAt time.Time `json:"published_at"`
	Downloads   int       `json:"downloads"`
	Verified    bool      `json:"verified"`
}

type searchResult struct {
	Meta struct {
		Limit         int    `json:"limit"`
		CurrentOffset int    `json:"current_offset"`
		NextOffset    int    `json:"next_offset"`
		NextURL       string `json:"next_url"`
	} `json:"meta"`
	Modules []module `json:"modules"`
}

type versionsResult struct {
	Modules []moduleSource `json:"modules"`
}

type moduleSource struct {
	// Source is the source of the module
	Source string `json:"source"`
	// Versions is the list of versions of the module
	Versions []moduleVersions `json:"versions"`
}

type moduleVersions struct {
	// Version is the version of the module
	Version string `json:"version"`
}
