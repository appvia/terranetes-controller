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
RR along with this program.  If not, see <http://www.gnu.org/licenses/>.
*/

package cmd

import "io"

// Config is the configuration for the tnctl command
type Config struct {
	// Workflow is the location of the workflow templates. This should point
	// the git repository that contains the templates. It is assumed the user
	// consuming these templates already has access the repository setup on
	// the machine and able to clone.
	// Example: https://github.com/appvia/terranetes-workflows. Note, if a
	// ref=TAG is not set, we will query the repository for the latest tag
	// (Github only)
	Workflow string `json:"workflow,omitempty"`
	// Sources defines a list of sources which the search command should
	// search terraform modules from. Currently we support the public
	// terraform registry and any Github user or organization.
	Sources []string `json:"sources,omitempty" yaml:"sources,omitempty"`
}

// Formatter is the interface that must be implemented by the formatter
type Formatter interface {
	// Printf prints a message to the output stream
	Printf(out io.Writer, format string, a ...interface{})
	// Println prints a message to the output stream
	Println(out io.Writer, format string, a ...interface{})
}

// ConfigInterface is the interface that must be implemented by the config struct
type ConfigInterface interface {
	// GetConfig returns the config for the cli if available
	GetConfig() (Config, error)
	// HasConfig return true if the config has been defined
	HasConfig() (bool, error)
	// SaveConfig saves the configuration to the file
	SaveConfig(Config) error
}

// NewDefaultConfig returns a default configuration
func NewDefaultConfig() *Config {
	return &Config{
		Workflow: "https://github.com/appvia/terranetes-workflows",
		Sources:  []string{"https://registry.terraform.io"},
	}
}
