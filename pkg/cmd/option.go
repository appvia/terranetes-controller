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
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/client-go/kubernetes"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// WithKubeClient sets the kubernetes client
func WithKubeClient(client kubernetes.Interface) OptionFunc {
	return func(f *factory) {
		f.kc = client
	}
}

// WithClient sets the kubernetes client
func WithClient(client client.Client) OptionFunc {
	return func(f *factory) {
		f.cc = client
	}
}

// WithConfiguration sets the configuration provider
func WithConfiguration(config ConfigInterface) OptionFunc {
	return func(f *factory) {
		f.cfg = config
	}
}

// WithStreams sets the stream
func WithStreams(stream genericclioptions.IOStreams) OptionFunc {
	return func(f *factory) {
		f.streams = stream
	}
}
