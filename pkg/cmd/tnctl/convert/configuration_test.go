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

package convert

import (
	"bytes"
	"context"
	"io/ioutil"
	"os"
	"path/filepath"
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
	"github.com/appvia/terranetes-controller/pkg/controller"
	"github.com/appvia/terranetes-controller/pkg/schema"
	"github.com/appvia/terranetes-controller/test/fixtures"
)

func TestCommand(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Running Test Suite")
}

var _ = Describe("Convert Configuration", func() {
	logrus.SetOutput(ioutil.Discard)

	var cc client.Client
	var streams genericclioptions.IOStreams
	var configuration *terraformv1alpha1.Configuration
	var cloudresource *terraformv1alpha1.CloudResource
	var provider *terraformv1alpha1.Provider
	var stderr *bytes.Buffer
	var command *cobra.Command
	var path string
	var err error

	BeforeEach(func() {
		cc = fake.NewClientBuilder().WithScheme(schema.GetScheme()).Build()
		streams, _, _, stderr = genericclioptions.NewTestIOStreams()

		configuration = fixtures.NewValidBucketConfiguration("default", "bucket")
		cloudresource = fixtures.NewCloudResource("default", "bucket")
		cloudresource.Status.ConfigurationName = configuration.Name

		namespace := fixtures.NewNamespace("default")

		secret := fixtures.NewValidAWSProviderSecret("terraform-system", "aws")
		provider = fixtures.NewValidAWSProvider("aws", secret)
		configuration.Spec.ProviderRef = &terraformv1alpha1.ProviderReference{Name: provider.Name}

		controller.EnsureConditionsRegistered(terraformv1alpha1.DefaultConfigurationConditions, configuration)
		controller.EnsureConditionsRegistered(terraformv1alpha1.DefaultCloudResourceConditions, cloudresource)

		path, err = ioutil.TempDir(os.TempDir(), "test-XXXXXXX")
		Expect(err).ToNot(HaveOccurred())
		Expect(path).ToNot(BeNil())

		factory, err := cmd.NewFactory(
			cmd.WithClient(cc),
			cmd.WithStreams(streams),
		)
		Expect(err).ToNot(HaveOccurred())
		Expect(factory).ToNot(BeNil())
		Expect(cc.Create(context.Background(), configuration)).To(Succeed())
		Expect(cc.Create(context.Background(), cloudresource)).To(Succeed())
		Expect(cc.Create(context.Background(), secret)).To(Succeed())
		Expect(cc.Create(context.Background(), provider)).To(Succeed())
		Expect(cc.Create(context.Background(), namespace)).To(Succeed())

		command = NewConfigurationCommand(factory)
	})

	JustAfterEach(func() {
		Expect(os.RemoveAll(path)).To(Succeed())
	})

	When("converting configuration", func() {
		Context("no source defined", func() {
			BeforeEach(func() {
				err = command.ExecuteContext(context.Background())
			})

			It("should fail", func() {
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(Equal("either file or name and namespace must be provided"))
			})
		})

		Context("from a file", func() {
			Context("and the file is missing", func() {
				BeforeEach(func() {
					command.SetArgs([]string{"--file", "missing.yaml"})

					err = command.ExecuteContext(context.Background())
				})

				It("should fail", func() {
					Expect(err).To(HaveOccurred())
					Expect(err.Error()).To(Equal("failed to read configuration file: file does not exist"))
				})
			})
		})

		Context("from the cluster", func() {
			BeforeEach(func() {
				command.SetArgs([]string{
					"--namespace", configuration.Namespace,
					"--path", path,
					configuration.Name,
				})
			})

			Context("and the namespace is not found", func() {
				BeforeEach(func() {
					Expect(cc.Delete(context.Background(), fixtures.NewNamespace("default"))).To(Succeed())

					err = command.ExecuteContext(context.Background())
				})

				It("should fail", func() {
					Expect(err).To(HaveOccurred())
					Expect(err.Error()).To(Equal("namespace \"default\" not found"))
				})
			})

			Context("and the configuration is missing", func() {
				BeforeEach(func() {
					Expect(cc.Delete(context.Background(), configuration)).To(Succeed())

					err = command.ExecuteContext(context.Background())
				})

				It("should return an error", func() {
					Expect(err).To(HaveOccurred())
					Expect(err.Error()).To(Equal("configuration (default/bucket) does not exist"))
				})

				It("should print the error to the output", func() {
					Expect(stderr.String()).To(Equal("Error: configuration (default/bucket) does not exist\n"))
				})
			})

			Context("and we have included provider configuration", func() {
				Context("but the provider is missing", func() {
					BeforeEach(func() {
						Expect(cc.Delete(context.Background(), provider)).To(Succeed())

						err = command.ExecuteContext(context.Background())
					})

					It("should return an error", func() {
						Expect(err).To(HaveOccurred())
						Expect(err.Error()).To(Equal("provider: \"\" does not exist in cluster"))
					})
				})

				Context("and the provider is present", func() {
					BeforeEach(func() {
						err = command.ExecuteContext(context.Background())
					})

					It("should succeed", func() {
						Expect(err).ToNot(HaveOccurred())
					})

					It("should have the main.tf file", func() {
						Expect(filepath.Join(path, "main.tf")).To(BeAnExistingFile())
					})

					It("should have the provider.tf file", func() {
						Expect(filepath.Join(path, "provider.tf")).To(BeAnExistingFile())
					})

					It("should not have the .checkov.yml file", func() {
						Expect(filepath.Join(path, ".checkov.yml")).ToNot(BeAnExistingFile())
					})
				})
			})

			Context("and we have included checkov policies", func() {
				var policy *terraformv1alpha1.Policy

				BeforeEach(func() {
					policy = fixtures.NewMatchAllPolicyConstraint("default")
					Expect(cc.Create(context.Background(), policy)).To(Succeed())
				})

				Context("but no checkov policies exist", func() {
					BeforeEach(func() {
						Expect(cc.Delete(context.Background(), policy)).To(Succeed())

						err = command.ExecuteContext(context.Background())
					})

					It("should succeed", func() {
						Expect(err).ToNot(HaveOccurred())
					})

					It("should have the main.tf file", func() {
						Expect(filepath.Join(path, "main.tf")).To(BeAnExistingFile())
					})

					It("should have the provider.tf file", func() {
						Expect(filepath.Join(path, "provider.tf")).To(BeAnExistingFile())
					})

					It("should not have the .checkov.yml file", func() {
						Expect(filepath.Join(path, ".checkov.yml")).ToNot(BeAnExistingFile())
					})
				})

				Context("and checkov policies exist", func() {
					BeforeEach(func() {
						err = command.ExecuteContext(context.Background())
					})

					It("should succeed", func() {
						Expect(err).ToNot(HaveOccurred())
					})

					It("should have the main.tf file", func() {
						Expect(filepath.Join(path, "main.tf")).To(BeAnExistingFile())
					})

					It("should have the provider.tf file", func() {
						Expect(filepath.Join(path, "provider.tf")).To(BeAnExistingFile())
					})

					It("should have the .checkov.yml file", func() {
						Expect(filepath.Join(path, ".checkov.yml")).To(BeAnExistingFile())
					})
				})
			})
		})
	})
})
