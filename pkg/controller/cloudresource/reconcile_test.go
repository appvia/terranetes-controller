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
	"testing"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/sirupsen/logrus"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/utils/ptr"
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

func TestController(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Running Test Suite")
}

var _ = Describe("CloudResource Reconcilation", func() {
	logrus.SetOutput(io.Discard)

	var cc client.Client
	var result reconcile.Result
	var rerr error
	var ctrl *Controller
	var cloudresource *terraformv1alpha1.CloudResource
	var recorder *controllertests.FakeRecorder
	var plan *terraformv1alpha1.Plan
	var revision *terraformv1alpha1.Revision

	BeforeEach(func() {
		cc = fake.NewClientBuilder().
			WithScheme(schema.GetScheme()).
			WithStatusSubresource(&terraformv1alpha1.CloudResource{}).
			Build()
		recorder = &controllertests.FakeRecorder{}
		ctrl = &Controller{
			cc:       cc,
			recorder: recorder,
		}

		revision = fixtures.NewAWSBucketRevision("revision.v1")
		revision.Spec.Plan.Revision = "v0.0.1"
		revision.Spec.Inputs = []terraformv1alpha1.RevisionInput{
			{
				Key:         "database_name",
				Required:    ptr.To(true),
				Description: "The name of the database",
				Default: &runtime.RawExtension{
					Raw: []byte(`{"value":"mydb"}`),
				},
			},
			{
				Key:         "database_size",
				Required:    ptr.To(true),
				Description: "The name of the database",
				Default: &runtime.RawExtension{
					Raw: []byte(`{"value": 5}`),
				},
			},
			{
				Key:         "database_engine",
				Required:    ptr.To(true),
				Description: "The name of the database engine",
			},
		}
		revision.Spec.Configuration.Module = "git::https://github.com/appvia/terranetes-controller.git?ref=master"
		revision.Spec.Configuration.Variables = &runtime.RawExtension{
			Raw: []byte("{\"test\": \"default\"}"),
		}
		plan = fixtures.NewPlan("plan.v1", revision)
		revision.Spec.Plan.Name = plan.Name
		cloudresource = fixtures.NewCloudResourceWithRevision("default", "database", revision)
		cloudresource.Spec.Variables = nil
		cloudresource.Spec.WriteConnectionSecretToRef = &terraformv1alpha1.WriteConnectionSecret{
			Name: "mysecret",
		}

		Expect(cc.Create(context.Background(), revision)).To(Succeed())
		Expect(cc.Create(context.Background(), plan)).To(Succeed())
		Expect(cc.Create(context.Background(), cloudresource)).To(Succeed())
	})

	When("reconciling a cloud resource", func() {
		Context("and the plan is missing", func() {
			BeforeEach(func() {
				Expect(cc.Delete(context.Background(), plan)).To(Succeed())

				result, _, rerr = controllertests.Roll(context.TODO(), ctrl, cloudresource, 0)
			})

			It("should not return an error", func() {
				Expect(rerr).ToNot(HaveOccurred())
			})

			It("should indicate on the conditions", func() {
				Expect(cc.Get(context.TODO(), cloudresource.GetNamespacedName(), cloudresource)).To(Succeed())

				cond := cloudresource.GetCommonStatus().GetCondition(corev1alpha1.ConditionReady)
				Expect(cond).ToNot(BeNil())
				Expect(cond.Type).To(Equal(corev1alpha1.ConditionReady))
				Expect(cond.Status).To(Equal(metav1.ConditionFalse))
				Expect(cond.Reason).To(Equal(corev1alpha1.ReasonActionRequired))
				Expect(cond.Message).To(Equal("Cloud resource plan \"plan.v1\" does not exist"))
			})

			It("should requeue", func() {
				Expect(result.Requeue).To(BeFalse())
				Expect(result.RequeueAfter).To(Equal(5 * time.Minute))
			})
		})

		Context("and the revision does not exist in plan", func() {
			BeforeEach(func() {
				Expect(cc.Get(context.Background(), plan.GetNamespacedName(), plan)).To(Succeed())
				plan.Spec.Revisions[0].Revision = "v1.0.0-not-there"
				Expect(cc.Update(context.Background(), plan)).To(Succeed())

				result, _, rerr = controllertests.Roll(context.TODO(), ctrl, cloudresource, 0)
			})

			It("should not return an error", func() {
				Expect(rerr).ToNot(HaveOccurred())
			})

			It("should indicate on the conditions", func() {
				Expect(cc.Get(context.TODO(), cloudresource.GetNamespacedName(), cloudresource)).To(Succeed())

				cond := cloudresource.GetCommonStatus().GetCondition(corev1alpha1.ConditionReady)
				Expect(cond).ToNot(BeNil())
				Expect(cond.Type).To(Equal(corev1alpha1.ConditionReady))
				Expect(cond.Status).To(Equal(metav1.ConditionFalse))
				Expect(cond.Reason).To(Equal(corev1alpha1.ReasonActionRequired))
				Expect(cond.Message).To(Equal("Revision: \"v0.0.1\" does not exist in plan (spec.plan.revision)"))
			})

			It("should requeue", func() {
				Expect(result.Requeue).To(BeFalse())
				Expect(result.RequeueAfter).To(Equal(5 * time.Minute))
			})
		})

		Context("and the revision is missing", func() {
			BeforeEach(func() {
				Expect(cc.Delete(context.Background(), revision)).To(Succeed())

				result, _, rerr = controllertests.Roll(context.TODO(), ctrl, cloudresource, 0)
			})

			It("should not return an error", func() {
				Expect(rerr).ToNot(HaveOccurred())
			})

			It("should indicate on the conditions", func() {
				Expect(cc.Get(context.TODO(), cloudresource.GetNamespacedName(), cloudresource)).To(Succeed())

				cond := cloudresource.GetCommonStatus().GetCondition(corev1alpha1.ConditionReady)
				Expect(cond).ToNot(BeNil())
				Expect(cond.Type).To(Equal(corev1alpha1.ConditionReady))
				Expect(cond.Status).To(Equal(metav1.ConditionFalse))
				Expect(cond.Reason).To(Equal(corev1alpha1.ReasonActionRequired))
				Expect(cond.Message).To(Equal("Revision: \"v0.0.1\" does not exist or has been removed"))
			})

			It("should requeue", func() {
				Expect(result.Requeue).To(BeFalse())
				Expect(result.RequeueAfter).To(Equal(5 * time.Minute))
			})
		})

		Context("and both plan and revision exist", func() {
			Context("but the configuration does not exist", func() {
				Context("and no overrides are provided", func() {
					BeforeEach(func() {
						result, _, rerr = controllertests.Roll(context.TODO(), ctrl, cloudresource, 0)
					})

					It("should not return an error", func() {
						Expect(rerr).ToNot(HaveOccurred())
					})

					It("should indicate on the conditions", func() {
						Expect(cc.Get(context.TODO(), cloudresource.GetNamespacedName(), cloudresource)).To(Succeed())

						cond := cloudresource.GetCommonStatus().GetCondition(terraformv1alpha1.ConditionConfigurationReady)
						Expect(cond).ToNot(BeNil())
						Expect(cond.Type).To(Equal(terraformv1alpha1.ConditionConfigurationReady))
						Expect(cond.Status).To(Equal(metav1.ConditionTrue))
						Expect(cond.Reason).To(Equal(corev1alpha1.ReasonReady))
						Expect(cond.Message).To(Equal("Provisioned the Configuration"))
					})

					It("should have created a configuration", func() {
						list := &terraformv1alpha1.ConfigurationList{}
						Expect(cc.List(context.Background(), list, client.MatchingLabels(map[string]string{
							terraformv1alpha1.CloudResourceNameLabel:         cloudresource.Name,
							terraformv1alpha1.CloudResourcePlanNameLabel:     revision.Spec.Plan.Name,
							terraformv1alpha1.CloudResourceRevisionLabel:     revision.Spec.Plan.Revision,
							terraformv1alpha1.CloudResourceRevisionNameLabel: revision.Name,
						}))).To(Succeed())
						Expect(list.Items).To(HaveLen(1))

						configuration := list.Items[0]
						Expect(configuration.GetOwnerReferences()).To(HaveLen(1))
						Expect(configuration.GetOwnerReferences()[0].UID).To(Equal(cloudresource.UID))
						Expect(configuration.GetOwnerReferences()[0].Name).To(Equal(cloudresource.Name))
						Expect(configuration.GetOwnerReferences()[0].Kind).To(Equal(cloudresource.Kind))
						Expect(configuration.GetOwnerReferences()[0].APIVersion).To(Equal(cloudresource.APIVersion))
						Expect(configuration.Spec.Plan).To(Equal(&terraformv1alpha1.PlanReference{
							Name:     revision.Spec.Plan.Name,
							Revision: revision.Spec.Plan.Revision,
						}))

						Expect(configuration.Spec.Module).To(Equal(revision.Spec.Configuration.Module))
						Expect(configuration.Spec.EnableAutoApproval).To(Equal(revision.Spec.Configuration.EnableAutoApproval))
						Expect(configuration.Spec.EnableDriftDetection).To(Equal(revision.Spec.Configuration.EnableDriftDetection))
						Expect(configuration.Spec.TerraformVersion).To(Equal(revision.Spec.Configuration.TerraformVersion))
						Expect(configuration.Spec.WriteConnectionSecretToRef).To(Equal(cloudresource.Spec.WriteConnectionSecretToRef))
						Expect(configuration.Spec.ValueFrom).To(ContainElements(revision.Spec.Configuration.ValueFrom))
						Expect(configuration.Spec.ValueFrom).To(ContainElements(cloudresource.Spec.ValueFrom))
						Expect(configuration.Spec.Variables).ToNot(BeNil())
						Expect(string(configuration.Spec.Variables.Raw)).To(Equal(`{"test":"default","database_name":"mydb","database_size":5}`))
					})
				})

				Context("and overrides are provided", func() {
					BeforeEach(func() {
						cloudresource.Spec.Variables = &runtime.RawExtension{
							Raw: []byte(`{"database_name":"override"}`),
						}
						Expect(cc.Update(context.Background(), cloudresource)).To(Succeed())

						result, _, rerr = controllertests.Roll(context.TODO(), ctrl, cloudresource, 0)
					})

					It("should not return an error", func() {
						Expect(rerr).ToNot(HaveOccurred())
					})

					It("should have created a configuration", func() {
						list := &terraformv1alpha1.ConfigurationList{}
						Expect(cc.List(context.Background(), list,
							client.InNamespace(cloudresource.Namespace),
							client.MatchingLabels(map[string]string{
								terraformv1alpha1.CloudResourceNameLabel:         cloudresource.Name,
								terraformv1alpha1.CloudResourcePlanNameLabel:     revision.Spec.Plan.Name,
								terraformv1alpha1.CloudResourceRevisionLabel:     revision.Spec.Plan.Revision,
								terraformv1alpha1.CloudResourceRevisionNameLabel: revision.Name,
							}))).To(Succeed())
						Expect(list.Items).To(HaveLen(1))

						configuration := list.Items[0]
						Expect(string(configuration.Spec.Variables.Raw)).To(Equal(`{"test":"default","database_name":"override","database_size":5}`))
					})
				})
			})

			Context("and a configuration exists", func() {
				var configuration *terraformv1alpha1.Configuration

				BeforeEach(func() {
					configuration = terraformv1alpha1.NewConfiguration(cloudresource.Namespace, "test-configuration")
					configuration.Labels = map[string]string{
						terraformv1alpha1.CloudResourceNameLabel:         cloudresource.Name,
						terraformv1alpha1.CloudResourcePlanNameLabel:     revision.Spec.Plan.Name,
						terraformv1alpha1.CloudResourceRevisionLabel:     revision.Spec.Plan.Revision,
						terraformv1alpha1.CloudResourceRevisionNameLabel: revision.Name,
					}
					controller.EnsureConditionsRegistered(terraformv1alpha1.DefaultConfigurationConditions, configuration)

					Expect(cc.Create(context.Background(), configuration)).To(Succeed())
				})

				Context("but multiple configurations exist", func() {
					BeforeEach(func() {
						configuration := terraformv1alpha1.NewConfiguration(cloudresource.Namespace, "test1-configuration")
						configuration.Labels = map[string]string{
							terraformv1alpha1.CloudResourceNameLabel:         cloudresource.Name,
							terraformv1alpha1.CloudResourcePlanNameLabel:     revision.Spec.Plan.Name,
							terraformv1alpha1.CloudResourceRevisionLabel:     revision.Spec.Plan.Revision,
							terraformv1alpha1.CloudResourceRevisionNameLabel: revision.Name,
						}
						Expect(cc.Create(context.Background(), configuration)).To(Succeed())

						result, _, rerr = controllertests.Roll(context.TODO(), ctrl, cloudresource, 0)
					})

					It("should indicate the condition on the resource", func() {
						Expect(cc.Get(context.TODO(), cloudresource.GetNamespacedName(), cloudresource)).To(Succeed())

						cond := cloudresource.GetCommonStatus().GetCondition(corev1alpha1.ConditionReady)
						Expect(cond).ToNot(BeNil())
						Expect(cond.Type).To(Equal(corev1alpha1.ConditionReady))
						Expect(cond.Status).To(Equal(metav1.ConditionFalse))
						Expect(cond.Reason).To(Equal(corev1alpha1.ReasonActionRequired))
						Expect(cond.Message).To(Equal("Multiple configurations found for cloud resource"))
					})
				})

				Context("and no multiples found", func() {
					BeforeEach(func() {
						result, _, rerr = controllertests.Roll(context.TODO(), ctrl, cloudresource, 0)
					})

					It("should not return an error", func() {
						Expect(rerr).ToNot(HaveOccurred())
					})

					It("should indicate the condition on the resource", func() {
						Expect(cc.Get(context.TODO(), cloudresource.GetNamespacedName(), cloudresource)).To(Succeed())

						cond := cloudresource.GetCommonStatus().GetCondition(terraformv1alpha1.ConditionConfigurationReady)
						Expect(cond).ToNot(BeNil())
						Expect(cond.Type).To(Equal(terraformv1alpha1.ConditionConfigurationReady))
						Expect(cond.Status).To(Equal(metav1.ConditionTrue))
						Expect(cond.Reason).To(Equal(corev1alpha1.ReasonReady))
						Expect(cond.Message).To(Equal("Configuration has been updated"))
					})

					It("should have raised an event", func() {
						Expect(recorder.Events).To(HaveLen(1))
						Expect(recorder.Events).To(ContainElement(
							"(default/database) Normal ConfigurationUpdated: Updated the cloud resource configuration",
						))
					})

					It("should have updated a configuration", func() {
						list := &terraformv1alpha1.ConfigurationList{}
						Expect(cc.List(context.Background(), list,
							client.InNamespace(cloudresource.Namespace),
							client.MatchingLabels(map[string]string{
								terraformv1alpha1.CloudResourceNameLabel:         cloudresource.Name,
								terraformv1alpha1.CloudResourcePlanNameLabel:     revision.Spec.Plan.Name,
								terraformv1alpha1.CloudResourceRevisionLabel:     revision.Spec.Plan.Revision,
								terraformv1alpha1.CloudResourceRevisionNameLabel: revision.Name,
							}))).To(Succeed())
						Expect(list.Items).To(HaveLen(1))

						configuration := list.Items[0]
						Expect(configuration.GetOwnerReferences()).To(HaveLen(1))
						Expect(configuration.GetOwnerReferences()[0].UID).To(Equal(cloudresource.UID))
						Expect(configuration.GetOwnerReferences()[0].Name).To(Equal(cloudresource.Name))
						Expect(configuration.GetOwnerReferences()[0].Kind).To(Equal(cloudresource.Kind))
						Expect(configuration.GetOwnerReferences()[0].APIVersion).To(Equal(cloudresource.APIVersion))
						Expect(configuration.Spec.Plan).To(Equal(&terraformv1alpha1.PlanReference{
							Name:     revision.Spec.Plan.Name,
							Revision: revision.Spec.Plan.Revision,
						}))

						Expect(configuration.Spec.Module).To(Equal(revision.Spec.Configuration.Module))
						Expect(configuration.Spec.EnableAutoApproval).To(Equal(revision.Spec.Configuration.EnableAutoApproval))
						Expect(configuration.Spec.EnableDriftDetection).To(Equal(revision.Spec.Configuration.EnableDriftDetection))
						Expect(configuration.Spec.TerraformVersion).To(Equal(revision.Spec.Configuration.TerraformVersion))
						Expect(configuration.Spec.ValueFrom).To(ContainElements(revision.Spec.Configuration.ValueFrom))
						Expect(configuration.Spec.ValueFrom).To(ContainElements(cloudresource.Spec.ValueFrom))
						Expect(configuration.Spec.Variables).ToNot(BeNil())
						Expect(string(configuration.Spec.Variables.Raw)).To(Equal("{\"database_name\":\"mydb\",\"database_size\":5,\"test\":\"default\"}"))
					})
				})

				Context("and the cloud resource does not have an update available", func() {
					BeforeEach(func() {
						result, _, rerr = controllertests.Roll(context.TODO(), ctrl, cloudresource, 0)
					})

					It("should not return an error", func() {
						Expect(rerr).ToNot(HaveOccurred())
					})

					It("should indicate no updates available", func() {
						Expect(cc.Get(context.TODO(), cloudresource.GetNamespacedName(), cloudresource)).To(Succeed())
						Expect(cloudresource.Status.UpdateAvailable).To(Equal("None"))
					})
				})

				Context("and the cloud resource has an update available", func() {
					BeforeEach(func() {
						plan.Spec.Revisions = append(plan.Spec.Revisions, terraformv1alpha1.PlanRevision{
							Name:     "Updated",
							Revision: "v3.0.0",
						})
						Expect(cc.Update(context.Background(), plan)).To(Succeed())

						result, _, rerr = controllertests.Roll(context.TODO(), ctrl, cloudresource, 0)
					})

					It("should not return an error", func() {
						Expect(rerr).ToNot(HaveOccurred())
					})

					It("should indicate no updates available", func() {
						Expect(cc.Get(context.TODO(), cloudresource.GetNamespacedName(), cloudresource)).To(Succeed())
						Expect(cloudresource.Status.UpdateAvailable).To(Equal("Update v3.0.0 available"))
					})
				})
			})
		})
	})
})
