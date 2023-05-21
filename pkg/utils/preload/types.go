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

import (
	"context"
	"errors"
)

var (
	// ErrNotReady indicates that one or more components are not ready and we should retry
	ErrNotReady = errors.New("cloud resources not ready")
)

// Interface is the external interface for the preload package
type Interface interface {
	// Load is used to load the preload data
	Load(ctx context.Context) (Data, error)
}

// Entry is a single entry in the preload data
type Entry struct {
	// Description is a human readable description of the entry
	Description string `json:"description"`
	// Value is the value of the entry
	Value interface{} `json:"value"`
}

// Data is a map of key/value pairs
type Data map[string]*Entry
