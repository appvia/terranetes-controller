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
	"fmt"
	"io/ioutil"
	"testing"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/patrickmn/go-cache"
	"github.com/sirupsen/logrus"
	batchv1 "k8s.io/api/batch/v1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	kfake "k8s.io/client-go/kubernetes/fake"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	corev1alphav1 "github.com/appvia/terranetes-controller/pkg/apis/core/v1alpha1"
	terraformv1alphav1 "github.com/appvia/terranetes-controller/pkg/apis/terraform/v1alpha1"
	"github.com/appvia/terranetes-controller/pkg/schema"
	"github.com/appvia/terranetes-controller/pkg/utils/kubernetes"
	controllertests "github.com/appvia/terranetes-controller/test"
	"github.com/appvia/terranetes-controller/test/fixtures"
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

	verifyPolicyArguments := []string{
		"--comment=Evaluating Against Security Policy",
		"--command=/usr/local/bin/checkov --config /run/checkov/checkov.yaml -f /run/plan.json -o json -o cli --output-file-path /run >/dev/null",
		"--command=/bin/cat /run/results_cli.txt",
		"--command=/run/bin/kubectl -n $(KUBE_NAMESPACE) delete secret $(POLICY_REPORT_NAME) --ignore-not-found >/dev/null",
		"--command=/run/bin/kubectl -n $(KUBE_NAMESPACE) create secret generic $(POLICY_REPORT_NAME) --from-file=/run/results_json.json >/dev/null",
		"--is-failure=/run/steps/terraform.failed",
		"--wait-on=/run/steps/terraform.complete",
	}

	Setup := func(objects ...runtime.Object) {
		secret := fixtures.NewValidAWSProviderSecret("default", "aws")
		cc = fake.NewFakeClientWithScheme(schema.GetScheme(), append([]runtime.Object{
			fixtures.NewNamespace(cfgNamespace),
			fixtures.NewValidAWSReadyProvider("aws", secret),
			secret,
		}, objects...)...)
		recorder = &controllertests.FakeRecorder{}
		ctrl = &Controller{
			cc:                  cc,
			kc:                  kfake.NewSimpleClientset(),
			cache:               cache.New(5*time.Minute, 10*time.Minute),
			recorder:            recorder,
			EnableInfracosts:    false,
			EnableWatchers:      true,
			ExecutorImage:       "ghcr.io/appvia/terraform-executor",
			InfracostsImage:     "infracosts/infracost:latest",
			ControllerNamespace: "default",
			PolicyImage:         "bridgecrew/checkov:2.0.1140",
			TerraformImage:      "hashicorp/terraform:1.1.9",
		}
		ctrl.cache.SetDefault(cfgNamespace, fixtures.NewNamespace(cfgNamespace))
	}

	// PROVIDERS
	When("we have provider issues", func() {
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
				Expect(cond.Message).To(Equal("Provider referenced \"does_not_exist\" does not exist"))
			})

			It("should have raised a event", func() {
				Expect(recorder.Events).To(HaveLen(1))
				Expect(recorder.Events[0]).To(ContainSubstring("Provider referenced \"does_not_exist\" does not exist"))
			})

			It("should ask us to requeue", func() {
				Expect(result).To(Equal(reconcile.Result{RequeueAfter: 5 * time.Minute}))
				Expect(rerr).To(BeNil())
			})

			It("should not create any jobs", func() {
				list := &batchv1.JobList{}

				Expect(cc.List(context.TODO(), list, client.InNamespace(ctrl.ControllerNamespace))).ToNot(HaveOccurred())
				Expect(len(list.Items)).To(Equal(0))
			})
		})

		When("provider is not ready", func() {
			BeforeEach(func() {
				configuration = fixtures.NewValidBucketConfiguration(cfgNamespace, "bucket")
				configuration.Spec.ProviderRef.Name = "not_ready"
				Setup(configuration, fixtures.NewValidAWSNotReadyProvider("not_ready", nil))
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

				Expect(cc.List(context.TODO(), list, client.InNamespace(ctrl.ControllerNamespace))).ToNot(HaveOccurred())
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

				Expect(cc.List(context.TODO(), list, client.InNamespace(ctrl.ControllerNamespace))).ToNot(HaveOccurred())
				Expect(len(list.Items)).To(Equal(1))
			})

			It("should be using the default service account", func() {
				list := &batchv1.JobList{}

				Expect(cc.List(context.TODO(), list, client.InNamespace(ctrl.ControllerNamespace))).ToNot(HaveOccurred())
				Expect(len(list.Items)).To(Equal(1))
				Expect(list.Items[0].Spec.Template.Spec.ServiceAccountName).To(Equal("terraform-executor"))
			})
		})

		When("using a provider with injected identity", func() {
			serviceAccount := "injected-service-account"

			BeforeEach(func() {
				configuration = fixtures.NewValidBucketConfiguration(cfgNamespace, "bucket")
				configuration.Spec.ProviderRef.Name = "injected"

				provider := fixtures.NewValidAWSReadyProvider(configuration.Spec.ProviderRef.Name, fixtures.NewValidAWSProviderSecret(ctrl.ControllerNamespace, configuration.Spec.ProviderRef.Name))
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

				Expect(cc.List(context.TODO(), list, client.InNamespace(ctrl.ControllerNamespace))).ToNot(HaveOccurred())
				Expect(len(list.Items)).To(Equal(1))
			})

			It("should be using the custom provider identity", func() {
				list := &batchv1.JobList{}

				Expect(cc.List(context.TODO(), list, client.InNamespace(ctrl.ControllerNamespace))).ToNot(HaveOccurred())
				Expect(len(list.Items)).To(Equal(1))
				Expect(list.Items[0].Spec.Template.Spec.ServiceAccountName).To(Equal(serviceAccount))
			})
		})
	})

	// PROVIDER POLICY
	When("provider has rbac", func() {
		When("policy denies the use of the provider by namespace labels", func() {
			BeforeEach(func() {
				// @note: we create a configuration pointing to a new provider, a provider with a policy and a secret
				// for the provider
				configuration = fixtures.NewValidBucketConfiguration(cfgNamespace, "bucket")
				configuration.Spec.ProviderRef.Name = "policy"

				provider := fixtures.NewValidAWSReadyProvider(configuration.Spec.ProviderRef.Name, fixtures.NewValidAWSProviderSecret(ctrl.ControllerNamespace, configuration.Spec.ProviderRef.Name))
				provider.Spec.Selector = &terraformv1alphav1.Selector{
					Namespace: &metav1.LabelSelector{
						MatchLabels: map[string]string{"does_not_match": "true"},
					},
				}
				secret := fixtures.NewValidAWSProviderSecret(ctrl.ControllerNamespace, configuration.Spec.ProviderRef.Name)

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

				Expect(cc.List(context.TODO(), list, client.InNamespace(ctrl.ControllerNamespace))).ToNot(HaveOccurred())
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

				provider := fixtures.NewValidAWSReadyProvider(configuration.Spec.ProviderRef.Name, fixtures.NewValidAWSProviderSecret(ctrl.ControllerNamespace, configuration.Spec.ProviderRef.Name))
				provider.Spec.Selector = &terraformv1alphav1.Selector{
					Resource: &metav1.LabelSelector{
						MatchLabels: map[string]string{"does_not_match": "false"},
					},
				}
				secret := fixtures.NewValidAWSProviderSecret(ctrl.ControllerNamespace, configuration.Spec.ProviderRef.Name)

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

				Expect(cc.List(context.TODO(), list, client.InNamespace(ctrl.ControllerNamespace))).ToNot(HaveOccurred())
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

				provider := fixtures.NewValidAWSReadyProvider(configuration.Spec.ProviderRef.Name, fixtures.NewValidAWSProviderSecret(ctrl.ControllerNamespace, configuration.Spec.ProviderRef.Name))
				provider.Spec.Selector = &terraformv1alphav1.Selector{
					Resource: &metav1.LabelSelector{
						MatchLabels: map[string]string{"does_match": "true"},
					},
					Namespace: &metav1.LabelSelector{
						MatchLabels: map[string]string{"name": configuration.Namespace},
					},
				}
				secret := fixtures.NewValidAWSProviderSecret(ctrl.ControllerNamespace, configuration.Spec.ProviderRef.Name)

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

				Expect(cc.List(context.TODO(), list, client.InNamespace(ctrl.ControllerNamespace))).ToNot(HaveOccurred())
				Expect(len(list.Items)).To(Equal(1))
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

				cond := configuration.Status.GetCondition(terraformv1alphav1.ConditionTerraformPolicy)
				Expect(cond.Status).To(Equal(metav1.ConditionFalse))
				Expect(cond.Reason).To(Equal(corev1alphav1.ReasonError))
				Expect(cond.Message).To(Equal("Failed to find matching policy constraints"))
				Expect(cond.Detail).To(Equal("multiple policies match configuration: all0, all1"))
			})

			It("should not create any jobs", func() {
				list := &batchv1.JobList{}

				Expect(cc.List(context.TODO(), list, client.InNamespace(ctrl.ControllerNamespace))).ToNot(HaveOccurred())
				Expect(len(list.Items)).To(Equal(0))
			})
		})
	})

	// AUTHENTICATION
	When("configuration has authentication", func() {
		When("the authentication does not exist", func() {
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
				Expect(cond.Message).To(Equal("Authentication secret (spec.auth) does not exist"))
			})

			It("should not create any jobs", func() {
				list := &batchv1.JobList{}

				Expect(cc.List(context.TODO(), list, client.InNamespace(ctrl.ControllerNamespace))).ToNot(HaveOccurred())
				Expect(len(list.Items)).To(Equal(0))
			})

			It("should have raised a event", func() {
				Expect(recorder.Events).ToNot(BeEmpty())
				Expect(recorder.Events[0]).To(ContainSubstring("Authentication secret (spec.auth) does not exist"))
			})
		})

		When("the authentication exists", func() {
			BeforeEach(func() {
				configuration = fixtures.NewValidBucketConfiguration(cfgNamespace, "bucket")

				secret := &v1.Secret{}
				secret.Name = "ssh"
				secret.Namespace = configuration.Namespace
				secret.Data = map[string][]byte{"SSH_AUTH_KEY": []byte("test")}
				configuration.Spec.Auth = &v1.SecretReference{Name: secret.Name}

				Setup(configuration, secret)
				result, _, rerr = controllertests.Roll(context.TODO(), ctrl, configuration, 3)
			})

			It("should have the conditions", func() {
				Expect(cc.Get(context.TODO(), configuration.GetNamespacedName(), configuration)).ToNot(HaveOccurred())
				Expect(configuration.Status.Conditions).To(HaveLen(defaultConditions))
			})

			It("should indicate the plan is in progress", func() {
				Expect(cc.Get(context.TODO(), configuration.GetNamespacedName(), configuration)).ToNot(HaveOccurred())

				cond := configuration.GetCommonStatus().GetCondition(terraformv1alphav1.ConditionTerraformPlan)
				Expect(cond.Status).To(Equal(metav1.ConditionFalse))
				Expect(cond.Reason).To(Equal(corev1alphav1.ReasonInProgress))
				Expect(cond.Message).To(Equal("Terraform plan in progress"))
			})

			It("should have added the secret the job configuration secret", func() {
				secret := &v1.Secret{}
				secret.Namespace = ctrl.ControllerNamespace
				secret.Name = configuration.GetTerraformConfigSecretName()

				found, err := kubernetes.GetIfExists(context.TODO(), cc, secret)
				Expect(err).ToNot(HaveOccurred())
				Expect(found).To(BeTrue())
				Expect(secret.Data).To(HaveKey("SSH_AUTH_KEY"))
			})

			It("should have created a job", func() {
				list := &batchv1.JobList{}

				Expect(cc.List(context.TODO(), list, client.InNamespace(ctrl.ControllerNamespace))).ToNot(HaveOccurred())
				Expect(len(list.Items)).To(Equal(1))
			})
		})
	})

	// TERRAFORM VERSION
	When("configuration has a terraform version", func() {
		When("version is overriding the controller version", func() {
			BeforeEach(func() {
				configuration = fixtures.NewValidBucketConfiguration(cfgNamespace, "bucket")
				configuration.Spec.TerraformVersion = "test"
				Setup(configuration)
				result, _, rerr = controllertests.Roll(context.TODO(), ctrl, configuration, 3)
			})

			It("should have created job for the terraform plan", func() {
				list := &batchv1.JobList{}

				Expect(cc.List(context.TODO(), list, client.InNamespace(ctrl.ControllerNamespace))).ToNot(HaveOccurred())
				Expect(len(list.Items)).To(Equal(1))
				Expect(list.Items[0].Spec.Template.Spec.Containers[0].Image).To(Equal("hashicorp/terraform:test"))
			})
		})

		When("no override is present on the configuration", func() {
			BeforeEach(func() {
				configuration = fixtures.NewValidBucketConfiguration(cfgNamespace, "bucket")
				Setup(configuration)
				result, _, rerr = controllertests.Roll(context.TODO(), ctrl, configuration, 3)
			})

			It("should have created job for the terraform plan", func() {
				list := &batchv1.JobList{}

				Expect(cc.List(context.TODO(), list, client.InNamespace(ctrl.ControllerNamespace))).ToNot(HaveOccurred())
				Expect(len(list.Items)).To(Equal(1))
				Expect(list.Items[0].Spec.Template.Spec.Containers[0].Image).To(Equal("hashicorp/terraform:1.1.9"))
			})
		})

	})

	// COSTS
	When("predicted costs is enabled", func() {
		When("the costs token is missing", func() {
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

				Expect(cc.List(context.TODO(), list, client.InNamespace(ctrl.ControllerNamespace))).ToNot(HaveOccurred())
				Expect(len(list.Items)).To(Equal(0))
			})
		})

		When("cost token is invalid", func() {
			BeforeEach(func() {
				configuration = fixtures.NewValidBucketConfiguration(cfgNamespace, "bucket")
				secret := &v1.Secret{}
				secret.Name = "token"
				secret.Namespace = ctrl.ControllerNamespace

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

				Expect(cc.List(context.TODO(), list, client.InNamespace(ctrl.ControllerNamespace))).ToNot(HaveOccurred())
				Expect(len(list.Items)).To(Equal(0))
			})
		})

		When("configuration has successful run a plan but no cost report", func() {
			BeforeEach(func() {
				configuration = fixtures.NewValidBucketConfiguration(cfgNamespace, "bucket")

				// create two successful plan
				plan := fixtures.NewTerraformJob(configuration, ctrl.ControllerNamespace, terraformv1alphav1.StageTerraformPlan)
				plan.Status.Conditions = []batchv1.JobCondition{{Type: batchv1.JobComplete, Status: v1.ConditionTrue}}
				plan.Status.Succeeded = 1
				// create fake terraform state
				state := fixtures.NewTerraformState(configuration)
				state.Namespace = ctrl.ControllerNamespace
				// create fake costs secret
				token := fixtures.NewCostsSecret(ctrl.ControllerNamespace, "infracost")
				token.Namespace = ctrl.ControllerNamespace

				Setup(configuration, plan, state, token)
				ctrl.EnableInfracosts = true
				ctrl.InfracostsSecretName = "infracost"

				result, _, rerr = controllertests.Roll(context.TODO(), ctrl, configuration, 10)
			})

			It("the cost status should be empty", func() {
				Expect(cc.Get(context.TODO(), configuration.GetNamespacedName(), configuration)).ToNot(HaveOccurred())

				Expect(configuration.Status.Costs).ToNot(BeNil())
				Expect(configuration.Status.Costs.Enabled).To(BeFalse())
				Expect(configuration.Status.Costs.Monthly).To(BeEmpty())
				Expect(configuration.Status.Costs.Hourly).To(BeEmpty())
			})
		})

		When("configuration has successful run a plan", func() {
			BeforeEach(func() {
				configuration = fixtures.NewValidBucketConfiguration(cfgNamespace, "bucket")

				// create two successful plan
				plan := fixtures.NewTerraformJob(configuration, ctrl.ControllerNamespace, terraformv1alphav1.StageTerraformPlan)
				plan.Status.Conditions = []batchv1.JobCondition{{Type: batchv1.JobComplete, Status: v1.ConditionTrue}}
				plan.Status.Succeeded = 1
				// create fake terraform state
				state := fixtures.NewTerraformState(configuration)
				state.Namespace = ctrl.ControllerNamespace
				// create fake costs report
				report := fixtures.NewCostsReport(configuration)
				report.Namespace = ctrl.ControllerNamespace
				// create fake costs secret
				token := fixtures.NewCostsSecret(ctrl.ControllerNamespace, "infracost")
				token.Namespace = ctrl.ControllerNamespace

				Setup(configuration, plan, state, report, token)
				ctrl.EnableInfracosts = true
				ctrl.InfracostsSecretName = "infracost"

				result, _, rerr = controllertests.Roll(context.TODO(), ctrl, configuration, 10)
			})

			It("should indicate the costs is enabled and costs available on the status", func() {
				Expect(cc.Get(context.TODO(), configuration.GetNamespacedName(), configuration)).ToNot(HaveOccurred())

				Expect(configuration.Status.Costs).ToNot(BeNil())
				Expect(configuration.Status.Costs.Enabled).To(BeTrue())
				Expect(configuration.Status.Costs.Monthly).To(Equal("$100"))
				Expect(configuration.Status.Costs.Hourly).To(Equal("$0.01"))
			})

			It("should have copied the secret into the configuration namespace", func() {
				expected := "\n{\n\t\"totalHourlyCost\": \"0.01\",\n  \"totalMonthlyCost\": \"100.00\"\n}\n"
				secret := &v1.Secret{}
				secret.Namespace = configuration.Namespace
				secret.Name = configuration.GetTerraformCostSecretName()
				found, err := kubernetes.GetIfExists(context.TODO(), cc, secret)

				Expect(err).ToNot(HaveOccurred())
				Expect(found).To(BeTrue())
				Expect(secret.Data).To(HaveKey("costs.json"))
				Expect(string(secret.Data["costs.json"])).To(Equal(expected))
			})
		})
	})

	// VALUEFROM FIELDS
	When("the configuration has valueFrom definitions", func() {

		When("using values from the valueFrom field", func() {
			BeforeEach(func() {
				configuration = fixtures.NewValidBucketConfiguration(cfgNamespace, "bucket")
				Setup()
			})

			When("secret is missing and not optional", func() {
				BeforeEach(func() {
					configuration.Spec.ValueFrom = []terraformv1alphav1.ValueFromSource{
						{Secret: "missing", Key: "key"},
					}
					Expect(ctrl.cc.Create(context.TODO(), configuration)).ToNot(HaveOccurred())
					result, _, rerr = controllertests.Roll(context.TODO(), ctrl, configuration, 3)
				})

				It("should have the conditions", func() {
					Expect(cc.Get(context.TODO(), configuration.GetNamespacedName(), configuration)).ToNot(HaveOccurred())
					Expect(configuration.Status.Conditions).To(HaveLen(defaultConditions))
				})

				It("should indicate the failure on the conditions", func() {
					Expect(cc.Get(context.TODO(), configuration.GetNamespacedName(), configuration)).ToNot(HaveOccurred())

					cond := configuration.Status.GetCondition(corev1alphav1.ConditionReady)
					Expect(cond.Status).To(Equal(metav1.ConditionFalse))
					Expect(cond.Reason).To(Equal(corev1alphav1.ReasonActionRequired))
					Expect(cond.Message).To(Equal("Secret spec.valueFrom[0] (apps/missing) does not exist"))
					Expect(cond.Detail).To(Equal(""))
				})

				It("should not create any jobs", func() {
					list := &batchv1.JobList{}

					Expect(cc.List(context.TODO(), list, client.InNamespace(ctrl.ControllerNamespace))).ToNot(HaveOccurred())
					Expect(len(list.Items)).To(Equal(0))
				})
			})

			When("secret is missing but optional", func() {
				BeforeEach(func() {
					configuration.Spec.ValueFrom = []terraformv1alphav1.ValueFromSource{
						{Secret: "missing", Key: "key", Optional: true},
					}
					Expect(ctrl.cc.Create(context.TODO(), configuration)).ToNot(HaveOccurred())
					result, _, rerr = controllertests.Roll(context.TODO(), ctrl, configuration, 3)
				})

				It("should have created a job", func() {
					list := &batchv1.JobList{}
					Expect(cc.List(context.TODO(), list, client.InNamespace(ctrl.ControllerNamespace))).ToNot(HaveOccurred())

					Expect(len(list.Items)).To(Equal(1))
				})
			})

			When("key is missing and not optional", func() {
				BeforeEach(func() {
					secret := &v1.Secret{}
					secret.Namespace = configuration.Namespace
					secret.Name = "exists"

					configuration.Spec.ValueFrom = []terraformv1alphav1.ValueFromSource{{Secret: "exists", Key: "missing"}}
					Expect(ctrl.cc.Create(context.TODO(), configuration)).ToNot(HaveOccurred())
					Expect(ctrl.cc.Create(context.TODO(), secret)).ToNot(HaveOccurred())

					result, _, rerr = controllertests.Roll(context.TODO(), ctrl, configuration, 3)
				})

				It("should have the conditions", func() {
					Expect(cc.Get(context.TODO(), configuration.GetNamespacedName(), configuration)).ToNot(HaveOccurred())
					Expect(configuration.Status.Conditions).To(HaveLen(defaultConditions))
				})

				It("should indicate the failure on the conditions", func() {
					Expect(cc.Get(context.TODO(), configuration.GetNamespacedName(), configuration)).ToNot(HaveOccurred())

					cond := configuration.Status.GetCondition(corev1alphav1.ConditionReady)
					Expect(cond.Status).To(Equal(metav1.ConditionFalse))
					Expect(cond.Reason).To(Equal(corev1alphav1.ReasonActionRequired))
					Expect(cond.Message).To(Equal(`Secret spec.valueFrom[0] (apps/exists) does not contain key: "missing"`))
				})

				It("should not create any jobs", func() {
					list := &batchv1.JobList{}

					Expect(cc.List(context.TODO(), list, client.InNamespace(ctrl.ControllerNamespace))).ToNot(HaveOccurred())
					Expect(len(list.Items)).To(Equal(0))
				})
			})

			When("key is missing but optional", func() {
				BeforeEach(func() {
					configuration.Spec.ValueFrom = []terraformv1alphav1.ValueFromSource{
						{Secret: "missing", Key: "key", Optional: true},
					}
					Expect(ctrl.cc.Create(context.TODO(), configuration)).ToNot(HaveOccurred())
					result, _, rerr = controllertests.Roll(context.TODO(), ctrl, configuration, 3)
				})

				It("should have the conditions", func() {
					Expect(cc.Get(context.TODO(), configuration.GetNamespacedName(), configuration)).ToNot(HaveOccurred())
					Expect(configuration.Status.Conditions).To(HaveLen(defaultConditions))
				})

				It("should have created a job", func() {
					list := &batchv1.JobList{}

					Expect(cc.List(context.TODO(), list, client.InNamespace(ctrl.ControllerNamespace))).ToNot(HaveOccurred())
					Expect(len(list.Items)).To(Equal(1))
				})

				It("should have the conditions", func() {
					Expect(cc.Get(context.TODO(), configuration.GetNamespacedName(), configuration)).ToNot(HaveOccurred())
					Expect(configuration.Status.Conditions).To(HaveLen(defaultConditions))
				})

				It("should have created a job", func() {
					list := &batchv1.JobList{}
					Expect(cc.List(context.TODO(), list, client.InNamespace(ctrl.ControllerNamespace))).ToNot(HaveOccurred())
					Expect(len(list.Items)).To(Equal(1))
				})

			})

			When("secret and key exist", func() {
				BeforeEach(func() {
					secret := &v1.Secret{}
					secret.Namespace = configuration.Namespace
					secret.Name = "exists"
					secret.Data = map[string][]byte{"my": []byte("value")}

					configuration.Spec.ValueFrom = []terraformv1alphav1.ValueFromSource{{Secret: "exists", Key: "my"}}
					Expect(ctrl.cc.Create(context.TODO(), configuration)).ToNot(HaveOccurred())
					Expect(ctrl.cc.Create(context.TODO(), secret)).ToNot(HaveOccurred())

					result, _, rerr = controllertests.Roll(context.TODO(), ctrl, configuration, 5)
				})

				It("should have the conditions", func() {
					Expect(cc.Get(context.TODO(), configuration.GetNamespacedName(), configuration)).ToNot(HaveOccurred())
					Expect(configuration.Status.Conditions).To(HaveLen(defaultConditions))
				})

				It("should indicate the plan is in progress", func() {
					Expect(cc.Get(context.TODO(), configuration.GetNamespacedName(), configuration)).ToNot(HaveOccurred())

					cond := configuration.Status.GetCondition(terraformv1alphav1.ConditionTerraformPlan)
					Expect(cond.Status).To(Equal(metav1.ConditionFalse))
					Expect(cond.Reason).To(Equal(corev1alphav1.ReasonInProgress))
				})

				It("should have created a job", func() {
					list := &batchv1.JobList{}

					Expect(cc.List(context.TODO(), list, client.InNamespace(ctrl.ControllerNamespace))).ToNot(HaveOccurred())
					Expect(len(list.Items)).To(Equal(1))
				})

				It("should have added the value to the configuration config", func() {
					expected := "{\"my\":\"value\",\"name\":\"test\"}\n"

					secret := &v1.Secret{}
					secret.Namespace = ctrl.ControllerNamespace
					secret.Name = configuration.GetTerraformConfigSecretName()

					found, err := kubernetes.GetIfExists(context.TODO(), ctrl.cc, secret)
					Expect(err).ToNot(HaveOccurred())
					Expect(found).To(BeTrue())
					Expect(secret.Data).To(HaveKey(terraformv1alphav1.TerraformVariablesConfigMapKey))
					Expect(string(secret.Data[terraformv1alphav1.TerraformVariablesConfigMapKey])).To(Equal(expected))
				})
			})
		})
	})

	// ADDITIONAL SECRETS
	When("the controller has been configured with additional secrets", func() {
		BeforeEach(func() {
			configuration = fixtures.NewValidBucketConfiguration(cfgNamespace, "bucket")
			Setup(configuration)

			ctrl.ExecutorSecrets = []string{"secret1", "secret2"}
			result, _, rerr = controllertests.Roll(context.TODO(), ctrl, configuration, 3)
		})

		It("should have the conditions", func() {
			Expect(cc.Get(context.TODO(), configuration.GetNamespacedName(), configuration)).ToNot(HaveOccurred())
			Expect(configuration.Status.Conditions).To(HaveLen(defaultConditions))
		})

		It("should indicate the failure on the conditions", func() {
			Expect(cc.Get(context.TODO(), configuration.GetNamespacedName(), configuration)).ToNot(HaveOccurred())

			cond := configuration.Status.GetCondition(terraformv1alphav1.ConditionProviderReady)
			Expect(cond.Status).To(Equal(metav1.ConditionTrue))
			Expect(cond.Reason).To(Equal(corev1alphav1.ReasonReady))
			Expect(cond.Message).To(Equal("Provider ready"))
		})

		It("should have create a plan", func() {
			list := &batchv1.JobList{}

			Expect(cc.List(context.TODO(), list, client.InNamespace(ctrl.ControllerNamespace))).ToNot(HaveOccurred())
			Expect(len(list.Items)).To(Equal(1))
		})

		It("should have the additional secrets added", func() {
			list := &batchv1.JobList{}
			Expect(cc.List(context.TODO(), list, client.InNamespace(ctrl.ControllerNamespace))).ToNot(HaveOccurred())
			Expect(len(list.Items)).To(Equal(1))

			job := list.Items[0]
			Expect(job.Spec.Template.Spec.Containers).To(HaveLen(1))
			Expect(job.Spec.Template.Spec.Containers[0].EnvFrom).To(HaveLen(3))
			Expect(job.Spec.Template.Spec.Containers[0].EnvFrom[0].SecretRef.Name).To(Equal("aws"))

			Expect(job.Spec.Template.Spec.Containers[0].EnvFrom[1].SecretRef.Name).To(Equal("secret1"))
			Expect(job.Spec.Template.Spec.Containers[0].EnvFrom[1].SecretRef.Optional).ToNot(BeNil())
			Expect(*job.Spec.Template.Spec.Containers[0].EnvFrom[1].SecretRef.Optional).To(BeTrue())

			Expect(job.Spec.Template.Spec.Containers[0].EnvFrom[2].SecretRef.Name).To(Equal("secret2"))
			Expect(job.Spec.Template.Spec.Containers[0].EnvFrom[2].SecretRef.Optional).ToNot(BeNil())
			Expect(*job.Spec.Template.Spec.Containers[0].EnvFrom[2].SecretRef.Optional).To(BeTrue())
		})
	})

	When("configuration has not yet run the terraform plan", func() {
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
			secret.Namespace = ctrl.ControllerNamespace
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
			secret.Namespace = ctrl.ControllerNamespace
			secret.Name = configuration.GetTerraformConfigSecretName()

			found, err := kubernetes.GetIfExists(context.TODO(), cc, secret)
			Expect(err).ToNot(HaveOccurred())
			Expect(found).To(BeTrue())

			backend := string(secret.Data[terraformv1alphav1.TerraformBackendConfigMapKey])
			Expect(backend).ToNot(BeZero())
			Expect(backend).To(Equal(expected))
		})

		It("should have a provider.tf", func() {
			expected := "provider \"aws\" {\n}\n"
			secret := &v1.Secret{}
			secret.Namespace = ctrl.ControllerNamespace
			secret.Name = configuration.GetTerraformConfigSecretName()

			found, err := kubernetes.GetIfExists(context.TODO(), cc, secret)
			Expect(err).ToNot(HaveOccurred())
			Expect(found).To(BeTrue())

			backend := string(secret.Data[terraformv1alphav1.TerraformProviderConfigMapKey])
			Expect(backend).ToNot(BeZero())
			Expect(backend).To(Equal(expected))
		})

		It("should have the following variables in the config", func() {
			expected := "{\"name\":\"test\"}\n"

			secret := &v1.Secret{}
			secret.Namespace = ctrl.ControllerNamespace
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

			Expect(cc.List(context.TODO(), list, client.InNamespace(ctrl.ControllerNamespace))).ToNot(HaveOccurred())
			Expect(len(list.Items)).To(Equal(1))
		})

		It("it should have the configuration labels", func() {
			list := &batchv1.JobList{}
			Expect(cc.List(context.TODO(), list, client.InNamespace(ctrl.ControllerNamespace))).ToNot(HaveOccurred())
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

		It("should have a out of sync status", func() {
			Expect(cc.Get(context.TODO(), configuration.GetNamespacedName(), configuration)).ToNot(HaveOccurred())

			Expect(configuration.Status.ResourceStatus).To(Equal(terraformv1alphav1.ResourcesOutOfSync))
		})

		It("should not have a terraform version yet", func() {
			Expect(cc.Get(context.TODO(), configuration.GetNamespacedName(), configuration)).ToNot(HaveOccurred())

			Expect(configuration.Status.TerraformVersion).To(BeEmpty())
		})

		It("should ask us to requeue", func() {
			Expect(result).To(Equal(reconcile.Result{RequeueAfter: 5 * time.Second}))
			Expect(rerr).To(BeNil())
		})
	})

	// DRIFT
	When("configuration has drift options", func() {

		When("drift annotation is tagged but configuration has not opted in for detection", func() {
			BeforeEach(func() {
				configuration = fixtures.NewValidBucketConfiguration(cfgNamespace, "bucket")
				configuration.Annotations = map[string]string{terraformv1alphav1.DriftAnnotation: "true"}

				Setup(configuration)
				result, _, rerr = controllertests.Roll(context.TODO(), ctrl, configuration, 3)
			})

			It("should have the conditions", func() {
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

			It("should have created job for the terraform plan", func() {
				list := &batchv1.JobList{}

				Expect(cc.List(context.TODO(), list, client.InNamespace(ctrl.ControllerNamespace))).ToNot(HaveOccurred())
				Expect(len(list.Items)).To(Equal(1))
			})

			It("it should have the configuration labels", func() {
				list := &batchv1.JobList{}
				Expect(cc.List(context.TODO(), list, client.InNamespace(ctrl.ControllerNamespace))).ToNot(HaveOccurred())
				Expect(len(list.Items)).To(Equal(1))

				labels := list.Items[0].GetLabels()
				Expect(labels).To(HaveKey(terraformv1alphav1.ConfigurationNameLabel))
				Expect(labels).To(HaveKey(terraformv1alphav1.ConfigurationNamespaceLabel))
				Expect(labels).To(HaveKey(terraformv1alphav1.ConfigurationGenerationLabel))
				Expect(labels).To(HaveKey(terraformv1alphav1.ConfigurationStageLabel))
				Expect(labels).To(HaveKey(terraformv1alphav1.DriftAnnotation))
				Expect(labels[terraformv1alphav1.ConfigurationStageLabel]).To(Equal(terraformv1alphav1.StageTerraformPlan))
				Expect(labels[terraformv1alphav1.ConfigurationNameLabel]).To(Equal(configuration.Name))
				Expect(labels[terraformv1alphav1.ConfigurationNamespaceLabel]).To(Equal(configuration.Namespace))
				Expect(labels[terraformv1alphav1.DriftAnnotation]).To(Equal("true"))
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
		})

		When("drift check has already been run", func() {
			BeforeEach(func() {
				configuration = fixtures.NewValidBucketConfiguration(cfgNamespace, "bucket")
				configuration.Spec.EnableDriftDetection = true
				configuration.Annotations = map[string]string{
					terraformv1alphav1.DriftAnnotation: "true",
					terraformv1alphav1.ApplyAnnotation: "false",
				}

				plan := fixtures.NewTerraformJob(configuration, ctrl.ControllerNamespace, terraformv1alphav1.StageTerraformPlan)
				plan.Status.Conditions = []batchv1.JobCondition{{Type: batchv1.JobComplete, Status: v1.ConditionTrue}}
				plan.Status.Succeeded = 1

				Setup(configuration, plan)
				result, _, rerr = controllertests.Roll(context.TODO(), ctrl, configuration, 5)
			})

			It("should not create another job", func() {
				list := &batchv1.JobList{}
				Expect(cc.List(context.TODO(), list, client.InNamespace(ctrl.ControllerNamespace))).ToNot(HaveOccurred())

				Expect(len(list.Items)).To(Equal(1))
			})

			It("should have the conditions", func() {
				Expect(cc.Get(context.TODO(), configuration.GetNamespacedName(), configuration)).ToNot(HaveOccurred())
				Expect(configuration.Status.Conditions).To(HaveLen(defaultConditions))
			})

			It("should indicate the terraform plan is running", func() {
				Expect(cc.Get(context.TODO(), configuration.GetNamespacedName(), configuration)).ToNot(HaveOccurred())

				cond := configuration.Status.GetCondition(terraformv1alphav1.ConditionTerraformApply)
				Expect(cond.Status).To(Equal(metav1.ConditionFalse))
				Expect(cond.Reason).To(Equal(corev1alphav1.ReasonActionRequired))
				Expect(cond.Message).To(Equal("Waiting for terraform apply annotation to be set to true"))
			})
		})

		When("drift annotation changes", func() {
			BeforeEach(func() {
				configuration = fixtures.NewValidBucketConfiguration(cfgNamespace, "bucket")
				configuration.Spec.EnableAutoApproval = true
				configuration.Annotations = map[string]string{terraformv1alphav1.DriftAnnotation: "changed"}

				job := &batchv1.Job{}
				job.Name = "test"
				job.Namespace = ctrl.ControllerNamespace
				job.Labels = map[string]string{
					terraformv1alphav1.ConfigurationGenerationLabel: fmt.Sprintf("%d", configuration.GetGeneration()),
					terraformv1alphav1.ConfigurationNameLabel:       configuration.Name,
					terraformv1alphav1.ConfigurationNamespaceLabel:  configuration.Namespace,
					terraformv1alphav1.ConfigurationStageLabel:      terraformv1alphav1.StageTerraformPlan,
					terraformv1alphav1.ConfigurationUIDLabel:        string(configuration.GetUID()),
					terraformv1alphav1.DriftAnnotation:              "different_before",
				}
				Setup(configuration, job)
				result, _, rerr = controllertests.Roll(context.TODO(), ctrl, configuration, 3)
			})

			It("should indicate the terraform plan is running", func() {
				Expect(cc.Get(context.TODO(), configuration.GetNamespacedName(), configuration)).ToNot(HaveOccurred())

				cond := configuration.Status.GetCondition(terraformv1alphav1.ConditionTerraformPlan)
				Expect(cond.Status).To(Equal(metav1.ConditionFalse))
				Expect(cond.Reason).To(Equal(corev1alphav1.ReasonInProgress))
				Expect(cond.Message).To(Equal("Terraform plan is running"))
			})

			It("should have an out of sync status", func() {
				Expect(cc.Get(context.TODO(), configuration.GetNamespacedName(), configuration)).ToNot(HaveOccurred())
				Expect(configuration.Status.ResourceStatus).To(Equal(terraformv1alphav1.ResourcesOutOfSync))
			})

			It("should create another job", func() {
				list := &batchv1.JobList{}
				Expect(cc.List(context.TODO(), list, client.InNamespace(ctrl.ControllerNamespace))).ToNot(HaveOccurred())
				Expect(len(list.Items)).To(Equal(2))
			})
		})
	})

	// CHECKOV
	When("checkov is configured", func() {
		When("configuration matches a policy", func() {
			BeforeEach(func() {
				configuration = fixtures.NewValidBucketConfiguration(cfgNamespace, "bucket")
				Setup(configuration)
				constraint := fixtures.NewMatchAllPolicyConstraint("all")
				constraint.Spec.Constraints.Checkov.Checks = []string{"check0", "check1"}
				constraint.Spec.Constraints.Checkov.SkipChecks = nil

				Expect(ctrl.cc.Create(context.TODO(), constraint)).ToNot(HaveOccurred())

				result, _, rerr = controllertests.Roll(context.TODO(), ctrl, configuration, 3)
			})

			It("should have conditions", func() {
				Expect(cc.Get(context.TODO(), configuration.GetNamespacedName(), configuration)).ToNot(HaveOccurred())
				Expect(configuration.Status.Conditions).To(HaveLen(defaultConditions))
			})

			It("should created a terraform plan job", func() {
				list := &batchv1.JobList{}

				Expect(cc.List(context.TODO(), list, client.InNamespace(ctrl.ControllerNamespace))).ToNot(HaveOccurred())
				Expect(len(list.Items)).To(Equal(1))
			})

			It("should add a verify-policy container to the job", func() {
				list := &batchv1.JobList{}

				Expect(cc.List(context.TODO(), list, client.InNamespace(ctrl.ControllerNamespace))).ToNot(HaveOccurred())
				Expect(len(list.Items)).To(Equal(1))
				job := list.Items[0]

				Expect(len(job.Spec.Template.Spec.Containers)).To(Equal(2))
				Expect(job.Spec.Template.Spec.Containers[1].Name).To(Equal("verify-policy"))
				Expect(job.Spec.Template.Spec.Containers[1].Command).To(Equal([]string{"/run/bin/step"}))
				Expect(job.Spec.Template.Spec.Containers[1].Args).To(Equal(verifyPolicyArguments))
				Expect(job.Spec.Template.Spec.Containers[1].VolumeMounts).To(HaveLen(3))
				Expect(job.Spec.Template.Spec.Containers[1].VolumeMounts[0].Name).To(Equal("checkov"))
				Expect(job.Spec.Template.Spec.Containers[1].VolumeMounts[0].MountPath).To(Equal("/run/checkov"))
			})

			It("should have a checkov configuration secret", func() {
				secret := &v1.Secret{}
				secret.Namespace = ctrl.ControllerNamespace
				secret.Name = configuration.GetTerraformConfigSecretName()

				found, err := kubernetes.GetIfExists(context.TODO(), ctrl.cc, secret)
				Expect(err).ToNot(HaveOccurred())
				Expect(found).To(BeTrue())
				Expect(secret.Data).To(HaveKey(terraformv1alphav1.CheckovJobTemplateConfigMapKey))
				Expect(string(secret.Data[terraformv1alphav1.CheckovJobTemplateConfigMapKey])).To(Equal("framework:\n  - terraform_plan\nsoft-fail: true\ncompact: true\ncheck:\n  - check0\n  - check1"))
			})
		})

		When("configuration matches multiple policies, highest priority wins", func() {
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

			It("should created a terraform plan job", func() {
				list := &batchv1.JobList{}

				Expect(cc.List(context.TODO(), list, client.InNamespace(ctrl.ControllerNamespace))).ToNot(HaveOccurred())
				Expect(len(list.Items)).To(Equal(1))
			})

			It("should have selected priority policy", func() {
				list := &batchv1.JobList{}

				Expect(cc.List(context.TODO(), list, client.InNamespace(ctrl.ControllerNamespace))).ToNot(HaveOccurred())
				Expect(len(list.Items)).To(Equal(1))
				job := list.Items[0]

				Expect(len(job.Spec.Template.Spec.Containers)).To(Equal(2))
				Expect(job.Spec.Template.Spec.Containers[1].Name).To(Equal("verify-policy"))
				Expect(job.Spec.Template.Spec.Containers[1].Command).To(Equal([]string{"/run/bin/step"}))
				Expect(job.Spec.Template.Spec.Containers[1].Args).To(Equal(verifyPolicyArguments))
			})

			It("should have a checkov configuration secret", func() {
				secret := &v1.Secret{}
				secret.Namespace = ctrl.ControllerNamespace
				secret.Name = configuration.GetTerraformConfigSecretName()

				found, err := kubernetes.GetIfExists(context.TODO(), ctrl.cc, secret)
				Expect(err).ToNot(HaveOccurred())
				Expect(found).To(BeTrue())
				Expect(secret.Data).To(HaveKey(terraformv1alphav1.CheckovJobTemplateConfigMapKey))
				Expect(string(secret.Data[terraformv1alphav1.CheckovJobTemplateConfigMapKey])).To(Equal("framework:\n  - terraform_plan\nsoft-fail: true\ncompact: true\ncheck:\n  - priority"))
			})
		})

		When("we have external checks defined on the security policy", func() {
			BeforeEach(func() {
				configuration = fixtures.NewValidBucketConfiguration(cfgNamespace, "bucket")
				Setup(configuration)

				// @notes: we add a policy with and external check
				external := fixtures.NewMatchAllPolicyConstraint("external")
				external.Spec.Constraints.Checkov.Checks = []string{"check0"}
				external.Spec.Constraints.Checkov.External = []terraformv1alphav1.ExternalCheck{
					{
						Name: "test",
						URL:  "https://example.com//dir",
						SecretRef: &v1.SecretReference{
							Name: "test-secret",
						},
					},
				}

				Expect(ctrl.cc.Create(context.TODO(), external)).ToNot(HaveOccurred())

				result, _, rerr = controllertests.Roll(context.TODO(), ctrl, configuration, 3)
			})

			It("should have conditions", func() {
				Expect(cc.Get(context.TODO(), configuration.GetNamespacedName(), configuration)).ToNot(HaveOccurred())
				Expect(configuration.Status.Conditions).To(HaveLen(defaultConditions))
			})

			It("should create a terraform plan job", func() {
				list := &batchv1.JobList{}

				Expect(cc.List(context.TODO(), list, client.InNamespace(ctrl.ControllerNamespace))).ToNot(HaveOccurred())
				Expect(len(list.Items)).To(Equal(1))
			})

			It("should have an init container retrieving the source", func() {
				list := &batchv1.JobList{}

				Expect(cc.List(context.TODO(), list, client.InNamespace(ctrl.ControllerNamespace))).ToNot(HaveOccurred())
				Expect(len(list.Items)).To(Equal(1))
				Expect(len(list.Items[0].Spec.Template.Spec.InitContainers)).To(Equal(3))

				source := list.Items[0].Spec.Template.Spec.InitContainers[2]
				Expect(source.Name).To(Equal("policy-external-test"))
				Expect(source.Command).To(Equal([]string{"/run/bin/step"}))
				Expect(source.Args).To(Equal([]string{
					"--comment=Retrieve external source for test",
					"--command=/bin/mkdir -p /run/policy",
					"--command=/bin/source --dest=/run/policy/test --source=https://example.com//dir",
				}))
				Expect(source.EnvFrom).To(Equal([]v1.EnvFromSource{
					{
						SecretRef: &v1.SecretEnvSource{
							LocalObjectReference: v1.LocalObjectReference{Name: "test-secret"},
							Optional:             nil,
						},
					},
				}))
				Expect(len(source.VolumeMounts)).To(Equal(1))
				Expect(source.VolumeMounts[0].Name).To(Equal("run"))
			})

			It("should have updated the command line for checkov scan", func() {
				list := &batchv1.JobList{}

				Expect(cc.List(context.TODO(), list, client.InNamespace(ctrl.ControllerNamespace))).ToNot(HaveOccurred())
				Expect(len(list.Items)).To(Equal(1))
				job := list.Items[0]

				Expect(len(job.Spec.Template.Spec.Containers)).To(Equal(2))
				Expect(job.Spec.Template.Spec.Containers[1].Name).To(Equal("verify-policy"))
				Expect(job.Spec.Template.Spec.Containers[1].Command).To(Equal([]string{"/run/bin/step"}))
				Expect(job.Spec.Template.Spec.Containers[1].Args).To(Equal([]string{
					"--comment=Evaluating Against Security Policy",
					"--command=/usr/local/bin/checkov --config /run/checkov/checkov.yaml -f /run/plan.json -o json -o cli --output-file-path /run >/dev/null",
					"--command=/bin/cat /run/results_cli.txt",
					"--command=/run/bin/kubectl -n $(KUBE_NAMESPACE) delete secret $(POLICY_REPORT_NAME) --ignore-not-found >/dev/null",
					"--command=/run/bin/kubectl -n $(KUBE_NAMESPACE) create secret generic $(POLICY_REPORT_NAME) --from-file=/run/results_json.json >/dev/null",
					"--is-failure=/run/steps/terraform.failed",
					"--wait-on=/run/steps/terraform.complete",
				}))
			})

			It("should have a checkov configuration secret", func() {
				secret := &v1.Secret{}
				secret.Namespace = ctrl.ControllerNamespace
				secret.Name = configuration.GetTerraformConfigSecretName()

				found, err := kubernetes.GetIfExists(context.TODO(), ctrl.cc, secret)
				Expect(err).ToNot(HaveOccurred())
				Expect(found).To(BeTrue())
				Expect(secret.Data).To(HaveKey(terraformv1alphav1.CheckovJobTemplateConfigMapKey))
				Expect(string(secret.Data[terraformv1alphav1.CheckovJobTemplateConfigMapKey])).To(Equal("framework:\n  - terraform_plan\nsoft-fail: true\ncompact: true\ncheck:\n  - check0\nexternal-checks-dir:\n  - /run/policy/test"))
			})
		})

		When("configuration has matched a policy", func() {
			BeforeEach(func() {
				configuration = fixtures.NewValidBucketConfiguration(cfgNamespace, "bucket")

				// @note: we create a policy, we set the terraform plan job to complete and we create
				// a fake security report
				policy := fixtures.NewMatchAllPolicyConstraint("all")
				policy.Spec.Constraints.Checkov.Checks = []string{"check0"}
				plan := fixtures.NewTerraformJob(configuration, ctrl.ControllerNamespace, terraformv1alphav1.StageTerraformPlan)
				plan.Status.Conditions = []batchv1.JobCondition{{Type: batchv1.JobComplete, Status: v1.ConditionTrue}}
				plan.Status.Succeeded = 1

				report := &v1.Secret{}
				report.Namespace = ctrl.ControllerNamespace
				report.Name = configuration.GetTerraformPolicySecretName()
				report.Data = map[string][]byte{"results_json.json": []byte(`{"summary":{"failed": 1}}`)}

				Setup(configuration, policy, plan, report)
			})

			When("policy report is missing due to interval error", func() {
				BeforeEach(func() {
					report := &v1.Secret{}
					report.Namespace = ctrl.ControllerNamespace
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

			When("policy report contains failed checks", func() {
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

					Expect(cc.List(context.TODO(), list, client.InNamespace(ctrl.ControllerNamespace))).ToNot(HaveOccurred())
					Expect(len(list.Items)).To(Equal(1))
				})

				It("should copied the report into the configuration namespace", func() {
					secret := &v1.Secret{}
					secret.Namespace = configuration.Namespace
					secret.Name = configuration.GetTerraformPolicySecretName()
					found, err := kubernetes.GetIfExists(context.TODO(), ctrl.cc, secret)
					Expect(err).ToNot(HaveOccurred())
					Expect(found).To(BeTrue())
					Expect(secret.Data).To(HaveKey("results_json.json"))
					Expect(string(secret.Data["results_json.json"])).To(ContainSubstring("summary"))
				})
			})

			When("policy report contains no failed checks", func() {
				BeforeEach(func() {
					report := &v1.Secret{}
					report.Namespace = ctrl.ControllerNamespace
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

					Expect(cc.List(context.TODO(), list, client.InNamespace(ctrl.ControllerNamespace))).ToNot(HaveOccurred())
					Expect(len(list.Items)).To(Equal(2))
				})

				It("should copied the report into the configuration namespace", func() {
					secret := &v1.Secret{}
					secret.Namespace = configuration.Namespace
					secret.Name = configuration.GetTerraformPolicySecretName()
					found, err := kubernetes.GetIfExists(context.TODO(), ctrl.cc, secret)
					Expect(err).ToNot(HaveOccurred())
					Expect(found).To(BeTrue())
					Expect(secret.Data).To(HaveKey("results_json.json"))
					Expect(string(secret.Data["results_json.json"])).To(ContainSubstring("summary"))
				})
			})
		})
	})

	When("using a custom job template", func() {
		templateName := "template"

		BeforeEach(func() {
			configuration = fixtures.NewValidBucketConfiguration(cfgNamespace, "bucket")
			Setup(configuration)

			cm := fixtures.NewJobTemplateConfigmap(ctrl.ControllerNamespace, templateName)
			ctrl.JobTemplate = cm.Name

			Expect(ctrl.cc.Create(context.TODO(), cm)).ToNot(HaveOccurred())
		})

		When("the template is missing", func() {
			BeforeEach(func() {
				ctrl.cc.Delete(context.TODO(), fixtures.NewJobTemplateConfigmap(ctrl.ControllerNamespace, templateName))
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
				cm := fixtures.NewJobTemplateConfigmap(ctrl.ControllerNamespace, templateName)
				invalid := fixtures.NewJobTemplateConfigmap(ctrl.ControllerNamespace, templateName)
				invalid.Data = map[string]string{}

				ctrl.cc.Delete(context.TODO(), cm)
				ctrl.cc.Create(context.TODO(), invalid)

				result, _, rerr = controllertests.Roll(context.TODO(), ctrl, configuration, 3)
			})

			It("should have the template", func() {
				cm := fixtures.NewJobTemplateConfigmap(ctrl.ControllerNamespace, templateName)
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

	// BEFORE TERRAFORM APPLY
	When("terraform apply has not been provisoned", func() {
		When("the configuration needs approval", func() {
			BeforeEach(func() {
				configuration = fixtures.NewValidBucketConfiguration(cfgNamespace, "bucket")
				configuration.Annotations = map[string]string{terraformv1alphav1.ApplyAnnotation: "false"}
				plan := fixtures.NewTerraformJob(configuration, ctrl.ControllerNamespace, terraformv1alphav1.StageTerraformPlan)
				plan.Status.Conditions = []batchv1.JobCondition{{Type: batchv1.JobComplete, Status: v1.ConditionTrue}}
				plan.Status.Succeeded = 1

				Setup(configuration, plan)
				result, _, rerr = controllertests.Roll(context.TODO(), ctrl, configuration, 5)
			})

			It("should have the conditions", func() {
				Expect(cc.Get(context.TODO(), configuration.GetNamespacedName(), configuration)).ToNot(HaveOccurred())
				Expect(configuration.Status.Conditions).To(HaveLen(defaultConditions))
			})

			It("should indicate configuration is waiting approval", func() {
				Expect(cc.Get(context.TODO(), configuration.GetNamespacedName(), configuration)).ToNot(HaveOccurred())

				cond := configuration.Status.GetCondition(terraformv1alphav1.ConditionTerraformApply)
				Expect(cond.Status).To(Equal(metav1.ConditionFalse))
				Expect(cond.Reason).To(Equal(corev1alphav1.ReasonActionRequired))
				Expect(cond.Message).To(Equal("Waiting for terraform apply annotation to be set to true"))
			})

			It("should have raised a kubernetes event", func() {
				Expect(recorder.Events).To(HaveLen(1))
				Expect(recorder.Events[0]).To(Equal("(apps/bucket) Warning Action Required: Waiting for terraform apply annotation to be set to true"))
			})

			It("should not have created a job", func() {
				list := &batchv1.JobList{}
				Expect(cc.List(context.TODO(), list, client.InNamespace(ctrl.ControllerNamespace))).ToNot(HaveOccurred())
				Expect(len(list.Items)).To(Equal(1))
			})
		})

		When("the configuration is approved", func() {
			BeforeEach(func() {
				configuration = fixtures.NewValidBucketConfiguration(cfgNamespace, "bucket")
				plan := fixtures.NewTerraformJob(configuration, ctrl.ControllerNamespace, terraformv1alphav1.StageTerraformPlan)
				plan.Status.Conditions = []batchv1.JobCondition{{Type: batchv1.JobComplete, Status: v1.ConditionTrue}}
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

				Expect(cc.List(context.TODO(), list, client.InNamespace(ctrl.ControllerNamespace))).ToNot(HaveOccurred())
				Expect(len(list.Items)).To(Equal(2))
			})

			It("it should have the configuration labels", func() {
				list := &batchv1.JobList{}
				Expect(cc.List(context.TODO(), list, client.InNamespace(ctrl.ControllerNamespace))).ToNot(HaveOccurred())

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

	// AFTER SUCCESSFUL APPLY
	When("terraform apply has been provisioned", func() {
		BeforeEach(func() {
			configuration = fixtures.NewValidBucketConfiguration(cfgNamespace, "bucket")
			// create two successful jobs
			plan := fixtures.NewTerraformJob(configuration, ctrl.ControllerNamespace, terraformv1alphav1.StageTerraformPlan)
			plan.Status.Conditions = []batchv1.JobCondition{{Type: batchv1.JobComplete, Status: v1.ConditionTrue}}
			plan.Status.Succeeded = 1

			apply := fixtures.NewTerraformJob(configuration, ctrl.ControllerNamespace, terraformv1alphav1.StageTerraformApply)
			apply.Status.Conditions = []batchv1.JobCondition{{Type: batchv1.JobComplete, Status: v1.ConditionTrue}}
			apply.Status.Succeeded = 1

			// create a fake terraform state
			state := fixtures.NewTerraformState(configuration)
			state.Namespace = "default"

			Setup(configuration, plan, apply, state)
			result, _, rerr = controllertests.Roll(context.TODO(), ctrl, configuration, 3)
		})

		It("should have the conditions", func() {
			Expect(cc.Get(context.TODO(), configuration.GetNamespacedName(), configuration)).ToNot(HaveOccurred())
			Expect(configuration.Status.Conditions).To(HaveLen(defaultConditions))
		})

		It("should indicate the terraform apply has run", func() {
			Expect(cc.Get(context.TODO(), configuration.GetNamespacedName(), configuration)).ToNot(HaveOccurred())

			cond := configuration.Status.GetCondition(terraformv1alphav1.ConditionTerraformApply)
			Expect(cond.Status).To(Equal(metav1.ConditionTrue))
			Expect(cond.Reason).To(Equal(corev1alphav1.ReasonReady))
			Expect(cond.Message).To(Equal("Terraform apply is complete"))
		})

		It("should a last reconciliation time on the status", func() {
			Expect(cc.Get(context.TODO(), configuration.GetNamespacedName(), configuration)).ToNot(HaveOccurred())

			Expect(configuration.Status.LastReconcile).ToNot(BeNil())
			Expect(configuration.Status.LastReconcile.Time).ToNot(BeNil())
			Expect(configuration.Status.LastReconcile.Generation).To(Equal(int64(0)))
		})

		It("should a last success reconciliation time on the status", func() {
			Expect(cc.Get(context.TODO(), configuration.GetNamespacedName(), configuration)).ToNot(HaveOccurred())

			Expect(configuration.Status.LastSuccess).ToNot(BeNil())
			Expect(configuration.Status.LastSuccess.Time).ToNot(BeNil())
			Expect(configuration.Status.LastSuccess.Generation).To(Equal(int64(0)))
		})

		It("should have a version on status", func() {
			Expect(cc.Get(context.TODO(), configuration.GetNamespacedName(), configuration)).ToNot(HaveOccurred())
			Expect(configuration.Status.TerraformVersion).To(Equal("1.1.9"))
		})

		It("should have a resource count on the status", func() {
			Expect(cc.Get(context.TODO(), configuration.GetNamespacedName(), configuration)).ToNot(HaveOccurred())
			Expect(configuration.Status.Resources).To(Equal(1))
		})

		It("should have a in resource status", func() {
			Expect(cc.Get(context.TODO(), configuration.GetNamespacedName(), configuration)).ToNot(HaveOccurred())

			Expect(configuration.Status.ResourceStatus).To(Equal(terraformv1alphav1.ResourcesInSync))
		})

		It("should have created a secret containing the module output", func() {
			Expect(cc.Get(context.TODO(), configuration.GetNamespacedName(), configuration)).ToNot(HaveOccurred())

			secret := &v1.Secret{}
			secret.Name = configuration.Spec.WriteConnectionSecretToRef.Name
			secret.Namespace = configuration.Namespace

			found, err := kubernetes.GetIfExists(context.TODO(), cc, secret)
			Expect(err).ToNot(HaveOccurred())
			Expect(found).To(BeTrue())
			Expect(secret.Data).ToNot(BeNil())
			Expect(secret.Data).To(HaveKey("TEST_OUTPUT"))
			Expect(secret.Data["TEST_OUTPUT"]).To(Equal([]byte("test")))
		})
	})

	// SECRET KEY MAPPINGS
	When("we have secret key mappings on the configuration", func() {
		BeforeEach(func() {
			configuration = fixtures.NewValidBucketConfiguration(cfgNamespace, "bucket")
			configuration.Spec.WriteConnectionSecretToRef.Keys = []string{
				"test_output:mysql_host",
			}

			// create two successful jobs
			plan := fixtures.NewTerraformJob(configuration, ctrl.ControllerNamespace, terraformv1alphav1.StageTerraformPlan)
			plan.Status.Conditions = []batchv1.JobCondition{{Type: batchv1.JobComplete, Status: v1.ConditionTrue}}
			plan.Status.Succeeded = 1

			apply := fixtures.NewTerraformJob(configuration, ctrl.ControllerNamespace, terraformv1alphav1.StageTerraformApply)
			apply.Status.Conditions = []batchv1.JobCondition{{Type: batchv1.JobComplete, Status: v1.ConditionTrue}}
			apply.Status.Succeeded = 1

			// create a fake terraform state
			state := fixtures.NewTerraformState(configuration)
			state.Namespace = "default"

			Setup(configuration, plan, apply, state)
			result, _, rerr = controllertests.Roll(context.TODO(), ctrl, configuration, 3)
		})

		It("should have the conditions", func() {
			Expect(cc.Get(context.TODO(), configuration.GetNamespacedName(), configuration)).ToNot(HaveOccurred())
			Expect(configuration.Status.Conditions).To(HaveLen(defaultConditions))
		})

		It("should indicate the terraform apply has run", func() {
			Expect(cc.Get(context.TODO(), configuration.GetNamespacedName(), configuration)).ToNot(HaveOccurred())

			cond := configuration.Status.GetCondition(terraformv1alphav1.ConditionTerraformApply)
			Expect(cond.Status).To(Equal(metav1.ConditionTrue))
			Expect(cond.Reason).To(Equal(corev1alphav1.ReasonReady))
			Expect(cond.Message).To(Equal("Terraform apply is complete"))
		})

		It("should have a in resource status", func() {
			Expect(cc.Get(context.TODO(), configuration.GetNamespacedName(), configuration)).ToNot(HaveOccurred())

			Expect(configuration.Status.ResourceStatus).To(Equal(terraformv1alphav1.ResourcesInSync))
		})

		It("should have created a secret containing the module output", func() {
			Expect(cc.Get(context.TODO(), configuration.GetNamespacedName(), configuration)).ToNot(HaveOccurred())

			secret := &v1.Secret{}
			secret.Name = configuration.Spec.WriteConnectionSecretToRef.Name
			secret.Namespace = configuration.Namespace

			found, err := kubernetes.GetIfExists(context.TODO(), cc, secret)
			Expect(err).ToNot(HaveOccurred())
			Expect(found).To(BeTrue())
			Expect(secret.Data).ToNot(BeNil())
			Expect(secret.Data).To(HaveKey("MYSQL_HOST"))
			Expect(secret.Data["MYSQL_HOST"]).To(Equal([]byte("test")))
		})
	})
})
