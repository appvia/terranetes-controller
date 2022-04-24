/*
 * Copyright (C) 2022  Rohith Jayawardene <gambol99@gmail.com>
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

package configuration

import (
	"context"
	"io/ioutil"
	"testing"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/sirupsen/logrus"
	batchv1 "k8s.io/api/batch/v1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	corev1alphav1 "github.com/appvia/terraform-controller/pkg/apis/core/v1alpha1"
	terraformv1alphav1 "github.com/appvia/terraform-controller/pkg/apis/terraform/v1alpha1"
	"github.com/appvia/terraform-controller/pkg/schema"
	"github.com/appvia/terraform-controller/pkg/utils/kubernetes"
	controllertests "github.com/appvia/terraform-controller/test"
	"github.com/appvia/terraform-controller/test/fixtures"
)

func TestReconcile(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Running Test Suite")
}

var _ = Describe("Configuration Controller", func() {
	logrus.SetOutput(ioutil.Discard)

	var cc client.Client
	var result reconcile.Result
	var rerr error
	var ctrl *Controller
	var configuration *terraformv1alphav1.Configuration

	cfgNamespace := "apps"

	Setup := func(objects ...runtime.Object) {
		cc = fake.NewFakeClientWithScheme(schema.GetScheme(), append([]runtime.Object{
			fixtures.NewNamespace(cfgNamespace),
			fixtures.NewValidAWSReadyProvider("default", "aws"),
			fixtures.NewValidAWSProviderSecret("default", "aws"),
		}, objects...)...)
		ctrl = &Controller{
			cc:            cc,
			recorder:      record.NewFakeRecorder(10),
			JobNamespace:  "default",
			ExecutorImage: "quay.io/appvia/terraform-executor",
			GitImage:      "appvia/git:latest",
		}
	}

	When("provider does not exist", func() {
		BeforeEach(func() {
			configuration = fixtures.NewValidBucketConfiguration(cfgNamespace, "bucket")
			configuration.Spec.ProviderRef.Name = "does_not_exist"
			Setup(configuration)
			result, _, rerr = controllertests.Roll(context.TODO(), ctrl, configuration, 3)
		})

		It("should have the conditions", func() {
			Expect(cc.Get(context.TODO(), configuration.GetNamespacedName(), configuration)).ToNot(HaveOccurred())
			Expect(configuration.Status.Conditions).To(HaveLen(4))
		})

		It("should indicate the provider is missing", func() {
			Expect(cc.Get(context.TODO(), configuration.GetNamespacedName(), configuration)).ToNot(HaveOccurred())

			Expect(configuration.Status.Conditions[0].Type).To(Equal(terraformv1alphav1.ConditionProviderReady))
			Expect(configuration.Status.Conditions[0].Status).To(Equal(metav1.ConditionFalse))
			Expect(configuration.Status.Conditions[0].Reason).To(Equal(corev1alphav1.ReasonActionRequired))
			Expect(configuration.Status.Conditions[0].Message).To(Equal("Provider referenced (default/does_not_exist) does not exist"))
		})

		It("should ask us to requeue", func() {
			Expect(result).To(Equal(reconcile.Result{RequeueAfter: 5 * time.Minute}))
			Expect(rerr).To(BeNil())
		})
	})

	When("provider is not ready", func() {
		BeforeEach(func() {
			configuration = fixtures.NewValidBucketConfiguration(cfgNamespace, "bucket")
			configuration.Spec.ProviderRef.Name = "not_ready"
			Setup(configuration, fixtures.NewValidAWSNotReadyProvider("default", "not_ready"))
			result, _, rerr = controllertests.Roll(context.TODO(), ctrl, configuration, 3)
		})

		It("should have the conditions", func() {
			Expect(cc.Get(context.TODO(), configuration.GetNamespacedName(), configuration)).ToNot(HaveOccurred())
			Expect(configuration.Status.Conditions).To(HaveLen(4))
		})

		It("should indicate the provider is not ready", func() {
			Expect(cc.Get(context.TODO(), configuration.GetNamespacedName(), configuration)).ToNot(HaveOccurred())

			cond := configuration.Status.GetCondition(terraformv1alphav1.ConditionProviderReady)
			Expect(cond.Status).To(Equal(metav1.ConditionFalse))
			Expect(cond.Reason).To(Equal(corev1alphav1.ReasonWarning))
			Expect(cond.Message).To(Equal("Provider is not ready"))
		})

		It("should ask us to requeue", func() {
			Expect(result).To(Equal(reconcile.Result{RequeueAfter: 30 * time.Second}))
			Expect(rerr).To(BeNil())
		})
	})

	When("terraform plan has not been provisioned", func() {
		BeforeEach(func() {
			configuration = fixtures.NewValidBucketConfiguration(cfgNamespace, "bucket")
			Setup(configuration)
			result, _, rerr = controllertests.Roll(context.TODO(), ctrl, configuration, 3)
		})

		It("should have the conditions", func() {
			Expect(cc.Get(context.TODO(), configuration.GetNamespacedName(), configuration)).ToNot(HaveOccurred())
			Expect(configuration.Status.Conditions).To(HaveLen(4))
		})

		It("should indicate the provider is ready", func() {
			Expect(cc.Get(context.TODO(), configuration.GetNamespacedName(), configuration)).ToNot(HaveOccurred())

			cond := configuration.Status.GetCondition(terraformv1alphav1.ConditionProviderReady)
			Expect(cond.Status).To(Equal(metav1.ConditionTrue))
			Expect(cond.Reason).To(Equal(corev1alphav1.ReasonReady))
			Expect(cond.Message).To(Equal("Provider ready"))
		})

		It("should have created the generated configuration", func() {
			cm := &v1.ConfigMap{}
			cm.Namespace = ctrl.JobNamespace
			cm.Name = string(configuration.GetUID())

			found, err := kubernetes.GetIfExists(context.TODO(), cc, cm)
			Expect(err).ToNot(HaveOccurred())
			Expect(found).To(BeTrue())

			Expect(cm.Data).To(HaveKey(terraformv1alphav1.TerraformVariablesConfigMapKey))
			Expect(cm.Data).To(HaveKey(terraformv1alphav1.TerraformBackendConfigMapKey))
		})

		It("should indicate the terraform plan is running", func() {
			Expect(cc.Get(context.TODO(), configuration.GetNamespacedName(), configuration)).ToNot(HaveOccurred())

			cond := configuration.Status.GetCondition(terraformv1alphav1.ConditionTerraformPlan)
			Expect(cond.Status).To(Equal(metav1.ConditionFalse))
			Expect(cond.Reason).To(Equal(corev1alphav1.ReasonInProgress))
			Expect(cond.Message).To(Equal("Terraform plan in progress"))
		})

		It("should have created job for the terraform plan", func() {
			list := &batchv1.JobList{}

			Expect(cc.List(context.TODO(), list, client.InNamespace(ctrl.JobNamespace))).ToNot(HaveOccurred())
			Expect(len(list.Items)).To(Equal(1))
		})

		It("it should have the configuration labels", func() {
			list := &batchv1.JobList{}
			Expect(cc.List(context.TODO(), list, client.InNamespace(ctrl.JobNamespace))).ToNot(HaveOccurred())
			Expect(len(list.Items)).To(Equal(1))

			labels := list.Items[0].GetLabels()
			Expect(labels).To(HaveKey(terraformv1alphav1.ConfigurationNameLabel))
			Expect(labels).To(HaveKey(terraformv1alphav1.ConfigurationNamespaceLabel))
			Expect(labels).To(HaveKey(terraformv1alphav1.ConfigurationGenerationLabel))
			Expect(labels).To(HaveKey(terraformv1alphav1.ConfigurationStageLabel))
			Expect(labels[terraformv1alphav1.ConfigurationStageLabel]).To(Equal(terraformv1alphav1.StageTerraformPlan))
			Expect(labels[terraformv1alphav1.ConfigurationNameLabel]).To(Equal(configuration.Name))
			Expect(labels[terraformv1alphav1.ConfigurationNamespaceLabel]).To(Equal(configuration.Namespace))
		})

		It("should have created a watch job in the configuration namespace", func() {
			list := &batchv1.JobList{}
			Expect(cc.List(context.TODO(), list, client.InNamespace(configuration.Namespace))).ToNot(HaveOccurred())
			Expect(len(list.Items)).To(Equal(1))
			Expect(len(list.Items[0].Spec.Template.Spec.Containers)).To(Equal(1))

			container := list.Items[0].Spec.Template.Spec.Containers[0]
			Expect(container.Name).To(Equal("watch"))
			Expect(container.Command).To(Equal([]string{"sh"}))
		})

		It("should have added a approval annotation to the configuration", func() {
			Expect(cc.Get(context.TODO(), configuration.GetNamespacedName(), configuration)).ToNot(HaveOccurred())
			Expect(configuration.Annotations).To(HaveKey(terraformv1alphav1.ApplyAnnotation))
		})

		It("should ask us to requeue", func() {
			Expect(result).To(Equal(reconcile.Result{RequeueAfter: 5 * time.Second}))
			Expect(rerr).To(BeNil())
		})
	})

	When("terraform apply has not been provisoned", func() {
		BeforeEach(func() {
			configuration = fixtures.NewValidBucketConfiguration(cfgNamespace, "bucket")
			plan := fixtures.NewTerraformJob(configuration, ctrl.JobNamespace, terraformv1alphav1.StageTerraformPlan)
			plan.Status.Succeeded = 1

			Setup(configuration, plan)
			result, _, rerr = controllertests.Roll(context.TODO(), ctrl, configuration, 5)
		})

		It("should have the conditions", func() {
			Expect(cc.Get(context.TODO(), configuration.GetNamespacedName(), configuration)).ToNot(HaveOccurred())
			Expect(configuration.Status.Conditions).To(HaveLen(4))
		})

		It("should indicate the terraform apply is running", func() {
			Expect(cc.Get(context.TODO(), configuration.GetNamespacedName(), configuration)).ToNot(HaveOccurred())

			cond := configuration.Status.GetCondition(terraformv1alphav1.ConditionTerraformPlan)
			Expect(cond.Status).To(Equal(metav1.ConditionTrue))
			Expect(cond.Reason).To(Equal(corev1alphav1.ReasonReady))
			Expect(cond.Message).To(Equal("Terraform plan is complete"))
		})

		It("should have created job for the terraform apply", func() {
			list := &batchv1.JobList{}

			Expect(cc.List(context.TODO(), list, client.InNamespace(ctrl.JobNamespace))).ToNot(HaveOccurred())
			Expect(len(list.Items)).To(Equal(2))
		})

		It("it should have the configuration labels", func() {
			list := &batchv1.JobList{}
			Expect(cc.List(context.TODO(), list, client.InNamespace(ctrl.JobNamespace))).ToNot(HaveOccurred())

			labels := list.Items[0].GetLabels()
			Expect(labels).To(HaveKey(terraformv1alphav1.ConfigurationNameLabel))
			Expect(labels).To(HaveKey(terraformv1alphav1.ConfigurationNamespaceLabel))
			Expect(labels).To(HaveKey(terraformv1alphav1.ConfigurationGenerationLabel))
			Expect(labels).To(HaveKey(terraformv1alphav1.ConfigurationStageLabel))
			Expect(labels[terraformv1alphav1.ConfigurationStageLabel]).To(Equal(terraformv1alphav1.StageTerraformApply))
			Expect(labels[terraformv1alphav1.ConfigurationNameLabel]).To(Equal(configuration.Name))
			Expect(labels[terraformv1alphav1.ConfigurationNamespaceLabel]).To(Equal(configuration.Namespace))
		})

		It("should have created a watch job in the configuration namespace", func() {
			list := &batchv1.JobList{}
			Expect(cc.List(context.TODO(), list, client.InNamespace(configuration.Namespace))).ToNot(HaveOccurred())
			Expect(len(list.Items)).To(Equal(1))
			Expect(len(list.Items[0].Spec.Template.Spec.Containers)).To(Equal(1))

			container := list.Items[0].Spec.Template.Spec.Containers[0]
			Expect(container.Name).To(Equal("watch"))
			Expect(container.Command).To(Equal([]string{"sh"}))
		})

		It("should ask us to requeue", func() {
			Expect(result).To(Equal(reconcile.Result{RequeueAfter: 5 * time.Second}))
			Expect(rerr).To(BeNil())
		})
	})
})
