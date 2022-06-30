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
	"bytes"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestNewFactory(t *testing.T) {
	factory, err := NewFactory(genericclioptions.IOStreams{})
	assert.NoError(t, err)
	assert.NotNil(t, factory)
}

func TestGetStreams(t *testing.T) {
	factory, err := NewFactory(genericclioptions.IOStreams{Out: os.Stdout})
	assert.NoError(t, err)
	assert.NotNil(t, factory)
	assert.NotNil(t, factory.GetStreams())
}

func TestGetConfigPath(t *testing.T) {
	factory, err := NewFactory(genericclioptions.IOStreams{Out: os.Stdout})
	assert.NoError(t, err)
	assert.NotNil(t, factory)
	assert.NotEmpty(t, factory.GetConfigPath())
}

func TestStdout(t *testing.T) {
	factory, err := NewFactory(genericclioptions.IOStreams{Out: os.Stdout})
	assert.NoError(t, err)
	assert.NotNil(t, factory)
	assert.NotNil(t, factory.Stdout())
}

func TestPrintln(t *testing.T) {
	b := &bytes.Buffer{}
	factory, err := NewFactory(genericclioptions.IOStreams{Out: b})
	assert.NoError(t, err)
	assert.NotNil(t, factory)

	factory.Println("Hello %s", "World")
	assert.Equal(t, "Hello World\n", b.String())
}

func TestPrintf(t *testing.T) {
	b := &bytes.Buffer{}
	factory, err := NewFactory(genericclioptions.IOStreams{Out: b})
	assert.NoError(t, err)
	assert.NotNil(t, factory)

	factory.Printf("Hello %s", "World")
	assert.Equal(t, "Hello World", b.String())
}

func TestGetClient(t *testing.T) {
	factory, err := NewFactoryWithClient(fake.NewClientBuilder().Build(), genericclioptions.IOStreams{})
	assert.NoError(t, err)
	assert.NotNil(t, factory)

	cc, err := factory.GetClient()
	assert.NoError(t, err)
	assert.NotNil(t, cc)
}
