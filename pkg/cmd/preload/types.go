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

package preload

// Config is the configuration for the preload command
type Config struct {
	// Cloud is the cloud vendor we are dealing with
	Cloud string
	// Cluster is the name of the underlying resource we are using to find the context
	// from - i.e. the eks, gke or aks cluster name
	Cluster string
	// Context is the name of the context we should update / create
	Context string
	// EnableOverride is a flag to enable overriding the context if it already exists
	EnableOverride bool
	// Provider is the provider name which triggered the preloading
	Provider string
	// Region is the cloud vendor region we are dealing with
	Region string
}
