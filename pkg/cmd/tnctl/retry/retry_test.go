/*
 * Copyright (C) 2023  Appvia Ltd <info@appvia.io>
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

package retry

import (
	"bytes"
	"context"
	"os"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/spf13/cobra"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	terraformv1alpha1 "github.com/appvia/terranetes-controller/pkg/apis/terraform/v1alpha1"
	"github.com/appvia/terranetes-controller/pkg/cmd"
	"github.com/appvia/terranetes-controller/pkg/schema"
	"github.com/appvia/terranetes-controller/test/fixtures"
)

func TestRetryCommand(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Running Test Suite")
}

var _ = Describe("Retry Command", func() {
	var cc client.Client
	var configuration *terraformv1alpha1.Configuration
	var cloudresource *terraformv1alpha1.CloudResource
	var cm *cobra.Command
	var streams genericclioptions.IOStreams
	var stdout *bytes.Buffer
	var stderr *bytes.Buffer
	var err error

	BeforeEach(func() {
		cc = fake.NewClientBuilder().WithScheme(schema.GetScheme()).Build()
		streams, _, stdout, stderr = genericclioptions.NewTestIOStreams()
		factory, _ := cmd.NewFactory(
			cmd.WithClient(cc),
			cmd.WithStreams(streams),
		)
		cm = NewCommand(factory)

		configuration = fixtures.NewValidBucketConfiguration("default", "bucket")
		cloudresource = fixtures.NewCloudResource("default", "bucket")

		Expect(cc.Create(context.Background(), configuration)).To(Succeed())
		Expect(cc.Create(context.Background(), cloudresource)).To(Succeed())
	})

	When("retrying a configuration", func() {
		BeforeEach(func() {
			os.Args = []string{"retry", "configuration", configuration.Name, "-n", configuration.Namespace}
		})

		Context("when the configuration is not found", func() {
			BeforeEach(func() {
				Expect(cc.Delete(context.Background(), configuration)).To(Succeed())

				err = cm.ExecuteContext(context.Background())
			})

			It("should return an error", func() {
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(Equal("resource (default/bucket) does not exist"))
			})

			It("should print an error message", func() {
				Expect(stderr.String()).To(Equal("Error: resource (default/bucket) does not exist\n"))
			})
		})

		Context("when the configuration is found", func() {
			BeforeEach(func() {
				err = cm.ExecuteContext(context.Background())
			})

			It("should not return an error", func() {
				Expect(err).ToNot(HaveOccurred())
			})

			It("should have updated the annotations", func() {
				Expect(cc.Get(context.Background(), configuration.GetNamespacedName(), configuration)).To(Succeed())
				Expect(configuration.Annotations).ToNot(BeEmpty())
				Expect(configuration.Annotations[terraformv1alpha1.RetryAnnotation]).ToNot(BeEmpty())
			})

			It("should print a success message", func() {
				Expect(stdout.String()).To(ContainSubstring("Resource \"bucket\" has been marked for retry"))
			})
		})
	})

	When("retrying a cloudresource", func() {
		BeforeEach(func() {
			os.Args = []string{"retry", "cloudresource", configuration.Name, "-n", configuration.Namespace}
		})

		Context("when the cloudresource is not found", func() {
			BeforeEach(func() {
				Expect(cc.Delete(context.Background(), cloudresource)).To(Succeed())

				err = cm.ExecuteContext(context.Background())
			})

			It("should return an error", func() {
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(Equal("resource (default/bucket) does not exist"))
				_ = stdout
			})

			It("should print an error message", func() {
				Expect(stderr.String()).To(Equal("Error: resource (default/bucket) does not exist\n"))
			})
		})

		Context("when the cloudresource is found", func() {
			BeforeEach(func() {
				err = cm.ExecuteContext(context.Background())
			})

			It("should not return an error", func() {
				Expect(err).ToNot(HaveOccurred())
			})

			It("should have updated the annotations", func() {
				Expect(cc.Get(context.Background(), cloudresource.GetNamespacedName(), cloudresource)).To(Succeed())
				Expect(cloudresource.Annotations).ToNot(BeEmpty())
				Expect(cloudresource.Annotations[terraformv1alpha1.RetryAnnotation]).ToNot(BeEmpty())
			})

			It("should print a success message", func() {
				Expect(stdout.String()).To(ContainSubstring("Resource \"bucket\" has been marked for retry"))
			})
		})
	})
})
