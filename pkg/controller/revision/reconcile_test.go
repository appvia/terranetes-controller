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

package revision

import (
	"context"
	"io/ioutil"
	"testing"
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
	"github.com/appvia/terranetes-controller/pkg/schema"
	"github.com/appvia/terranetes-controller/pkg/utils/kubernetes"
	controllertests "github.com/appvia/terranetes-controller/test"
	"github.com/appvia/terranetes-controller/test/fixtures"
)

func TestController(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Running Test Suite")
}

var _ = Describe("Revisions Controller", func() {
	logrus.SetOutput(ioutil.Discard)

	var cc client.Client
	var result reconcile.Result
	var rerr error
	var ctrl *Controller
	var revision *terraformv1alpha1.Revision

	BeforeEach(func() {
		cc = fake.NewFakeClientWithScheme(schema.GetScheme())
		recorder := &controllertests.FakeRecorder{}
		ctrl = &Controller{
			cc:       cc,
			recorder: recorder,
		}
	})

	When("a revision is created or updated", func() {
		Context("and the revision does not exist", func() {
			BeforeEach(func() {
				revision = fixtures.NewAWSBucketRevision("test")

				result, _, rerr = controllertests.Roll(context.TODO(), ctrl, revision, 0)
			})

			It("should not return an error", func() {
				Expect(rerr).To(BeNil())
				Expect(result).To(Equal(reconcile.Result{}))
			})
		})
	})

	When("the plan does not exist", func() {
		BeforeEach(func() {
			revision = fixtures.NewAWSBucketRevision("test")
			Expect(cc.Create(context.Background(), revision)).To(Succeed())

			result, _, rerr = controllertests.Roll(context.TODO(), ctrl, revision, 0)
		})

		It("should not return an error", func() {
			Expect(rerr).ToNot(HaveOccurred())
			Expect(result.Requeue).To(BeFalse())
		})

		It("should requeue after a short period", func() {
			Expect(result.RequeueAfter).To(Equal(10 * time.Minute))
		})

		It("should have the correct conditions", func() {
			Expect(cc.Get(context.TODO(), revision.GetNamespacedName(), revision)).To(Succeed())

			cond := revision.GetCommonStatus().GetCondition(corev1alpha1.ConditionReady)
			Expect(cond).ToNot(BeNil())
			Expect(cond.Type).To(Equal(corev1alpha1.ConditionReady))
			Expect(cond.Status).To(Equal(metav1.ConditionTrue))
			Expect(cond.Reason).To(Equal(corev1alpha1.ReasonReady))
			Expect(cond.Message).To(Equal("Resource ready"))
		})

		It("should have created a plan", func() {
			list := &terraformv1alpha1.PlanList{}

			Expect(cc.List(context.Background(), list)).To(Succeed())
			Expect(list.Items).To(HaveLen(1))
			Expect(list.Items[0].Spec.Revisions).ToNot(BeEmpty())
			Expect(list.Items[0].Spec.Revisions).To(HaveLen(1))
			Expect(list.Items[0].HasRevision(revision.Spec.Plan.Revision)).To(BeTrue())
		})
	})

	When("the plan exists", func() {
		BeforeEach(func() {
			revision = fixtures.NewAWSBucketRevision("test")
			Expect(cc.Create(context.Background(), revision)).To(Succeed())
		})

		Context("but the revision is not present", func() {
			BeforeEach(func() {
				plan := fixtures.NewConfigurationPlan(revision.Spec.Plan.Name)
				plan.Spec.Revisions = []terraformv1alpha1.PlanRevision{
					{
						Revision: "v0.0.0",
						Name:     "another",
					},
				}
				Expect(cc.Create(context.Background(), plan)).To(Succeed())

				result, _, rerr = controllertests.Roll(context.TODO(), ctrl, revision, 0)
			})

			It("should not return an error", func() {
				Expect(rerr).ToNot(HaveOccurred())
				Expect(result.Requeue).To(BeFalse())
			})

			It("should have the correct conditions", func() {
				Expect(cc.Get(context.TODO(), revision.GetNamespacedName(), revision)).To(Succeed())

				cond := revision.GetCommonStatus().GetCondition(corev1alpha1.ConditionReady)
				Expect(cond).ToNot(BeNil())
				Expect(cond.Type).To(Equal(corev1alpha1.ConditionReady))
				Expect(cond.Status).To(Equal(metav1.ConditionTrue))
				Expect(cond.Reason).To(Equal(corev1alpha1.ReasonReady))
				Expect(cond.Message).To(Equal("Resource ready"))
			})

			It("should not have created a new plan", func() {
				list := &terraformv1alpha1.PlanList{}

				Expect(cc.List(context.Background(), list)).To(Succeed())
				Expect(list.Items).To(HaveLen(1))
			})

			It("should have updated the existing plan", func() {
				plan := fixtures.NewConfigurationPlan(revision.Spec.Plan.Name)
				Expect(cc.Get(context.Background(), plan.GetNamespacedName(), plan)).To(Succeed())

				Expect(plan.Spec.Revisions).To(HaveLen(2))
				Expect(plan.HasRevision(revision.Spec.Plan.Revision)).To(BeTrue())
				Expect(plan.HasRevision("v0.0.0")).To(BeTrue())
			})
		})

		Context("and the revision is present", func() {
			BeforeEach(func() {
				plan := fixtures.NewConfigurationPlan(revision.Spec.Plan.Name)
				plan.Spec.Revisions = []terraformv1alpha1.PlanRevision{
					{
						Revision: revision.Spec.Plan.Revision,
						Name:     revision.Spec.Plan.Name,
					},
				}
				Expect(cc.Create(context.Background(), plan)).To(Succeed())

				result, _, rerr = controllertests.Roll(context.TODO(), ctrl, revision, 0)
			})

			It("should not return an error", func() {
				Expect(rerr).ToNot(HaveOccurred())
				Expect(result.Requeue).To(BeFalse())
			})

			It("should have the correct conditions", func() {
				Expect(cc.Get(context.TODO(), revision.GetNamespacedName(), revision)).To(Succeed())

				cond := revision.GetCommonStatus().GetCondition(corev1alpha1.ConditionReady)
				Expect(cond).ToNot(BeNil())
				Expect(cond.Type).To(Equal(corev1alpha1.ConditionReady))
				Expect(cond.Status).To(Equal(metav1.ConditionTrue))
				Expect(cond.Reason).To(Equal(corev1alpha1.ReasonReady))
				Expect(cond.Message).To(Equal("Resource ready"))
			})

			It("should not have created a new plan", func() {
				list := &terraformv1alpha1.PlanList{}

				Expect(cc.List(context.Background(), list)).To(Succeed())
				Expect(list.Items).To(HaveLen(1))
			})

			It("should not have add the revision to the plan", func() {
				plan := fixtures.NewConfigurationPlan(revision.Spec.Plan.Name)
				Expect(cc.Get(context.Background(), plan.GetNamespacedName(), plan)).To(Succeed())

				Expect(plan.Spec.Revisions).To(HaveLen(1))
				Expect(plan.HasRevision(revision.Spec.Plan.Revision)).To(BeTrue())
			})
		})

		Context("and the revision is in use by a cloud resource", func() {
			BeforeEach(func() {
				cloudresource := fixtures.NewCloudResource("default", "test")
				cloudresource.Labels = map[string]string{
					terraformv1alpha1.CloudResourcePlanNameLabel: revision.Spec.Plan.Name,
					terraformv1alpha1.CloudResourceRevisionLabel: revision.Spec.Plan.Revision,
				}
				cloudresource.Spec.Plan.Name = revision.Spec.Plan.Name
				cloudresource.Spec.Plan.Revision = revision.Spec.Plan.Revision
				Expect(cc.Create(context.Background(), cloudresource)).To(Succeed())

				plan := fixtures.NewConfigurationPlan(revision.Spec.Plan.Name)
				plan.Spec.Revisions = []terraformv1alpha1.PlanRevision{
					{
						Revision: revision.Spec.Plan.Revision,
						Name:     revision.Spec.Plan.Name,
					},
				}
				Expect(cc.Create(context.Background(), plan)).To(Succeed())

				result, _, rerr = controllertests.Roll(context.TODO(), ctrl, revision, 0)
			})

			It("should return an error", func() {
				Expect(rerr).ToNot(HaveOccurred())
				Expect(result.Requeue).To(BeFalse())
			})

			It("should update the in use count on the status", func() {
				Expect(cc.Get(context.Background(), revision.GetNamespacedName(), revision)).To(Succeed())
				Expect(revision.Status.InUse).To(Equal(1))
			})
		})
	})

	When("a revision is deleted", func() {
		var plan *terraformv1alpha1.Plan

		BeforeEach(func() {
			revision = fixtures.NewAWSBucketRevision("test")
			revision.Finalizers = []string{controllerName}
			revision.DeletionTimestamp = &metav1.Time{Time: time.Now()}

			Expect(cc.Create(context.Background(), revision)).To(Succeed())

			plan = fixtures.NewConfigurationPlan(revision.Spec.Plan.Name)
			plan.Spec.Revisions = []terraformv1alpha1.PlanRevision{
				{
					Revision: "0.0.0",
					Name:     revision.Spec.Plan.Name,
				},
				{
					Revision: revision.Spec.Plan.Revision,
					Name:     revision.Spec.Plan.Name,
				},
			}
			Expect(cc.Create(context.Background(), plan)).To(Succeed())
		})

		Context("but no plan exists", func() {
			BeforeEach(func() {
				Expect(cc.Delete(context.Background(), fixtures.NewConfigurationPlan(revision.Spec.Plan.Name))).To(Succeed())

				result, _, rerr = controllertests.Roll(context.TODO(), ctrl, revision, 0)
			})

			It("should not return an error", func() {
				Expect(rerr).ToNot(HaveOccurred())
				Expect(result.Requeue).To(BeFalse())
			})

			It("should no longer exist", func() {
				found, err := kubernetes.GetIfExists(context.Background(), cc, revision)
				Expect(err).ToNot(HaveOccurred())
				Expect(found).To(BeFalse())
			})
		})

		Context("and the plan exists but revision is not present", func() {
			BeforeEach(func() {
				plan.Spec.Revisions = []terraformv1alpha1.PlanRevision{
					{
						Revision: "0.0.0",
						Name:     revision.Spec.Plan.Name,
					},
				}
				Expect(cc.Update(context.Background(), plan)).To(Succeed())

				result, _, rerr = controllertests.Roll(context.Background(), ctrl, revision, 0)
			})

			It("should not return an error", func() {
				Expect(rerr).ToNot(HaveOccurred())
				Expect(result.Requeue).To(BeFalse())
			})

			It("should have removed the revision from the plan", func() {
				Expect(cc.Get(context.Background(), plan.GetNamespacedName(), plan)).To(Succeed())
				Expect(plan.Spec.Revisions).To(HaveLen(1))
				Expect(plan.HasRevision("0.0.0")).To(BeTrue())
			})

			It("should no longer exist", func() {
				found, err := kubernetes.GetIfExists(context.Background(), cc, revision)
				Expect(err).ToNot(HaveOccurred())
				Expect(found).To(BeFalse())
			})
		})

		Context("and the plan exists", func() {
			BeforeEach(func() {
				result, _, rerr = controllertests.Roll(context.Background(), ctrl, revision, 0)
			})

			It("should not return an error", func() {
				Expect(rerr).ToNot(HaveOccurred())
				Expect(result.Requeue).To(BeFalse())
			})

			It("should have removed the revision from the plan", func() {
				Expect(cc.Get(context.Background(), plan.GetNamespacedName(), plan)).To(Succeed())
				Expect(plan.Spec.Revisions).To(HaveLen(1))
				Expect(plan.HasRevision(revision.Spec.Plan.Revision)).To(BeFalse())
				Expect(plan.HasRevision("0.0.0")).To(BeTrue())
			})

			It("should no longer exist", func() {
				found, err := kubernetes.GetIfExists(context.Background(), cc, revision)
				Expect(err).ToNot(HaveOccurred())
				Expect(found).To(BeFalse())
			})
		})
	})
})
