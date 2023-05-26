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

package plan

import (
	"context"
	"io/ioutil"
	"testing"

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

func TestReconcile(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Running Test Suite")
}

var _ = Describe("Plan Controller", func() {
	logrus.SetOutput(ioutil.Discard)

	var cc client.Client
	var result reconcile.Result
	var recorder *controllertests.FakeRecorder
	var rerr error
	var ctrl *Controller
	var plan *terraformv1alpha1.Plan

	BeforeEach(func() {
		cc = fake.NewClientBuilder().
			WithScheme(schema.GetScheme()).
			WithStatusSubresource(&terraformv1alpha1.Plan{}).
			Build()

		recorder = &controllertests.FakeRecorder{}
		ctrl = &Controller{
			cc:       cc,
			recorder: recorder,
		}
	})

	When("a plan is created", func() {
		BeforeEach(func() {
			plan = fixtures.NewConfigurationPlan("test")
			plan.Spec.Revisions = []terraformv1alpha1.PlanRevision{
				{
					Name:     "test",
					Revision: "0.0.1",
				},
				{
					Name:     "test-1",
					Revision: "0.0.2",
				},
			}
			Expect(cc.Create(context.Background(), plan)).To(Succeed())
		})

		Context("and we no longer have any revisions", func() {
			BeforeEach(func() {
				plan.Spec.Revisions = nil
				Expect(cc.Update(context.Background(), plan)).To(Succeed())

				result, _, rerr = controllertests.Roll(context.TODO(), ctrl, plan, 0)
			})

			It("should not error", func() {
				Expect(rerr).ToNot(HaveOccurred())
				Expect(result.Requeue).To(BeFalse())
				Expect(result.RequeueAfter).To(BeZero())
			})

			It("should delete the plan", func() {
				found, err := kubernetes.GetIfExists(context.Background(), cc, plan)
				Expect(err).ToNot(HaveOccurred())
				Expect(found).To(BeTrue())
				Expect(plan.DeletionTimestamp).ToNot(BeNil())
			})
		})

		Context("and the revision semver is invalid", func() {
			BeforeEach(func() {
				plan.Spec.Revisions = []terraformv1alpha1.PlanRevision{
					{
						Name:     "test",
						Revision: "BAD",
					},
				}
				Expect(cc.Update(context.Background(), plan)).To(Succeed())

				result, _, rerr = controllertests.Roll(context.Background(), ctrl, plan, 0)
			})

			It("should not error", func() {
				Expect(rerr).ToNot(HaveOccurred())
				Expect(result.Requeue).To(BeFalse())
				Expect(result.RequeueAfter).To(BeZero())
			})

			It("should have conditions", func() {
				Expect(cc.Get(context.TODO(), plan.GetNamespacedName(), plan)).To(Succeed())

				cond := plan.GetCommonStatus().GetCondition(corev1alpha1.ConditionReady)
				Expect(cond).ToNot(BeNil())
				Expect(cond.Type).To(Equal(corev1alpha1.ConditionReady))
				Expect(cond.Status).To(Equal(metav1.ConditionFalse))
				Expect(cond.Reason).To(Equal(corev1alpha1.ReasonActionRequired))
				Expect(cond.Message).To(Equal("Failed to sort the revision in plan: test, error: Invalid Semantic Version"))
			})

			It("should not delete the plan", func() {
				found, err := kubernetes.GetIfExists(context.Background(), cc, plan)
				Expect(err).ToNot(HaveOccurred())
				Expect(found).To(BeTrue())
			})
		})

		Context("and we have a plan and revisions", func() {
			BeforeEach(func() {
				result, _, rerr = controllertests.Roll(context.Background(), ctrl, plan, 0)
			})

			It("should not error", func() {
				Expect(rerr).ToNot(HaveOccurred())
				Expect(result.Requeue).To(BeFalse())
				Expect(result.RequeueAfter).To(BeZero())
			})

			It("should have conditions", func() {
				Expect(cc.Get(context.TODO(), plan.GetNamespacedName(), plan)).To(Succeed())

				cond := plan.GetCommonStatus().GetCondition(corev1alpha1.ConditionReady)
				Expect(cond).ToNot(BeNil())
				Expect(cond.Type).To(Equal(corev1alpha1.ConditionReady))
				Expect(cond.Status).To(Equal(metav1.ConditionTrue))
				Expect(cond.Reason).To(Equal(corev1alpha1.ReasonReady))
				Expect(cond.Message).To(Equal("Resource ready"))
			})

			It("should not delete the plan", func() {
				found, err := kubernetes.GetIfExists(context.Background(), cc, plan)
				Expect(err).ToNot(HaveOccurred())
				Expect(found).To(BeTrue())
			})

			It("should have updates the plan status", func() {
				found, err := kubernetes.GetIfExists(context.Background(), cc, plan)
				Expect(err).ToNot(HaveOccurred())
				Expect(found).To(BeTrue())
				Expect(plan.Status.Latest.Revision).To(Equal("0.0.2"))
				Expect(plan.Status.Latest.Name).To(Equal("test-1"))
			})
		})

		Context("and the plan has no more revisions", func() {
			BeforeEach(func() {
				plan.Spec.Revisions = []terraformv1alpha1.PlanRevision{}
				Expect(cc.Update(context.Background(), plan)).To(Succeed())

				result, _, rerr = controllertests.Roll(context.Background(), ctrl, plan, 0)
			})

			It("should not error", func() {
				Expect(rerr).ToNot(HaveOccurred())
				Expect(result.Requeue).To(BeFalse())
				Expect(result.RequeueAfter).To(BeZero())
			})

			It("should delete the plan", func() {
				Expect(cc.Get(context.TODO(), plan.GetNamespacedName(), plan)).ToNot(HaveOccurred())
				Expect(plan.DeletionTimestamp).ToNot(BeNil())
			})

			It("should raise an event", func() {
				Expect(recorder.Events).To(HaveLen(1))
				Expect(recorder.Events).To(ContainElement(
					"(/test) Normal DeletedPlan: Plan has been deleted",
				))
			})
		})
	})
})
