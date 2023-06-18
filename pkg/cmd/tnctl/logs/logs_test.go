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

package logs

import (
	"bytes"
	"context"
	"fmt"
	"io/ioutil"
	"strings"
	"testing"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	k8sfake "k8s.io/client-go/kubernetes/fake"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	corev1alpha1 "github.com/appvia/terranetes-controller/pkg/apis/core/v1alpha1"
	terraformv1alpha1 "github.com/appvia/terranetes-controller/pkg/apis/terraform/v1alpha1"
	"github.com/appvia/terranetes-controller/pkg/cmd"
	"github.com/appvia/terranetes-controller/pkg/controller"
	"github.com/appvia/terranetes-controller/pkg/schema"
	"github.com/appvia/terranetes-controller/test/fixtures"
)

func TestLogsCommand(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Running Test Suite")
}

var _ = Describe("Logs Command", func() {
	logrus.SetOutput(ioutil.Discard)

	var cc client.Client
	var kc *k8sfake.Clientset
	var streams genericclioptions.IOStreams
	var configuration *terraformv1alpha1.Configuration
	var cloudresource *terraformv1alpha1.CloudResource
	var command *cobra.Command
	var stderr *bytes.Buffer
	var err error

	BeforeEach(func() {
		cc = fake.NewClientBuilder().WithScheme(schema.GetScheme()).Build()
		kc = k8sfake.NewSimpleClientset()
		streams, _, _, stderr = genericclioptions.NewTestIOStreams()
		configuration = fixtures.NewValidBucketConfiguration("default", "bucket")
		cloudresource = fixtures.NewCloudResource("default", "bucket")
		cloudresource.Status.ConfigurationName = configuration.Name

		controller.EnsureConditionsRegistered(terraformv1alpha1.DefaultConfigurationConditions, configuration)
		controller.EnsureConditionsRegistered(terraformv1alpha1.DefaultCloudResourceConditions, cloudresource)

		factory, err := cmd.NewFactory(
			cmd.WithClient(cc),
			cmd.WithKubeClient(kc),
			cmd.WithStreams(streams),
		)
		Expect(err).ToNot(HaveOccurred())
		Expect(factory).ToNot(BeNil())
		Expect(cc.Create(context.Background(), configuration)).To(Succeed())
		Expect(cc.Create(context.Background(), cloudresource)).To(Succeed())

		command = NewCommand(factory)
	})

	When("no configuration provided", func() {
		BeforeEach(func() {
			command.SetArgs([]string{"configuration"})

			err = command.ExecuteContext(context.Background())
		})

		It("should return an error", func() {
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("accepts 1 arg(s)"))
		})
	})

	When("namespace does not exists", func() {
		BeforeEach(func() {
			command.SetArgs([]string{"--namespace", "does-not-exist", "configuration", "missing"})

			err = command.ExecuteContext(context.Background())
		})

		It("should return an error", func() {
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(Equal("resource \"missing\" not found"))
		})
	})

	When("retriving a configurations logs", func() {
		BeforeEach(func() {
			command.SetArgs([]string{"--timeout", "10ms", "--namespace", configuration.Namespace, "configuration", configuration.Name})
		})

		Context("configuration has no conditions", func() {
			BeforeEach(func() {
				configuration.Status.Conditions = nil
				Expect(cc.Status().Update(context.Background(), configuration)).To(Succeed())

				err = command.ExecuteContext(context.Background())
			})

			It("should have an error", func() {
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(Equal("neither plan, apply or destroy have been run for this resource"))
			})
		})

		for _, stage := range []corev1alpha1.ConditionType{
			terraformv1alpha1.ConditionTerraformPlan,
			terraformv1alpha1.ConditionTerraformApply,
		} {
			name := strings.ToLower(string(stage))

			Context(fmt.Sprintf("and we are in the %s stage", name), func() {
				BeforeEach(func() {
					condition := configuration.Status.GetCondition(stage)
					condition.Reason = corev1alpha1.ReasonInProgress
					condition.Status = metav1.ConditionTrue
					Expect(cc.Status().Update(context.Background(), configuration)).To(Succeed())

					// create the watcher
					pod := fixtures.NewConfigurationPodWatcher(configuration, string(stage))
					Expect(cc.Create(context.Background(), pod)).To(Succeed())
				})

				Context("but no pods exist", func() {
					BeforeEach(func() {
						pod := fixtures.NewConfigurationPodWatcher(configuration, string(stage))
						Expect(cc.Delete(context.Background(), pod)).To(Succeed())

						err = command.ExecuteContext(context.Background())
					})

					It("should not error", func() {
						Expect(err).To(HaveOccurred())
					})

					It("should print a message", func() {
						Expect(err.Error()).To(Equal("no pods found for resource \"bucket\""))
					})
				})

				Context("and pods exist", func() {
					BeforeEach(func() {
						err = command.ExecuteContext(context.Background())
					})

					It("should not error", func() {
						Expect(err).To(HaveOccurred())
					})
				})
			})
		}

		Context("configuration has been destroyed", func() {
			BeforeEach(func() {
				configuration.DeletionTimestamp = &metav1.Time{Time: time.Now()}
				Expect(cc.Update(context.Background(), configuration)).To(Succeed())

				pod := fixtures.NewConfigurationPodWatcher(configuration, terraformv1alpha1.StageTerraformDestroy)
				Expect(cc.Create(context.Background(), pod)).To(Succeed())
			})

			Context("but no pods exist", func() {
				BeforeEach(func() {
					pod := fixtures.NewConfigurationPodWatcher(configuration, terraformv1alpha1.StageTerraformDestroy)
					Expect(cc.Delete(context.Background(), pod)).To(Succeed())

					err = command.ExecuteContext(context.Background())
				})

				It("should not error", func() {
					Expect(err).To(HaveOccurred())
				})

				It("should print a message", func() {
					Expect(err.Error()).To(Equal("resource \"bucket\" not found"))
				})
			})

			Context("and pods exist", func() {
				BeforeEach(func() {
					err = command.ExecuteContext(context.Background())
				})

				It("should not error", func() {
					Expect(err).To(HaveOccurred())
				})
			})
		})
	})

	When("retriving a cloudresource logs", func() {
		BeforeEach(func() {
			cond := cloudresource.Status.GetCondition(terraformv1alpha1.ConditionConfigurationReady)
			cond.Status = metav1.ConditionTrue
			cond.Reason = corev1alpha1.ReasonReady
			Expect(cc.Status().Update(context.Background(), cloudresource)).To(Succeed())

			command.SetArgs([]string{"--timeout", "10ms", "--namespace", cloudresource.Namespace, "cloudresource", cloudresource.Name})
		})

		Context("cloudresource does not exist", func() {
			BeforeEach(func() {
				Expect(cc.Delete(context.Background(), cloudresource)).To(Succeed())

				err = command.ExecuteContext(context.Background())
			})

			It("should return an error", func() {
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(Equal("cloudresource (default/bucket) does not exist"))
			})

			It("should print a message", func() {
				Expect(stderr.String()).To(ContainSubstring("Error: cloudresource (default/bucket) does not exist"))
			})
		})

		Context("cloudresource has no conditions", func() {
			BeforeEach(func() {
				cloudresource.Status.Conditions = nil
				Expect(cc.Status().Update(context.Background(), cloudresource)).To(Succeed())

				err = command.ExecuteContext(context.Background())
			})

			It("should have an error", func() {
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(Equal("cloudresource (default/bucket) has no conditions yet"))
			})
		})

		Context("and the configuration has not been provisioned", func() {
			BeforeEach(func() {
				cond := cloudresource.Status.GetCondition(terraformv1alpha1.ConditionConfigurationReady)
				cond.Status = metav1.ConditionFalse
				cond.Reason = corev1alpha1.ReasonReady
				Expect(cc.Status().Update(context.Background(), cloudresource)).To(Succeed())

				err = command.ExecuteContext(context.Background())
			})

			It("should have an error", func() {
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(Equal("cloudresource (default/bucket) has no configuration yet"))
			})
		})

		Context("and the configuration has failed", func() {
			BeforeEach(func() {
				cond := cloudresource.Status.GetCondition(terraformv1alpha1.ConditionConfigurationReady)
				cond.Status = metav1.ConditionFalse
				cond.Reason = corev1alpha1.ReasonError
				cond.Message = "configuration failed"
				Expect(cc.Status().Update(context.Background(), cloudresource)).To(Succeed())

				err = command.ExecuteContext(context.Background())
			})

			It("should have an error", func() {
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(Equal("cloudresource (default/bucket) failed to provison configuration: configuration failed"))
			})
		})

		for _, stage := range []corev1alpha1.ConditionType{
			terraformv1alpha1.ConditionTerraformPlan,
			terraformv1alpha1.ConditionTerraformApply,
		} {
			name := strings.ToLower(string(stage))

			Context(fmt.Sprintf("and we are in the %s stage", name), func() {
				BeforeEach(func() {
					condition := cloudresource.Status.GetCondition(stage)
					condition.Reason = corev1alpha1.ReasonInProgress
					condition.Status = metav1.ConditionTrue
					Expect(cc.Status().Update(context.Background(), cloudresource)).To(Succeed())

					// create the watcher
					pod := fixtures.NewConfigurationPodWatcher(cloudresource, string(stage))
					Expect(cc.Create(context.Background(), pod)).To(Succeed())
				})

				Context("but no pods exist", func() {
					BeforeEach(func() {
						pod := fixtures.NewConfigurationPodWatcher(cloudresource, string(stage))
						Expect(cc.Delete(context.Background(), pod)).To(Succeed())

						err = command.ExecuteContext(context.Background())
					})

					It("should not error", func() {
						Expect(err).To(HaveOccurred())
					})

					It("should print a message", func() {
						Expect(err.Error()).To(Equal("no pods found for resource \"bucket\""))
					})
				})

				Context("and pods exist", func() {
					BeforeEach(func() {
						err = command.ExecuteContext(context.Background())
					})

					It("should not error", func() {
						Expect(err).To(HaveOccurred())
					})
				})
			})
		}

		Context("cloudresource has been destroyed", func() {
			BeforeEach(func() {
				cloudresource.DeletionTimestamp = &metav1.Time{Time: time.Now()}
				configuration.DeletionTimestamp = &metav1.Time{Time: time.Now()}
				cloudresource.Finalizers = []string{"does-not-matter"}
				configuration.Finalizers = []string{"does-not-matter"}
				Expect(cc.Update(context.Background(), cloudresource)).To(Succeed())
				Expect(cc.Update(context.Background(), configuration)).To(Succeed())

				pod := fixtures.NewConfigurationPodWatcher(configuration, terraformv1alpha1.StageTerraformDestroy)
				Expect(cc.Create(context.Background(), pod)).To(Succeed())
			})

			Context("and the configuration does not exist", func() {
				BeforeEach(func() {
					configuration.Finalizers = nil
					Expect(cc.Update(context.Background(), configuration)).To(Succeed())

					err = command.ExecuteContext(context.Background())
				})

				It("should not error", func() {
					Expect(err).To(HaveOccurred())
					Expect(err.Error()).To(Equal("resource \"bucket\" not found"))
				})
			})

			Context("but no pods exist", func() {
				BeforeEach(func() {
					pod := fixtures.NewConfigurationPodWatcher(cloudresource, terraformv1alpha1.StageTerraformDestroy)
					Expect(cc.Delete(context.Background(), pod)).To(Succeed())

					err = command.ExecuteContext(context.Background())
				})

				It("should not error", func() {
					Expect(err).To(HaveOccurred())
					Expect(err.Error()).To(Equal("no pods found for resource \"bucket\""))
				})
			})

			Context("and pods exist", func() {
				BeforeEach(func() {
					err = command.ExecuteContext(context.Background())
				})

				It("should not error", func() {
					Expect(err).To(HaveOccurred())
				})
			})
		})
	})
})
