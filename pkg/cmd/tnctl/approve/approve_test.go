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
	"os"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	terraformv1alpha1 "github.com/appvia/terranetes-controller/pkg/apis/terraform/v1alpha1"
	"github.com/appvia/terranetes-controller/pkg/cmd"
	"github.com/appvia/terranetes-controller/pkg/schema"
	"github.com/appvia/terranetes-controller/test/fixtures"
)

func TestApproveCommand(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Running Test Suite")
}

var _ = Describe("Approve Command", func() {
	logrus.SetOutput(ioutil.Discard)

	var cc client.Client
	var configuration *terraformv1alpha1.Configuration
	var cloudresource *terraformv1alpha1.CloudResource
	var streams genericclioptions.IOStreams
	var stderr *bytes.Buffer
	var stdout *bytes.Buffer
	var command *cobra.Command
	var err error

	BeforeEach(func() {
		cc = fake.NewClientBuilder().WithScheme(schema.GetScheme()).Build()
		streams, _, stdout, stderr = genericclioptions.NewTestIOStreams()
		factory, _ := cmd.NewFactory(
			cmd.WithClient(cc),
			cmd.WithStreams(streams),
		)
		command = NewCommand(factory)

		configuration = fixtures.NewValidBucketConfiguration("default", "bucket")
		cloudresource = fixtures.NewCloudResource("default", "bucket")

		Expect(cc.Create(context.Background(), configuration)).To(Succeed())
		Expect(cc.Create(context.Background(), cloudresource)).To(Succeed())
	})

	When("approving a configuration", func() {
		BeforeEach(func() {
			os.Args = []string{"approve", "configuration", configuration.Name}
		})

		Context("when no names provided", func() {
			BeforeEach(func() {
				os.Args = []string{"approve", "configuration"}
			})

			It("should return an error", func() {
				Expect(command.Execute()).ToNot(Succeed())
				Expect(stderr.String()).To(Equal("Error: requires at least 1 arg(s), only received 0\n"))
			})
		})

		Context("when the configuration is not found", func() {
			BeforeEach(func() {
				Expect(cc.Delete(context.Background(), configuration)).To(Succeed())

				err = command.ExecuteContext(context.Background())
			})

			It("should return an error", func() {
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(Equal("resource bucket not found"))
			})

			It("should print the error message", func() {
				Expect(stderr.String()).To(Equal("Error: resource bucket not found\n"))
			})
		})

		Context("when the configuration is found", func() {
			Context("but the configuration does not require approval", func() {
				BeforeEach(func() {
					err = command.ExecuteContext(context.Background())
				})

				It("should not return an error", func() {
					Expect(err).NotTo(HaveOccurred())
				})
			})

			Context("and the configuration requires approval", func() {
				BeforeEach(func() {
					configuration.Annotations = map[string]string{terraformv1alpha1.ApplyAnnotation: "false"}
					Expect(cc.Update(context.Background(), configuration)).To(Succeed())

					err = command.ExecuteContext(context.Background())
				})

				It("should have approve the configuration", func() {
					Expect(cc.Get(context.Background(), configuration.GetNamespacedName(), configuration)).To(Succeed())
					Expect(configuration.GetAnnotations()).ToNot(BeEmpty())
					Expect(configuration.GetAnnotations()[terraformv1alpha1.ApplyAnnotation]).To(Equal("true"))
				})

				It("should print the approved message", func() {
					Expect(stdout.String()).To(ContainSubstring("Configuration bucket has been approved\n"))
				})
			})
		})
	})

	When("approving a cloudresource", func() {
		BeforeEach(func() {
			os.Args = []string{"approve", "cloudresource", cloudresource.Name}
		})

		Context("when the cloudresource is not found", func() {
			BeforeEach(func() {
				Expect(cc.Delete(context.Background(), cloudresource)).To(Succeed())

				err = command.ExecuteContext(context.Background())
			})

			It("should return an error", func() {
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(Equal("resource bucket not found"))
			})

			It("should print the error message", func() {
				Expect(stderr.String()).To(Equal("Error: resource bucket not found\n"))
			})
		})

		Context("when the cloudresource is found", func() {
			Context("but the cloudresource does not require approval", func() {
				BeforeEach(func() {
					err = command.ExecuteContext(context.Background())
				})

				It("should not return an error", func() {
					Expect(err).NotTo(HaveOccurred())
				})
			})

			Context("and the cloudresource requires approval", func() {
				BeforeEach(func() {
					cloudresource.Annotations = map[string]string{terraformv1alpha1.ApplyAnnotation: "false"}
					Expect(cc.Update(context.Background(), cloudresource)).To(Succeed())

					err = command.ExecuteContext(context.Background())
				})

				It("should have approve the cloudresource", func() {
					Expect(cc.Get(context.Background(), cloudresource.GetNamespacedName(), cloudresource)).To(Succeed())
					Expect(cloudresource.GetAnnotations()).ToNot(BeEmpty())
					Expect(cloudresource.GetAnnotations()[terraformv1alpha1.ApplyAnnotation]).To(Equal("true"))
				})

				It("should print the approved message", func() {
					Expect(stdout.String()).To(ContainSubstring("CloudResource bucket has been approved\n"))
				})
			})
		})
	})
})
