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

package kubectl

import (
	"fmt"
	"os"
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/spf13/cobra"
	"k8s.io/cli-runtime/pkg/genericclioptions"

	"github.com/appvia/terranetes-controller/pkg/cmd"
	"github.com/appvia/terranetes-controller/pkg/utils"
)

var _ = Describe("Plugin Command", func() {
	var factory cmd.Factory
	var streams genericclioptions.IOStreams
	var err error
	var cm *cobra.Command

	BeforeEach(func() {
		var err error
		streams, _, _, _ = genericclioptions.NewTestIOStreams()
		factory, err = cmd.NewFactory(cmd.WithStreams(streams))
		Expect(err).NotTo(HaveOccurred())
		Expect(factory).NotTo(BeNil())

		cm = NewPluginCommand(factory)
	})

	When("generate plugins", func() {
		When("directory defined", func() {
			var tmpdir string

			BeforeEach(func() {
				tmpdir, err = os.MkdirTemp(os.TempDir(), "tnctl-plugin-test-XXXX")
				Expect(err).NotTo(HaveOccurred())
				Expect(tmpdir).NotTo(BeNil())

				os.Args = []string{"kubectl", "plugin", "-d", tmpdir}
				err = cm.Execute()
			})

			AfterEach(func() {
				os.RemoveAll(tmpdir)
			})

			It("should not error", func() {
				Expect(err).NotTo(HaveOccurred())
			})

			It("should create the files", func() {
				commands := []string{
					"approve",
					"config",
					"describe",
					"logs",
					"search",
				}

				for _, name := range commands {
					found, err := utils.FileExists(filepath.Join(tmpdir, fmt.Sprintf("kubectl-tnctl-%s", name)))
					Expect(err).NotTo(HaveOccurred())
					Expect(found).To(BeTrue())
				}
			})
		})
	})
})
