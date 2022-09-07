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

package fixtures

import (
	"fmt"
	"io"

	"k8s.io/cli-runtime/pkg/genericclioptions"
	k8sclient "k8s.io/client-go/kubernetes"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/appvia/terranetes-controller/pkg/cmd"
)

// Factory is a test factory for the cli
type Factory struct {
	Config        cmd.Config
	KubeClient    k8sclient.Interface
	RuntimeClient client.Client
	Streams       genericclioptions.IOStreams
}

// GetConfig returns the config for the cli if available
func (f *Factory) GetConfig() (cmd.Config, bool, error) {
	return f.Config, true, nil
}

// GetClient returns the client for the kubernetes api
func (f *Factory) GetClient() (client.Client, error) {
	return f.RuntimeClient, nil
}

// GetKubeClient returns the kubernetes client
func (f *Factory) GetKubeClient() (k8sclient.Interface, error) {
	return f.KubeClient, nil
}

// GetStreams returns the input and output streams for the command
func (f *Factory) GetStreams() genericclioptions.IOStreams {
	return f.Streams
}

// Printf prints a message to the output stream
func (f *Factory) Printf(format string, a ...interface{}) {
	_, _ = f.Streams.Out.Write([]byte(fmt.Sprintf(format, a...)))
}

// Println prints a message to the output stream
func (f *Factory) Println(format string, a ...interface{}) {
	fmt.Fprintf(f.Streams.Out, format, a...)
}

// SaveConfig saves the configuration to the file
func (f *Factory) SaveConfig(config cmd.Config) error {
	f.Config = config

	return nil
}

// Stdout returns the stdout io writer
func (f *Factory) Stdout() io.Writer {
	return f.Streams.Out
}
