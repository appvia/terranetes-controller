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
	"io"
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
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	corev1alpha1 "github.com/appvia/terranetes-controller/pkg/apis/core/v1alpha1"
	terraformv1alpha1 "github.com/appvia/terranetes-controller/pkg/apis/terraform/v1alpha1"
	"github.com/appvia/terranetes-controller/pkg/controller"
	"github.com/appvia/terranetes-controller/pkg/schema"
	"github.com/appvia/terranetes-controller/pkg/utils/kubernetes"
	controllertests "github.com/appvia/terranetes-controller/test"
	"github.com/appvia/terranetes-controller/test/fixtures"
)

func TestController(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Running Test Suite")
}

func makeFakeController(cc client.Client) *Controller {
	recorder := &controllertests.FakeRecorder{}
	ctrl := &Controller{
		cc:                           cc,
		kc:                           kfake.NewSimpleClientset(),
		cache:                        cache.New(5*time.Minute, 10*time.Minute),
		recorder:                     recorder,
		BackoffLimit:                 2,
		BinaryPath:                   "/usr/local/bin/tofu",
		ControllerNamespace:          "terraform-system",
		DefaultExecutorCPULimit:      "1",
		DefaultExecutorCPURequest:    "5m",
		DefaultExecutorMemoryLimit:   "1Gi",
		DefaultExecutorMemoryRequest: "32Mi",
		EnableInfracosts:             false,
		EnableWatchers:               true,
		ExecutorImage:                "ghcr.io/appvia/terranetes-executor",
		InfracostsImage:              "infracosts/infracost:latest",
		PolicyImage:                  "bridgecrew/checkov:2.0.1140",
		TerraformImage:               "ghcr.io/opentofu/opentofu:latest",
	}

	return ctrl
}

var _ = Describe("Configuration Controller Default Injection", func() {
	logrus.SetOutput(io.Discard)

	var cc client.Client
	var result reconcile.Result
	var rerr error
	var ctrl *Controller
	var configuration *terraformv1alpha1.Configuration
	var recorder *controllertests.FakeRecorder

	namespace := "default"

	BeforeEach(func() {
		secret := fixtures.NewValidAWSProviderSecret(namespace, "aws")
		configuration = fixtures.NewValidBucketConfiguration(namespace, "test")
		configuration.Spec.EnableAutoApproval = true

		cc = fake.NewClientBuilder().
			WithScheme(schema.GetScheme()).
			WithStatusSubresource(&terraformv1alpha1.Configuration{}).
			WithRuntimeObjects(
				append([]runtime.Object{
					fixtures.NewValidAWSReadyProvider("aws", secret),
					secret,
					configuration,
				})...).
			Build()

		ctrl = makeFakeController(cc)
		recorder = ctrl.recorder.(*controllertests.FakeRecorder)
		ctrl.cache.SetDefault(namespace, fixtures.NewNamespace(namespace))
	})

	When("creating a configuration", func() {
		var policy *terraformv1alpha1.Policy

		BeforeEach(func() {
			// create a default policy
			policy = fixtures.NewPolicy("defaults")
			policy.Spec.Defaults = []terraformv1alpha1.DefaultVariables{
				{
					Secrets: []string{"test"},
				},
			}
			Expect(cc.Create(context.Background(), policy)).To(Succeed())

			// create a secret
			secret := &v1.Secret{}
			secret.Namespace = ctrl.ControllerNamespace
			secret.Name = policy.Spec.Defaults[0].Secrets[0] // from above
			Expect(cc.Create(context.Background(), secret)).To(Succeed())
		})

		Context("and the configuration and the number of running jobs has exceeded the threshold", func() {
			running := 3

			BeforeEach(func() {
				ctrl.ConfigurationThreshold = 0.2

				for i := 1; i <= running; i++ {
					cfg := fixtures.NewValidBucketConfiguration(namespace, fmt.Sprintf("test-%d", i))
					Expect(cc.Create(context.Background(), cfg)).To(Succeed())

					job := fixtures.NewRunningTerraformJob(cfg, terraformv1alpha1.StageTerraformApply)
					job.Namespace = ctrl.ControllerNamespace
					Expect(cc.Create(context.Background(), job)).To(Succeed())
				}

				result, _, rerr = controllertests.Roll(context.TODO(), ctrl, configuration, 0)
			})

			It("should not error", func() {
				Expect(rerr).To(BeNil())
			})

			It("should not create a plan job", func() {
				list := &batchv1.JobList{}
				Expect(cc.List(context.TODO(), list, client.InNamespace(ctrl.ControllerNamespace))).ToNot(HaveOccurred())
				Expect(len(list.Items)).To(Equal(running))
			})

			It("should not have deferred the configuration", func() {
				Expect(result.Requeue).To(BeFalse())
				Expect(result.RequeueAfter).To(Equal(30 * time.Second))
			})

			It("Should have updated the configuration status", func() {
				Expect(cc.Get(context.TODO(), configuration.GetNamespacedName(), configuration)).ToNot(HaveOccurred())
				cond := configuration.Status.GetCondition(corev1alpha1.ConditionReady)
				Expect(cond.Status).To(Equal(metav1.ConditionFalse))
				Expect(cond.Reason).To(Equal(string(corev1alpha1.ReasonWarning)))
				Expect(cond.Message).To(Equal("Configuration is over the threshold for running configurations, waiting in queue"))
			})
		})

		Context("and the configuration and the number of running jobs does not exceeded the threshold", func() {
			running := 5
			total := 10

			BeforeEach(func() {
				ctrl.ConfigurationThreshold = 0.8

				// create 10 configurations, of which 5 are running
				for i := 1; i <= total; i++ {
					cfg := fixtures.NewValidBucketConfiguration(namespace, fmt.Sprintf("test-%d", i))
					Expect(cc.Create(context.Background(), cfg)).To(Succeed())
				}

				for i := 1; i <= running; i++ {
					cfg := fixtures.NewValidBucketConfiguration(namespace, fmt.Sprintf("test-%d", i))
					job := fixtures.NewRunningTerraformJob(cfg, terraformv1alpha1.StageTerraformApply)
					job.Namespace = ctrl.ControllerNamespace

					Expect(cc.Create(context.Background(), job)).To(Succeed())
				}

				result, _, rerr = controllertests.Roll(context.TODO(), ctrl, configuration, 0)
			})

			It("should not error", func() {
				Expect(rerr).To(BeNil())
			})

			It("should not create a plan job", func() {
				list := &batchv1.JobList{}
				Expect(cc.List(context.TODO(), list, client.InNamespace(ctrl.ControllerNamespace))).ToNot(HaveOccurred())
				Expect(len(list.Items)).To(Equal(running + 1))
			})

			It("should not have deferred the configuration", func() {
				Expect(result.Requeue).To(BeFalse())
				Expect(result.RequeueAfter).To(Equal(10 * time.Second))
			})

			It("Should have updated the configuration status", func() {
				Expect(cc.Get(context.TODO(), configuration.GetNamespacedName(), configuration)).ToNot(HaveOccurred())
				cond := configuration.Status.GetCondition(terraformv1alpha1.ConditionTerraformPlan)
				Expect(cond.Status).To(Equal(metav1.ConditionFalse))
				Expect(cond.Reason).To(Equal(string(corev1alpha1.ReasonInProgress)))
				Expect(cond.Message).To(Equal("Terraform plan is running"))
			})
		})

		Context("and the configuration and the number of jobs does not exceeded the threshold", func() {
			running := 6
			stopped := 4
			total := 10

			BeforeEach(func() {
				ctrl.ConfigurationThreshold = 0.8

				// create 10 configurations, of which 5 are running
				for i := 1; i <= total; i++ {
					cfg := fixtures.NewValidBucketConfiguration(namespace, fmt.Sprintf("test-%d", i))
					Expect(cc.Create(context.Background(), cfg)).To(Succeed())
				}

				for i := 1; i <= stopped; i++ {
					cfg := fixtures.NewValidBucketConfiguration(namespace, fmt.Sprintf("test-%d", i))
					job := fixtures.NewCompletedTerraformJob(cfg, terraformv1alpha1.StageTerraformPlan)
					job.Namespace = ctrl.ControllerNamespace

					Expect(cc.Create(context.Background(), job)).To(Succeed())
				}

				for i := 1; i <= running; i++ {
					cfg := fixtures.NewValidBucketConfiguration(namespace, fmt.Sprintf("test-%d", i))
					job := fixtures.NewRunningTerraformJob(cfg, terraformv1alpha1.StageTerraformApply)
					job.Namespace = ctrl.ControllerNamespace

					Expect(cc.Create(context.Background(), job)).To(Succeed())
				}

				result, _, rerr = controllertests.Roll(context.TODO(), ctrl, configuration, 0)
			})

			It("should not error", func() {
				Expect(rerr).To(BeNil())
			})

			It("should not create a plan job", func() {
				list := &batchv1.JobList{}
				Expect(cc.List(context.TODO(), list, client.InNamespace(ctrl.ControllerNamespace))).ToNot(HaveOccurred())
				Expect(len(list.Items)).To(Equal(running + stopped + 1))
			})

			It("should not have deferred the configuration", func() {
				Expect(result.Requeue).To(BeFalse())
				Expect(result.RequeueAfter).To(Equal(10 * time.Second))
			})

			It("Should have updated the configuration status", func() {
				Expect(cc.Get(context.TODO(), configuration.GetNamespacedName(), configuration)).ToNot(HaveOccurred())
				cond := configuration.Status.GetCondition(terraformv1alpha1.ConditionTerraformPlan)
				Expect(cond.Status).To(Equal(metav1.ConditionFalse))
				Expect(cond.Reason).To(Equal(string(corev1alpha1.ReasonInProgress)))
				Expect(cond.Message).To(Equal("Terraform plan is running"))
			})
		})

		Context("and the referenced secret does not exist", func() {
			BeforeEach(func() {
				secret := &v1.Secret{}
				secret.Namespace = ctrl.ControllerNamespace
				secret.Name = "test"
				Expect(cc.Delete(context.Background(), secret)).To(Succeed())

				result, _, rerr = controllertests.Roll(context.TODO(), ctrl, configuration, 0)
			})

			It("should ask to requeue", func() {
				Expect(rerr).To(BeNil())
				Expect(result.Requeue).To(BeFalse())
				Expect(result.RequeueAfter).To(Equal(5 * time.Minute))
			})

			It("should have recorded an event", func() {
				Expect(recorder.Events).ToNot(BeEmpty())
				Expect(recorder.Events[0]).To(ContainSubstring(""))
			})

			It("should indicate on the conditions the missing secret", func() {
				Expect(cc.Get(context.TODO(), configuration.GetNamespacedName(), configuration)).ToNot(HaveOccurred())

				cond := configuration.Status.GetCondition(corev1alpha1.ConditionReady)
				Expect(cond.Status).To(Equal(metav1.ConditionFalse))
				Expect(cond.Reason).To(Equal(corev1alpha1.ReasonActionRequired))
				Expect(cond.Message).To(Equal("Default secret (terraform-system/test) does not exist, please contact administrator"))
			})
		})

		Context("and the configuration we have a namespace label selector", func() {
			BeforeEach(func() {
				// ensure we are in the cache
				namespace := fixtures.NewNamespace(configuration.Namespace)
				namespace.Labels = map[string]string{"name": "match"}
				ctrl.cache.SetDefault(namespace.Name, namespace)

				/// update the policy to using a namespace label selector
				Expect(cc.Delete(context.Background(), policy.DeepCopy())).To(Succeed())
				policy = fixtures.NewPolicy(policy.Name)
				policy.Spec.Defaults = []terraformv1alpha1.DefaultVariables{
					{
						Selector: terraformv1alpha1.DefaultVariablesSelector{
							Namespace: &metav1.LabelSelector{
								MatchLabels: map[string]string{"name": "match"},
							},
						},
						Secrets: []string{"test"},
					},
				}
				Expect(cc.Create(context.Background(), policy)).To(Succeed())
			})

			Context("and the configuration namespace label matches", func() {
				BeforeEach(func() {
					result, _, rerr = controllertests.Roll(context.TODO(), ctrl, configuration, 0)
				})

				It("should not error", func() {
					Expect(rerr).To(BeNil())
				})

				It("should create a plan job", func() {
					list := &batchv1.JobList{}

					Expect(cc.List(context.TODO(), list, client.InNamespace(ctrl.ControllerNamespace))).ToNot(HaveOccurred())
					Expect(len(list.Items)).To(Equal(1))
				})

				It("should not have injected the default secrets", func() {
					list := &batchv1.JobList{}
					Expect(cc.List(context.TODO(), list, client.InNamespace(ctrl.ControllerNamespace))).ToNot(HaveOccurred())
					Expect(len(list.Items)).To(Equal(1))

					job := list.Items[0]
					Expect(job.Spec.Template.Spec.InitContainers).To(HaveLen(2))
					Expect(job.Spec.Template.Spec.InitContainers[0].Name).To(Equal("setup"))
					Expect(job.Spec.Template.Spec.InitContainers[0].EnvFrom).ToNot(BeEmpty())
					Expect(job.Spec.Template.Spec.InitContainers[0].EnvFrom).To(HaveLen(2))
					Expect(job.Spec.Template.Spec.InitContainers[0].EnvFrom[0].SecretRef.Name).To(Equal(configuration.GetTerraformConfigSecretName()))
					Expect(job.Spec.Template.Spec.InitContainers[0].EnvFrom[1].SecretRef.Name).To(Equal("test"))
					Expect(job.Spec.Template.Spec.InitContainers[1].Name).To(Equal("init"))
					Expect(job.Spec.Template.Spec.InitContainers[1].EnvFrom).ToNot(BeEmpty())
				})
			})

			Context("and the configuration namespace does not match", func() {
				BeforeEach(func() {
					Expect(cc.Delete(context.Background(), policy.DeepCopy())).To(Succeed())

					policy = fixtures.NewPolicy(policy.Name)
					policy.Spec.Defaults = []terraformv1alpha1.DefaultVariables{
						{
							Selector: terraformv1alpha1.DefaultVariablesSelector{
								Namespace: &metav1.LabelSelector{
									MatchLabels: map[string]string{"name": "no_match"},
								},
							},
							Secrets: []string{"test"},
						},
					}
					Expect(cc.Create(context.Background(), policy)).To(Succeed())

					result, _, rerr = controllertests.Roll(context.TODO(), ctrl, configuration, 0)
				})

				It("should not error", func() {
					Expect(rerr).To(BeNil())
				})

				It("should create a plan job", func() {
					list := &batchv1.JobList{}

					Expect(cc.List(context.TODO(), list, client.InNamespace(ctrl.ControllerNamespace))).ToNot(HaveOccurred())
					Expect(len(list.Items)).To(Equal(1))
				})

				It("should not have injected the default secrets", func() {
					list := &batchv1.JobList{}
					Expect(cc.List(context.TODO(), list, client.InNamespace(ctrl.ControllerNamespace))).ToNot(HaveOccurred())
					Expect(len(list.Items)).To(Equal(1))

					job := list.Items[0]
					Expect(job.Spec.Template.Spec.InitContainers).To(HaveLen(2))
					Expect(job.Spec.Template.Spec.InitContainers[0].Name).To(Equal("setup"))
					Expect(job.Spec.Template.Spec.InitContainers[0].EnvFrom).ToNot(BeEmpty())
					Expect(job.Spec.Template.Spec.InitContainers[0].EnvFrom).To(HaveLen(1))
					Expect(job.Spec.Template.Spec.InitContainers[1].Name).To(Equal("init"))
				})
			})
		})

		Context("and the configuration we have a namespace expressions selector", func() {
			BeforeEach(func() {
				// ensure we are in the cache
				namespace := fixtures.NewNamespace(configuration.Namespace)
				namespace.Labels = map[string]string{"name": "match"}
				ctrl.cache.SetDefault(namespace.Name, namespace)

				/// update the policy to using a namespace label selector
				Expect(cc.Delete(context.Background(), policy.DeepCopy())).To(Succeed())
				policy = fixtures.NewPolicy(policy.Name)
				policy.Spec.Defaults = []terraformv1alpha1.DefaultVariables{
					{
						Selector: terraformv1alpha1.DefaultVariablesSelector{
							Namespace: &metav1.LabelSelector{
								MatchExpressions: []metav1.LabelSelectorRequirement{
									{
										Key:      "name",
										Operator: metav1.LabelSelectorOpIn,
										Values:   []string{"match"},
									},
								},
							},
						},
						Secrets: []string{"test"},
					},
				}
				Expect(cc.Create(context.Background(), policy)).To(Succeed())
			})

			Context("and the configuration namespace label matches", func() {
				BeforeEach(func() {
					result, _, rerr = controllertests.Roll(context.TODO(), ctrl, configuration, 0)
				})

				It("should not error", func() {
					Expect(rerr).To(BeNil())
				})

				It("should create a plan job", func() {
					list := &batchv1.JobList{}

					Expect(cc.List(context.TODO(), list, client.InNamespace(ctrl.ControllerNamespace))).ToNot(HaveOccurred())
					Expect(len(list.Items)).To(Equal(1))
				})

				It("should not have injected the default secrets", func() {
					list := &batchv1.JobList{}
					Expect(cc.List(context.TODO(), list, client.InNamespace(ctrl.ControllerNamespace))).ToNot(HaveOccurred())
					Expect(len(list.Items)).To(Equal(1))

					job := list.Items[0]
					Expect(job.Spec.Template.Spec.InitContainers).To(HaveLen(2))
					Expect(job.Spec.Template.Spec.InitContainers[0].Name).To(Equal("setup"))
					Expect(job.Spec.Template.Spec.InitContainers[0].EnvFrom).ToNot(BeEmpty())
					Expect(job.Spec.Template.Spec.InitContainers[0].EnvFrom).To(HaveLen(2))
					Expect(job.Spec.Template.Spec.InitContainers[0].EnvFrom[0].SecretRef.Name).To(Equal(configuration.GetTerraformConfigSecretName()))
					Expect(job.Spec.Template.Spec.InitContainers[0].EnvFrom[1].SecretRef.Name).To(Equal("test"))
					Expect(job.Spec.Template.Spec.InitContainers[1].Name).To(Equal("init"))
					Expect(job.Spec.Template.Spec.InitContainers[1].EnvFrom).ToNot(BeEmpty())
				})
			})

			Context("and the configuration namespace does not match", func() {
				BeforeEach(func() {
					Expect(cc.Delete(context.Background(), policy.DeepCopy())).To(Succeed())

					policy = fixtures.NewPolicy(policy.Name)
					policy.Spec.Defaults = []terraformv1alpha1.DefaultVariables{
						{
							Selector: terraformv1alpha1.DefaultVariablesSelector{
								Namespace: &metav1.LabelSelector{
									MatchExpressions: []metav1.LabelSelectorRequirement{
										{
											Key:      "name",
											Operator: metav1.LabelSelectorOpIn,
											Values:   []string{"no_match"},
										},
									},
								},
							},
							Secrets: []string{"test"},
						},
					}
					Expect(cc.Create(context.Background(), policy)).To(Succeed())

					result, _, rerr = controllertests.Roll(context.TODO(), ctrl, configuration, 0)
				})

				It("should not error", func() {
					Expect(rerr).To(BeNil())
				})

				It("should create a plan job", func() {
					list := &batchv1.JobList{}

					Expect(cc.List(context.TODO(), list, client.InNamespace(ctrl.ControllerNamespace))).ToNot(HaveOccurred())
					Expect(len(list.Items)).To(Equal(1))
				})

				It("should not have injected the default secrets", func() {
					list := &batchv1.JobList{}
					Expect(cc.List(context.TODO(), list, client.InNamespace(ctrl.ControllerNamespace))).ToNot(HaveOccurred())
					Expect(len(list.Items)).To(Equal(1))

					job := list.Items[0]
					Expect(job.Spec.Template.Spec.InitContainers).To(HaveLen(2))
					Expect(job.Spec.Template.Spec.InitContainers[0].Name).To(Equal("setup"))
					Expect(job.Spec.Template.Spec.InitContainers[0].EnvFrom).ToNot(BeEmpty())
					Expect(job.Spec.Template.Spec.InitContainers[0].EnvFrom).To(HaveLen(1))
					Expect(job.Spec.Template.Spec.InitContainers[1].Name).To(Equal("init"))
				})
			})
		})

		Context("and we have default secrets", func() {
			Context("and no job for the terraform plan exists", func() {
				BeforeEach(func() {
					result, _, rerr = controllertests.Roll(context.TODO(), ctrl, configuration, 0)
				})

				It("should not error", func() {
					Expect(rerr).To(BeNil())
				})

				It("should create a plan job", func() {
					list := &batchv1.JobList{}

					Expect(cc.List(context.TODO(), list, client.InNamespace(ctrl.ControllerNamespace))).ToNot(HaveOccurred())
					Expect(len(list.Items)).To(Equal(1))
				})

				It("should create a job containing the default secrets", func() {
					list := &batchv1.JobList{}
					Expect(cc.List(context.TODO(), list, client.InNamespace(ctrl.ControllerNamespace))).ToNot(HaveOccurred())
					Expect(len(list.Items)).To(Equal(1))

					job := list.Items[0]
					Expect(job.Spec.Template.Spec.InitContainers).To(HaveLen(2))
					Expect(job.Spec.Template.Spec.InitContainers[0].Name).To(Equal("setup"))
					Expect(job.Spec.Template.Spec.InitContainers[0].EnvFrom).ToNot(BeEmpty())
					Expect(job.Spec.Template.Spec.InitContainers[0].EnvFrom[0].SecretRef.Name).To(Equal(configuration.GetTerraformConfigSecretName()))
					Expect(job.Spec.Template.Spec.InitContainers[0].EnvFrom[1].SecretRef.Name).To(Equal("test"))
					Expect(job.Spec.Template.Spec.InitContainers[1].Name).To(Equal("init"))
					Expect(job.Spec.Template.Spec.InitContainers[1].EnvFrom).ToNot(BeEmpty())
				})
			})
		})

		Context("and the configuration does not match the selector", func() {
			BeforeEach(func() {
				Expect(cc.Delete(context.Background(), policy.DeepCopy())).To(Succeed())

				policy = fixtures.NewPolicy(policy.Name)
				policy.Spec.Defaults = []terraformv1alpha1.DefaultVariables{
					{
						Selector: terraformv1alpha1.DefaultVariablesSelector{
							Modules: []string{"no_match"},
						},
						Secrets: []string{"test"},
					},
				}
				Expect(cc.Create(context.Background(), policy)).To(Succeed())

				result, _, rerr = controllertests.Roll(context.TODO(), ctrl, configuration, 0)
			})

			It("should not have injected the default secrets", func() {
				list := &batchv1.JobList{}
				Expect(cc.List(context.TODO(), list, client.InNamespace(ctrl.ControllerNamespace))).ToNot(HaveOccurred())
				Expect(len(list.Items)).To(Equal(1))

				job := list.Items[0]
				Expect(job.Spec.Template.Spec.InitContainers).To(HaveLen(2))
				Expect(job.Spec.Template.Spec.InitContainers[0].Name).To(Equal("setup"))
				Expect(len(job.Spec.Template.Spec.InitContainers[0].EnvFrom)).To(Equal(1))
				Expect(job.Spec.Template.Spec.InitContainers[1].Name).To(Equal("init"))
				Expect(len(job.Spec.Template.Spec.InitContainers[1].EnvFrom)).To(Equal(0))
			})
		})
	})
})

var _ = Describe("Configuration Controller with Contexts", func() {
	logrus.SetOutput(io.Discard)

	var cc client.Client
	var result reconcile.Result
	var rerr error
	var ctrl *Controller
	var configuration *terraformv1alpha1.Configuration

	namespace := "default"

	BeforeEach(func() {
		secret := fixtures.NewValidAWSProviderSecret(namespace, "aws")
		configuration = fixtures.NewValidBucketConfiguration(namespace, "test")
		configuration.Spec.ValueFrom = []terraformv1alpha1.ValueFromSource{
			{
				Context: ptr.To("default"),
				Key:     "foo",
				Name:    "test",
			},
			{
				Context: ptr.To("default"),
				Key:     "complex",
				Name:    "complex",
			},
		}
		configuration.Spec.Variables = &runtime.RawExtension{
			Raw: []byte(`{"hello":"world"}`),
		}

		cc = fake.NewClientBuilder().
			WithScheme(schema.GetScheme()).
			WithStatusSubresource(&terraformv1alpha1.Configuration{}).
			WithRuntimeObjects(
				append([]runtime.Object{
					fixtures.NewValidAWSReadyProvider("aws", secret),
					secret,
				})...).
			Build()

		ctrl = makeFakeController(cc)
		ctrl.cache.SetDefault(namespace, fixtures.NewNamespace(namespace))
	})

	When("create a configuration with a context", func() {
		BeforeEach(func() {
			txt := fixtures.NewTerranettesContext("default")
			txt.Spec.Variables = map[string]runtime.RawExtension{
				"foo": {
					Raw: []byte(`{"description": "foo", "value": "should_be_me"}`),
				},
				"complex": {
					Raw: []byte(`{"description": "complex", "value": ["subnet0", "subnet1", "subnet2"]}`),
				},
			}
			Expect(cc.Create(context.Background(), txt)).To(Succeed())
		})

		Context("and the context does not exist", func() {
			BeforeEach(func() {
				cc.Delete(context.Background(), fixtures.NewTerranettesContext("default"))
			})

			Context("and the context is optional", func() {
				BeforeEach(func() {
					configuration.Spec.ValueFrom[0].Optional = true
					configuration.Spec.ValueFrom[1].Optional = true
					Expect(cc.Create(context.Background(), configuration)).To(Succeed())

					result, _, rerr = controllertests.Roll(context.Background(), ctrl, configuration, 0)
				})

				It("should not have failed", func() {
					Expect(rerr).ToNot(HaveOccurred())
					Expect(result.Requeue).To(BeFalse())
				})

				It("should have the appropriate conditions", func() {
					Expect(cc.Get(context.TODO(), configuration.GetNamespacedName(), configuration)).ToNot(HaveOccurred())

					cond := configuration.GetCommonStatus().GetCondition(terraformv1alpha1.ConditionTerraformPlan)
					Expect(cond.Status).To(Equal(metav1.ConditionFalse))
					Expect(cond.Reason).To(Equal(corev1alpha1.ReasonInProgress))
					Expect(cond.Message).To(Equal("Terraform plan is running"))
				})

				It("should create the terraform plan", func() {
					list := &batchv1.JobList{}

					Expect(cc.List(context.TODO(), list, client.InNamespace(ctrl.ControllerNamespace))).ToNot(HaveOccurred())
					Expect(len(list.Items)).To(Equal(1))
				})

				It("should have the job configuration secret", func() {
					secret := &v1.Secret{}
					secret.Namespace = ctrl.ControllerNamespace
					secret.Name = configuration.GetTerraformConfigSecretName()

					found, err := kubernetes.GetIfExists(context.Background(), cc, secret)
					Expect(err).ToNot(HaveOccurred())
					Expect(found).To(BeTrue())
				})
			})

			Context("and the context is required", func() {
				BeforeEach(func() {
					configuration.Spec.ValueFrom[0].Optional = false
					Expect(cc.Create(context.Background(), configuration)).To(Succeed())

					result, _, rerr = controllertests.Roll(context.Background(), ctrl, configuration, 0)
				})

				It("should not create any jobs", func() {
					list := &batchv1.JobList{}

					Expect(cc.List(context.TODO(), list, client.InNamespace(ctrl.ControllerNamespace))).ToNot(HaveOccurred())
					Expect(len(list.Items)).To(Equal(0))
				})

				It("should have appropriate conditions", func() {
					Expect(cc.Get(context.TODO(), configuration.GetNamespacedName(), configuration)).ToNot(HaveOccurred())

					cond := configuration.GetCommonStatus().GetCondition(corev1alpha1.ConditionReady)
					Expect(cond.Status).To(Equal(metav1.ConditionFalse))
					Expect(cond.Reason).To(Equal(corev1alpha1.ReasonActionRequired))
					Expect(cond.Message).To(Equal("spec.valueFrom[0].context (default) does not exist"))
				})
			})
		})

		Context("and the context exists", func() {
			Context("but the value is missing, but optional", func() {
				BeforeEach(func() {
					configuration.Spec.ValueFrom[0].Optional = true
					configuration.Spec.ValueFrom[0].Key = "missing"
					Expect(cc.Create(context.Background(), configuration)).To(Succeed())

					result, _, rerr = controllertests.Roll(context.Background(), ctrl, configuration, 0)
				})

				It("should not error", func() {
					Expect(rerr).ToNot(HaveOccurred())
					Expect(result.Requeue).To(BeFalse())
				})

				It("should have the appropriate conditions ", func() {
					Expect(cc.Get(context.Background(), configuration.GetNamespacedName(), configuration)).ToNot(HaveOccurred())

					cond := configuration.GetCommonStatus().GetCondition(terraformv1alpha1.ConditionTerraformPlan)
					Expect(cond.Status).To(Equal(metav1.ConditionFalse))
					Expect(cond.Reason).To(Equal(corev1alpha1.ReasonInProgress))
					Expect(cond.Message).To(Equal("Terraform plan is running"))
				})

				It("should create the terraform plan", func() {
					list := &batchv1.JobList{}

					Expect(cc.List(context.Background(), list, client.InNamespace(ctrl.ControllerNamespace))).ToNot(HaveOccurred())
					Expect(len(list.Items)).To(Equal(1))
				})
			})

			Context("but the value is present", func() {
				BeforeEach(func() {
					Expect(cc.Create(context.Background(), configuration)).To(Succeed())

					result, _, rerr = controllertests.Roll(context.Background(), ctrl, configuration, 0)
				})

				It("should not error", func() {
					Expect(rerr).ToNot(HaveOccurred())
					Expect(result.Requeue).To(BeFalse())
				})

				It("should have the appropriate conditions ", func() {
					Expect(cc.Get(context.Background(), configuration.GetNamespacedName(), configuration)).ToNot(HaveOccurred())

					cond := configuration.GetCommonStatus().GetCondition(terraformv1alpha1.ConditionTerraformPlan)
					Expect(cond.Status).To(Equal(metav1.ConditionFalse))
					Expect(cond.Reason).To(Equal(corev1alpha1.ReasonInProgress))
					Expect(cond.Message).To(Equal("Terraform plan is running"))
				})

				It("should create the terraform plan", func() {
					list := &batchv1.JobList{}

					Expect(cc.List(context.Background(), list, client.InNamespace(ctrl.ControllerNamespace))).ToNot(HaveOccurred())
					Expect(len(list.Items)).To(Equal(1))
				})

				It("should have the context variable in the job configuration secret", func() {
					secret := &v1.Secret{}
					secret.Namespace = ctrl.ControllerNamespace
					secret.Name = configuration.GetTerraformConfigSecretName()

					expected := "{\"complex\":[\"subnet0\",\"subnet1\",\"subnet2\"],\"hello\":\"world\",\"test\":\"should_be_me\"}\n"

					found, err := kubernetes.GetIfExists(context.Background(), cc, secret)
					Expect(err).ToNot(HaveOccurred())
					Expect(found).To(BeTrue())
					Expect(string(secret.Data[terraformv1alpha1.TerraformVariablesConfigMapKey])).ToNot(BeEmpty())
					Expect(string(secret.Data[terraformv1alpha1.TerraformVariablesConfigMapKey])).To(Equal(expected))
				})
			})
		})
	})
})

var _ = Describe("Configuration Controller, no reconciliation", func() {
	logrus.SetOutput(io.Discard)

	var cc client.Client
	var result reconcile.Result
	var rerr error
	var ctrl *Controller
	var configuration *terraformv1alpha1.Configuration

	namespace := "default"

	BeforeEach(func() {
		secret := fixtures.NewValidAWSProviderSecret(namespace, "aws")
		configuration = fixtures.NewValidBucketConfiguration(namespace, "test")

		cc = fake.NewClientBuilder().
			WithScheme(schema.GetScheme()).
			WithStatusSubresource(&terraformv1alpha1.Configuration{}).
			WithRuntimeObjects(
				append([]runtime.Object{
					fixtures.NewValidAWSReadyProvider("aws", secret),
					secret,
				})...).
			Build()

		ctrl = makeFakeController(cc)
		ctrl.cache.SetDefault(namespace, fixtures.NewNamespace(namespace))
	})

	When("creating or updating a configuration", func() {
		Context("with no reconciliation annotation present", func() {
			BeforeEach(func() {
				configuration.Annotations = map[string]string{
					terraformv1alpha1.ReconcileAnnotation: "false",
				}
				Expect(cc.Create(context.Background(), configuration)).To(Succeed())

				result, _, rerr = controllertests.Roll(context.Background(), ctrl, configuration, 0)
			})

			It("should not error", func() {
				Expect(rerr).ToNot(HaveOccurred())
				Expect(result.Requeue).To(BeFalse())
			})

			It("should not create any jobs", func() {
				list := &batchv1.JobList{}

				Expect(cc.List(context.Background(), list, client.InNamespace(ctrl.ControllerNamespace))).ToNot(HaveOccurred())
				Expect(len(list.Items)).To(Equal(0))
			})

			It("should indicate that the configuration is not reconciling", func() {
				Expect(cc.Get(context.Background(), configuration.GetNamespacedName(), configuration)).ToNot(HaveOccurred())

				cond := configuration.GetCommonStatus().GetCondition(corev1alpha1.ConditionReady)
				Expect(cond.Status).To(Equal(metav1.ConditionFalse))
				Expect(cond.Reason).To(Equal(corev1alpha1.ReasonWarning))
				Expect(cond.Message).To(Equal("Configuration has reconciling annotation set as false, ignoring changes"))
			})
		})

		Context("with a reconciliation annotation is not present", func() {
			BeforeEach(func() {
				Expect(cc.Create(context.Background(), configuration)).To(Succeed())

				result, _, rerr = controllertests.Roll(context.Background(), ctrl, configuration, 0)
			})

			It("should not error", func() {
				Expect(rerr).ToNot(HaveOccurred())
				Expect(result.Requeue).To(BeFalse())
			})

			It("should a terraform plan job", func() {
				list := &batchv1.JobList{}

				Expect(cc.List(context.Background(), list, client.InNamespace(ctrl.ControllerNamespace))).ToNot(HaveOccurred())
				Expect(len(list.Items)).To(Equal(1))
			})

			It("should indicate that the configuration is reconciling", func() {
				Expect(cc.Get(context.Background(), configuration.GetNamespacedName(), configuration)).ToNot(HaveOccurred())

				cond := configuration.GetCommonStatus().GetCondition(terraformv1alpha1.ConditionTerraformPlan)
				Expect(cond.Status).To(Equal(metav1.ConditionFalse))
				Expect(cond.Reason).To(Equal(corev1alpha1.ReasonInProgress))
				Expect(cond.Message).To(Equal("Terraform plan is running"))
			})
		})
	})
})

var _ = Describe("Configuration Controller", func() {
	logrus.SetOutput(io.Discard)

	var cc client.Client
	var result reconcile.Result
	var rerr error
	var ctrl *Controller
	var configuration *terraformv1alpha1.Configuration
	var recorder *controllertests.FakeRecorder

	cfgNamespace := "apps"
	defaultConditions := 5

	verifyPolicyArguments := []string{
		"--comment=Evaluating Against Security Policy",
		"--command=/usr/local/bin/checkov --config /run/checkov/checkov.yaml --framework terraform_plan -f /run/tfplan.json --soft-fail -o json -o cli --output-file-path /run --repo-root-for-plan-enrichment /data --download-external-modules true >/dev/null",
		"--command=/bin/cat /run/results_cli.txt",
		"--namespace=$(KUBE_NAMESPACE)",
		"--upload=$(POLICY_REPORT_NAME)=/run/results_json.json",
		"--is-failure=/run/steps/terraform.failed",
		"--wait-on=/run/steps/terraform.complete",
	}

	Setup := func(objects ...runtime.Object) {
		secret := fixtures.NewValidAWSProviderSecret("default", "aws")

		cc = fake.NewClientBuilder().
			WithScheme(schema.GetScheme()).
			WithStatusSubresource(&terraformv1alpha1.Configuration{}).
			WithRuntimeObjects(
				append([]runtime.Object{
					fixtures.NewNamespace(cfgNamespace),
					fixtures.NewValidAWSReadyProvider("aws", secret),
					secret,
				}, objects...)...).
			Build()

		recorder = &controllertests.FakeRecorder{}
		ctrl = &Controller{
			cc:                           cc,
			kc:                           kfake.NewSimpleClientset(),
			cache:                        cache.New(5*time.Minute, 10*time.Minute),
			recorder:                     recorder,
			BinaryPath:                   "/usr/local/bin/tofu",
			DefaultExecutorCPULimit:      "1",
			DefaultExecutorCPURequest:    "5m",
			DefaultExecutorMemoryLimit:   "1Gi",
			DefaultExecutorMemoryRequest: "32Mi",
			EnableInfracosts:             false,
			EnableWatchers:               true,
			ExecutorImage:                "ghcr.io/appvia/terranetes-executor",
			InfracostsImage:              "infracosts/infracost:latest",
			ControllerNamespace:          "default",
			PolicyImage:                  "bridgecrew/checkov:2.0.1140",
			TerraformImage:               "ghcr.io/opentofu/opentofu:latest",
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

				cond := configuration.Status.GetCondition(terraformv1alpha1.ConditionProviderReady)
				Expect(cond.Type).To(Equal(terraformv1alpha1.ConditionProviderReady))
				Expect(cond.Status).To(Equal(metav1.ConditionFalse))
				Expect(cond.Reason).To(Equal(corev1alpha1.ReasonActionRequired))
				Expect(cond.Message).To(Equal("Provider referenced \"does_not_exist\" does not exist"))
			})

			It("should have raised a event", func() {
				Expect(recorder.Events).ToNot(BeEmpty())
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

				cond := configuration.Status.GetCondition(terraformv1alpha1.ConditionProviderReady)
				Expect(cond.Status).To(Equal(metav1.ConditionFalse))
				Expect(cond.Reason).To(Equal(corev1alpha1.ReasonWarning))
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
				Expect(list.Items[0].Spec.Template.Spec.ServiceAccountName).To(Equal("terranetes-executor"))
			})
		})

		When("using a provider with injected identity", func() {
			serviceAccount := "injected-service-account"

			BeforeEach(func() {
				configuration = fixtures.NewValidBucketConfiguration(cfgNamespace, "bucket")
				configuration.Spec.ProviderRef.Name = "injected"

				provider := fixtures.NewValidAWSReadyProvider(configuration.Spec.ProviderRef.Name, fixtures.NewValidAWSProviderSecret(ctrl.ControllerNamespace, configuration.Spec.ProviderRef.Name))
				provider.Spec.Source = terraformv1alpha1.SourceInjected
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

	// RETRYABLE CONFIGURATION
	When("the user is attempting to retry a configuration", func() {
		BeforeEach(func() {
			configuration = fixtures.NewValidBucketConfiguration(cfgNamespace, "bucket")
			configuration.Finalizers = []string{"test"}
			configuration.Status.LastReconcile = &corev1alpha1.LastReconcileStatus{Time: metav1.Time{Time: time.Now()}}
			configuration.Annotations[terraformv1alpha1.RetryAnnotation] = fmt.Sprintf("%d", time.Now().Unix())

			// step the plan condition to success
			cond := controller.ConditionMgr(configuration, terraformv1alpha1.ConditionTerraformPlan, nil)
			cond.Success("Plan succeeded")
		})

		Context("and the retryable annotation is invalid", func() {
			BeforeEach(func() {
				configuration.Annotations[terraformv1alpha1.RetryAnnotation] = "invalid"
				Setup(configuration)

				result, _, rerr = controllertests.Roll(context.TODO(), ctrl, configuration, 10)
			})

			It("should not create a new job", func() {
				list := &batchv1.JobList{}

				Expect(cc.List(context.TODO(), list, client.InNamespace(ctrl.ControllerNamespace))).ToNot(HaveOccurred())
				Expect(len(list.Items)).To(Equal(0))
			})
		})

		Context("the configuration has a reconciled before", func() {
			Context("but the last reconcile is after the retry timestamp", func() {
				BeforeEach(func() {
					configuration.Status.LastReconcile = &corev1alpha1.LastReconcileStatus{
						Time: metav1.Time{Time: time.Now().Add(1 * time.Hour)},
					}
					Setup(configuration)

					result, _, rerr = controllertests.Roll(context.TODO(), ctrl, configuration, 10)
				})
			})

			Context("and the last reconcile is a before the retry timestamp", func() {
				BeforeEach(func() {
					configuration.Status.LastReconcile = &corev1alpha1.LastReconcileStatus{Time: metav1.Time{Time: time.Now().Add(-1 * time.Hour)}}
					configuration.Annotations[terraformv1alpha1.RetryAnnotation] = fmt.Sprintf("%d", time.Now().Unix())
					Setup(configuration)

					result, _, rerr = controllertests.Roll(context.TODO(), ctrl, configuration, 20)
				})

				It("should create a new job", func() {
					list := &batchv1.JobList{}

					Expect(cc.List(context.TODO(), list, client.InNamespace(ctrl.ControllerNamespace))).ToNot(HaveOccurred())
					Expect(len(list.Items)).To(Equal(1))
				})
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
				provider.Spec.Selector = &terraformv1alpha1.Selector{
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

				cond := configuration.GetCommonStatus().GetCondition(terraformv1alpha1.ConditionProviderReady)
				Expect(cond.Type).To(Equal(terraformv1alpha1.ConditionProviderReady))
				Expect(cond.Status).To(Equal(metav1.ConditionFalse))
				Expect(cond.Reason).To(Equal(corev1alpha1.ReasonActionRequired))
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
				provider.Spec.Selector = &terraformv1alpha1.Selector{
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

				cond := configuration.GetCommonStatus().GetCondition(terraformv1alpha1.ConditionProviderReady)
				Expect(cond.Type).To(Equal(terraformv1alpha1.ConditionProviderReady))
				Expect(cond.Status).To(Equal(metav1.ConditionFalse))
				Expect(cond.Reason).To(Equal(corev1alpha1.ReasonActionRequired))
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
				provider.Spec.Selector = &terraformv1alpha1.Selector{
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

				cond := configuration.GetCommonStatus().GetCondition(terraformv1alpha1.ConditionProviderReady)
				Expect(cond.Type).To(Equal(terraformv1alpha1.ConditionProviderReady))
				Expect(cond.Status).To(Equal(metav1.ConditionTrue))
				Expect(cond.Reason).To(Equal(corev1alpha1.ReasonReady))
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

				cond := configuration.Status.GetCondition(terraformv1alpha1.ConditionTerraformPolicy)
				Expect(cond.Status).To(Equal(metav1.ConditionFalse))
				Expect(cond.Reason).To(Equal(corev1alpha1.ReasonError))
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

				cond := configuration.Status.GetCondition(corev1alpha1.ConditionReady)
				Expect(cond.Status).To(Equal(metav1.ConditionFalse))
				Expect(cond.Reason).To(Equal(corev1alpha1.ReasonActionRequired))
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

				cond := configuration.GetCommonStatus().GetCondition(terraformv1alpha1.ConditionTerraformPlan)
				Expect(cond.Status).To(Equal(metav1.ConditionFalse))
				Expect(cond.Reason).To(Equal(corev1alpha1.ReasonInProgress))
				Expect(cond.Message).To(Equal("Terraform plan is running"))
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
				Expect(list.Items[0].Spec.Template.Spec.Containers[0].Image).To(Equal("ghcr.io/opentofu/opentofu:test"))
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
				Expect(list.Items[0].Spec.Template.Spec.Containers[0].Image).To(Equal("ghcr.io/opentofu/opentofu:latest"))
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

				cond := configuration.Status.GetCondition(corev1alpha1.ConditionReady)
				Expect(cond.Status).To(Equal(metav1.ConditionFalse))
				Expect(cond.Reason).To(Equal(corev1alpha1.ReasonActionRequired))
				Expect(cond.Message).To(Equal("Cost analytics secret (default/not_there) does not exist, contact platform administrator"))
			})

			It("should have raised a event", func() {
				Expect(recorder.Events).ToNot(BeEmpty())
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

				cond := configuration.Status.GetCondition(corev1alpha1.ConditionReady)
				Expect(cond.Status).To(Equal(metav1.ConditionFalse))
				Expect(cond.Reason).To(Equal(corev1alpha1.ReasonActionRequired))
				Expect(cond.Message).To(Equal("Cost analytics secret (default/token) does not contain a token, contact platform administrator"))
			})

			It("should have raised a event", func() {
				Expect(recorder.Events).ToNot(BeEmpty())
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
				plan := fixtures.NewTerraformJob(configuration, ctrl.ControllerNamespace, terraformv1alpha1.StageTerraformPlan)
				plan.Status.Conditions = []batchv1.JobCondition{{Type: batchv1.JobComplete, Status: v1.ConditionTrue}}
				plan.Status.Succeeded = 1
				// create fake terraform state
				state := fixtures.NewTerraformState(configuration)
				state.Namespace = ctrl.ControllerNamespace
				// create fake costs secret
				token := fixtures.NewCostsSecret(ctrl.ControllerNamespace, "infracost")
				token.Namespace = ctrl.ControllerNamespace
				// create fake plan secret
				tfplan := fixtures.NewTerraformPlanWithDiff(configuration, ctrl.ControllerNamespace)

				Setup(configuration, plan, state, token, tfplan)
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
				plan := fixtures.NewTerraformJob(configuration, ctrl.ControllerNamespace, terraformv1alpha1.StageTerraformPlan)
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
				// create fake plan secret
				tfplan := fixtures.NewTerraformPlanWithDiff(configuration, ctrl.ControllerNamespace)

				Setup(configuration, plan, state, report, token, tfplan)
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
					configuration.Spec.ValueFrom = []terraformv1alpha1.ValueFromSource{
						{Secret: ptr.To("missing"), Key: "key"},
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

					cond := configuration.Status.GetCondition(corev1alpha1.ConditionReady)
					Expect(cond.Status).To(Equal(metav1.ConditionFalse))
					Expect(cond.Reason).To(Equal(corev1alpha1.ReasonActionRequired))
					Expect(cond.Message).To(Equal("spec.valueFrom[0].secret (apps/missing) does not exist"))
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
					configuration.Spec.ValueFrom = []terraformv1alpha1.ValueFromSource{
						{Secret: ptr.To("missing"), Key: "key", Optional: true},
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

					configuration.Spec.ValueFrom = []terraformv1alpha1.ValueFromSource{{Secret: ptr.To("exists"), Key: "missing"}}
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

					cond := configuration.Status.GetCondition(corev1alpha1.ConditionReady)
					Expect(cond.Status).To(Equal(metav1.ConditionFalse))
					Expect(cond.Reason).To(Equal(corev1alpha1.ReasonActionRequired))
					Expect(cond.Message).To(Equal(`spec.valueFrom[0] (apps/exists) does not contain key: "missing"`))
				})

				It("should not create any jobs", func() {
					list := &batchv1.JobList{}

					Expect(cc.List(context.TODO(), list, client.InNamespace(ctrl.ControllerNamespace))).ToNot(HaveOccurred())
					Expect(len(list.Items)).To(Equal(0))
				})
			})

			When("key is missing but optional", func() {
				BeforeEach(func() {
					configuration.Spec.ValueFrom = []terraformv1alpha1.ValueFromSource{
						{Secret: ptr.To("missing"), Key: "key", Optional: true},
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

					configuration.Spec.ValueFrom = []terraformv1alpha1.ValueFromSource{{Secret: ptr.To("exists"), Key: "my"}}
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

					cond := configuration.Status.GetCondition(terraformv1alpha1.ConditionTerraformPlan)
					Expect(cond.Status).To(Equal(metav1.ConditionFalse))
					Expect(cond.Reason).To(Equal(corev1alpha1.ReasonInProgress))
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
					Expect(secret.Data).To(HaveKey(terraformv1alpha1.TerraformVariablesConfigMapKey))
					Expect(string(secret.Data[terraformv1alpha1.TerraformVariablesConfigMapKey])).To(Equal(expected))
				})
			})

			When("secret and key exist and we are remapping", func() {
				BeforeEach(func() {
					secret := &v1.Secret{}
					secret.Namespace = configuration.Namespace
					secret.Name = "exists"
					secret.Data = map[string][]byte{"DB_HOST": []byte("value")}

					configuration.Spec.ValueFrom = []terraformv1alpha1.ValueFromSource{{Secret: ptr.To("exists"), Key: "DB_HOST", Name: "database_host"}}
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

					cond := configuration.Status.GetCondition(terraformv1alpha1.ConditionTerraformPlan)
					Expect(cond.Status).To(Equal(metav1.ConditionFalse))
					Expect(cond.Reason).To(Equal(corev1alpha1.ReasonInProgress))
				})

				It("should have created a job", func() {
					list := &batchv1.JobList{}

					Expect(cc.List(context.TODO(), list, client.InNamespace(ctrl.ControllerNamespace))).ToNot(HaveOccurred())
					Expect(len(list.Items)).To(Equal(1))
				})

				It("should have added the value to the configuration config", func() {
					expected := "{\"database_host\":\"value\",\"name\":\"test\"}\n"

					secret := &v1.Secret{}
					secret.Namespace = ctrl.ControllerNamespace
					secret.Name = configuration.GetTerraformConfigSecretName()

					found, err := kubernetes.GetIfExists(context.TODO(), ctrl.cc, secret)
					Expect(err).ToNot(HaveOccurred())
					Expect(found).To(BeTrue())
					Expect(secret.Data).To(HaveKey(terraformv1alpha1.TerraformVariablesConfigMapKey))
					Expect(string(secret.Data[terraformv1alpha1.TerraformVariablesConfigMapKey])).To(Equal(expected))
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
			result, _, rerr = controllertests.Roll(context.TODO(), ctrl, configuration, 4)
		})

		It("should have the conditions", func() {
			Expect(cc.Get(context.TODO(), configuration.GetNamespacedName(), configuration)).ToNot(HaveOccurred())
			Expect(configuration.Status.Conditions).To(HaveLen(defaultConditions))
		})

		It("should indicate the failure on the conditions", func() {
			Expect(cc.Get(context.TODO(), configuration.GetNamespacedName(), configuration)).ToNot(HaveOccurred())

			cond := configuration.Status.GetCondition(terraformv1alpha1.ConditionProviderReady)
			Expect(cond.Status).To(Equal(metav1.ConditionTrue))
			Expect(cond.Reason).To(Equal(corev1alpha1.ReasonReady))
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
			Expect(job.Spec.Template.Spec.InitContainers[0].Name).To(Equal("setup"))
			Expect(job.Spec.Template.Spec.InitContainers[0].EnvFrom).To(HaveLen(3))
			Expect(job.Spec.Template.Spec.InitContainers[0].EnvFrom[1].SecretRef.Name).To(Equal("secret1"))
			Expect(job.Spec.Template.Spec.InitContainers[0].EnvFrom[2].SecretRef.Name).To(Equal("secret2"))

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

			cond := configuration.Status.GetCondition(terraformv1alpha1.ConditionProviderReady)
			Expect(cond.Status).To(Equal(metav1.ConditionTrue))
			Expect(cond.Reason).To(Equal(corev1alpha1.ReasonReady))
			Expect(cond.Message).To(Equal("Provider ready"))
		})

		It("should have created the generated configuration secret", func() {
			secret := &v1.Secret{}
			secret.Namespace = ctrl.ControllerNamespace
			secret.Name = configuration.GetTerraformConfigSecretName()

			found, err := kubernetes.GetIfExists(context.TODO(), cc, secret)
			Expect(err).ToNot(HaveOccurred())
			Expect(found).To(BeTrue())

			Expect(secret.Data).To(HaveKey(terraformv1alpha1.TerraformVariablesConfigMapKey))
			Expect(secret.Data).To(HaveKey(terraformv1alpha1.TerraformBackendSecretKey))
		})

		It("should have create a terraform backend configuration", func() {
			expected := `
terraform {
	backend "kubernetes" {
		in_cluster_config = true
		namespace         = "default"
		labels            = {
			"terraform.appvia.io/configuration" = "bucket"
			"terraform.appvia.io/configuration-uid" = "1234-122-1234-1234"
			"terraform.appvia.io/generation" = "0"
			"terraform.appvia.io/namespace" = "apps"
		}
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

			backend := string(secret.Data[terraformv1alpha1.TerraformBackendSecretKey])
			Expect(backend).ToNot(BeZero())
			Expect(backend).To(Equal(expected))
		})

		It("should have a provider.tf.json", func() {
			expected := "{\n  \"provider\": {\n    \"aws\": {}\n  }\n}\n"
			secret := &v1.Secret{}
			secret.Namespace = ctrl.ControllerNamespace
			secret.Name = configuration.GetTerraformConfigSecretName()

			found, err := kubernetes.GetIfExists(context.TODO(), cc, secret)
			Expect(err).ToNot(HaveOccurred())
			Expect(found).To(BeTrue())

			backend := string(secret.Data[terraformv1alpha1.TerraformProviderConfigMapKey])
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

			backend := string(secret.Data[terraformv1alpha1.TerraformVariablesConfigMapKey])
			Expect(backend).ToNot(BeZero())
			Expect(backend).To(Equal(expected))
		})

		It("should indicate the terraform plan is running", func() {
			Expect(cc.Get(context.TODO(), configuration.GetNamespacedName(), configuration)).ToNot(HaveOccurred())

			cond := configuration.Status.GetCondition(terraformv1alpha1.ConditionTerraformPlan)
			Expect(cond.Status).To(Equal(metav1.ConditionFalse))
			Expect(cond.Reason).To(Equal(corev1alpha1.ReasonInProgress))
			Expect(cond.Message).To(Equal("Terraform plan is running"))
		})

		It("should have created job for the terraform plan", func() {
			list := &batchv1.JobList{}

			Expect(cc.List(context.TODO(), list, client.InNamespace(ctrl.ControllerNamespace))).ToNot(HaveOccurred())
			Expect(len(list.Items)).To(Equal(1))
		})

		It("should have a terraform container", func() {
			list := &batchv1.JobList{}
			Expect(cc.List(context.TODO(), list, client.InNamespace(ctrl.ControllerNamespace))).ToNot(HaveOccurred())
			Expect(len(list.Items)).To(Equal(1))

			expected := []string{
				"--comment=Executing Terraform",
				"--command=/usr/local/bin/tofu plan --var-file variables.tfvars.json -out=/run/plan.out -lock=false -no-color -input=false",
				"--command=/usr/local/bin/tofu show -json /run/plan.out > /run/tfplan.json",
				"--command=/bin/cp /run/tfplan.json /run/plan.json",
				"--command=/bin/gzip /run/plan.json",
				"--command=/bin/mv /run/plan.json.gz /run/plan.json",
				"--namespace=$(KUBE_NAMESPACE)",
				"--upload=$(TERRAFORM_PLAN_JSON_NAME)=/run/plan.json",
				"--upload=$(TERRAFORM_PLAN_OUT_NAME)=/run/plan.out",
				"--on-error=/run/steps/terraform.failed",
				"--on-success=/run/steps/terraform.complete",
			}
			job := list.Items[0]
			container := job.Spec.Template.Spec.Containers[0]

			Expect(container.Name).To(Equal("terraform"))
			Expect(container.Command).To(Equal([]string{"/run/bin/step"}))
			Expect(container.Args).To(Equal(expected))

			Expect(len(container.EnvFrom)).To(Equal(1))
			Expect(container.EnvFrom[0].SecretRef).ToNot(BeNil())
			Expect(container.EnvFrom[0].SecretRef.Name).To(Equal("aws"))

			Expect(len(container.Env)).To(Equal(8))
			Expect(container.Env[5].Name).To(Equal("TERRAFORM_STATE_NAME"))
			Expect(container.Env[5].Value).To(Equal(configuration.GetTerraformStateSecretName()))
			Expect(container.Env[6].Name).To(Equal("TERRAFORM_PLAN_OUT_NAME"))
			Expect(container.Env[6].Value).To(Equal(configuration.GetTerraformPlanOutSecretName()))
			Expect(container.Env[7].Name).To(Equal("TERRAFORM_PLAN_JSON_NAME"))
			Expect(container.Env[7].Value).To(Equal(configuration.GetTerraformPlanJSONSecretName()))

			Expect(container.VolumeMounts[0].Name).To(Equal("run"))
			Expect(container.VolumeMounts[1].Name).To(Equal("source"))
		})

		It("it should have the configuration labels", func() {
			list := &batchv1.JobList{}
			Expect(cc.List(context.TODO(), list, client.InNamespace(ctrl.ControllerNamespace))).ToNot(HaveOccurred())
			Expect(len(list.Items)).To(Equal(1))

			labels := list.Items[0].GetLabels()
			Expect(labels).To(HaveKey(terraformv1alpha1.ConfigurationNameLabel))
			Expect(labels).To(HaveKey(terraformv1alpha1.ConfigurationNamespaceLabel))
			Expect(labels).To(HaveKey(terraformv1alpha1.ConfigurationGenerationLabel))
			Expect(labels).To(HaveKey(terraformv1alpha1.ConfigurationStageLabel))
			Expect(labels[terraformv1alpha1.ConfigurationStageLabel]).To(Equal(terraformv1alpha1.StageTerraformPlan))
			Expect(labels[terraformv1alpha1.ConfigurationNameLabel]).To(Equal(configuration.Name))
			Expect(labels[terraformv1alpha1.ConfigurationNamespaceLabel]).To(Equal(configuration.Namespace))
		})

		It("should have created a watch job in the configuration namespace", func() {
			list := &batchv1.JobList{}
			Expect(cc.List(context.TODO(), list, client.InNamespace(configuration.Namespace))).ToNot(HaveOccurred())
			Expect(len(list.Items)).To(Equal(1))
			Expect(len(list.Items[0].Spec.Template.Spec.Containers)).To(Equal(1))

			container := list.Items[0].Spec.Template.Spec.Containers[0]
			Expect(container.Name).To(Equal("watch"))
			Expect(container.Command).To(Equal([]string{"/watch_logs.sh"}))
			Expect(container.Args).To(Equal([]string{"-e", "http://controller.default.svc.cluster.local/v1/builds/apps/bucket/logs?generation=0&name=bucket&namespace=apps&stage=plan&uid=1234-122-1234-1234"}))
		})

		It("should have added a approval annotation to the configuration", func() {
			Expect(cc.Get(context.TODO(), configuration.GetNamespacedName(), configuration)).ToNot(HaveOccurred())
			Expect(configuration.Annotations).To(HaveKey(terraformv1alpha1.ApplyAnnotation))
		})

		It("should have a out of sync status", func() {
			Expect(cc.Get(context.TODO(), configuration.GetNamespacedName(), configuration)).ToNot(HaveOccurred())

			Expect(configuration.Status.ResourceStatus).To(Equal(terraformv1alpha1.ResourcesOutOfSync))
		})

		It("should not have a terraform version yet", func() {
			Expect(cc.Get(context.TODO(), configuration.GetNamespacedName(), configuration)).ToNot(HaveOccurred())

			Expect(configuration.Status.TerraformVersion).To(BeEmpty())
		})

		It("should ask us to requeue", func() {
			Expect(result).To(Equal(reconcile.Result{RequeueAfter: 10 * time.Second}))
			Expect(rerr).To(BeNil())
		})
	})

	// CONTEXT INJECTION
	When("using a context injection", func() {
		BeforeEach(func() {
			configuration = fixtures.NewValidBucketConfiguration(cfgNamespace, "bucket")
			Setup(configuration)

			ctrl.EnableContextInjection = true
			result, _, rerr = controllertests.Roll(context.TODO(), ctrl, configuration, 4)
		})

		It("should have the conditions", func() {
			Expect(cc.Get(context.TODO(), configuration.GetNamespacedName(), configuration)).ToNot(HaveOccurred())
			Expect(configuration.Status.Conditions).To(HaveLen(defaultConditions))
		})

		It("should have create a plan", func() {
			list := &batchv1.JobList{}

			Expect(cc.List(context.TODO(), list, client.InNamespace(ctrl.ControllerNamespace))).ToNot(HaveOccurred())
			Expect(len(list.Items)).To(Equal(1))
		})

		It("should have create job configuration secret", func() {
			secret := &v1.Secret{}
			secret.Namespace = ctrl.ControllerNamespace
			secret.Name = configuration.GetTerraformConfigSecretName()

			found, err := kubernetes.GetIfExists(context.TODO(), cc, secret)
			Expect(err).ToNot(HaveOccurred())
			Expect(found).To(BeTrue())
		})

		It("should have included the additional context variables", func() {
			secret := &v1.Secret{}
			secret.Namespace = ctrl.ControllerNamespace
			secret.Name = configuration.GetTerraformConfigSecretName()

			found, err := kubernetes.GetIfExists(context.TODO(), cc, secret)
			Expect(err).ToNot(HaveOccurred())
			Expect(found).To(BeTrue())
			Expect(secret.Data).To(HaveKey(terraformv1alpha1.TerraformVariablesConfigMapKey))
			Expect(secret.Data).To(HaveKey(terraformv1alpha1.TerraformBackendSecretKey))

			expected := "{\"name\":\"test\",\"terranetes\":{\"name\":\"bucket\",\"namespace\":\"apps\"}}\n"

			Expect(string(secret.Data[terraformv1alpha1.TerraformVariablesConfigMapKey])).To(Equal(expected))
		})
	})

	// CUSTOM BACKEND TEMPLATE
	When("using a provider specific backend template", func() {
		var template *v1.Secret

		BeforeEach(func() {
			configuration = fixtures.NewValidBucketConfiguration(cfgNamespace, "bucket")
			Setup(configuration)

			provider := &terraformv1alpha1.Provider{}
			provider.Name = configuration.Spec.ProviderRef.Name
			Expect(cc.Get(context.Background(), provider.GetNamespacedName(), provider)).ToNot(HaveOccurred())

			template = fixtures.NewBackendTemplateSecret(ctrl.ControllerNamespace, "provider-template")
			provider.Spec.BackendTemplate = &v1.SecretReference{
				Namespace: template.Namespace,
				Name:      template.Name,
			}
			Expect(cc.Create(context.Background(), template)).ToNot(HaveOccurred())
			Expect(cc.Update(context.Background(), provider)).ToNot(HaveOccurred())
		})

		Context("and the secret is not present", func() {
			BeforeEach(func() {
				Expect(cc.Delete(context.Background(), template)).ToNot(HaveOccurred())

				result, _, rerr = controllertests.Roll(context.TODO(), ctrl, configuration, 0)
			})

			It("should indicate the error", func() {
				Expect(cc.Get(context.TODO(), configuration.GetNamespacedName(), configuration)).ToNot(HaveOccurred())

				cond := configuration.Status.GetCondition(corev1alpha1.ConditionReady)
				Expect(cond.Status).To(Equal(metav1.ConditionFalse))
				Expect(cond.Reason).To(Equal(corev1alpha1.ReasonActionRequired))
				Expect(cond.Message).To(Equal(`Backend template secret "default/provider-template" not found, contact administrator`))
			})
		})

		Context("and the template is not present", func() {
			BeforeEach(func() {
				template = fixtures.NewBackendTemplateSecret(ctrl.ControllerNamespace, "provider-template")
				Expect(cc.Delete(context.Background(), template)).ToNot(HaveOccurred())

				template = fixtures.NewBackendTemplateSecret(ctrl.ControllerNamespace, "provider-template")
				template.Data = map[string][]byte{}
				Expect(cc.Create(context.Background(), template)).ToNot(HaveOccurred())

				result, _, rerr = controllertests.Roll(context.TODO(), ctrl, configuration, 0)
			})

			It("should indicate the error", func() {
				Expect(cc.Get(context.TODO(), configuration.GetNamespacedName(), configuration)).ToNot(HaveOccurred())

				cond := configuration.Status.GetCondition(corev1alpha1.ConditionReady)
				Expect(cond.Status).To(Equal(metav1.ConditionFalse))
				Expect(cond.Reason).To(Equal(corev1alpha1.ReasonActionRequired))
				Expect(cond.Message).To(Equal(`Backend template secret "default/provider-template" does not contain the backend.tf key`))
			})
		})

		Context("and the template is present", func() {
			BeforeEach(func() {
				result, _, rerr = controllertests.Roll(context.TODO(), ctrl, configuration, 0)
			})

			It("should have used the custom backend template from the provider", func() {
				secret := &v1.Secret{}
				secret.Namespace = ctrl.ControllerNamespace
				secret.Name = configuration.GetTerraformConfigSecretName()

				found, err := kubernetes.GetIfExists(context.Background(), cc, secret)
				Expect(err).ToNot(HaveOccurred())
				Expect(found).To(BeTrue())

				Expect(secret.Data).To(HaveKey(terraformv1alpha1.TerraformBackendSecretKey))
				Expect(string(secret.Data[terraformv1alpha1.TerraformBackendSecretKey])).To(ContainSubstring(`backend "s3"`))
				Expect(string(secret.Data[terraformv1alpha1.TerraformBackendSecretKey])).To(ContainSubstring(`terranetes-controller-state"`))
				Expect(string(secret.Data[terraformv1alpha1.TerraformBackendSecretKey])).To(ContainSubstring(`AWS_SECRET_ACCESS_KEY"`))
			})
		})
	})

	When("using a custom backend template", func() {
		When("the template is not present", func() {
			BeforeEach(func() {
				configuration = fixtures.NewValidBucketConfiguration(cfgNamespace, "bucket")
				Setup(configuration)
				ctrl.BackendTemplate = "not_there"

				result, _, rerr = controllertests.Roll(context.TODO(), ctrl, configuration, 10)
			})

			It("should have the conditions", func() {
				Expect(cc.Get(context.TODO(), configuration.GetNamespacedName(), configuration)).ToNot(HaveOccurred())
				Expect(configuration.Status.Conditions).To(HaveLen(defaultConditions))
			})

			It("should indicate the configuration failed", func() {
				Expect(cc.Get(context.TODO(), configuration.GetNamespacedName(), configuration)).ToNot(HaveOccurred())

				cond := configuration.Status.GetCondition(corev1alpha1.ConditionReady)
				Expect(cond.Status).To(Equal(metav1.ConditionFalse))
				Expect(cond.Reason).To(Equal(corev1alpha1.ReasonActionRequired))
				Expect(cond.Message).To(Equal("Backend template secret \"default/not_there\" not found, contact administrator"))
			})

			It("should not create any jobs", func() {
				list := &batchv1.JobList{}

				Expect(cc.List(context.TODO(), list, client.InNamespace(ctrl.ControllerNamespace))).ToNot(HaveOccurred())
				Expect(len(list.Items)).To(Equal(0))
			})
		})

		When("the template is present but missing required keys", func() {
			BeforeEach(func() {
				configuration = fixtures.NewValidBucketConfiguration(cfgNamespace, "bucket")
				backend := fixtures.NewBackendTemplateSecret(ctrl.ControllerNamespace, "missing_key")
				delete(backend.Data, "backend.tf")
				Setup(configuration, backend)
				ctrl.BackendTemplate = "missing_key"

				result, _, rerr = controllertests.Roll(context.TODO(), ctrl, configuration, 10)
			})

			It("should have the conditions", func() {
				Expect(cc.Get(context.TODO(), configuration.GetNamespacedName(), configuration)).ToNot(HaveOccurred())
				Expect(configuration.Status.Conditions).To(HaveLen(defaultConditions))
			})

			It("should indicate the configuration failed", func() {
				Expect(cc.Get(context.TODO(), configuration.GetNamespacedName(), configuration)).ToNot(HaveOccurred())

				cond := configuration.Status.GetCondition(corev1alpha1.ConditionReady)
				Expect(cond.Status).To(Equal(metav1.ConditionFalse))
				Expect(cond.Reason).To(Equal(corev1alpha1.ReasonActionRequired))
				Expect(cond.Message).To(Equal("Backend template secret \"default/missing_key\" does not contain the backend.tf key"))
			})

			It("should not create any jobs", func() {
				list := &batchv1.JobList{}

				Expect(cc.List(context.TODO(), list, client.InNamespace(ctrl.ControllerNamespace))).ToNot(HaveOccurred())
				Expect(len(list.Items)).To(Equal(0))
			})
		})

		When("the template is present but missing required key is empty", func() {
			BeforeEach(func() {
				configuration = fixtures.NewValidBucketConfiguration(cfgNamespace, "bucket")
				backend := fixtures.NewBackendTemplateSecret(ctrl.ControllerNamespace, "missing_key")
				backend.Data["backend.tf"] = []byte("")
				Setup(configuration, backend)
				ctrl.BackendTemplate = "missing_key"

				result, _, rerr = controllertests.Roll(context.TODO(), ctrl, configuration, 10)
			})

			It("should have the conditions", func() {
				Expect(cc.Get(context.TODO(), configuration.GetNamespacedName(), configuration)).ToNot(HaveOccurred())
				Expect(configuration.Status.Conditions).To(HaveLen(defaultConditions))
			})

			It("should indicate the configuration failed", func() {
				Expect(cc.Get(context.TODO(), configuration.GetNamespacedName(), configuration)).ToNot(HaveOccurred())

				cond := configuration.Status.GetCondition(corev1alpha1.ConditionReady)
				Expect(cond.Status).To(Equal(metav1.ConditionFalse))
				Expect(cond.Reason).To(Equal(corev1alpha1.ReasonActionRequired))
				Expect(cond.Message).To(Equal("Backend template secret \"default/missing_key\" does not contain the backend.tf key"))
			})

			It("should not create any jobs", func() {
				list := &batchv1.JobList{}

				Expect(cc.List(context.TODO(), list, client.InNamespace(ctrl.ControllerNamespace))).ToNot(HaveOccurred())
				Expect(len(list.Items)).To(Equal(0))
			})
		})

		When("the template is present, valid and plan has not been run", func() {
			BeforeEach(func() {
				configuration = fixtures.NewValidBucketConfiguration(cfgNamespace, "bucket")
				backend := fixtures.NewBackendTemplateSecret(ctrl.ControllerNamespace, "backend-s3")
				Setup(configuration, backend)
				ctrl.BackendTemplate = "backend-s3"

				result, _, rerr = controllertests.Roll(context.TODO(), ctrl, configuration, 10)
			})

			It("should have the conditions", func() {
				Expect(cc.Get(context.TODO(), configuration.GetNamespacedName(), configuration)).ToNot(HaveOccurred())
				Expect(configuration.Status.Conditions).To(HaveLen(defaultConditions))
			})

			It("should indicate the provider is ready", func() {
				Expect(cc.Get(context.TODO(), configuration.GetNamespacedName(), configuration)).ToNot(HaveOccurred())

				cond := configuration.Status.GetCondition(terraformv1alpha1.ConditionProviderReady)
				Expect(cond.Status).To(Equal(metav1.ConditionTrue))
				Expect(cond.Reason).To(Equal(corev1alpha1.ReasonReady))
				Expect(cond.Message).To(Equal("Provider ready"))
			})

			It("should have created the generated configuration secret", func() {
				secret := &v1.Secret{}
				secret.Namespace = ctrl.ControllerNamespace
				secret.Name = configuration.GetTerraformConfigSecretName()

				found, err := kubernetes.GetIfExists(context.TODO(), cc, secret)
				Expect(err).ToNot(HaveOccurred())
				Expect(found).To(BeTrue())

				Expect(secret.Data).To(HaveKey(terraformv1alpha1.TerraformVariablesConfigMapKey))
				Expect(secret.Data).To(HaveKey(terraformv1alpha1.TerraformBackendSecretKey))
			})

			It("should have create a terraform backend configuration", func() {
				expected := `
terraform {
  backend "s3" {
    bucket     = "terranetes-controller-state"
    key        = "cluster_one/apps/bucket"
    region     = "eu-west-2"
    access_key = "AWS_ACCESS_KEY_ID"
    secret_key = "AWS_SECRET_ACCESS_KEY"
  }
}
`
				secret := &v1.Secret{}
				secret.Namespace = ctrl.ControllerNamespace
				secret.Name = configuration.GetTerraformConfigSecretName()

				found, err := kubernetes.GetIfExists(context.TODO(), cc, secret)
				Expect(err).ToNot(HaveOccurred())
				Expect(found).To(BeTrue())

				backend := string(secret.Data[terraformv1alpha1.TerraformBackendSecretKey])
				Expect(backend).ToNot(BeZero())
				Expect(backend).To(Equal(expected))
			})

			It("should indicate the terraform plan is running", func() {
				Expect(cc.Get(context.TODO(), configuration.GetNamespacedName(), configuration)).ToNot(HaveOccurred())

				cond := configuration.Status.GetCondition(terraformv1alpha1.ConditionTerraformPlan)
				Expect(cond.Status).To(Equal(metav1.ConditionFalse))
				Expect(cond.Reason).To(Equal(corev1alpha1.ReasonInProgress))
				Expect(cond.Message).To(Equal("Terraform plan is running"))
			})

			It("should have created job for the terraform plan", func() {
				list := &batchv1.JobList{}

				Expect(cc.List(context.TODO(), list, client.InNamespace(ctrl.ControllerNamespace))).ToNot(HaveOccurred())
				Expect(len(list.Items)).To(Equal(1))
			})
		})
	})

	// DRIFT
	When("configuration has drift options", func() {
		When("drift annotation is tagged but configuration has not opted in for detection", func() {
			BeforeEach(func() {
				configuration = fixtures.NewValidBucketConfiguration(cfgNamespace, "bucket")
				configuration.Annotations = map[string]string{terraformv1alpha1.DriftAnnotation: "true"}

				Setup(configuration)
				result, _, rerr = controllertests.Roll(context.TODO(), ctrl, configuration, 3)
			})

			It("should have the conditions", func() {
				Expect(cc.Get(context.TODO(), configuration.GetNamespacedName(), configuration)).ToNot(HaveOccurred())
				Expect(configuration.Status.Conditions).To(HaveLen(defaultConditions))
			})

			It("should indicate the terraform plan is running", func() {
				Expect(cc.Get(context.TODO(), configuration.GetNamespacedName(), configuration)).ToNot(HaveOccurred())

				cond := configuration.Status.GetCondition(terraformv1alpha1.ConditionTerraformPlan)
				Expect(cond.Status).To(Equal(metav1.ConditionFalse))
				Expect(cond.Reason).To(Equal(corev1alpha1.ReasonInProgress))
				Expect(cond.Message).To(Equal("Terraform plan is running"))
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
				Expect(labels).To(HaveKey(terraformv1alpha1.ConfigurationNameLabel))
				Expect(labels).To(HaveKey(terraformv1alpha1.ConfigurationNamespaceLabel))
				Expect(labels).To(HaveKey(terraformv1alpha1.ConfigurationGenerationLabel))
				Expect(labels).To(HaveKey(terraformv1alpha1.ConfigurationStageLabel))
				Expect(labels).To(HaveKey(terraformv1alpha1.DriftAnnotation))
				Expect(labels[terraformv1alpha1.ConfigurationStageLabel]).To(Equal(terraformv1alpha1.StageTerraformPlan))
				Expect(labels[terraformv1alpha1.ConfigurationNameLabel]).To(Equal(configuration.Name))
				Expect(labels[terraformv1alpha1.ConfigurationNamespaceLabel]).To(Equal(configuration.Namespace))
				Expect(labels[terraformv1alpha1.DriftAnnotation]).To(Equal("true"))
			})

			It("should have created a watch job in the configuration namespace", func() {
				list := &batchv1.JobList{}
				Expect(cc.List(context.TODO(), list, client.InNamespace(configuration.Namespace))).ToNot(HaveOccurred())
				Expect(len(list.Items)).To(Equal(1))
				Expect(len(list.Items[0].Spec.Template.Spec.Containers)).To(Equal(1))

				container := list.Items[0].Spec.Template.Spec.Containers[0]
				Expect(container.Name).To(Equal("watch"))
				Expect(container.Command).To(Equal([]string{"/watch_logs.sh"}))
				Expect(container.Args).To(Equal([]string{"-e", "http://controller.default.svc.cluster.local/v1/builds/apps/bucket/logs?generation=0&name=bucket&namespace=apps&stage=plan&uid=1234-122-1234-1234"}))
			})
		})

		When("drift check has already been run", func() {
			BeforeEach(func() {
				configuration = fixtures.NewValidBucketConfiguration(cfgNamespace, "bucket")
				configuration.Spec.EnableDriftDetection = true
				configuration.Annotations = map[string]string{
					terraformv1alpha1.DriftAnnotation: "true",
					terraformv1alpha1.ApplyAnnotation: "false",
				}

				plan := fixtures.NewTerraformJob(configuration, ctrl.ControllerNamespace, terraformv1alpha1.StageTerraformPlan)
				plan.Status.Conditions = []batchv1.JobCondition{{Type: batchv1.JobComplete, Status: v1.ConditionTrue}}
				plan.Status.Succeeded = 1
				tfplan := fixtures.NewTerraformPlanWithDiff(configuration, ctrl.ControllerNamespace)

				Setup(configuration, plan, tfplan)
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

				cond := configuration.Status.GetCondition(terraformv1alpha1.ConditionTerraformApply)
				Expect(cond.Status).To(Equal(metav1.ConditionFalse))
				Expect(cond.Reason).To(Equal(corev1alpha1.ReasonActionRequired))
				Expect(cond.Message).To(Equal("Waiting for terraform apply annotation to be set to true"))
			})
		})

		When("drift annotation changes", func() {
			BeforeEach(func() {
				configuration = fixtures.NewValidBucketConfiguration(cfgNamespace, "bucket")
				configuration.Spec.EnableAutoApproval = true
				configuration.Annotations = map[string]string{terraformv1alpha1.DriftAnnotation: "changed"}

				job := &batchv1.Job{}
				job.Name = "test"
				job.Namespace = ctrl.ControllerNamespace
				job.Labels = map[string]string{
					terraformv1alpha1.ConfigurationGenerationLabel: fmt.Sprintf("%d", configuration.GetGeneration()),
					terraformv1alpha1.ConfigurationNameLabel:       configuration.Name,
					terraformv1alpha1.ConfigurationNamespaceLabel:  configuration.Namespace,
					terraformv1alpha1.ConfigurationStageLabel:      terraformv1alpha1.StageTerraformPlan,
					terraformv1alpha1.ConfigurationUIDLabel:        string(configuration.GetUID()),
					terraformv1alpha1.DriftAnnotation:              "different_before",
				}
				Setup(configuration, job)
				result, _, rerr = controllertests.Roll(context.TODO(), ctrl, configuration, 3)
			})

			It("should indicate the terraform plan is running", func() {
				Expect(cc.Get(context.TODO(), configuration.GetNamespacedName(), configuration)).ToNot(HaveOccurred())

				cond := configuration.Status.GetCondition(terraformv1alpha1.ConditionTerraformPlan)
				Expect(cond.Status).To(Equal(metav1.ConditionFalse))
				Expect(cond.Reason).To(Equal(corev1alpha1.ReasonInProgress))
				Expect(cond.Message).To(Equal("Terraform plan is running"))
			})

			It("should have an out of sync status", func() {
				Expect(cc.Get(context.TODO(), configuration.GetNamespacedName(), configuration)).ToNot(HaveOccurred())
				Expect(configuration.Status.ResourceStatus).To(Equal(terraformv1alpha1.ResourcesOutOfSync))
			})

			It("should create another job", func() {
				list := &batchv1.JobList{}
				Expect(cc.List(context.TODO(), list, client.InNamespace(ctrl.ControllerNamespace))).ToNot(HaveOccurred())
				Expect(len(list.Items)).To(Equal(2))
			})
		})
	})

	// CHECKOV
	When("checkov with an external source", func() {
		BeforeEach(func() {
			configuration = fixtures.NewValidBucketConfiguration(cfgNamespace, "bucket")
			Setup(configuration)

			constraint := fixtures.NewMatchAllPolicyConstraint("all")
			constraint.Spec.Constraints.Checkov.Source = &terraformv1alpha1.ExternalSource{
				URL:           "https://github.com/appvia/terranetes-policy?ref=main",
				Configuration: "config.yaml",
			}

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

		It("should add a source init container to the job", func() {
			list := &batchv1.JobList{}

			Expect(cc.List(context.TODO(), list, client.InNamespace(ctrl.ControllerNamespace))).ToNot(HaveOccurred())
			Expect(len(list.Items)).To(Equal(1))
			job := list.Items[0]

			expectedArgs := []string{
				"--comment=Retrieve policy source",
				"--command=/bin/source --dest=/run/checkov --source=https://github.com/appvia/terranetes-policy?ref=main",
			}

			Expect(len(job.Spec.Template.Spec.Containers)).To(Equal(2))
			Expect(job.Spec.Template.Spec.InitContainers[2].Name).To(Equal("policy-source"))
			Expect(job.Spec.Template.Spec.InitContainers[2].Command).To(Equal([]string{"/run/bin/step"}))
			Expect(job.Spec.Template.Spec.InitContainers[2].Args).To(Equal(expectedArgs))
		})

		It("should not have checkov configuration in job secret", func() {
			secret := &v1.Secret{}
			secret.Namespace = ctrl.ControllerNamespace
			secret.Name = configuration.GetTerraformConfigSecretName()

			found, err := kubernetes.GetIfExists(context.TODO(), ctrl.cc, secret)
			Expect(err).ToNot(HaveOccurred())
			Expect(found).To(BeTrue())
			Expect(secret.Data).ToNot(HaveKey(terraformv1alpha1.CheckovJobTemplateConfigMapKey))
		})

		It("should have updated the command line for checkov scan", func() {
			list := &batchv1.JobList{}

			Expect(cc.List(context.TODO(), list, client.InNamespace(ctrl.ControllerNamespace))).ToNot(HaveOccurred())
			Expect(len(list.Items)).To(Equal(1))
			job := list.Items[0]

			expected := []string{
				"--comment=Evaluating Against Security Policy",
				"--command=/usr/local/bin/checkov --config /run/checkov/config.yaml --framework terraform_plan -f /run/tfplan.json --soft-fail -o json -o cli --output-file-path /run --repo-root-for-plan-enrichment /data --download-external-modules true >/dev/null",
				"--command=/bin/cat /run/results_cli.txt",
				"--namespace=$(KUBE_NAMESPACE)",
				"--upload=$(POLICY_REPORT_NAME)=/run/results_json.json",
				"--is-failure=/run/steps/terraform.failed",
				"--wait-on=/run/steps/terraform.complete",
			}

			Expect(len(job.Spec.Template.Spec.Containers)).To(Equal(2))
			Expect(job.Spec.Template.Spec.Containers[1].Name).To(Equal("verify-policy"))
			Expect(job.Spec.Template.Spec.Containers[1].Command).To(Equal([]string{"/run/bin/step"}))
			Expect(job.Spec.Template.Spec.Containers[1].Args).To(Equal(expected))
		})
	})

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
				Expect(secret.Data).To(HaveKey(terraformv1alpha1.CheckovJobTemplateConfigMapKey))
				Expect(string(secret.Data[terraformv1alpha1.CheckovJobTemplateConfigMapKey])).To(Equal("framework:\n  - terraform_plan\nsoft-fail: true\ncompact: true\ncheck:\n  - check0\n  - check1"))
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
				priority.Spec.Constraints.Checkov.Selector = &terraformv1alpha1.Selector{}
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
				Expect(secret.Data).To(HaveKey(terraformv1alpha1.CheckovJobTemplateConfigMapKey))
				Expect(string(secret.Data[terraformv1alpha1.CheckovJobTemplateConfigMapKey])).To(Equal("framework:\n  - terraform_plan\nsoft-fail: true\ncompact: true\ncheck:\n  - priority"))
			})
		})

		When("we have external checks defined on the security policy", func() {
			BeforeEach(func() {
				configuration = fixtures.NewValidBucketConfiguration(cfgNamespace, "bucket")
				Setup(configuration)

				// @notes: we add a policy with and external check
				external := fixtures.NewMatchAllPolicyConstraint("external")
				external.Spec.Constraints.Checkov.Checks = []string{"check0"}
				external.Spec.Constraints.Checkov.External = []terraformv1alpha1.ExternalCheck{
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
					"--command=/usr/local/bin/checkov --config /run/checkov/checkov.yaml --framework terraform_plan -f /run/tfplan.json --soft-fail -o json -o cli --output-file-path /run --repo-root-for-plan-enrichment /data --download-external-modules true >/dev/null",
					"--command=/bin/cat /run/results_cli.txt",
					"--namespace=$(KUBE_NAMESPACE)",
					"--upload=$(POLICY_REPORT_NAME)=/run/results_json.json",
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
				Expect(secret.Data).To(HaveKey(terraformv1alpha1.CheckovJobTemplateConfigMapKey))
				Expect(string(secret.Data[terraformv1alpha1.CheckovJobTemplateConfigMapKey])).To(Equal("framework:\n  - terraform_plan\nsoft-fail: true\ncompact: true\ncheck:\n  - check0\nexternal-checks-dir:\n  - /run/policy/test"))
			})
		})

		When("configuration has matched a policy", func() {
			BeforeEach(func() {
				configuration = fixtures.NewValidBucketConfiguration(cfgNamespace, "bucket")

				// @note: we create a policy, we set the terraform plan job to complete and we create
				// a fake security report
				policy := fixtures.NewMatchAllPolicyConstraint("all")
				policy.Spec.Constraints.Checkov.Checks = []string{"check0"}
				plan := fixtures.NewTerraformJob(configuration, ctrl.ControllerNamespace, terraformv1alpha1.StageTerraformPlan)
				plan.Status.Conditions = []batchv1.JobCondition{{Type: batchv1.JobComplete, Status: v1.ConditionTrue}}
				plan.Status.Succeeded = 1
				tfplan := fixtures.NewTerraformPlanWithDiff(configuration, ctrl.ControllerNamespace)

				report := &v1.Secret{}
				report.Namespace = ctrl.ControllerNamespace
				report.Name = configuration.GetTerraformPolicySecretName()
				report.Data = map[string][]byte{"results_json.json": []byte(`{"summary":{"failed": 1}}`)}

				Setup(configuration, policy, plan, report, tfplan)
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

					cond := configuration.Status.GetCondition(terraformv1alpha1.ConditionTerraformPolicy)
					Expect(cond.Status).To(Equal(metav1.ConditionFalse))
					Expect(cond.Reason).To(Equal(corev1alpha1.ReasonWarning))
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

					cond := configuration.Status.GetCondition(terraformv1alpha1.ConditionTerraformPolicy)
					Expect(cond.Status).To(Equal(metav1.ConditionFalse))
					Expect(cond.Reason).To(Equal(corev1alpha1.ReasonActionRequired))
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

					cond := configuration.Status.GetCondition(terraformv1alpha1.ConditionTerraformPolicy)
					Expect(cond.Status).To(Equal(metav1.ConditionTrue))
					Expect(cond.Reason).To(Equal(corev1alpha1.ReasonReady))
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

			When("policy contains no passes or failures", func() {
				BeforeEach(func() {
					report := &v1.Secret{}
					report.Namespace = ctrl.ControllerNamespace
					report.Name = configuration.GetTerraformPolicySecretName()

					// @note: delete the old secret adding a passed one
					Expect(ctrl.cc.Delete(context.TODO(), report)).ToNot(HaveOccurred())
					report.Data = map[string][]byte{"results_json.json": []byte(`{"failed": 0, "passed": 0}}`)}
					Expect(ctrl.cc.Create(context.TODO(), report)).ToNot(HaveOccurred())

					result, _, rerr = controllertests.Roll(context.TODO(), ctrl, configuration, 3)
				})

				It("should have the conditions", func() {
					Expect(cc.Get(context.TODO(), configuration.GetNamespacedName(), configuration)).ToNot(HaveOccurred())
					Expect(configuration.Status.Conditions).To(HaveLen(defaultConditions))
				})

				It("should indicate the we failed", func() {
					Expect(cc.Get(context.TODO(), configuration.GetNamespacedName(), configuration)).ToNot(HaveOccurred())

					cond := configuration.Status.GetCondition(terraformv1alpha1.ConditionTerraformPolicy)
					Expect(cond.Status).To(Equal(metav1.ConditionTrue))
					Expect(cond.Reason).To(Equal(corev1alpha1.ReasonReady))
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
				})
			})

			When("policy report zero, but missing passed field", func() {
				BeforeEach(func() {
					report := &v1.Secret{}
					report.Namespace = ctrl.ControllerNamespace
					report.Name = configuration.GetTerraformPolicySecretName()

					// @note: delete the old secret adding a passed one
					Expect(ctrl.cc.Delete(context.TODO(), report)).ToNot(HaveOccurred())
					report.Data = map[string][]byte{"results_json.json": []byte(`{"failed": 0}}`)}
					Expect(ctrl.cc.Create(context.TODO(), report)).ToNot(HaveOccurred())

					result, _, rerr = controllertests.Roll(context.TODO(), ctrl, configuration, 3)
				})

				It("should have the conditions", func() {
					Expect(cc.Get(context.TODO(), configuration.GetNamespacedName(), configuration)).ToNot(HaveOccurred())
					Expect(configuration.Status.Conditions).To(HaveLen(defaultConditions))
				})

				It("should indicate the we failed", func() {
					Expect(cc.Get(context.TODO(), configuration.GetNamespacedName(), configuration)).ToNot(HaveOccurred())

					cond := configuration.Status.GetCondition(terraformv1alpha1.ConditionTerraformPolicy)
					Expect(cond.Status).To(Equal(metav1.ConditionFalse))
					Expect(cond.Reason).To(Equal(corev1alpha1.ReasonError))
					Expect(cond.Message).To(Equal("Security report is missing passed field"))
				})

				It("should have not create an apply job", func() {
					list := &batchv1.JobList{}

					Expect(cc.List(context.TODO(), list, client.InNamespace(ctrl.ControllerNamespace))).ToNot(HaveOccurred())
					Expect(len(list.Items)).To(Equal(1))
				})
			})

			When("policy report zero, but missing failed field", func() {
				BeforeEach(func() {
					report := &v1.Secret{}
					report.Namespace = ctrl.ControllerNamespace
					report.Name = configuration.GetTerraformPolicySecretName()

					// @note: delete the old secret adding a passed one
					Expect(ctrl.cc.Delete(context.TODO(), report)).ToNot(HaveOccurred())
					report.Data = map[string][]byte{"results_json.json": []byte(`{"passed": 0}}`)}
					Expect(ctrl.cc.Create(context.TODO(), report)).ToNot(HaveOccurred())

					result, _, rerr = controllertests.Roll(context.TODO(), ctrl, configuration, 3)
				})

				It("should have the conditions", func() {
					Expect(cc.Get(context.TODO(), configuration.GetNamespacedName(), configuration)).ToNot(HaveOccurred())
					Expect(configuration.Status.Conditions).To(HaveLen(defaultConditions))
				})

				It("should indicate the we failed", func() {
					Expect(cc.Get(context.TODO(), configuration.GetNamespacedName(), configuration)).ToNot(HaveOccurred())

					cond := configuration.Status.GetCondition(terraformv1alpha1.ConditionTerraformPolicy)
					Expect(cond.Status).To(Equal(metav1.ConditionFalse))
					Expect(cond.Reason).To(Equal(corev1alpha1.ReasonError))
					Expect(cond.Message).To(Equal("Security report is missing failed field"))
				})

				It("should have not create an apply job", func() {
					list := &batchv1.JobList{}

					Expect(cc.List(context.TODO(), list, client.InNamespace(ctrl.ControllerNamespace))).ToNot(HaveOccurred())
					Expect(len(list.Items)).To(Equal(1))
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

				cond := configuration.Status.GetCondition(corev1alpha1.ConditionReady)
				Expect(cond.Status).To(Equal(metav1.ConditionFalse))
				Expect(cond.Reason).To(Equal(corev1alpha1.ReasonActionRequired))
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

				cond := configuration.Status.GetCondition(corev1alpha1.ConditionReady)
				Expect(cond.Status).To(Equal(metav1.ConditionFalse))
				Expect(cond.Reason).To(Equal(corev1alpha1.ReasonActionRequired))
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

				cond := configuration.Status.GetCondition(terraformv1alpha1.ConditionTerraformPlan)
				Expect(cond.Status).To(Equal(metav1.ConditionFalse))
				Expect(cond.Reason).To(Equal(corev1alpha1.ReasonInProgress))
				Expect(cond.Message).To(Equal("Terraform plan in progress"))
			})
		})
	})

	// BEFORE TERRAFORM APPLY
	When("terraform apply has not been provisoned", func() {
		When("the configuration needs approval", func() {
			BeforeEach(func() {
				configuration = fixtures.NewValidBucketConfiguration(cfgNamespace, "bucket")
				configuration.Annotations = map[string]string{terraformv1alpha1.ApplyAnnotation: "false"}
				plan := fixtures.NewTerraformJob(configuration, ctrl.ControllerNamespace, terraformv1alpha1.StageTerraformPlan)
				plan.Status.Conditions = []batchv1.JobCondition{{Type: batchv1.JobComplete, Status: v1.ConditionTrue}}
				plan.Status.Succeeded = 1
				tfplan := fixtures.NewTerraformPlanWithDiff(configuration, ctrl.ControllerNamespace)

				Setup(configuration, plan, tfplan)
				result, _, rerr = controllertests.Roll(context.TODO(), ctrl, configuration, 5)
			})

			It("should have the conditions", func() {
				Expect(cc.Get(context.TODO(), configuration.GetNamespacedName(), configuration)).ToNot(HaveOccurred())
				Expect(configuration.Status.Conditions).To(HaveLen(defaultConditions))
			})

			It("should indicate configuration is waiting approval", func() {
				Expect(cc.Get(context.TODO(), configuration.GetNamespacedName(), configuration)).ToNot(HaveOccurred())

				cond := configuration.Status.GetCondition(terraformv1alpha1.ConditionTerraformApply)
				Expect(cond.Status).To(Equal(metav1.ConditionFalse))
				Expect(cond.Reason).To(Equal(corev1alpha1.ReasonActionRequired))
				Expect(cond.Message).To(Equal("Waiting for terraform apply annotation to be set to true"))
			})

			It("should indicate the ready condition is false", func() {
				Expect(cc.Get(context.TODO(), configuration.GetNamespacedName(), configuration)).ToNot(HaveOccurred())

				cond := configuration.Status.GetCondition(corev1alpha1.ConditionReady)
				Expect(cond.Status).To(Equal(metav1.ConditionFalse))
				Expect(cond.Reason).To(Equal(corev1alpha1.ReasonInProgress))
				Expect(cond.Message).To(Equal("Waiting for changes to be approved"))
			})

			It("should indicate the resource out of sync", func() {
				Expect(cc.Get(context.TODO(), configuration.GetNamespacedName(), configuration)).ToNot(HaveOccurred())
				Expect(configuration.Status.ResourceStatus).To(Equal(terraformv1alpha1.ResourcesOutOfSync))
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
				plan := fixtures.NewTerraformJob(configuration, ctrl.ControllerNamespace, terraformv1alpha1.StageTerraformPlan)
				plan.Status.Conditions = []batchv1.JobCondition{{Type: batchv1.JobComplete, Status: v1.ConditionTrue}}
				plan.Status.Succeeded = 1
				tfplan := fixtures.NewTerraformPlanWithDiff(configuration, ctrl.ControllerNamespace)
				Setup(configuration, plan, tfplan)
				result, _, rerr = controllertests.Roll(context.TODO(), ctrl, configuration, 5)
			})

			It("should have the conditions", func() {
				Expect(cc.Get(context.TODO(), configuration.GetNamespacedName(), configuration)).ToNot(HaveOccurred())
				Expect(configuration.Status.Conditions).To(HaveLen(defaultConditions))
			})

			It("should indicate the terraform apply is running", func() {
				Expect(cc.Get(context.TODO(), configuration.GetNamespacedName(), configuration)).ToNot(HaveOccurred())

				cond := configuration.Status.GetCondition(terraformv1alpha1.ConditionTerraformPlan)
				Expect(cond.Status).To(Equal(metav1.ConditionTrue))
				Expect(cond.Reason).To(Equal(corev1alpha1.ReasonReady))
				Expect(cond.Message).To(Equal("Terraform plan is complete"))
			})

			It("should have created job for the terraform apply", func() {
				list := &batchv1.JobList{}

				Expect(cc.List(context.TODO(), list, client.InNamespace(ctrl.ControllerNamespace))).ToNot(HaveOccurred())
				Expect(len(list.Items)).To(Equal(2))
			})

			It("should have a terraform container", func() {
				list := &batchv1.JobList{}
				Expect(cc.List(context.TODO(), list, client.InNamespace(ctrl.ControllerNamespace))).ToNot(HaveOccurred())
				Expect(len(list.Items)).To(Equal(2))

				expected := []string{
					"--comment=Executing Terraform",
					"--command=/usr/local/bin/tofu apply --var-file variables.tfvars.json -lock=false -no-color -input=false -auto-approve",
					"--on-error=/run/steps/terraform.failed",
					"--on-success=/run/steps/terraform.complete",
				}
				job := list.Items[0]
				container := job.Spec.Template.Spec.Containers[0]

				Expect(container.Name).To(Equal("terraform"))
				Expect(container.Command).To(Equal([]string{"/run/bin/step"}))
				Expect(container.Args).To(Equal(expected))

				Expect(len(container.EnvFrom)).To(Equal(1))
				Expect(container.EnvFrom[0].SecretRef).ToNot(BeNil())
				Expect(container.EnvFrom[0].SecretRef.Name).To(Equal("aws"))

				Expect(len(container.Env)).To(Equal(8))
				Expect(container.Env[5].Name).To(Equal("TERRAFORM_STATE_NAME"))
				Expect(container.Env[5].Value).To(Equal(configuration.GetTerraformStateSecretName()))
				Expect(container.Env[6].Name).To(Equal("TERRAFORM_PLAN_OUT_NAME"))
				Expect(container.Env[6].Value).To(Equal(configuration.GetTerraformPlanOutSecretName()))
				Expect(container.Env[7].Name).To(Equal("TERRAFORM_PLAN_JSON_NAME"))
				Expect(container.Env[7].Value).To(Equal(configuration.GetTerraformPlanJSONSecretName()))

				Expect(container.VolumeMounts).To(HaveLen(3))
				Expect(container.VolumeMounts[0].Name).To(Equal("planout"))
				Expect(container.VolumeMounts[1].Name).To(Equal("run"))
				Expect(container.VolumeMounts[2].Name).To(Equal("source"))
			})

			It("it should have the configuration labels", func() {
				list := &batchv1.JobList{}
				Expect(cc.List(context.TODO(), list, client.InNamespace(ctrl.ControllerNamespace))).ToNot(HaveOccurred())

				labels := list.Items[0].GetLabels()
				Expect(labels).To(HaveKey(terraformv1alpha1.ConfigurationNameLabel))
				Expect(labels).To(HaveKey(terraformv1alpha1.ConfigurationNamespaceLabel))
				Expect(labels).To(HaveKey(terraformv1alpha1.ConfigurationGenerationLabel))
				Expect(labels).To(HaveKey(terraformv1alpha1.ConfigurationStageLabel))
				Expect(labels[terraformv1alpha1.ConfigurationStageLabel]).To(Equal(terraformv1alpha1.StageTerraformApply))
				Expect(labels[terraformv1alpha1.ConfigurationNameLabel]).To(Equal(configuration.Name))
				Expect(labels[terraformv1alpha1.ConfigurationNamespaceLabel]).To(Equal(configuration.Namespace))
			})

			It("should have created a watch job in the configuration namespace", func() {
				list := &batchv1.JobList{}
				Expect(cc.List(context.TODO(), list, client.InNamespace(configuration.Namespace))).ToNot(HaveOccurred())
				Expect(len(list.Items)).To(Equal(1))
				Expect(len(list.Items[0].Spec.Template.Spec.Containers)).To(Equal(1))

				container := list.Items[0].Spec.Template.Spec.Containers[0]
				Expect(container.Name).To(Equal("watch"))
				Expect(container.Command).To(Equal([]string{"/watch_logs.sh"}))
				Expect(container.Args).To(Equal([]string{"-e", "http://controller.default.svc.cluster.local/v1/builds/apps/bucket/logs?generation=0&name=bucket&namespace=apps&stage=apply&uid=1234-122-1234-1234"}))
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
			plan := fixtures.NewTerraformJob(configuration, ctrl.ControllerNamespace, terraformv1alpha1.StageTerraformPlan)
			plan.Status.Conditions = []batchv1.JobCondition{{Type: batchv1.JobComplete, Status: v1.ConditionTrue}}
			plan.Status.Succeeded = 1
			tfplan := fixtures.NewTerraformPlanWithDiff(configuration, ctrl.ControllerNamespace)

			apply := fixtures.NewTerraformJob(configuration, ctrl.ControllerNamespace, terraformv1alpha1.StageTerraformApply)
			apply.Status.Conditions = []batchv1.JobCondition{{Type: batchv1.JobComplete, Status: v1.ConditionTrue}}
			apply.Status.Succeeded = 1
			apply.Labels[terraformv1alpha1.JobPlanIDLabel] = fixtures.TFPlanID

			// create a fake terraform state
			state := fixtures.NewTerraformState(configuration)
			state.Namespace = "default"

			Setup(configuration, plan, apply, state, tfplan)
			result, _, rerr = controllertests.Roll(context.TODO(), ctrl, configuration, 3)
		})

		It("should have the conditions", func() {
			Expect(cc.Get(context.TODO(), configuration.GetNamespacedName(), configuration)).ToNot(HaveOccurred())
			Expect(configuration.Status.Conditions).To(HaveLen(defaultConditions))
		})

		It("should indicate the terraform apply has run", func() {
			Expect(cc.Get(context.TODO(), configuration.GetNamespacedName(), configuration)).ToNot(HaveOccurred())

			cond := configuration.Status.GetCondition(terraformv1alpha1.ConditionTerraformApply)
			Expect(cond.Message).To(Equal("Terraform apply is complete"))
			Expect(cond.Status).To(Equal(metav1.ConditionTrue))
			Expect(cond.Reason).To(Equal(corev1alpha1.ReasonReady))
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
			Expect(configuration.Status.Resources).ToNot(BeNil())
			Expect(*configuration.Status.Resources).To(Equal(1))
		})

		It("should have a in resource status", func() {
			Expect(cc.Get(context.TODO(), configuration.GetNamespacedName(), configuration)).ToNot(HaveOccurred())

			Expect(configuration.Status.ResourceStatus).To(Equal(terraformv1alpha1.ResourcesInSync))
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

	When("terraform apply is not needed and state secret exists", func() {
		BeforeEach(func() {
			configuration = fixtures.NewValidBucketConfiguration(cfgNamespace, "bucket")
			plan := fixtures.NewTerraformJob(configuration, ctrl.ControllerNamespace, terraformv1alpha1.StageTerraformPlan)
			plan.Status.Conditions = []batchv1.JobCondition{{Type: batchv1.JobComplete, Status: v1.ConditionTrue}}
			plan.Status.Succeeded = 1
			tfplan := fixtures.NewTerraformPlanNoop(configuration, ctrl.ControllerNamespace)

			// create a fake terraform state
			state := fixtures.NewTerraformState(configuration)
			state.Namespace = "default"

			Setup(configuration, plan, state, tfplan)
			result, _, rerr = controllertests.Roll(context.TODO(), ctrl, configuration, 10)
		})

		It("should have the conditions", func() {
			Expect(cc.Get(context.TODO(), configuration.GetNamespacedName(), configuration)).ToNot(HaveOccurred())
			Expect(configuration.Status.Conditions).To(HaveLen(defaultConditions))
		})

		It("should indicate the terraform apply was skipped run", func() {
			Expect(cc.Get(context.TODO(), configuration.GetNamespacedName(), configuration)).ToNot(HaveOccurred())

			cond := configuration.Status.GetCondition(terraformv1alpha1.ConditionTerraformApply)
			Expect(cond.Message).To(Equal("Nothing to apply"))
			Expect(cond.Status).To(Equal(metav1.ConditionTrue))
			Expect(cond.Reason).To(Equal(corev1alpha1.ReasonReady))
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
			Expect(configuration.Status.Resources).ToNot(BeNil())
			Expect(*configuration.Status.Resources).To(Equal(1))
		})

		It("should have a in resource status", func() {
			Expect(cc.Get(context.TODO(), configuration.GetNamespacedName(), configuration)).ToNot(HaveOccurred())

			Expect(configuration.Status.ResourceStatus).To(Equal(terraformv1alpha1.ResourcesInSync))
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

	When("terraform apply is not needed but state secret does not exist", func() {
		BeforeEach(func() {
			configuration = fixtures.NewValidBucketConfiguration(cfgNamespace, "bucket")
			plan := fixtures.NewTerraformJob(configuration, ctrl.ControllerNamespace, terraformv1alpha1.StageTerraformPlan)
			plan.Status.Conditions = []batchv1.JobCondition{{Type: batchv1.JobComplete, Status: v1.ConditionTrue}}
			plan.Status.Succeeded = 1
			tfplan := fixtures.NewTerraformPlanNoop(configuration, ctrl.ControllerNamespace)

			Setup(configuration, plan, tfplan)
			result, _, rerr = controllertests.Roll(context.TODO(), ctrl, configuration, 5)
		})

		It("should have the conditions", func() {
			Expect(cc.Get(context.TODO(), configuration.GetNamespacedName(), configuration)).ToNot(HaveOccurred())
			Expect(configuration.Status.Conditions).To(HaveLen(defaultConditions))
		})

		It("should indicate the terraform apply is running", func() {
			Expect(cc.Get(context.TODO(), configuration.GetNamespacedName(), configuration)).ToNot(HaveOccurred())

			cond := configuration.Status.GetCondition(terraformv1alpha1.ConditionTerraformApply)
			Expect(cond.Status).To(Equal(metav1.ConditionFalse))
			Expect(cond.Reason).To(Equal(corev1alpha1.ReasonInProgress))
			Expect(cond.Message).To(Equal("Terraform apply in progress"))
		})

		It("should have created job for the terraform apply", func() {
			list := &batchv1.JobList{}

			Expect(cc.List(context.TODO(), list, client.InNamespace(ctrl.ControllerNamespace))).ToNot(HaveOccurred())
			Expect(len(list.Items)).To(Equal(2))
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
			plan := fixtures.NewTerraformJob(configuration, ctrl.ControllerNamespace, terraformv1alpha1.StageTerraformPlan)
			plan.Status.Conditions = []batchv1.JobCondition{{Type: batchv1.JobComplete, Status: v1.ConditionTrue}}
			plan.Status.Succeeded = 1
			tfplan := fixtures.NewTerraformPlanWithDiff(configuration, ctrl.ControllerNamespace)

			apply := fixtures.NewTerraformJob(configuration, ctrl.ControllerNamespace, terraformv1alpha1.StageTerraformApply)
			apply.Status.Conditions = []batchv1.JobCondition{{Type: batchv1.JobComplete, Status: v1.ConditionTrue}}
			apply.Status.Succeeded = 1
			apply.Labels[terraformv1alpha1.JobPlanIDLabel] = fixtures.TFPlanID

			// create a fake terraform state
			state := fixtures.NewTerraformState(configuration)
			state.Namespace = "default"

			Setup(configuration, plan, apply, state, tfplan)
			result, _, rerr = controllertests.Roll(context.TODO(), ctrl, configuration, 3)
		})

		It("should have the conditions", func() {
			Expect(cc.Get(context.TODO(), configuration.GetNamespacedName(), configuration)).ToNot(HaveOccurred())
			Expect(configuration.Status.Conditions).To(HaveLen(defaultConditions))
		})

		It("should indicate the terraform apply has run", func() {
			Expect(cc.Get(context.TODO(), configuration.GetNamespacedName(), configuration)).ToNot(HaveOccurred())

			cond := configuration.Status.GetCondition(terraformv1alpha1.ConditionTerraformApply)
			Expect(cond.Status).To(Equal(metav1.ConditionTrue))
			Expect(cond.Reason).To(Equal(corev1alpha1.ReasonReady))
			Expect(cond.Message).To(Equal("Terraform apply is complete"))
		})

		It("should have a in resource status", func() {
			Expect(cc.Get(context.TODO(), configuration.GetNamespacedName(), configuration)).ToNot(HaveOccurred())

			Expect(configuration.Status.ResourceStatus).To(Equal(terraformv1alpha1.ResourcesInSync))
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
