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

package cloudresource

import (
	"context"
	"io"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/sirupsen/logrus"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	corev1alpha1 "github.com/appvia/terranetes-controller/pkg/apis/core/v1alpha1"
	terraformv1alpha1 "github.com/appvia/terranetes-controller/pkg/apis/terraform/v1alpha1"
	"github.com/appvia/terranetes-controller/pkg/controller"
	"github.com/appvia/terranetes-controller/pkg/schema"
	controllertests "github.com/appvia/terranetes-controller/test"
	"github.com/appvia/terranetes-controller/test/fixtures"
)

var _ = Describe("CloudResource Deletion", func() {
	logrus.SetOutput(io.Discard)

	var cc client.Client
	var result reconcile.Result
	var rerr error
	var ctrl *Controller
	var cloudresource *terraformv1alpha1.CloudResource
	var recorder *controllertests.FakeRecorder
	var plan *terraformv1alpha1.Plan
	var revision *terraformv1alpha1.Revision
	var configuration *terraformv1alpha1.Configuration

	BeforeEach(func() {
		cc = fake.NewClientBuilder().
			WithScheme(schema.GetScheme()).
			WithStatusSubresource(&terraformv1alpha1.CloudResource{}).
			WithStatusSubresource(&terraformv1alpha1.Configuration{}).
			Build()
		recorder = &controllertests.FakeRecorder{}
		ctrl = &Controller{
			cc:       cc,
			recorder: recorder,
		}

		revision = fixtures.NewAWSBucketRevision("revision.v1")
		revision.Spec.Plan.Revision = "v0.0.1"
		plan = fixtures.NewPlan("plan.v1", revision)
		revision.Spec.Plan.Name = plan.Name

		cloudresource = fixtures.NewCloudResourceWithRevision("default", "database", revision)
		cloudresource.Finalizers = []string{controllerName}
		cloudresource.Spec.Variables = nil

		configuration = terraformv1alpha1.NewConfiguration(cloudresource.Namespace, "database-1111")
		configuration.Finalizers = []string{controllerName}
		controller.EnsureConditionsRegistered(terraformv1alpha1.DefaultConfigurationConditions, configuration)
		cloudresource.Status.ConfigurationName = configuration.Name

		Expect(cc.Create(context.Background(), revision)).To(Succeed())
		Expect(cc.Create(context.Background(), plan)).To(Succeed())
		Expect(cc.Create(context.Background(), configuration)).To(Succeed())
		Expect(cc.Create(context.Background(), cloudresource)).To(Succeed())
		Expect(cc.Delete(context.Background(), cloudresource)).To(Succeed())
	})

	When("cloudresource is deleted", func() {
		Context("configuration exists", func() {
			BeforeEach(func() {
				// @note: we only want one reconcilation here
				result, rerr = ctrl.Reconcile(context.TODO(), reconcile.Request{
					NamespacedName: client.ObjectKey{
						Name:      cloudresource.Name,
						Namespace: cloudresource.Namespace,
					},
				})
			})

			It("should not error", func() {
				Expect(rerr).ToNot(HaveOccurred())
			})

			It("should indicate is deleting", func() {
				Expect(cc.Get(context.TODO(), cloudresource.GetNamespacedName(), cloudresource)).To(Succeed())
				Expect(cloudresource.Status.ResourceStatus).To(Equal(terraformv1alpha1.DestroyingResources))
			})

			It("should requeue", func() {
				Expect(result.Requeue).To(BeFalse())
				Expect(result.RequeueAfter).To(Equal(5 * time.Second))
			})

			It("should have triggered a delete on the configuration", func() {
				Expect(cc.Get(context.TODO(), configuration.GetNamespacedName(), configuration)).To(Succeed())
				Expect(configuration.DeletionTimestamp).ToNot(BeNil())
			})

			It("should indicate the status on the condition", func() {
				Expect(cc.Get(context.TODO(), cloudresource.GetNamespacedName(), cloudresource)).To(Succeed())

				cond := cloudresource.Status.GetCondition(corev1alpha1.ConditionReady)
				Expect(cond).ToNot(BeNil())
				Expect(cond.Type).To(Equal(corev1alpha1.ConditionReady))
				Expect(cond.Status).To(Equal(metav1.ConditionFalse))
				Expect(cond.Reason).To(Equal(corev1alpha1.ReasonDeleting))
				Expect(cond.Message).To(Equal("Waiting for the configuration to be deleted"))
			})
		})

		Context("and the configuration is deleting", func() {
			BeforeEach(func() {
				Expect(cc.Delete(context.Background(), configuration)).To(Succeed())
				Expect(cc.Get(context.Background(), configuration.GetNamespacedName(), configuration)).To(Succeed())
			})

			Context("configuration status has been updated", func() {
				BeforeEach(func() {
					cond := configuration.Status.GetCondition(corev1alpha1.ConditionReady)
					cond.Status = metav1.ConditionFalse
					cond.Message = "This should be updated on the cloudresource"
					Expect(cc.Status().Update(context.Background(), configuration)).To(Succeed())

					result, _, rerr = controllertests.Roll(context.TODO(), ctrl, cloudresource, 0)
				})

				It("should not error", func() {
					Expect(rerr).ToNot(HaveOccurred())
				})

				It("should requeue", func() {
					Expect(result.Requeue).To(BeFalse())
					Expect(result.RequeueAfter).To(Equal(5 * time.Second))
				})

				It("should not have deleted the cloudresource", func() {
					Expect(cc.Get(context.TODO(), configuration.GetNamespacedName(), configuration)).To(Succeed())
				})

				It("should have updated the cloudresource status", func() {
					Expect(cc.Get(context.Background(), cloudresource.GetNamespacedName(), cloudresource)).To(Succeed())
					Expect(cloudresource.Status.ResourceStatus).To(Equal(terraformv1alpha1.DestroyingResources))
					Expect(cloudresource.Status.GetCondition(corev1alpha1.ConditionReady).Message).To(Equal("This should be updated on the cloudresource"))
				})
			})

			Context("configuration has failed to delete", func() {
				BeforeEach(func() {
					configuration.Status.ResourceStatus = terraformv1alpha1.DestroyingResourcesFailed
					Expect(cc.Status().Update(context.Background(), configuration)).To(Succeed())

					result, _, rerr = controllertests.Roll(context.TODO(), ctrl, cloudresource, 0)
				})

				It("should not error", func() {
					Expect(rerr).ToNot(HaveOccurred())
				})

				It("should not requeue", func() {
					Expect(result.Requeue).To(BeFalse())
					Expect(result.RequeueAfter).To(Equal(0 * time.Second))
				})

				It("should not have deleted the cloudresource", func() {
					Expect(cc.Get(context.Background(), cloudresource.GetNamespacedName(), cloudresource)).To(Succeed())
				})

				It("should have updated the cloudresource conditions", func() {
					Expect(cc.Get(context.TODO(), cloudresource.GetNamespacedName(), cloudresource)).To(Succeed())

					cond := cloudresource.Status.GetCondition(corev1alpha1.ConditionReady)
					Expect(cond).ToNot(BeNil())
					Expect(cond.Type).To(Equal(corev1alpha1.ConditionReady))
					Expect(cond.Status).To(Equal(metav1.ConditionFalse))
					Expect(cond.Reason).To(Equal(corev1alpha1.ReasonActionRequired))
					Expect(cond.Message).To(Equal("Failed to delete CloudResource, please check Configuration status"))
				})
			})

			Context("configuration has deleted", func() {
				BeforeEach(func() {
					configuration.Finalizers = nil
					Expect(cc.Update(context.Background(), configuration)).To(Succeed())

					result, _, rerr = controllertests.Roll(context.TODO(), ctrl, cloudresource, 0)
				})

				It("should not error", func() {
					Expect(rerr).ToNot(HaveOccurred())
				})

				It("should have deleted the configuration", func() {
					Expect(cc.Get(context.Background(), configuration.GetNamespacedName(), configuration)).ToNot(Succeed())
				})

				It("should have deleted the cloudresource", func() {
					Expect(cc.Get(context.Background(), cloudresource.GetNamespacedName(), cloudresource)).ToNot(Succeed())
				})

				It("should have raised an event", func() {
					Expect(recorder.Events).To(HaveLen(1))
					Expect(recorder.Events).To(ContainElement(
						"(default/database) Normal Deleted: The configuration has been deleted",
					))
				})
			})
		})
	})
})
