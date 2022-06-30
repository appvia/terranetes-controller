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

package search

import (
	"context"
	"time"
)

// Query represents a search query
type Query struct {
	// Namespace is the namespace of the query
	Namespace string `json:"namespace"`
	// Provider is the provider your looking for modules for
	Provider string `json:"provider"`
	// Query is a provider specific query string
	Query string `json:"query"`
}

// Module is a agnostic representation of a module
type Module struct {
	// ID is a unique if for the module
	ID string `json:"id"`
	// CreatedAt is the module creation date
	CreatedAt time.Time `json:"created_at"`
	// Description is the module description
	Description string `json:"description"`
	// Downloads is the number of downloads the module has
	Downloads int `json:"downloads"`
	// Name is the module name
	Name string `json:"name"`
	// Namespace is the module namespace
	Namespace string `json:"namespace"`
	// Provider is the module provider
	Provider string `json:"provider"`
	// Registry is the registry the module is published to
	Registry string `json:"registry"`
	// RegistryType is the short name for the type of registry
	RegistryType string `json:"registry_type"`
	// Source is the module source
	Source string `json:"source"`
	// Stars is the number of stars the module has
	Stars int `json:"stars"`
	// Version is the latest version
	Version string `json:"version"`
}

// Response is the response from a search query
type Response struct {
	// Error is the error if one occurred
	Error error `json:"error"`
	// Modules is the list of modules found
	Modules []Module `json:"modules"`
	// Time is the amount of time taken to perform the search
	Time time.Duration `json:"time"`
}

// Interface for the search service
type Interface interface {
	// Find returns the list of matching modules
	Find(ctx context.Context, query Query) ([]Module, error)
	// ResolveSource returns the source for a module - this is only a requirement for
	// terraform registries as they don't show the tag on the module versions
	ResolveSource(ctx context.Context, module Module) (string, error)
	// Source returns the source for the search service
	Source() string
	// Versions returns a list of versions for a module
	Versions(ctx context.Context, module Module) ([]string, error)
}
