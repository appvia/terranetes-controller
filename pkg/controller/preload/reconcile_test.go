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

package preload

import (
	"context"
	"io"
	"testing"
	"time"

	//"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/sirupsen/logrus"

	//	batchv1 "k8s.io/api/batch/v1"
	batchv1 "k8s.io/api/batch/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	corev1alpha1 "github.com/appvia/terranetes-controller/pkg/apis/core/v1alpha1"
	terraformv1alpha1 "github.com/appvia/terranetes-controller/pkg/apis/terraform/v1alpha1"
	"github.com/appvia/terranetes-controller/pkg/schema"
	controllertests "github.com/appvia/terranetes-controller/test"
	"github.com/appvia/terranetes-controller/test/fixtures"
)

func TestReconcile(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Running Test Suite")
}

var _ = Describe("Preload Controller", func() {
	logrus.SetOutput(io.Discard)

	var cc client.Client
	var result reconcile.Result
	var rerr error
	var ctrl *Controller
	var provider *terraformv1alpha1.Provider

	BeforeEach(func() {
		cc = fake.NewClientBuilder().
			WithScheme(schema.GetScheme()).
			WithStatusSubresource(&terraformv1alpha1.Context{}).
			WithStatusSubresource(&terraformv1alpha1.Provider{}).
			Build()
		recorder := &controllertests.FakeRecorder{}
		ctrl = &Controller{
			cc:                  cc,
			recorder:            recorder,
			ContainerImage:      "appvia/terranetes-executor:latest",
			ControllerNamespace: "default",
		}
	})

	When("a provider has been provisioned", func() {
		When("using an unsupported provider", func() {
			BeforeEach(func() {
				provider = fixtures.NewValidAWSReadyProvider("aws", fixtures.NewValidAWSProviderSecret("default", "aws"))
				provider.Spec.Provider = "unsupported"
				provider.Spec.Preload = &terraformv1alpha1.PreloadConfiguration{
					Cluster: "test",
					Context: "test",
					Enabled: pointer.BoolPtr(true),
					Region:  "test",
				}

				Expect(cc.Create(context.Background(), provider)).To(Succeed())
				result, _, rerr = controllertests.Roll(context.TODO(), ctrl, provider, 0)
			})

			It("should not error", func() {
				Expect(rerr).NotTo(HaveOccurred())
				Expect(result.Requeue).To(BeFalse())
			})

			It("should update the conditions to warning", func() {
				Expect(cc.Get(context.TODO(), provider.GetNamespacedName(), provider)).ToNot(HaveOccurred())

				cond := provider.GetCommonStatus().GetCondition(terraformv1alpha1.ConditionProviderPreload)
				Expect(cond.Type).To(Equal(terraformv1alpha1.ConditionProviderPreload))
				Expect(cond.Status).To(Equal(metav1.ConditionFalse))
				Expect(cond.Reason).To(Equal(corev1alpha1.ReasonWarning))
				Expect(cond.Message).To(Equal("Loading contextual is supported on AWS only"))
			})
		})

		When("the provider has no conditions yet", func() {
			BeforeEach(func() {
				provider = fixtures.NewValidAWSNotReadyProvider("aws", fixtures.NewValidAWSProviderSecret("default", "aws"))
				provider.Spec.Preload = &terraformv1alpha1.PreloadConfiguration{
					Cluster: "test",
					Context: "test",
					Enabled: pointer.BoolPtr(true),
					Region:  "test",
				}
				provider.Status.Conditions = nil

				Expect(cc.Create(context.Background(), provider)).To(Succeed())
				result, _, rerr = controllertests.Roll(context.TODO(), ctrl, provider, 0)
			})

			It("should not error", func() {
				Expect(rerr).NotTo(HaveOccurred())
				Expect(result.Requeue).To(BeFalse())
			})

			It("should requeue", func() {
				Expect(result.RequeueAfter).To(Equal(time.Second * 15))
			})
		})

		When("and the provider is not ready", func() {
			BeforeEach(func() {
				provider = fixtures.NewValidAWSNotReadyProvider("aws", fixtures.NewValidAWSProviderSecret("default", "aws"))

				Expect(cc.Create(context.Background(), provider)).To(Succeed())
				result, _, rerr = controllertests.Roll(context.TODO(), ctrl, provider, 0)
			})

			It("should not error", func() {
				Expect(rerr).NotTo(HaveOccurred())
				Expect(result.Requeue).To(BeFalse())
			})

			It("should update the conditions to disabled", func() {
				Expect(cc.Get(context.TODO(), provider.GetNamespacedName(), provider)).ToNot(HaveOccurred())

				cond := provider.GetCommonStatus().GetCondition(terraformv1alpha1.ConditionProviderPreload)
				Expect(cond.Type).To(Equal(terraformv1alpha1.ConditionProviderPreload))
				Expect(cond.Status).To(Equal(metav1.ConditionFalse))
				Expect(cond.Reason).To(Equal(corev1alpha1.ReasonDisabled))
				Expect(cond.Message).To(Equal("Loading contextual data is not enabled"))
			})

			It("should not create any jobs", func() {
				jobs := &batchv1.JobList{}

				Expect(cc.List(context.Background(), jobs)).To(Succeed())
				Expect(jobs.Items).To(BeEmpty())
			})
		})

		When("and the preloading has been disabled afterwards", func() {
			BeforeEach(func() {
				provider = fixtures.NewValidAWSNotReadyProvider("aws", fixtures.NewValidAWSProviderSecret("default", "aws"))
				cond := provider.GetCommonStatus().GetCondition(terraformv1alpha1.ConditionProviderPreload)
				cond.Status = metav1.ConditionTrue
				cond.Reason = corev1alpha1.ReasonReady
				cond.Message = "Preloading is enabled"
				provider.Status.LastPreloadTime = &metav1.Time{Time: time.Now()}

				provider.Spec.Preload = &terraformv1alpha1.PreloadConfiguration{
					Enabled: pointer.BoolPtr(false),
				}
				Expect(cc.Create(context.Background(), provider)).To(Succeed())

				result, _, rerr = controllertests.Roll(context.TODO(), ctrl, provider, 0)
			})

			It("should not error", func() {
				Expect(rerr).NotTo(HaveOccurred())
				Expect(result.Requeue).To(BeFalse())
			})

			It("should update the conditions to disabled", func() {
				Expect(cc.Get(context.TODO(), provider.GetNamespacedName(), provider)).ToNot(HaveOccurred())

				cond := provider.GetCommonStatus().GetCondition(terraformv1alpha1.ConditionProviderPreload)
				Expect(cond.Type).To(Equal(terraformv1alpha1.ConditionProviderPreload))
				Expect(cond.Status).To(Equal(metav1.ConditionFalse))
				Expect(cond.Reason).To(Equal(corev1alpha1.ReasonDisabled))
				Expect(cond.Message).To(Equal("Loading contextual data is not enabled"))
			})

			It("should not create any jobs", func() {
				jobs := &batchv1.JobList{}

				Expect(cc.List(context.Background(), jobs)).To(Succeed())
				Expect(jobs.Items).To(BeEmpty())
			})
		})

		When("and the provider does not have a preload enabled", func() {
			BeforeEach(func() {
				provider = fixtures.NewValidAWSReadyProvider("aws", fixtures.NewValidAWSProviderSecret("default", "aws"))

				Expect(cc.Create(context.Background(), provider)).To(Succeed())
				result, _, rerr = controllertests.Roll(context.TODO(), ctrl, provider, 0)
			})

			It("should not error", func() {
				Expect(rerr).NotTo(HaveOccurred())
				Expect(result.Requeue).To(BeFalse())
			})

			It("should update the conditions to disabled", func() {
				Expect(cc.Get(context.TODO(), provider.GetNamespacedName(), provider)).ToNot(HaveOccurred())

				cond := provider.GetCommonStatus().GetCondition(terraformv1alpha1.ConditionProviderPreload)
				Expect(cond.Type).To(Equal(terraformv1alpha1.ConditionProviderPreload))
				Expect(cond.Status).To(Equal(metav1.ConditionFalse))
				Expect(cond.Reason).To(Equal(corev1alpha1.ReasonDisabled))
				Expect(cond.Message).To(Equal("Loading contextual data is not enabled"))
			})

			It("should not create any jobs", func() {
				jobs := &batchv1.JobList{}

				Expect(cc.List(context.Background(), jobs)).To(Succeed())
				Expect(jobs.Items).To(BeEmpty())
			})
		})
	})

	When("a provider has been provisioned", func() {
		Context("but the provider is not ready", func() {
			BeforeEach(func() {
				provider = fixtures.NewValidAWSNotReadyProvider("aws", fixtures.NewValidAWSProviderSecret("default", "aws"))
				provider.Spec.Preload = &terraformv1alpha1.PreloadConfiguration{
					Cluster:  "test-cluster",
					Context:  "test-context",
					Enabled:  pointer.Bool(true),
					Region:   "eu-west-2",
					Interval: &metav1.Duration{Duration: 1 * time.Hour},
				}
				Expect(cc.Create(context.Background(), provider)).To(Succeed())

				result, _, rerr = controllertests.Roll(context.TODO(), ctrl, provider, 0)
			})

			It("should not error", func() {
				Expect(rerr).NotTo(HaveOccurred())
				Expect(result.Requeue).To(BeFalse())
				Expect(result.RequeueAfter).To(Equal(time.Second * 15))
			})

			It("should update the conditions", func() {
				Expect(cc.Get(context.TODO(), provider.GetNamespacedName(), provider)).ToNot(HaveOccurred())

				cond := provider.GetCommonStatus().GetCondition(terraformv1alpha1.ConditionProviderPreload)
				Expect(cond.Type).To(Equal(terraformv1alpha1.ConditionProviderPreload))
				Expect(cond.Status).To(Equal(metav1.ConditionFalse))
				Expect(cond.Reason).To(Equal(corev1alpha1.ReasonInProgress))
				Expect(cond.Message).To(Equal("Waiting for provider to be ready"))
			})

			It("should not create any jobs", func() {
				jobs := &batchv1.JobList{}

				Expect(cc.List(context.Background(), jobs)).To(Succeed())
				Expect(jobs.Items).To(BeEmpty())
			})
		})
	})

	When("a provider has been provisioned and the provider is ready", func() {
		BeforeEach(func() {
			provider = fixtures.NewValidAWSReadyProvider("aws", fixtures.NewValidAWSProviderSecret("default", "aws"))
			provider.Spec.Preload = &terraformv1alpha1.PreloadConfiguration{
				Cluster: "test-cluster",
				Context: "test-context",
				Enabled: pointer.Bool(true),
				Region:  "eu-west-2",
			}

			Expect(cc.Create(context.Background(), provider)).To(Succeed())
		})

		Context("but we have an active job running", func() {
			BeforeEach(func() {
				job := fixtures.NewRunningPreloadJob("default", provider.Name)

				Expect(cc.Create(context.Background(), job)).To(Succeed())
				result, _, rerr = controllertests.Roll(context.TODO(), ctrl, provider, 0)
			})

			It("should not error", func() {
				Expect(rerr).NotTo(HaveOccurred())
				Expect(result.Requeue).To(BeFalse())
			})

			It("should requeue", func() {
				Expect(result.RequeueAfter).To(Equal(time.Second * 5))
			})

			It("should not create a preload job", func() {
				jobs := &batchv1.JobList{}

				Expect(cc.List(context.Background(), jobs)).To(Succeed())
				Expect(jobs.Items).To(HaveLen(1))
			})

			It("should update the conditions", func() {
				Expect(cc.Get(context.TODO(), provider.GetNamespacedName(), provider)).ToNot(HaveOccurred())

				cond := provider.GetCommonStatus().GetCondition(terraformv1alpha1.ConditionProviderPreload)
				Expect(cond.Type).To(Equal(terraformv1alpha1.ConditionProviderPreload))
				Expect(cond.Status).To(Equal(metav1.ConditionFalse))
				Expect(cond.Reason).To(Equal(corev1alpha1.ReasonInProgress))
				Expect(cond.Message).To(Equal("Contextual data is currently being loaded under job: preload-test"))
			})
		})

		Context("and we have active jobs running, with another provider", func() {
			BeforeEach(func() {
				job := fixtures.NewRunningPreloadJob("default", "other")

				Expect(cc.Create(context.Background(), job)).To(Succeed())
				result, _, rerr = controllertests.Roll(context.TODO(), ctrl, provider, 0)
			})

			It("should not error", func() {
				Expect(rerr).NotTo(HaveOccurred())
				Expect(result.Requeue).To(BeFalse())
			})

			It("should create a preload job", func() {
				jobs := &batchv1.JobList{}

				Expect(cc.List(context.Background(), jobs)).To(Succeed())
				Expect(jobs.Items).To(HaveLen(2))
			})
		})

		Context("and we have a failed job", func() {
			BeforeEach(func() {
				job := fixtures.NewFailedPreloadJob("default", provider.Name)

				Expect(cc.Create(context.Background(), job)).To(Succeed())
				result, _, rerr = controllertests.Roll(context.TODO(), ctrl, provider, 0)
			})

			It("should not error", func() {
				Expect(rerr).NotTo(HaveOccurred())
				Expect(result.Requeue).To(BeFalse())
			})

			It("should create a preload job", func() {
				jobs := &batchv1.JobList{}

				Expect(cc.List(context.Background(), jobs)).To(Succeed())
				Expect(jobs.Items).To(HaveLen(2))
			})
		})

		Context("and we have no active jobs", func() {
			BeforeEach(func() {
				result, _, rerr = controllertests.Roll(context.TODO(), ctrl, provider, 0)
			})

			It("should not error", func() {
				Expect(rerr).NotTo(HaveOccurred())
				Expect(result.Requeue).To(BeFalse())
			})

			It("should update the conditions", func() {
				Expect(cc.Get(context.TODO(), provider.GetNamespacedName(), provider)).ToNot(HaveOccurred())

				cond := provider.GetCommonStatus().GetCondition(terraformv1alpha1.ConditionProviderPreload)
				Expect(cond.Type).To(Equal(terraformv1alpha1.ConditionProviderPreload))
				Expect(cond.Status).To(Equal(metav1.ConditionFalse))
				Expect(cond.Reason).To(Equal(corev1alpha1.ReasonInProgress))
				Expect(cond.Message).To(ContainSubstring("Contextual data is currently running under job: preload-"))
			})

			It("should create a preload job", func() {
				jobs := &batchv1.JobList{}

				Expect(cc.List(context.Background(), jobs)).To(Succeed())
				Expect(jobs.Items).To(HaveLen(1))
			})

			It("should have create a preload job", func() {
				jobs := &batchv1.JobList{}
				Expect(cc.List(context.Background(), jobs)).To(Succeed())
				Expect(jobs.Items).To(HaveLen(1))

				job := jobs.Items[0]
				Expect(job.Labels).To(HaveKeyWithValue(terraformv1alpha1.PreloadProviderLabel, provider.Name))
				Expect(job.Labels).To(HaveKeyWithValue(terraformv1alpha1.PreloadJobLabel, "true"))

				contaniner := job.Spec.Template.Spec.Containers[0]
				Expect(contaniner.Name).To(Equal("loader"))
				Expect(contaniner.Image).To(Equal(ctrl.ContainerImage))
				Expect(contaniner.Command).To(Equal([]string{"/bin/preload"}))
				Expect(contaniner.Env[0].Name).To(Equal("CLOUD"))
				Expect(contaniner.Env[0].Value).To(Equal(string(provider.Spec.Provider)))
				Expect(contaniner.Env[1].Name).To(Equal("CLUSTER"))
				Expect(contaniner.Env[1].Value).To(Equal(provider.Spec.Preload.Cluster))
				Expect(contaniner.Env[2].Name).To(Equal("CONTEXT"))
				Expect(contaniner.Env[2].Value).To(Equal(provider.Spec.Preload.Context))
				Expect(contaniner.Env[3].Name).To(Equal("PROVIDER"))
				Expect(contaniner.Env[3].Value).To(Equal(provider.Name))
				Expect(contaniner.Env[4].Name).To(Equal("KUBE_NAMESPACE"))
				Expect(contaniner.Env[5].Name).To(Equal("REGION"))
				Expect(contaniner.Env[5].Value).To(Equal("eu-west-2"))
			})
		})

		Context("and active job completed successfully", func() {
			BeforeEach(func() {
				job := fixtures.NewCompletedPreloadJob("default", provider.Name)
				Expect(cc.Create(context.Background(), job)).To(Succeed())

				txt := fixtures.NewTerranettesContext(provider.Spec.Preload.Context)
				Expect(cc.Create(context.Background(), txt)).To(Succeed())

				provider.Status.LastPreloadTime = &metav1.Time{Time: time.Now().Add(-10 * time.Minute)}
				Expect(cc.Update(context.Background(), provider)).To(Succeed())

				result, _, rerr = controllertests.Roll(context.TODO(), ctrl, provider, 0)
			})

			It("should not error", func() {
				Expect(rerr).NotTo(HaveOccurred())
				Expect(result.Requeue).To(BeFalse())
			})

			It("should not create a preload job", func() {
				jobs := &batchv1.JobList{}

				Expect(cc.List(context.Background(), jobs)).To(Succeed())
				Expect(jobs.Items).To(HaveLen(1))
			})

			It("should update the conditions", func() {
				Expect(cc.Get(context.Background(), provider.GetNamespacedName(), provider)).ToNot(HaveOccurred())

				cond := provider.GetCommonStatus().GetCondition(terraformv1alpha1.ConditionProviderPreload)
				Expect(cond.Type).To(Equal(terraformv1alpha1.ConditionProviderPreload))
				Expect(cond.Status).To(Equal(metav1.ConditionTrue))
				Expect(cond.Reason).To(Equal(corev1alpha1.ReasonReady))
				Expect(cond.Message).To(Equal("Contextual data successfully loaded"))
			})
		})

		Context("no active jobs, but had a recent job", func() {
			BeforeEach(func() {
				Expect(cc.Delete(context.Background(), provider)).To(Succeed())

				provider.Status.LastPreloadTime = &metav1.Time{Time: time.Now()}
				provider.ResourceVersion = ""

				Expect(cc.Create(context.Background(), provider)).To(Succeed())
				result, _, rerr = controllertests.Roll(context.TODO(), ctrl, provider, 0)
			})

			It("should not error", func() {
				Expect(rerr).NotTo(HaveOccurred())
				Expect(result.Requeue).To(BeFalse())
			})

			It("should not create a preload job", func() {
				jobs := &batchv1.JobList{}

				Expect(cc.List(context.Background(), jobs)).To(Succeed())
				Expect(jobs.Items).To(BeEmpty())
			})
		})

		Context("no active jobs, and no recent job", func() {
			BeforeEach(func() {
				Expect(cc.Delete(context.Background(), provider)).To(Succeed())

				provider.Status.LastPreloadTime = &metav1.Time{Time: time.Now().Add(-time.Hour)}
				provider.Spec.Preload.Interval = &metav1.Duration{Duration: 1 * time.Minute}
				provider.ResourceVersion = ""

				Expect(cc.Create(context.Background(), provider)).To(Succeed())
				result, _, rerr = controllertests.Roll(context.TODO(), ctrl, provider, 0)
			})

			It("should not error", func() {
				Expect(rerr).NotTo(HaveOccurred())
				Expect(result.Requeue).To(BeFalse())
			})

			It("should create a preload job", func() {
				jobs := &batchv1.JobList{}

				Expect(cc.List(context.Background(), jobs)).To(Succeed())
				Expect(jobs.Items).ToNot(BeEmpty())
			})
		})
	})
})
