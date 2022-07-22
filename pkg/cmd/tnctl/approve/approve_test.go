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

package approve

import (
	"bytes"
	"context"
	"io/ioutil"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/sirupsen/logrus"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	terraformv1alphav1 "github.com/appvia/terranetes-controller/pkg/apis/terraform/v1alpha1"
	"github.com/appvia/terranetes-controller/pkg/cmd"
	"github.com/appvia/terranetes-controller/pkg/schema"
	"github.com/appvia/terranetes-controller/test/fixtures"
)

func TestReconcile(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Running Test Suite")
}

var _ = Describe("Approve Command", func() {
	logrus.SetOutput(ioutil.Discard)

	var cc client.Client
	var factory cmd.Factory
	var streams genericclioptions.IOStreams
	var stdout *bytes.Buffer
	var command *Command
	var err error

	BeforeEach(func() {
		cc = fake.NewFakeClientWithScheme(schema.GetScheme())
		streams, _, stdout, _ = genericclioptions.NewTestIOStreams()
		factory, _ = cmd.NewFactoryWithClient(cc, streams)
		command = &Command{Factory: factory}
		command.Names = []string{"test"}
		command.Namespace = "default"
	})

	When("the command is created", func() {
		It("should create a new command", func() {
			Expect(NewCommand(factory)).ToNot(BeNil())
		})
	})

	When("name is not provided", func() {
		BeforeEach(func() {
			command.Names = nil
			err = command.Run(context.Background())
		})

		It("should return an error", func() {
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(Equal("name is required"))
		})

		It("should be empty", func() {
			Expect(stdout.String()).To(BeEmpty())
		})
	})

	When("namespace is not provided", func() {
		BeforeEach(func() {
			command.Namespace = ""
			err = command.Run(context.Background())
		})

		It("should return an error", func() {
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(Equal("namespace is required"))
		})

		It("should be empty", func() {
			Expect(stdout.String()).To(BeEmpty())
		})
	})

	When("configuration does not exist", func() {
		BeforeEach(func() {
			err = command.Run(context.Background())
		})

		It("should return an error", func() {
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(Equal("configuration test not found"))
		})
	})

	When("configuration exists", func() {
		var configuration *terraformv1alphav1.Configuration

		BeforeEach(func() {
			configuration = fixtures.NewValidBucketConfiguration("default", "test")
			configuration.Annotations = map[string]string{terraformv1alphav1.ApplyAnnotation: "false"}
		})

		When("no approval annotation already exists", func() {
			BeforeEach(func() {
				configuration.Annotations[terraformv1alphav1.ApplyAnnotation] = "true"
				Expect(cc.Create(context.Background(), configuration)).To(Succeed())

				err = command.Run(context.Background())
			})

			It("should not error", func() {
				Expect(err).ToNot(HaveOccurred())
			})

			It("should be empty", func() {
				Expect(stdout.String()).To(BeEmpty())
			})
		})

		When("no approval annotation already exists", func() {
			BeforeEach(func() {
				Expect(cc.Create(context.Background(), configuration)).To(Succeed())
				err = command.Run(context.Background())
			})

			It("should not error", func() {
				Expect(err).ToNot(HaveOccurred())
			})

			It("should not be empty", func() {
				Expect(stdout.String()).ToNot(BeEmpty())
			})

			It("should indicates approval", func() {
				Expect(stdout.String()).To(ContainSubstring("Configuration test has been approved\n"))
			})
		})
	})
})
