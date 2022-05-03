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
			JobNamespace:  "default",
			ExecutorImage: "quay.io/appvia/terraform-executor",
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

	When("the costs analytics token is missing", func() {
		BeforeEach(func() {
			configuration = fixtures.NewValidBucketConfiguration(cfgNamespace, "bucket")
			Setup(configuration)
			ctrl.EnableCostAnalytics = true
			ctrl.CostAnalyticsSecretName = "not_there"

			result, _, rerr = controllertests.Roll(context.TODO(), ctrl, configuration, 3)
		})

		It("should have the conditions", func() {
			Expect(cc.Get(context.TODO(), configuration.GetNamespacedName(), configuration)).ToNot(HaveOccurred())
			Expect(configuration.Status.Conditions).To(HaveLen(4))
		})

		It("should indicate the costs analytics token is invalid", func() {
			Expect(cc.Get(context.TODO(), configuration.GetNamespacedName(), configuration)).ToNot(HaveOccurred())

			cond := configuration.Status.GetCondition(corev1alphav1.ConditionReady)
			Expect(cond.Status).To(Equal(metav1.ConditionFalse))
			Expect(cond.Reason).To(Equal(corev1alphav1.ReasonActionRequired))
			Expect(cond.Message).To(Equal("Cost analytics secret (default/not_there) does not exist, contact platform administrator"))
		})
	})

	When("authentication secret is missing", func() {
		BeforeEach(func() {
			configuration = fixtures.NewValidBucketConfiguration(cfgNamespace, "bucket")
			configuration.Spec.Auth = &v1.SecretReference{
				Namespace: "default",
				Name:      "not_there",
			}
			Setup(configuration)
			result, _, rerr = controllertests.Roll(context.TODO(), ctrl, configuration, 3)
		})

		It("should have the conditions", func() {
			Expect(cc.Get(context.TODO(), configuration.GetNamespacedName(), configuration)).ToNot(HaveOccurred())
			Expect(configuration.Status.Conditions).To(HaveLen(4))
		})

		It("should indicate the credentials are missing", func() {
			Expect(cc.Get(context.TODO(), configuration.GetNamespacedName(), configuration)).ToNot(HaveOccurred())

			cond := configuration.Status.GetCondition(corev1alphav1.ConditionReady)
			Expect(cond.Status).To(Equal(metav1.ConditionFalse))
			Expect(cond.Reason).To(Equal(corev1alphav1.ReasonActionRequired))
			Expect(cond.Message).To(Equal("Authentication secret (spec.scmAuth) does not exist"))
		})
	})

	When("the authentication secret is invalid", func() {
		var secret *v1.Secret

		BeforeEach(func() {
			configuration = fixtures.NewValidBucketConfiguration(cfgNamespace, "bucket")
			configuration.Spec.Auth = &v1.SecretReference{
				Namespace: cfgNamespace,
				Name:      "auth",
			}
			secret = &v1.Secret{}
			secret.Namespace = cfgNamespace
			secret.Data = make(map[string][]byte)
			secret.Name = "auth"
		})

		When("is has not valid key", func() {
			BeforeEach(func() {
				Setup(configuration, secret)
				//lint:ignore
				controllertests.Roll(context.TODO(), ctrl, configuration, 3)
			})

			It("should indicate on the conditions", func() {
				Expect(cc.Get(context.TODO(), configuration.GetNamespacedName(), configuration)).ToNot(HaveOccurred())

				cond := configuration.Status.GetCondition(corev1alphav1.ConditionReady)
				Expect(cond.Status).To(Equal(metav1.ConditionFalse))
				Expect(cond.Reason).To(Equal(corev1alphav1.ReasonActionRequired))
				Expect(cond.Message).To(Equal("Authentication secret needs either GIT_USERNAME & GIT_PASSWORD or SSH_AUTH_KEY"))
			})
		})

		When("is has ssh key", func() {
			BeforeEach(func() {
				secret.Data["SSH_AUTH_KEY"] = []byte("test")

				Setup(configuration, secret)
				controllertests.Roll(context.TODO(), ctrl, configuration, 3)
			})

			It("should be ok", func() {
				Expect(cc.Get(context.TODO(), configuration.GetNamespacedName(), configuration)).ToNot(HaveOccurred())

				cond := configuration.Status.GetCondition(terraformv1alphav1.ConditionTerraformPlan)
				Expect(cond.Status).To(Equal(metav1.ConditionFalse))
				Expect(cond.Reason).To(Equal(corev1alphav1.ReasonInProgress))
				Expect(cond.Message).To(Equal("Terraform plan in progress"))
			})
		})

		When("is has a git username but no password", func() {
			BeforeEach(func() {
				secret.Data["GIT_USERNAME"] = []byte("test")

				Setup(configuration, secret)
				controllertests.Roll(context.TODO(), ctrl, configuration, 3)
			})

			It("should indicate the GIT_PASSWORD is missing", func() {
				Expect(cc.Get(context.TODO(), configuration.GetNamespacedName(), configuration)).ToNot(HaveOccurred())

				cond := configuration.Status.GetCondition(corev1alphav1.ConditionReady)
				Expect(cond.Status).To(Equal(metav1.ConditionFalse))
				Expect(cond.Reason).To(Equal(corev1alphav1.ReasonActionRequired))
				Expect(cond.Message).To(Equal("Authentication secret needs either GIT_USERNAME & GIT_PASSWORD or SSH_AUTH_KEY"))
			})
		})

		When("is has a git username but not password", func() {
			BeforeEach(func() {
				secret.Data["GIT_PASSWORD"] = []byte("test")

				Setup(configuration, secret)
				controllertests.Roll(context.TODO(), ctrl, configuration, 3)
			})

			It("should indicate the GIT_USERNAME is missing", func() {
				Expect(cc.Get(context.TODO(), configuration.GetNamespacedName(), configuration)).ToNot(HaveOccurred())

				cond := configuration.Status.GetCondition(corev1alphav1.ConditionReady)
				Expect(cond.Status).To(Equal(metav1.ConditionFalse))
				Expect(cond.Reason).To(Equal(corev1alphav1.ReasonActionRequired))
				Expect(cond.Message).To(Equal("Authentication secret needs either GIT_USERNAME & GIT_PASSWORD or SSH_AUTH_KEY"))
			})
		})

		When("both GIT_USERNAME and GIT_PASSWORD are given", func() {
			BeforeEach(func() {
				secret.Data["GIT_USERNAME"] = []byte("test")
				secret.Data["GIT_PASSWORD"] = []byte("test")

				Setup(configuration, secret)
				controllertests.Roll(context.TODO(), ctrl, configuration, 3)
			})

			It("should be ok", func() {
				Expect(cc.Get(context.TODO(), configuration.GetNamespacedName(), configuration)).ToNot(HaveOccurred())

				cond := configuration.Status.GetCondition(terraformv1alphav1.ConditionTerraformPlan)
				Expect(cond.Status).To(Equal(metav1.ConditionFalse))
				Expect(cond.Reason).To(Equal(corev1alphav1.ReasonInProgress))
				Expect(cond.Message).To(Equal("Terraform plan in progress"))
			})
		})
	})

	When("using authentication on the configuration", func() {

		It("should have a secret mapped onto the plan", func() {

		})

		It("should have copied the secret into the job namespace", func() {

		})
	})

	When("cost analytics token is invalid", func() {
		BeforeEach(func() {
			configuration = fixtures.NewValidBucketConfiguration(cfgNamespace, "bucket")
			secret := &v1.Secret{}
			secret.Name = "token"
			secret.Namespace = ctrl.JobNamespace

			Setup(configuration, secret)
			ctrl.EnableCostAnalytics = true
			ctrl.CostAnalyticsSecretName = "token"

			result, _, rerr = controllertests.Roll(context.TODO(), ctrl, configuration, 3)
		})

		It("should have the conditions", func() {
			Expect(cc.Get(context.TODO(), configuration.GetNamespacedName(), configuration)).ToNot(HaveOccurred())
			Expect(configuration.Status.Conditions).To(HaveLen(4))
		})

		It("should indicate the costs analytics token is invalid", func() {
			Expect(cc.Get(context.TODO(), configuration.GetNamespacedName(), configuration)).ToNot(HaveOccurred())

			cond := configuration.Status.GetCondition(corev1alphav1.ConditionReady)
			Expect(cond.Status).To(Equal(metav1.ConditionFalse))
			Expect(cond.Reason).To(Equal(corev1alphav1.ReasonActionRequired))
			Expect(cond.Message).To(Equal("Cost analytics secret (default/token) does not contain a token, contact platform administrator"))
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

		It("should have created the generated configuration secret", func() {
			secret := &v1.Secret{}
			secret.Namespace = ctrl.JobNamespace
			secret.Name = configuration.GetTerraformConfigSecretName()

			found, err := kubernetes.GetIfExists(context.TODO(), cc, secret)
			Expect(err).ToNot(HaveOccurred())
			Expect(found).To(BeTrue())

			Expect(secret.Data).To(HaveKey(terraformv1alphav1.TerraformVariablesConfigMapKey))
			Expect(secret.Data).To(HaveKey(terraformv1alphav1.TerraformBackendConfigMapKey))
		})

		It("should have create a terraform backend configuration", func() {
			expected := `
terraform {
	backend "kubernetes" {
		in_cluster_config = true
		namespace         = "default"
		secret_suffix     = "1234-122-1234-1234"
	}
}
`
			secret := &v1.Secret{}
			secret.Namespace = ctrl.JobNamespace
			secret.Name = configuration.GetTerraformConfigSecretName()

			found, err := kubernetes.GetIfExists(context.TODO(), cc, secret)
			Expect(err).ToNot(HaveOccurred())
			Expect(found).To(BeTrue())

			backend := string(secret.Data[terraformv1alphav1.TerraformBackendConfigMapKey])
			Expect(backend).ToNot(BeZero())
			Expect(backend).To(Equal(expected))
		})

		It("should have a provider.tf", func() {
			expected := `
provider "aws" {}
`
			secret := &v1.Secret{}
			secret.Namespace = ctrl.JobNamespace
			secret.Name = configuration.GetTerraformConfigSecretName()

			found, err := kubernetes.GetIfExists(context.TODO(), cc, secret)
			Expect(err).ToNot(HaveOccurred())
			Expect(found).To(BeTrue())

			backend := string(secret.Data[terraformv1alphav1.TerraformProviderConfigMapKey])
			Expect(backend).ToNot(BeZero())
			Expect(backend).To(Equal(expected))
		})

		It("should have the following variables in the config", func() {
			expected := `{"name":"test"}`

			secret := &v1.Secret{}
			secret.Namespace = ctrl.JobNamespace
			secret.Name = configuration.GetTerraformConfigSecretName()

			found, err := kubernetes.GetIfExists(context.TODO(), cc, secret)
			Expect(err).ToNot(HaveOccurred())
			Expect(found).To(BeTrue())

			backend := string(secret.Data[terraformv1alphav1.TerraformVariablesConfigMapKey])
			Expect(backend).ToNot(BeZero())
			Expect(backend).To(Equal(expected))
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
			plan.Status.Conditions = []batchv1.JobCondition{
				{
					Type:   batchv1.JobComplete,
					Status: v1.ConditionTrue,
				},
			}
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
