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

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/spf13/cobra"
	"k8s.io/cli-runtime/pkg/genericclioptions"

	"github.com/appvia/terranetes-controller/pkg/cmd"
)

var _ = Describe("Sources Command", func() {
	var factory cmd.Factory
	var streams genericclioptions.IOStreams
	var stdout *bytes.Buffer
	var rerr error
	var c *cobra.Command
	var configFile *os.File

	BeforeEach(func() {
		var err error
		streams, _, stdout, _ = genericclioptions.NewTestIOStreams()
		factory, err = cmd.NewFactory(streams)
		Expect(err).NotTo(HaveOccurred())
		Expect(factory).NotTo(BeNil())

		configFile, err = os.CreateTemp(os.TempDir(), "config-sources.XXXXX")
		Expect(err).NotTo(HaveOccurred())
		Expect(configFile).NotTo(BeNil())
		Expect(configFile.Close()).To(Succeed())
		os.Setenv(cmd.ConfigPathEnvName, configFile.Name())

		c = NewSourcesCommand(factory)
	})

	AfterEach(func() {
		os.Remove(configFile.Name())
	})

	When("adding a new source", func() {
		When("source does not exist", func() {
			BeforeEach(func() {
				os.Args = []string{"sources", "add", "https://foo"}
				rerr = c.Execute()
			})

			It("should have updated the configuration", func() {
				Expect(rerr).NotTo(HaveOccurred())

				content, err := os.ReadFile(configFile.Name())
				Expect(err).NotTo(HaveOccurred())
				Expect(content).ToNot(BeEmpty())
				Expect(string(content)).To(Equal("sources:\n- https://foo\n"))

				Expect(stdout.String()).To(ContainSubstring("Successfully saved configuration"))
			})
		})

		When("source already exists", func() {
			BeforeEach(func() {
				os.WriteFile(configFile.Name(), []byte("sources:\n- https://foo\n"), 0644)
				os.Args = []string{"sources", "add", "https://foo"}
				rerr = c.Execute()
			})

			It("should not have updated the configuration", func() {
				Expect(rerr).NotTo(HaveOccurred())

				content, err := os.ReadFile(configFile.Name())
				Expect(err).NotTo(HaveOccurred())
				Expect(content).ToNot(BeEmpty())
				Expect(string(content)).To(Equal("sources:\n- https://foo\n"))

				Expect(stdout.String()).To(ContainSubstring("Source already exists"))
			})
		})
	})

	When("deleting a source from the configuration", func() {
		BeforeEach(func() {
			os.WriteFile(configFile.Name(), []byte("sources:\n- https://foo\n"), 0644)

			os.Args = []string{"sources", "remove", "https://foo"}
			rerr = c.Execute()
		})

		It("should have deleted the source", func() {
			Expect(rerr).NotTo(HaveOccurred())

			content, err := os.ReadFile(configFile.Name())
			Expect(err).NotTo(HaveOccurred())
			Expect(content).ToNot(BeEmpty())
			Expect(string(content)).To(Equal("{}\n"))

			Expect(stdout.String()).To(ContainSubstring("Successfully saved configuration"))
		})
	})

	When("listing the sources from the configuration", func() {
		BeforeEach(func() {
			os.WriteFile(configFile.Name(), []byte("sources:\n- https://foo\n"), 0644)

			os.Args = []string{"sources", "list"}
		})

		When("config does not exist", func() {
			BeforeEach(func() {
				os.Remove(configFile.Name())
				rerr = c.Execute()
			})

			It("should show missing config", func() {
				Expect(rerr).NotTo(HaveOccurred())
				Expect(stdout.String()).To(ContainSubstring("No configuration found at"))
			})
		})

		When("config exists", func() {
			BeforeEach(func() {
				rerr = c.Execute()
			})

			It("list the sources", func() {
				expected := "You currently have the following sources active\n- https://foo\n"

				Expect(rerr).NotTo(HaveOccurred())
				Expect(stdout.String()).To(Equal(expected))
			})
		})
	})
})
