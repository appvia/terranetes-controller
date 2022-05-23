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

package configuration

import (
	"context"
	"io/ioutil"
	"testing"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/patrickmn/go-cache"
	"github.com/sirupsen/logrus"
	batchv1 "k8s.io/api/batch/v1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
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
	var recorder *controllertests.FakeRecorder

	cfgNamespace := "apps"
	defaultConditions := 5

	Setup := func(objects ...runtime.Object) {
		cc = fake.NewFakeClientWithScheme(schema.GetScheme(), append([]runtime.Object{
			fixtures.NewNamespace(cfgNamespace),
			fixtures.NewValidAWSReadyProvider("default", "aws"),
			fixtures.NewValidAWSProviderSecret("default", "aws"),
		}, objects...)...)
		recorder = &controllertests.FakeRecorder{}
		ctrl = &Controller{
			cc:               cc,
			cache:            cache.New(5*time.Minute, 10*time.Minute),
			recorder:         recorder,
			EnableInfracosts: false,
			EnableWatchers:   true,
			ExecutorImage:    "quay.io/appvia/terraform-executor",
			InfracostsImage:  "infracosts/infracost:latest",
			JobNamespace:     "default",
			PolicyImage:      "bridgecrew/checkov:2.0.1140",
			TerraformImage:   "hashicorp/terraform:1.1.9",
		}
		ctrl.cache.SetDefault(cfgNamespace, fixtures.NewNamespace(cfgNamespace))
	}

	When("provider does not exist", func() {
		BeforeEach(func() {
			configuration = fixtures.NewValidBucketConfiguration(cfgNamespace, "bucket")
			configuration.Spec.ProviderRef.Name = "does_not_exist"
			Setup(configuration)
			result, _, rerr = controllertests.Roll(context.TODO(), ctrl, configuration, 2)
		})

		It("should have the conditions", func() {
			Expect(cc.Get(context.TODO(), configuration.GetNamespacedName(), configuration)).ToNot(HaveOccurred())
			Expect(configuration.Status.Conditions).To(HaveLen(defaultConditions))
		})

		It("should indicate the provider is missing", func() {
			Expect(cc.Get(context.TODO(), configuration.GetNamespacedName(), configuration)).ToNot(HaveOccurred())

			cond := configuration.Status.GetCondition(terraformv1alphav1.ConditionProviderReady)
			Expect(cond.Type).To(Equal(terraformv1alphav1.ConditionProviderReady))
			Expect(cond.Status).To(Equal(metav1.ConditionFalse))
			Expect(cond.Reason).To(Equal(corev1alphav1.ReasonActionRequired))
			Expect(cond.Message).To(Equal("Provider referenced (default/does_not_exist) does not exist"))
		})

		It("should have raised a event", func() {
			Expect(recorder.Events).To(HaveLen(1))
			Expect(recorder.Events[0]).To(ContainSubstring("Provider referenced (default/does_not_exist) does not exist"))
		})

		It("should ask us to requeue", func() {
			Expect(result).To(Equal(reconcile.Result{RequeueAfter: 5 * time.Minute}))
			Expect(rerr).To(BeNil())
		})

		It("should not create any jobs", func() {
			list := &batchv1.JobList{}

			Expect(cc.List(context.TODO(), list, client.InNamespace(ctrl.JobNamespace))).ToNot(HaveOccurred())
			Expect(len(list.Items)).To(Equal(0))
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
			Expect(configuration.Status.Conditions).To(HaveLen(defaultConditions))
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

		It("should not create any jobs", func() {
			list := &batchv1.JobList{}

			Expect(cc.List(context.TODO(), list, client.InNamespace(ctrl.JobNamespace))).ToNot(HaveOccurred())
			Expect(len(list.Items)).To(Equal(0))
		})
	})

	When("the provider has a selector policy associated", func() {
		When("policy denies the use of the provider by namespace labels", func() {
			BeforeEach(func() {
				// @note: we create a configuration pointing to a new provider, a provider with a policy and a secret
				// for the provider
				configuration = fixtures.NewValidBucketConfiguration(cfgNamespace, "bucket")
				configuration.Spec.ProviderRef.Name = "policy"

				provider := fixtures.NewValidAWSReadyProvider(configuration.Spec.ProviderRef.Namespace, configuration.Spec.ProviderRef.Name)
				provider.Spec.Selector = &terraformv1alphav1.Selector{
					Namespace: &metav1.LabelSelector{
						MatchLabels: map[string]string{"does_not_match": "true"},
					},
				}
				secret := fixtures.NewValidAWSProviderSecret(configuration.Spec.ProviderRef.Namespace, configuration.Spec.ProviderRef.Name)

				Setup(configuration, provider, secret)
				result, _, rerr = controllertests.Roll(context.TODO(), ctrl, configuration, 3)
			})

			It("should have the conditions", func() {
				Expect(cc.Get(context.TODO(), configuration.GetNamespacedName(), configuration)).ToNot(HaveOccurred())
				Expect(configuration.Status.Conditions).To(HaveLen(defaultConditions))
			})

			It("should indicate the provider is denied", func() {
				Expect(cc.Get(context.TODO(), configuration.GetNamespacedName(), configuration)).ToNot(HaveOccurred())

				cond := configuration.GetCommonStatus().GetCondition(terraformv1alphav1.ConditionProviderReady)
				Expect(cond.Type).To(Equal(terraformv1alphav1.ConditionProviderReady))
				Expect(cond.Status).To(Equal(metav1.ConditionFalse))
				Expect(cond.Reason).To(Equal(corev1alphav1.ReasonActionRequired))
				Expect(cond.Message).To(Equal("Provider policy does not permit the configuration to use it"))
			})

			It("should not create any jobs", func() {
				list := &batchv1.JobList{}

				Expect(cc.List(context.TODO(), list, client.InNamespace(ctrl.JobNamespace))).ToNot(HaveOccurred())
				Expect(len(list.Items)).To(Equal(0))
			})
		})

		When("policy denies the use of the provider by resource labels", func() {
			BeforeEach(func() {
				// @note: we create a configuration pointing to a new provider, a provider with a policy and a secret
				// for the provider
				configuration = fixtures.NewValidBucketConfiguration(cfgNamespace, "bucket")
				configuration.Spec.ProviderRef.Name = "policy"
				configuration.Labels = map[string]string{"does_not_match": "true"}

				provider := fixtures.NewValidAWSReadyProvider(configuration.Spec.ProviderRef.Namespace, configuration.Spec.ProviderRef.Name)
				provider.Spec.Selector = &terraformv1alphav1.Selector{
					Resource: &metav1.LabelSelector{
						MatchLabels: map[string]string{"does_not_match": "false"},
					},
				}
				secret := fixtures.NewValidAWSProviderSecret(configuration.Spec.ProviderRef.Namespace, configuration.Spec.ProviderRef.Name)

				Setup(configuration, provider, secret)
				result, _, rerr = controllertests.Roll(context.TODO(), ctrl, configuration, 3)
			})

			It("should have the conditions", func() {
				Expect(cc.Get(context.TODO(), configuration.GetNamespacedName(), configuration)).ToNot(HaveOccurred())
				Expect(configuration.Status.Conditions).To(HaveLen(defaultConditions))
			})

			It("should indicate the provider is denied", func() {
				Expect(cc.Get(context.TODO(), configuration.GetNamespacedName(), configuration)).ToNot(HaveOccurred())

				cond := configuration.GetCommonStatus().GetCondition(terraformv1alphav1.ConditionProviderReady)
				Expect(cond.Type).To(Equal(terraformv1alphav1.ConditionProviderReady))
				Expect(cond.Status).To(Equal(metav1.ConditionFalse))
				Expect(cond.Reason).To(Equal(corev1alphav1.ReasonActionRequired))
				Expect(cond.Message).To(Equal("Provider policy does not permit the configuration to use it"))
			})

			It("should not create any jobs", func() {
				list := &batchv1.JobList{}

				Expect(cc.List(context.TODO(), list, client.InNamespace(ctrl.JobNamespace))).ToNot(HaveOccurred())
				Expect(len(list.Items)).To(Equal(0))
			})
		})

		When("policy allows of the provider", func() {
			BeforeEach(func() {
				// @note: we create a configuration pointing to a new provider, a provider with a policy and a secret
				// for the provider
				configuration = fixtures.NewValidBucketConfiguration(cfgNamespace, "bucket")
				configuration.Spec.ProviderRef.Name = "policy"
				configuration.Labels = map[string]string{"does_match": "true"}

				provider := fixtures.NewValidAWSReadyProvider(configuration.Spec.ProviderRef.Namespace, configuration.Spec.ProviderRef.Name)
				provider.Spec.Selector = &terraformv1alphav1.Selector{
					Resource: &metav1.LabelSelector{
						MatchLabels: map[string]string{"does_match": "true"},
					},
					Namespace: &metav1.LabelSelector{
						MatchLabels: map[string]string{"name": configuration.Namespace},
					},
				}
				secret := fixtures.NewValidAWSProviderSecret(configuration.Spec.ProviderRef.Namespace, configuration.Spec.ProviderRef.Name)

				Setup(configuration, provider, secret)
				result, _, rerr = controllertests.Roll(context.TODO(), ctrl, configuration, 3)
			})

			It("should have the conditions", func() {
				Expect(cc.Get(context.TODO(), configuration.GetNamespacedName(), configuration)).ToNot(HaveOccurred())
				Expect(configuration.Status.Conditions).To(HaveLen(defaultConditions))
			})

			It("should indicate the provider is ready", func() {
				Expect(cc.Get(context.TODO(), configuration.GetNamespacedName(), configuration)).ToNot(HaveOccurred())

				cond := configuration.GetCommonStatus().GetCondition(terraformv1alphav1.ConditionProviderReady)
				Expect(cond.Type).To(Equal(terraformv1alphav1.ConditionProviderReady))
				Expect(cond.Status).To(Equal(metav1.ConditionTrue))
				Expect(cond.Reason).To(Equal(corev1alphav1.ReasonReady))
				Expect(cond.Message).To(Equal("Provider ready"))
			})

			It("should create any jobs", func() {
				list := &batchv1.JobList{}

				Expect(cc.List(context.TODO(), list, client.InNamespace(ctrl.JobNamespace))).ToNot(HaveOccurred())
				Expect(len(list.Items)).To(Equal(1))
			})
		})
	})

	When("the costs analytics token is missing", func() {
		BeforeEach(func() {
			configuration = fixtures.NewValidBucketConfiguration(cfgNamespace, "bucket")
			Setup(configuration)
			ctrl.EnableInfracosts = true
			ctrl.InfracostsSecretName = "not_there"

			result, _, rerr = controllertests.Roll(context.TODO(), ctrl, configuration, 2)
		})

		It("should have the conditions", func() {
			Expect(cc.Get(context.TODO(), configuration.GetNamespacedName(), configuration)).ToNot(HaveOccurred())
			Expect(configuration.Status.Conditions).To(HaveLen(defaultConditions))
		})

		It("should indicate the costs analytics token is invalid", func() {
			Expect(cc.Get(context.TODO(), configuration.GetNamespacedName(), configuration)).ToNot(HaveOccurred())

			cond := configuration.Status.GetCondition(corev1alphav1.ConditionReady)
			Expect(cond.Status).To(Equal(metav1.ConditionFalse))
			Expect(cond.Reason).To(Equal(corev1alphav1.ReasonActionRequired))
			Expect(cond.Message).To(Equal("Cost analytics secret (default/not_there) does not exist, contact platform administrator"))
		})

		It("should have raised a event", func() {
			Expect(recorder.Events).To(HaveLen(1))
			Expect(recorder.Events[0]).To(ContainSubstring("Cost analytics secret (default/not_there) does not exist, contact platform administrator"))
		})

		It("should not create any jobs", func() {
			list := &batchv1.JobList{}

			Expect(cc.List(context.TODO(), list, client.InNamespace(ctrl.JobNamespace))).ToNot(HaveOccurred())
			Expect(len(list.Items)).To(Equal(0))
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
			Expect(configuration.Status.Conditions).To(HaveLen(defaultConditions))
		})

		It("should indicate the credentials are missing", func() {
			Expect(cc.Get(context.TODO(), configuration.GetNamespacedName(), configuration)).ToNot(HaveOccurred())

			cond := configuration.Status.GetCondition(corev1alphav1.ConditionReady)
			Expect(cond.Status).To(Equal(metav1.ConditionFalse))
			Expect(cond.Reason).To(Equal(corev1alphav1.ReasonActionRequired))
			Expect(cond.Message).To(Equal("Authentication secret (spec.scmAuth) does not exist"))
		})

		It("should not create any jobs", func() {
			list := &batchv1.JobList{}

			Expect(cc.List(context.TODO(), list, client.InNamespace(ctrl.JobNamespace))).ToNot(HaveOccurred())
			Expect(len(list.Items)).To(Equal(0))
		})

		It("should have raised a event", func() {
			Expect(recorder.Events).To(HaveLen(1))
			Expect(recorder.Events[0]).To(ContainSubstring("Authentication secret (spec.scmAuth) does not exist"))
		})
	})

	When("cost analytics token is invalid", func() {
		BeforeEach(func() {
			configuration = fixtures.NewValidBucketConfiguration(cfgNamespace, "bucket")
			secret := &v1.Secret{}
			secret.Name = "token"
			secret.Namespace = ctrl.JobNamespace

			Setup(configuration, secret)
			ctrl.EnableInfracosts = true
			ctrl.InfracostsSecretName = "token"

			result, _, rerr = controllertests.Roll(context.TODO(), ctrl, configuration, 2)
		})

		It("should have the conditions", func() {
			Expect(cc.Get(context.TODO(), configuration.GetNamespacedName(), configuration)).ToNot(HaveOccurred())
			Expect(configuration.Status.Conditions).To(HaveLen(defaultConditions))
		})

		It("should indicate the costs analytics token is invalid", func() {
			Expect(cc.Get(context.TODO(), configuration.GetNamespacedName(), configuration)).ToNot(HaveOccurred())

			cond := configuration.Status.GetCondition(corev1alphav1.ConditionReady)
			Expect(cond.Status).To(Equal(metav1.ConditionFalse))
			Expect(cond.Reason).To(Equal(corev1alphav1.ReasonActionRequired))
			Expect(cond.Message).To(Equal("Cost analytics secret (default/token) does not contain a token, contact platform administrator"))
		})

		It("should have raised a event", func() {
			Expect(recorder.Events).To(HaveLen(1))
			Expect(recorder.Events[0]).To(ContainSubstring("Cost analytics secret (default/token) does not contain a token, contact platform administrator"))
		})

		It("should not create any jobs", func() {
			list := &batchv1.JobList{}

			Expect(cc.List(context.TODO(), list, client.InNamespace(ctrl.JobNamespace))).ToNot(HaveOccurred())
			Expect(len(list.Items)).To(Equal(0))
		})
	})

	When("we have multiple matching policy constraints", func() {
		BeforeEach(func() {
			configuration = fixtures.NewValidBucketConfiguration(cfgNamespace, "bucket")
			Setup(configuration)

			Expect(ctrl.cc.Create(context.TODO(), fixtures.NewMatchAllPolicyConstraint("all0"))).ToNot(HaveOccurred())
			Expect(ctrl.cc.Create(context.TODO(), fixtures.NewMatchAllPolicyConstraint("all1"))).ToNot(HaveOccurred())

			result, _, rerr = controllertests.Roll(context.TODO(), ctrl, configuration, 3)
		})

		It("should have conditions", func() {
			Expect(cc.Get(context.TODO(), configuration.GetNamespacedName(), configuration)).ToNot(HaveOccurred())
			Expect(configuration.Status.Conditions).To(HaveLen(defaultConditions))
		})

		It("should indicate the failure on the conditions", func() {
			Expect(cc.Get(context.TODO(), configuration.GetNamespacedName(), configuration)).ToNot(HaveOccurred())

			cond := configuration.Status.GetCondition(terraformv1alphav1.ConditionTerraformPlan)
			Expect(cond.Status).To(Equal(metav1.ConditionFalse))
			Expect(cond.Reason).To(Equal(corev1alphav1.ReasonError))
			Expect(cond.Message).To(Equal("Failed to find matching policy constraints"))
			Expect(cond.Detail).To(Equal("multiple policies match configuration: all0, all1"))
		})

		It("should not create any jobs", func() {
			list := &batchv1.JobList{}

			Expect(cc.List(context.TODO(), list, client.InNamespace(ctrl.JobNamespace))).ToNot(HaveOccurred())
			Expect(len(list.Items)).To(Equal(0))
		})
	})

	When("using static secrets for the provider", func() {
		BeforeEach(func() {
			configuration = fixtures.NewValidBucketConfiguration(cfgNamespace, "bucket")
			Setup(configuration)
			result, _, rerr = controllertests.Roll(context.TODO(), ctrl, configuration, 3)
		})

		It("should have the conditions", func() {
			Expect(cc.Get(context.TODO(), configuration.GetNamespacedName(), configuration)).ToNot(HaveOccurred())
			Expect(configuration.Status.Conditions).To(HaveLen(defaultConditions))
		})

		It("should have created a plan job", func() {
			list := &batchv1.JobList{}

			Expect(cc.List(context.TODO(), list, client.InNamespace(ctrl.JobNamespace))).ToNot(HaveOccurred())
			Expect(len(list.Items)).To(Equal(1))
		})

		It("should be using the default service account", func() {
			list := &batchv1.JobList{}

			Expect(cc.List(context.TODO(), list, client.InNamespace(ctrl.JobNamespace))).ToNot(HaveOccurred())
			Expect(len(list.Items)).To(Equal(1))
			Expect(list.Items[0].Spec.Template.Spec.ServiceAccountName).To(Equal("terraform-executor"))
		})
	})

	When("using a provider with injected identity", func() {
		serviceAccount := "injected-service-account"

		BeforeEach(func() {
			configuration = fixtures.NewValidBucketConfiguration(cfgNamespace, "bucket")
			configuration.Spec.ProviderRef.Name = "injected"

			provider := fixtures.NewValidAWSReadyProvider(ctrl.JobNamespace, configuration.Spec.ProviderRef.Name)
			provider.Spec.Source = terraformv1alphav1.SourceInjected
			provider.Spec.SecretRef = nil
			provider.Spec.ServiceAccount = &serviceAccount

			Setup(configuration, provider)
			result, _, rerr = controllertests.Roll(context.TODO(), ctrl, configuration, 3)
		})

		It("should have the conditions", func() {
			Expect(cc.Get(context.TODO(), configuration.GetNamespacedName(), configuration)).ToNot(HaveOccurred())
			Expect(configuration.Status.Conditions).To(HaveLen(defaultConditions))
		})

		It("should have created a plan job", func() {
			list := &batchv1.JobList{}

			Expect(cc.List(context.TODO(), list, client.InNamespace(ctrl.JobNamespace))).ToNot(HaveOccurred())
			Expect(len(list.Items)).To(Equal(1))
		})

		It("should be using the custom provider identity", func() {
			list := &batchv1.JobList{}

			Expect(cc.List(context.TODO(), list, client.InNamespace(ctrl.JobNamespace))).ToNot(HaveOccurred())
			Expect(len(list.Items)).To(Equal(1))
			Expect(list.Items[0].Spec.Template.Spec.ServiceAccountName).To(Equal(serviceAccount))
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
			Expect(configuration.Status.Conditions).To(HaveLen(defaultConditions))
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

	When("we have a matching policy constraint", func() {
		BeforeEach(func() {
			configuration = fixtures.NewValidBucketConfiguration(cfgNamespace, "bucket")
			Setup(configuration)
			constraint := fixtures.NewMatchAllPolicyConstraint("all")
			constraint.Spec.Constraints.Checkov.Checks = []string{"check0, check1"}
			constraint.Spec.Constraints.Checkov.SkipChecks = nil

			Expect(ctrl.cc.Create(context.TODO(), constraint)).ToNot(HaveOccurred())

			result, _, rerr = controllertests.Roll(context.TODO(), ctrl, configuration, 3)
		})

		It("should have conditions", func() {
			Expect(cc.Get(context.TODO(), configuration.GetNamespacedName(), configuration)).ToNot(HaveOccurred())
			Expect(configuration.Status.Conditions).To(HaveLen(defaultConditions))
		})

		It("should create a terraform plan job", func() {
			list := &batchv1.JobList{}

			Expect(cc.List(context.TODO(), list, client.InNamespace(ctrl.JobNamespace))).ToNot(HaveOccurred())
			Expect(len(list.Items)).To(Equal(1))
		})

		It("should add a verify-policy container to the job", func() {
			list := &batchv1.JobList{}
			expected := `--command=/usr/local/bin/checkov --framework terraform_plan -f /run/plan.json -o json -o cli --check check0 --check check1  --soft-fail --output-file-path /run >/dev/null`

			Expect(cc.List(context.TODO(), list, client.InNamespace(ctrl.JobNamespace))).ToNot(HaveOccurred())
			Expect(len(list.Items)).To(Equal(1))
			job := list.Items[0]

			Expect(len(job.Spec.Template.Spec.Containers)).To(Equal(2))
			Expect(job.Spec.Template.Spec.Containers[1].Name).To(Equal("verify-policy"))
			Expect(job.Spec.Template.Spec.Containers[1].Command).To(Equal([]string{"/run/bin/step"}))
			Expect(len(job.Spec.Template.Spec.Containers[1].Args)).To(Equal(6))
			Expect(job.Spec.Template.Spec.Containers[1].Args[1]).To(Equal(expected))
		})
	})

	When("we have multiple policies but with a heavy priority on one", func() {
		BeforeEach(func() {
			configuration = fixtures.NewValidBucketConfiguration(cfgNamespace, "bucket")
			Setup(configuration)

			// @notes: we add two policies here, the later one with the namespace should take priority
			// given the namespace selector.

			all := fixtures.NewMatchAllPolicyConstraint("all")
			all.Spec.Constraints.Checkov.Checks = []string{"check0"}

			priority := fixtures.NewMatchAllPolicyConstraint("priority")
			priority.Spec.Constraints.Checkov.Checks = []string{"priority"}
			priority.Spec.Constraints.Checkov.Selector = &terraformv1alphav1.Selector{}
			priority.Spec.Constraints.Checkov.Selector.Namespace = &metav1.LabelSelector{
				MatchLabels: map[string]string{"name": cfgNamespace},
			}

			Expect(ctrl.cc.Create(context.TODO(), all)).ToNot(HaveOccurred())
			Expect(ctrl.cc.Create(context.TODO(), priority)).ToNot(HaveOccurred())

			result, _, rerr = controllertests.Roll(context.TODO(), ctrl, configuration, 3)
		})

		It("should have conditions", func() {
			Expect(cc.Get(context.TODO(), configuration.GetNamespacedName(), configuration)).ToNot(HaveOccurred())
			Expect(configuration.Status.Conditions).To(HaveLen(defaultConditions))
		})

		It("should create a terraform plan job", func() {
			list := &batchv1.JobList{}

			Expect(cc.List(context.TODO(), list, client.InNamespace(ctrl.JobNamespace))).ToNot(HaveOccurred())
			Expect(len(list.Items)).To(Equal(1))
		})

		It("should have selected priority policy", func() {
			list := &batchv1.JobList{}
			expected := "--command=/usr/local/bin/checkov --framework terraform_plan -f /run/plan.json -o json -o cli --check priority  --soft-fail --output-file-path /run >/dev/null"

			Expect(cc.List(context.TODO(), list, client.InNamespace(ctrl.JobNamespace))).ToNot(HaveOccurred())
			Expect(len(list.Items)).To(Equal(1))
			job := list.Items[0]

			Expect(len(job.Spec.Template.Spec.Containers)).To(Equal(2))
			Expect(job.Spec.Template.Spec.Containers[1].Name).To(Equal("verify-policy"))
			Expect(job.Spec.Template.Spec.Containers[1].Command).To(Equal([]string{"/run/bin/step"}))
			Expect(len(job.Spec.Template.Spec.Containers[1].Args)).To(Equal(6))
			Expect(job.Spec.Template.Spec.Containers[1].Args[1]).To(Equal(expected))
		})
	})

	When("we have failed a security policy", func() {
		BeforeEach(func() {
			configuration = fixtures.NewValidBucketConfiguration(cfgNamespace, "bucket")

			// @note: we create a policy, we set the terraform plan job to complete and we create
			// a fake security report
			policy := fixtures.NewMatchAllPolicyConstraint("all")
			policy.Spec.Constraints.Checkov.Checks = []string{"check0"}
			plan := fixtures.NewTerraformJob(configuration, ctrl.JobNamespace, terraformv1alphav1.StageTerraformPlan)
			plan.Status.Conditions = []batchv1.JobCondition{{Type: batchv1.JobComplete, Status: v1.ConditionTrue}}
			plan.Status.Succeeded = 1

			report := &v1.Secret{}
			report.Namespace = ctrl.JobNamespace
			report.Name = configuration.GetTerraformPolicySecretName()
			report.Data = map[string][]byte{"results_json.json": []byte(`{"summary":{"failed": 1}}`)}

			Setup(configuration, policy, plan, report)
		})

		When("the policy secret is missing", func() {
			BeforeEach(func() {
				report := &v1.Secret{}
				report.Namespace = ctrl.JobNamespace
				report.Name = configuration.GetTerraformPolicySecretName()
				ctrl.cc.Delete(context.TODO(), report)

				result, _, rerr = controllertests.Roll(context.TODO(), ctrl, configuration, 3)
			})

			It("should have the conditions", func() {
				Expect(cc.Get(context.TODO(), configuration.GetNamespacedName(), configuration)).ToNot(HaveOccurred())
				Expect(configuration.Status.Conditions).To(HaveLen(defaultConditions))
			})

			It("should indicate the we failed", func() {
				Expect(cc.Get(context.TODO(), configuration.GetNamespacedName(), configuration)).ToNot(HaveOccurred())

				cond := configuration.Status.GetCondition(terraformv1alphav1.ConditionTerraformPolicy)
				Expect(cond.Status).To(Equal(metav1.ConditionFalse))
				Expect(cond.Reason).To(Equal(corev1alphav1.ReasonWarning))
				Expect(cond.Message).To(Equal("Failed to find the secret: (default/policy-1234-122-1234-1234) containing checkov scan"))
			})
		})

		When("the policy secret contains failed checks", func() {
			BeforeEach(func() {
				result, _, rerr = controllertests.Roll(context.TODO(), ctrl, configuration, 3)
			})

			It("should have the conditions", func() {
				Expect(cc.Get(context.TODO(), configuration.GetNamespacedName(), configuration)).ToNot(HaveOccurred())
				Expect(configuration.Status.Conditions).To(HaveLen(defaultConditions))
			})

			It("should indicate the we failed", func() {
				Expect(cc.Get(context.TODO(), configuration.GetNamespacedName(), configuration)).ToNot(HaveOccurred())

				cond := configuration.Status.GetCondition(terraformv1alphav1.ConditionTerraformPolicy)
				Expect(cond.Status).To(Equal(metav1.ConditionFalse))
				Expect(cond.Reason).To(Equal(corev1alphav1.ReasonActionRequired))
				Expect(cond.Message).To(Equal("Configuration has failed security policy, refusing to continue"))
			})

			It("should have raised a event", func() {
				Expect(recorder.Events).To(HaveLen(1))
				Expect(recorder.Events[0]).To(ContainSubstring("Configuration has failed security policy, refusing to continue"))
			})

			It("should have not create an apply job", func() {
				list := &batchv1.JobList{}

				Expect(cc.List(context.TODO(), list, client.InNamespace(ctrl.JobNamespace))).ToNot(HaveOccurred())
				Expect(len(list.Items)).To(Equal(1))
			})
		})

		When("the policy contains no fails", func() {
			BeforeEach(func() {
				report := &v1.Secret{}
				report.Namespace = ctrl.JobNamespace
				report.Name = configuration.GetTerraformPolicySecretName()

				// @note: delete the old secret adding a passed one
				Expect(ctrl.cc.Delete(context.TODO(), report)).ToNot(HaveOccurred())
				report.Data = map[string][]byte{"results_json.json": []byte(`{"summary":{"failed": 0}}`)}
				Expect(ctrl.cc.Create(context.TODO(), report)).ToNot(HaveOccurred())

				result, _, rerr = controllertests.Roll(context.TODO(), ctrl, configuration, 3)
			})

			It("should have the conditions", func() {
				Expect(cc.Get(context.TODO(), configuration.GetNamespacedName(), configuration)).ToNot(HaveOccurred())
				Expect(configuration.Status.Conditions).To(HaveLen(defaultConditions))
			})

			It("should indicate the we failed", func() {
				Expect(cc.Get(context.TODO(), configuration.GetNamespacedName(), configuration)).ToNot(HaveOccurred())

				cond := configuration.Status.GetCondition(terraformv1alphav1.ConditionTerraformPolicy)
				Expect(cond.Status).To(Equal(metav1.ConditionTrue))
				Expect(cond.Reason).To(Equal(corev1alphav1.ReasonReady))
				Expect(cond.Message).To(Equal("Passed security checks"))
			})

			It("should have not create an apply job", func() {
				list := &batchv1.JobList{}

				Expect(cc.List(context.TODO(), list, client.InNamespace(ctrl.JobNamespace))).ToNot(HaveOccurred())
				Expect(len(list.Items)).To(Equal(2))
			})
		})
	})

	When("the terraform version should be dictated by the controller", func() {
		BeforeEach(func() {
			configuration = fixtures.NewValidBucketConfiguration(cfgNamespace, "bucket")
			Setup(configuration)
			result, _, rerr = controllertests.Roll(context.TODO(), ctrl, configuration, 3)
		})

		It("should have the conditions", func() {
			Expect(cc.Get(context.TODO(), configuration.GetNamespacedName(), configuration)).ToNot(HaveOccurred())
			Expect(configuration.Status.Conditions).To(HaveLen(defaultConditions))
		})

		It("should have created job for the terraform plan", func() {
			list := &batchv1.JobList{}

			Expect(cc.List(context.TODO(), list, client.InNamespace(ctrl.JobNamespace))).ToNot(HaveOccurred())
			Expect(len(list.Items)).To(Equal(1))
			Expect(list.Items[0].Spec.Template.Spec.Containers[0].Image).To(Equal("hashicorp/terraform:1.1.9"))
		})
	})

	When("the terraform version should override by the configuration", func() {
		BeforeEach(func() {
			configuration = fixtures.NewValidBucketConfiguration(cfgNamespace, "bucket")
			configuration.Spec.TerraformVersion = "test"
			Setup(configuration)
			result, _, rerr = controllertests.Roll(context.TODO(), ctrl, configuration, 3)
		})

		It("should have the conditions", func() {
			Expect(cc.Get(context.TODO(), configuration.GetNamespacedName(), configuration)).ToNot(HaveOccurred())
			Expect(configuration.Status.Conditions).To(HaveLen(defaultConditions))
		})

		It("should have created job for the terraform plan", func() {
			list := &batchv1.JobList{}

			Expect(cc.List(context.TODO(), list, client.InNamespace(ctrl.JobNamespace))).ToNot(HaveOccurred())
			Expect(len(list.Items)).To(Equal(1))
			Expect(list.Items[0].Spec.Template.Spec.Containers[0].Image).To(Equal("hashicorp/terraform:test"))
		})
	})

	When("using a custom job template", func() {
		templateName := "template"

		BeforeEach(func() {
			configuration = fixtures.NewValidBucketConfiguration(cfgNamespace, "bucket")
			Setup(configuration)

			cm := fixtures.NewJobTemplateConfigmap(ctrl.JobNamespace, templateName)
			ctrl.JobTemplate = cm.Name

			Expect(ctrl.cc.Create(context.TODO(), cm)).ToNot(HaveOccurred())
		})

		When("the template is missing", func() {
			BeforeEach(func() {
				ctrl.cc.Delete(context.TODO(), fixtures.NewJobTemplateConfigmap(ctrl.JobNamespace, templateName))
				result, _, rerr = controllertests.Roll(context.TODO(), ctrl, configuration, 3)
			})

			It("should have conditions", func() {
				Expect(cc.Get(context.TODO(), configuration.GetNamespacedName(), configuration)).ToNot(HaveOccurred())
				Expect(configuration.Status.Conditions).To(HaveLen(defaultConditions))
			})

			It("should indicate action is required in the conditions", func() {
				Expect(cc.Get(context.TODO(), configuration.GetNamespacedName(), configuration)).ToNot(HaveOccurred())

				cond := configuration.Status.GetCondition(corev1alphav1.ConditionReady)
				Expect(cond.Status).To(Equal(metav1.ConditionFalse))
				Expect(cond.Reason).To(Equal(corev1alphav1.ReasonActionRequired))
				Expect(cond.Message).To(Equal("Custom job template (default/template) does not exists"))
			})

			It("should have raised a event", func() {
				Expect(recorder.Events).To(HaveLen(1))
				Expect(recorder.Events[0]).To(ContainSubstring("Custom job template (default/template) does not exists"))
			})
		})

		When("the template does not have the correct key", func() {
			BeforeEach(func() {
				// @step: delete the old one and create a new one with missing keys
				cm := fixtures.NewJobTemplateConfigmap(ctrl.JobNamespace, templateName)
				invalid := fixtures.NewJobTemplateConfigmap(ctrl.JobNamespace, templateName)
				invalid.Data = map[string]string{}

				ctrl.cc.Delete(context.TODO(), cm)
				ctrl.cc.Create(context.TODO(), invalid)

				result, _, rerr = controllertests.Roll(context.TODO(), ctrl, configuration, 3)
			})

			It("should have the template", func() {
				cm := fixtures.NewJobTemplateConfigmap(ctrl.JobNamespace, templateName)
				req := types.NamespacedName{Namespace: cm.Namespace, Name: cm.Name}

				Expect(ctrl.cc.Get(context.TODO(), req, &v1.ConfigMap{})).ToNot(HaveOccurred())
			})

			It("should have conditions", func() {
				Expect(cc.Get(context.TODO(), configuration.GetNamespacedName(), configuration)).ToNot(HaveOccurred())
				Expect(configuration.Status.Conditions).To(HaveLen(defaultConditions))
			})

			It("should indicate action is required due to missing keys", func() {
				Expect(cc.Get(context.TODO(), configuration.GetNamespacedName(), configuration)).ToNot(HaveOccurred())

				cond := configuration.Status.GetCondition(corev1alphav1.ConditionReady)
				Expect(cond.Status).To(Equal(metav1.ConditionFalse))
				Expect(cond.Reason).To(Equal(corev1alphav1.ReasonActionRequired))
				Expect(cond.Message).To(Equal("Custom job template (default/template) does not contain the \"job.yaml\" key"))
			})

			It("should have raised a event", func() {
				Expect(recorder.Events).To(HaveLen(1))
				Expect(recorder.Events[0]).To(ContainSubstring("Custom job template (default/template) does not contain the \"job.yaml\" key"))
			})
		})

		When("we have a valid template", func() {
			BeforeEach(func() {
				result, _, rerr = controllertests.Roll(context.TODO(), ctrl, configuration, 3)
			})

			It("should have conditions", func() {
				Expect(cc.Get(context.TODO(), configuration.GetNamespacedName(), configuration)).ToNot(HaveOccurred())
				Expect(configuration.Status.Conditions).To(HaveLen(defaultConditions))
			})

			It("should indicate the terraform plan is running", func() {
				Expect(cc.Get(context.TODO(), configuration.GetNamespacedName(), configuration)).ToNot(HaveOccurred())

				cond := configuration.Status.GetCondition(terraformv1alphav1.ConditionTerraformPlan)
				Expect(cond.Status).To(Equal(metav1.ConditionFalse))
				Expect(cond.Reason).To(Equal(corev1alphav1.ReasonInProgress))
				Expect(cond.Message).To(Equal("Terraform plan in progress"))
			})
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
			Expect(configuration.Status.Conditions).To(HaveLen(defaultConditions))
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
