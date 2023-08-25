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

package state

import (
	"bytes"
	"context"
	"fmt"
	"io/ioutil"
	"os"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	k8sfake "k8s.io/client-go/kubernetes/fake"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/appvia/terranetes-controller/pkg/cmd"
	"github.com/appvia/terranetes-controller/pkg/schema"
	"github.com/appvia/terranetes-controller/test/fixtures"
)

var _ = Describe("Listing the state", func() {
	logrus.SetOutput(ioutil.Discard)
	testUID := "4845842d-f29b-4d12-8f6a-b73c7bf82836"

	var cc client.Client
	var kc *k8sfake.Clientset
	var factory cmd.Factory
	var streams genericclioptions.IOStreams
	var stdout *bytes.Buffer
	var command *cobra.Command
	var err error

	BeforeEach(func() {
		cc = fake.NewClientBuilder().
			WithScheme(schema.GetScheme()).
			Build()
		kc = k8sfake.NewSimpleClientset()

		streams, _, stdout, _ = genericclioptions.NewTestIOStreams()
		factory = &fixtures.Factory{
			RuntimeClient: cc,
			KubeClient:    kc,
			Streams:       streams,
		}
		command = NewCommand(factory)
	})

	When("no configurations exist", func() {
		BeforeEach(func() {
			os.Args = []string{"state", "list"}
			err = command.Execute()
		})

		It("should not error", func() {
			Expect(err).NotTo(HaveOccurred())
		})

		It("should indicate no configurations found", func() {
			Expect(stdout.String()).To(Equal("No configurations found"))
		})
	})

	When("configurations exist", func() {

		BeforeEach(func() {
			cfg := fixtures.NewValidBucketConfiguration("default", "test")
			cfg.UID = types.UID(testUID)
			cc.Create(context.Background(), cfg)
			os.Args = []string{"state", "list"}
			err = command.Execute()
		})

		It("should not error", func() {
			Expect(err).NotTo(HaveOccurred())
		})

		It("should list the configurations", func() {
			Expect(stdout.String()).To(Equal("CONFIGURATION\tSTATE\tCONFIG\tPOLICY\tCOST\tAGE  \ntest         \tNone \tNone  \tNone  \tNone\t292y\t\n"))
		})
	})

	When("we have configuration secrets present", func() {
		BeforeEach(func() {
			cfg := fixtures.NewValidBucketConfiguration("default", "test")
			cfg.UID = types.UID(testUID)
			cc.Create(context.Background(), cfg)

			for _, prefix := range SecretPrefixes {
				name := fmt.Sprintf("%s%v", prefix, testUID)
				secret := &v1.Secret{}
				secret.Name = name
				secret.Namespace = "terraform-system"
				Expect(cc.Create(context.Background(), secret)).ToNot(HaveOccurred())
			}
			os.Args = []string{"state", "list"}
			err = command.Execute()
		})

		It("should not error", func() {
			Expect(err).NotTo(HaveOccurred())
		})

		It("should list the configurations", func() {
			Expect(stdout.String()).To(ContainSubstring("tfstate-default-4845842d-f29b-4d12-8f6a-b73c7bf82836"))
		})
	})
})
