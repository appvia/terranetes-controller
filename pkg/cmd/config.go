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

package cmd

import (
	"os"
	"path/filepath"

	"gopkg.in/yaml.v2"

	"github.com/appvia/terranetes-controller/pkg/utils"
)

// ConfigPathEnvName is the name of the environment variable that holds the config path
const ConfigPathEnvName = "TNCTL_CONFIG"

type fileConfig struct {
	path string
}

// NewFileConfiguration creates and returns a file configuration
func NewFileConfiguration(filename string) ConfigInterface {
	return &fileConfig{path: filename}
}

// HasConfig returns if the configuration file exists
func (f *fileConfig) HasConfig() (bool, error) {
	if exists, err := utils.FileExists(f.path); err != nil {
		return false, err
	} else if !exists {
		return false, nil
	}

	return true, nil
}

// SaveConfig saves the configuration to the file
func (f *fileConfig) SaveConfig(config Config) error {
	encoded, err := yaml.Marshal(&config)
	if err != nil {
		return err
	}

	// @step: ensure the directory exists
	if err := os.MkdirAll(filepath.Dir(f.path), 0750); err != nil {
		return err
	}

	return os.WriteFile(f.path, encoded, 0640)
}

// GetConfig returns the configuration
func (f *fileConfig) GetConfig() (Config, error) {
	config := Config{}

	found, err := utils.FileExists(f.path)
	if err != nil {
		return config, err
	}
	if !found {
		return config, nil
	}

	in, err := os.ReadFile(f.path)
	if err != nil {
		return config, err
	}

	if err := yaml.Unmarshal(in, &config); err != nil {
		return config, err
	}

	return config, nil
}
