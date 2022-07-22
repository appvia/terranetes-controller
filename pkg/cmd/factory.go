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
	"fmt"
	"io"
	"os"

	"k8s.io/cli-runtime/pkg/genericclioptions"
	k8sclient "k8s.io/client-go/kubernetes"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/config"
	"sigs.k8s.io/yaml"

	"github.com/appvia/terranetes-controller/pkg/schema"
	"github.com/appvia/terranetes-controller/pkg/utils"
)

// Factory is the interface that wraps the CLI
type Factory interface {
	// GetConfig returns the config for the cli if available
	GetConfig() (Config, bool, error)
	// GetConfigPath returns the path to the configuration file
	GetConfigPath() string
	// GetClient returns the client for the kubernetes api
	GetClient() (client.Client, error)
	// GetKubeClient returns the kubernetes client
	GetKubeClient() (k8sclient.Interface, error)
	// GetStreams returns the input and output streams for the command
	GetStreams() genericclioptions.IOStreams
	// Printf prints a message to the output stream
	Printf(format string, a ...interface{})
	// Println prints a message to the output stream
	Println(format string, a ...interface{})
	// SaveConfig saves the configuration to the file
	SaveConfig(Config) error
	// Stdout returns the stdout io writer
	Stdout() io.Writer
}

type factory struct {
	// cc is the kubernetes runtime client
	cc client.Client
	// streams is the input and output streams for the command
	streams genericclioptions.IOStreams
}

// NewFactory creates and returns a factory for the cli
func NewFactory(streams genericclioptions.IOStreams) (Factory, error) {
	return &factory{
		cc:      nil,
		streams: streams,
	}, nil
}

// NewFactoryWithClient creates and returns a factory for the cli
func NewFactoryWithClient(cc client.Client, streams genericclioptions.IOStreams) (Factory, error) {
	return &factory{
		cc:      cc,
		streams: streams,
	}, nil
}

// SaveConfig saves the configuration to the file
func (f *factory) SaveConfig(config Config) error {
	encoded, err := yaml.Marshal(&config)
	if err != nil {
		return err
	}

	return os.WriteFile(ConfigPath(), encoded, 0644)
}

// GetConfig returns true if we have a cli configuration file
func (f *factory) GetConfig() (Config, bool, error) {
	if exists, err := utils.FileExists(ConfigPath()); err != nil {
		return Config{}, false, err
	} else if !exists {
		return Config{}, false, nil
	}

	config, err := LoadConfig(ConfigPath())
	if err != nil {
		return Config{}, false, err
	}

	return config, true, nil
}

// GetConfigPath returns the path to the configuration file
func (f *factory) GetConfigPath() string {
	return ConfigPath()
}

// Printf prints a message to the output stream
func (f *factory) Printf(format string, a ...interface{}) {
	//nolint
	f.streams.Out.Write([]byte(fmt.Sprintf(format, a...)))
}

// Println prints a message to the output stream
func (f *factory) Println(format string, a ...interface{}) {
	//nolint
	f.streams.Out.Write([]byte(fmt.Sprintf(format+"\n", a...)))
}

// Stdout returns the stdout io writer
func (f *factory) Stdout() io.Writer {
	return f.streams.Out
}

// GetKubeClient returns the kubernetes client
func (f *factory) GetKubeClient() (k8sclient.Interface, error) {
	cfg, err := config.GetConfig()
	if err != nil {
		return nil, fmt.Errorf("failed to find kubeconfig: %v", err)
	}

	return k8sclient.NewForConfig(cfg)
}

// GetClient returns the client for the kubernetes api
func (f *factory) GetClient() (client.Client, error) {
	if f.cc == nil {
		cfg, err := config.GetConfig()
		if err != nil {
			return nil, fmt.Errorf("failed to find kubeconfig: %v", err)
		}

		cc, err := client.New(cfg, client.Options{Scheme: schema.GetScheme()})
		if err != nil {
			return nil, fmt.Errorf("failed to create kubernetes client: %v", err)
		}
		f.cc = cc
	}

	return f.cc, nil
}

// GetStreams returns the input and output streams for the command
func (f *factory) GetStreams() genericclioptions.IOStreams {
	return f.streams
}
