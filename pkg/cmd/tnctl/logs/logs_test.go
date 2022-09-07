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
	"io/ioutil"
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

	corev1alphav1 "github.com/appvia/terranetes-controller/pkg/apis/core/v1alpha1"
	terraformv1alphav1 "github.com/appvia/terranetes-controller/pkg/apis/terraform/v1alpha1"
	"github.com/appvia/terranetes-controller/pkg/cmd"
	"github.com/appvia/terranetes-controller/pkg/controller"
	"github.com/appvia/terranetes-controller/pkg/schema"
	"github.com/appvia/terranetes-controller/test/fixtures"
)

func TestReconcile(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Running Test Suite")
}

var _ = Describe("Logs Command", func() {
	logrus.SetOutput(ioutil.Discard)
	ctx := context.Background()

	var cc client.Client
	var kc *k8sfake.Clientset
	var factory cmd.Factory
	var streams genericclioptions.IOStreams
	var configuration *terraformv1alphav1.Configuration
	var stdout *bytes.Buffer
	var command *cobra.Command
	var err error

	BeforeEach(func() {
		cc = fake.NewFakeClientWithScheme(schema.GetScheme())
		kc = k8sfake.NewSimpleClientset()

		streams, _, stdout, _ = genericclioptions.NewTestIOStreams()
		factory = &fixtures.Factory{
			RuntimeClient: cc,
			KubeClient:    kc,
			Streams:       streams,
		}
		command = NewCommand(factory)
	})

	When("no configuration is provided", func() {
		BeforeEach(func() {
			err = command.Execute()
		})

		It("should return an error", func() {
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("accepts 1 arg(s), received 0"))
		})
	})

	When("namespace does not exists", func() {
		BeforeEach(func() {
			command.SetArgs([]string{"--namespace", "does-not-exist", "missing"})
			err = command.Execute()
		})

		It("should return an error", func() {
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(Equal("resource \"missing\" not found"))
			_ = stdout
		})
	})

	When("configuration found but no conditions", func() {
		BeforeEach(func() {
			configuration = fixtures.NewValidBucketConfiguration("default", "test")
			command.SetArgs([]string{configuration.Name})
		})

		When("configuration has no status", func() {
			BeforeEach(func() {
				Expect(cc.Create(ctx, configuration)).To(Succeed())
				err = command.Execute()
			})

			It("should have an error", func() {
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(Equal("neither plan, apply or destroy have been run for this configuration"))
			})
		})
	})

	When("configuration is found", func() {
		BeforeEach(func() {
			configuration = fixtures.NewValidBucketConfiguration("default", "test")
			controller.EnsureConditionsRegistered(terraformv1alphav1.DefaultConfigurationConditions, configuration)
			command.SetArgs([]string{"--namespace", configuration.Namespace, configuration.Name})
		})

		When("no pod exists", func() {
			BeforeEach(func() {
				Expect(cc.Create(ctx, configuration)).To(Succeed())
				err = command.Execute()
			})

			It("should not error", func() {
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(Equal("no pods found for configuration \"test\""))
			})
		})

		When("configuration is in plan phase and pod exists", func() {
			When("pod is not ready", func() {
				BeforeEach(func() {
					condition := configuration.Status.GetCondition(terraformv1alphav1.ConditionTerraformPlan)
					condition.Reason = corev1alphav1.ReasonInProgress
					condition.Status = metav1.ConditionTrue
					Expect(cc.Create(ctx, configuration)).To(Succeed())

					err = command.Execute()
				})

				It("should error", func() {
					Expect(err).To(HaveOccurred())
					Expect(err.Error()).To(Equal("no pods found for configuration \"test\""))
				})
			})

			When("pod is ready", func() {
				BeforeEach(func() {
					condition := configuration.Status.GetCondition(terraformv1alphav1.ConditionTerraformPlan)
					condition.Reason = corev1alphav1.ReasonInProgress
					condition.Status = metav1.ConditionTrue

					// create the pod for apply
					pod := fixtures.NewConfigurationPodWatcher(configuration, terraformv1alphav1.StageTerraformPlan)
					_, err = kc.CoreV1().Pods(configuration.Namespace).Create(ctx, pod, metav1.CreateOptions{})
					Expect(err).To(Succeed())
					Expect(cc.Create(ctx, configuration)).To(Succeed())

					err = command.Execute()
				})

				It("should not error", func() {
					Expect(err).ToNot(HaveOccurred())
				})
			})
		})

		When("configuration in apply phase", func() {
			When("no pod exists", func() {
				BeforeEach(func() {
					condition := configuration.Status.GetCondition(terraformv1alphav1.ConditionTerraformApply)
					condition.Reason = corev1alphav1.ReasonInProgress
					condition.Status = metav1.ConditionTrue
					Expect(cc.Create(ctx, configuration)).To(Succeed())

					err = command.Execute()
				})

				It("should error", func() {
					Expect(err).To(HaveOccurred())
					Expect(err.Error()).To(Equal("no pods found for configuration \"test\""))
				})
			})

			When("pod exists", func() {
				BeforeEach(func() {
					condition := configuration.Status.GetCondition(terraformv1alphav1.ConditionTerraformApply)
					condition.Reason = corev1alphav1.ReasonInProgress
					condition.Status = metav1.ConditionTrue

					// create the pod for apply
					pod := fixtures.NewConfigurationPodWatcher(configuration, terraformv1alphav1.StageTerraformApply)
					_, err = kc.CoreV1().Pods(configuration.Namespace).Create(ctx, pod, metav1.CreateOptions{})
					Expect(err).To(Succeed())
					Expect(cc.Create(ctx, configuration)).To(Succeed())

					err = command.Execute()
				})

				It("should not error", func() {
					Expect(err).ToNot(HaveOccurred())
				})
			})
		})

		When("configuration in destroy phase", func() {
			When("no pod exists", func() {
				BeforeEach(func() {
					configuration.DeletionTimestamp = &metav1.Time{Time: time.Now()}
					configuration.Finalizers = []string{"do-not-remove"}
					Expect(cc.Create(ctx, configuration)).To(Succeed())

					err = command.Execute()
				})

				It("should error", func() {
					Expect(err).To(HaveOccurred())
					Expect(err.Error()).To(Equal("no pods found for configuration \"test\""))
				})
			})

			When("pod exists", func() {
				BeforeEach(func() {
					configuration.DeletionTimestamp = &metav1.Time{Time: time.Now()}
					configuration.Finalizers = []string{"do-not-remove"}
					Expect(cc.Create(ctx, configuration)).To(Succeed())

					// create the pod for apply
					pod := fixtures.NewConfigurationPodWatcher(configuration, terraformv1alphav1.StageTerraformDestroy)
					_, err = kc.CoreV1().Pods(configuration.Namespace).Create(ctx, pod, metav1.CreateOptions{})
					Expect(err).To(Succeed())

					err = command.Execute()
				})

				It("should not error", func() {
					Expect(err).ToNot(HaveOccurred())
				})
			})
		})
	})
})
