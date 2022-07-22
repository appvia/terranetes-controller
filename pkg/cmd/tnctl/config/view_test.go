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

package config

import (
	"bytes"
	"os"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/spf13/cobra"
	"k8s.io/cli-runtime/pkg/genericclioptions"

	"github.com/appvia/terranetes-controller/pkg/cmd"
)

func TestReconcile(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Running Test Suite")
}

var _ = Describe("View Command", func() {
	var factory cmd.Factory
	var streams genericclioptions.IOStreams
	var stdout *bytes.Buffer
	var rerr error
	var c *cobra.Command
	var tempfile *os.File

	BeforeEach(func() {
		var err error

		streams, _, stdout, _ = genericclioptions.NewTestIOStreams()
		factory, err = cmd.NewFactory(streams)
		Expect(err).NotTo(HaveOccurred())
		Expect(factory).NotTo(BeNil())

		tempfile, err = os.CreateTemp(os.TempDir(), "config-view.XXXXX")
		Expect(err).NotTo(HaveOccurred())
		Expect(tempfile).NotTo(BeNil())

		os.Setenv(cmd.ConfigPathEnvName, tempfile.Name())

		c = NewViewCommand(factory)
	})

	AfterEach(func() {
		os.Remove(tempfile.Name())
	})

	When("the config file does not exist", func() {
		BeforeEach(func() {
			os.Remove(tempfile.Name())
			rerr = c.RunE(c, []string{"config view"})
		})

		It("should return an error", func() {
			Expect(rerr).ToNot(HaveOccurred())
			Expect(stdout.String()).To(ContainSubstring("No configuration found at"))
		})
	})

	When("the config is empty", func() {
		BeforeEach(func() {
			rerr = c.RunE(c, []string{"config view"})
		})

		It("should return empty", func() {
			Expect(rerr).ToNot(HaveOccurred())
			Expect(stdout.String()).To(Equal("{}\n\n"))
		})
	})

	When("the config is not empty, but invalid", func() {
		BeforeEach(func() {
			_, err := tempfile.WriteString("BHD:: - BA: 1")
			Expect(err).NotTo(HaveOccurred())

			rerr = c.RunE(c, []string{"config view"})
		})

		It("should return empty", func() {
			Expect(rerr).To(HaveOccurred())
		})
	})

	When("the config is valid", func() {
		BeforeEach(func() {
			_, err := tempfile.WriteString("sources: [a,b]")
			Expect(err).NotTo(HaveOccurred())

			rerr = c.RunE(c, []string{"config view"})
		})

		It("should not error", func() {
			Expect(rerr).ToNot(HaveOccurred())
		})

		It("should have an output", func() {
			Expect(stdout.String()).ToNot(BeEmpty())
			Expect(stdout.String()).To(Equal("sources:\n- a\n- b\n\n"))
		})
	})
})
