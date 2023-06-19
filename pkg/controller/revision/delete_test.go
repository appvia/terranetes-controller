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
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/sirupsen/logrus"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	terraformv1alpha1 "github.com/appvia/terranetes-controller/pkg/apis/terraform/v1alpha1"
	"github.com/appvia/terranetes-controller/pkg/schema"
	controllertests "github.com/appvia/terranetes-controller/test"
	"github.com/appvia/terranetes-controller/test/fixtures"
)

var _ = Describe("Deleting a Revision", func() {
	logrus.SetOutput(ioutil.Discard)

	var cc client.Client
	var result reconcile.Result
	var rerr error
	var ctrl *Controller
	var revision *terraformv1alpha1.Revision
	var plan *terraformv1alpha1.Plan
	var recorder *controllertests.FakeRecorder

	BeforeEach(func() {
		cc = fake.NewFakeClientWithScheme(schema.GetScheme())
		recorder = &controllertests.FakeRecorder{}
		ctrl = &Controller{
			cc:       cc,
			recorder: recorder,
		}
		revision = fixtures.NewAWSBucketRevision("test")
		revision.DeletionTimestamp = &metav1.Time{Time: time.Now()}
		plan = fixtures.NewPlan(revision.Spec.Plan.Name, revision)

		Expect(cc.Create(context.Background(), plan)).To(Succeed())
		Expect(cc.Create(context.Background(), revision)).To(Succeed())
	})

	When("deleting a revision", func() {
		CommonChecks := func() {
			It("should not return an error", func() {
				Expect(rerr).To(BeNil())
				Expect(result.Requeue).To(BeFalse())
				Expect(result.RequeueAfter).To(BeZero())
			})

			It("should delete the revision", func() {
				Expect(cc.Get(context.TODO(), revision.GetNamespacedName(), revision)).To(HaveOccurred())
			})
		}

		Context("and the plan does not exist", func() {
			BeforeEach(func() {
				Expect(cc.Delete(context.Background(), plan)).To(Succeed())

				result, _, rerr = controllertests.Roll(context.TODO(), ctrl, revision, 0)
			})

			CommonChecks()

			It("should have raised a event", func() {
				Expect(recorder.Events).To(HaveLen(1))
				Expect(recorder.Events).To(ContainElement(
					"(/test) Normal PlanNotFound: Plan associated to revision: bucket not found",
				))
			})
		})

		Context("and the revision does not exist in the plan", func() {
			BeforeEach(func() {
				plan.Spec.Revisions = nil
				Expect(cc.Update(context.Background(), plan)).To(Succeed())

				result, _, rerr = controllertests.Roll(context.TODO(), ctrl, revision, 0)
			})

			CommonChecks()

			It("should have raised a event", func() {
				Expect(recorder.Events).To(ContainElement(
					"(/test) Normal RevisionRemoved: Revision: bucket removed from plan: bucket",
				))
			})
		})

		Context("and the revision exists", func() {
			BeforeEach(func() {
				result, _, rerr = controllertests.Roll(context.TODO(), ctrl, revision, 0)
			})

			CommonChecks()

			It("should have raised a event", func() {
				Expect(recorder.Events).To(ContainElement(
					"(/test) Normal RevisionRemoved: Revision: bucket removed from plan: bucket",
				))
			})
		})
	})
})
