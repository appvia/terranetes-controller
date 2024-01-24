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

package expire

import (
	"context"
	"io"
	"testing"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/sirupsen/logrus"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

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

var _ = Describe("Revision Expiration Controller", func() {
	logrus.SetOutput(io.Discard)

	var cc client.Client
	var result reconcile.Result
	var recorder *controllertests.FakeRecorder
	var rerr error
	var ctrl *Controller
	var revision *terraformv1alpha1.Revision

	BeforeEach(func() {
		cc = fake.NewClientBuilder().
			WithScheme(schema.GetScheme()).
			WithStatusSubresource(&terraformv1alpha1.Revision{}).
			Build()

		recorder = &controllertests.FakeRecorder{}
		ctrl = &Controller{
			cc:       cc,
			recorder: recorder,
		}
	})

	When("reconcilation a revision", func() {
		BeforeEach(func() {
			revision = fixtures.NewAWSBucketRevisionAtVersion("first", "0.0.1")
			revision.Finalizers = []string{"test"}

			plan := fixtures.NewPlan(revision.Spec.Plan.Name, revision)
			cs := fixtures.NewCloudResourceWithRevision(revision.Namespace, "test", revision)

			Expect(cc.Create(context.Background(), plan)).To(Succeed())
			Expect(cc.Create(context.Background(), revision)).To(Succeed())
			Expect(cc.Create(context.Background(), cs)).To(Succeed())
		})

		Context("but we are the only revision", func() {
			BeforeEach(func() {
				result, _, rerr = controllertests.Roll(context.Background(), ctrl, revision, 0)
			})

			It("should ignore the resource", func() {
				Expect(rerr).ToNot(HaveOccurred())
				Expect(result.Requeue).To(BeFalse())
				Expect(result.RequeueAfter).To(Equal(60 * time.Minute))
			})

			It("should not delete the revision", func() {
				found, err := kubernetes.GetIfExists(context.Background(), cc, revision)
				Expect(err).ToNot(HaveOccurred())
				Expect(found).To(BeTrue())
				Expect(revision.DeletionTimestamp).To(BeNil())
			})
		})

		Context("but we are deleting", func() {
			BeforeEach(func() {
				Expect(cc.Delete(context.Background(), revision)).To(Succeed())

				result, _, rerr = controllertests.Roll(context.Background(), ctrl, revision, 0)
			})

			It("should ignore the resource", func() {
				Expect(rerr).ToNot(HaveOccurred())
				Expect(result.Requeue).To(BeFalse())
				Expect(result.RequeueAfter).To(Equal(60 * time.Minute))
			})
		})

		Context("and we have additional revisions", func() {
			BeforeEach(func() {
				additional := fixtures.NewAWSBucketRevisionAtVersion("additional", "0.0.2")
				additional.Finalizers = []string{"test"}

				Expect(cc.Create(context.Background(), additional)).To(Succeed())
			})

			Context("but we have not reached the expiration deadline", func() {
				BeforeEach(func() {
					ctrl.RevisionExpiration = 60 * time.Minute

					result, _, rerr = controllertests.Roll(context.Background(), ctrl, revision, 0)
				})

				It("should ignore the resource", func() {
					Expect(rerr).ToNot(HaveOccurred())
					Expect(result.Requeue).To(BeFalse())
					Expect(result.RequeueAfter).To(Equal(60 * time.Minute))
				})

				It("should have two items in the revision list", func() {
					list := &terraformv1alpha1.RevisionList{}
					Expect(cc.List(context.Background(), list)).To(Succeed())
					Expect(len(list.Items)).To(Equal(2))
				})

				It("should not delete the revision", func() {
					found, err := kubernetes.GetIfExists(context.Background(), cc, revision)
					Expect(err).ToNot(HaveOccurred())
					Expect(found).To(BeTrue())
					Expect(revision.DeletionTimestamp).To(BeNil())
				})
			})

			Context("and we have reached the expiration deadline", func() {
				BeforeEach(func() {
					ctrl.RevisionExpiration = -60 * time.Minute
				})

				Context("but we are the latest revision", func() {
					BeforeEach(func() {
						revision.Spec.Plan.Revision = "1.0.0"
						Expect(cc.Update(context.Background(), revision)).To(Succeed())

						result, _, rerr = controllertests.Roll(context.Background(), ctrl, revision, 0)
					})

					It("should ignore the resource", func() {
						Expect(rerr).ToNot(HaveOccurred())
						Expect(result.Requeue).To(BeFalse())
						Expect(result.RequeueAfter).To(Equal(60 * time.Minute))
					})

					It("should have two items in the revision list", func() {
						list := &terraformv1alpha1.RevisionList{}
						Expect(cc.List(context.Background(), list)).To(Succeed())
						Expect(len(list.Items)).To(Equal(2))
					})

					It("should not delete the revision", func() {
						found, err := kubernetes.GetIfExists(context.Background(), cc, revision)
						Expect(err).ToNot(HaveOccurred())
						Expect(found).To(BeTrue())
						Expect(revision.DeletionTimestamp).To(BeNil())
					})
				})

				Context("and we are not the latest revision", func() {
					Context("but we have cloud resources using us", func() {
						BeforeEach(func() {
							result, _, rerr = controllertests.Roll(context.Background(), ctrl, revision, 0)
						})

						It("should ignore the resource", func() {
							Expect(rerr).ToNot(HaveOccurred())
							Expect(result.Requeue).To(BeFalse())
							Expect(result.RequeueAfter).To(Equal(60 * time.Minute))
						})

						It("should have two items in the revision list", func() {
							list := &terraformv1alpha1.RevisionList{}
							Expect(cc.List(context.Background(), list)).To(Succeed())
							Expect(len(list.Items)).To(Equal(2))
						})

						It("should not delete the revision", func() {
							found, err := kubernetes.GetIfExists(context.Background(), cc, revision)
							Expect(err).ToNot(HaveOccurred())
							Expect(found).To(BeTrue())
							Expect(revision.DeletionTimestamp).To(BeNil())
						})
					})

					Context("and we have no cloud resources using us", func() {
						BeforeEach(func() {
							cs := fixtures.NewCloudResourceWithRevision(revision.Namespace, "test", revision)
							Expect(cc.Delete(context.Background(), cs)).To(Succeed())

							result, _, rerr = controllertests.Roll(context.Background(), ctrl, revision, 0)
						})

						It("should not error", func() {
							Expect(rerr).ToNot(HaveOccurred())
							Expect(result.Requeue).To(BeFalse())
							Expect(result.RequeueAfter).To(Equal(60 * time.Minute))
						})

						It("should delete the revision", func() {
							found, err := kubernetes.GetIfExists(context.Background(), cc, revision)
							Expect(err).ToNot(HaveOccurred())
							Expect(found).To(BeTrue())
							Expect(revision.DeletionTimestamp).ToNot(BeNil())
						})

						It("should have raised an event", func() {
							Expect(recorder.Events).ToNot(BeEmpty())
							Expect(recorder.Events[0]).To(Equal("(/first) Normal ExpiringRevision: Expiring the revision 0.0.1"))
						})
					})
				})
			})
		})
	})
})
