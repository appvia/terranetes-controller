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

package configuration

import (
	"context"
	"fmt"
	"io/ioutil"
	"strconv"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	cache "github.com/patrickmn/go-cache"
	"github.com/sirupsen/logrus"
	batchv1 "k8s.io/api/batch/v1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kfake "k8s.io/client-go/kubernetes/fake"
	"k8s.io/utils/pointer"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	corev1alpha1 "github.com/appvia/terranetes-controller/pkg/apis/core/v1alpha1"
	terraformv1alpha1 "github.com/appvia/terranetes-controller/pkg/apis/terraform/v1alpha1"
	"github.com/appvia/terranetes-controller/pkg/controller"
	"github.com/appvia/terranetes-controller/pkg/schema"
	"github.com/appvia/terranetes-controller/pkg/utils"
	"github.com/appvia/terranetes-controller/pkg/utils/kubernetes"
	controllertests "github.com/appvia/terranetes-controller/test"
	"github.com/appvia/terranetes-controller/test/fixtures"
)

var _ = Describe("Configuration Controller with Contexts", func() {
	logrus.SetOutput(ioutil.Discard)

	var cc client.Client
	var result reconcile.Result
	var rerr error
	var ctrl *Controller
	var configuration *terraformv1alpha1.Configuration
	var provider *terraformv1alpha1.Provider
	var recorder *controllertests.FakeRecorder

	BeforeEach(func() {
		secret := fixtures.NewValidAWSProviderSecret("terraform-system", "aws")
		provider = fixtures.NewValidAWSReadyProvider("aws", secret)
		configuration = fixtures.NewValidBucketConfiguration("default", "test")
		configuration.Status.Resources = ptr.To(1)
		controller.EnsureConditionsRegistered(terraformv1alpha1.DefaultConfigurationConditions, configuration)

		configuration.Finalizers = []string{controllerName}
		state := fixtures.NewTerraformState(configuration)
		state.Namespace = "terraform-system"

		namespace := fixtures.NewNamespace(configuration.Namespace)

		cc = fake.NewClientBuilder().
			WithScheme(schema.GetScheme()).
			WithStatusSubresource(&terraformv1alpha1.Configuration{}).
			Build()

		Expect(cc.Create(context.Background(), secret)).To(Succeed())
		Expect(cc.Create(context.Background(), provider)).To(Succeed())
		Expect(cc.Create(context.Background(), namespace)).To(Succeed())
		Expect(cc.Create(context.Background(), configuration)).To(Succeed())
		Expect(cc.Create(context.Background(), state)).To(Succeed())

		Expect(cc.Status().Update(context.Background(), configuration)).To(Succeed())
		Expect(cc.Get(context.Background(), configuration.GetNamespacedName(), configuration)).To(Succeed())
		Expect(cc.Delete(context.Background(), configuration)).To(Succeed())
		Expect(cc.Get(context.Background(), configuration.GetNamespacedName(), configuration)).To(Succeed())

		recorder = &controllertests.FakeRecorder{}
		ctrl = &Controller{
			cc:                  cc,
			kc:                  kfake.NewSimpleClientset(),
			cache:               cache.New(5*time.Minute, 10*time.Minute),
			recorder:            recorder,
			EnableInfracosts:    false,
			EnableWatchers:      true,
			ExecutorImage:       "ghcr.io/appvia/terranetes-executor",
			InfracostsImage:     "infracosts/infracost:latest",
			ControllerNamespace: "terraform-system",
			PolicyImage:         "bridgecrew/checkov:2.0.1140",
			TerraformImage:      "hashicorp/terraform:1.1.9",
		}

		ctrl.cache.SetDefault("default", fixtures.NewNamespace("default"))
	})

	When("a configuration is deleted", func() {
		Context("we have a orphaned annotation", func() {
			BeforeEach(func() {
				configuration.Annotations = utils.MergeStringMaps(configuration.Annotations,
					map[string]string{
						terraformv1alpha1.OrphanAnnotation: "true",
					},
				)
				Expect(cc.Update(context.Background(), configuration)).To(Succeed())

				result, _, rerr = controllertests.Roll(context.TODO(), ctrl, configuration, 0)
			})

			It("should delete the configuration", func() {
				Expect(cc.Get(context.TODO(), configuration.GetNamespacedName(), configuration)).To(HaveOccurred())
			})

			It("should have deleted the configuration secrets", func() {
				secret, found, err := kubernetes.GetSecretIfExists(context.TODO(), cc, ctrl.ControllerNamespace, configuration.GetTerraformStateSecretName())
				Expect(err).ToNot(HaveOccurred())
				Expect(found).To(BeFalse())
				Expect(secret).To(BeNil())
			})
		})

		Context("and we are checking for resources", func() {
			Context("but the resources is nil", func() {
				BeforeEach(func() {
					configuration.Status.Resources = nil

					Expect(cc.Status().Update(context.Background(), configuration)).To(Succeed())
					result, _, rerr = controllertests.Roll(context.TODO(), ctrl, configuration, 0)
				})

				It("should try to destroy the configuration", func() {
					list := &batchv1.JobList{}
					Expect(cc.List(context.Background(), list)).To(Succeed())
					Expect(list.Items).ToNot(HaveLen(0))
				})
			})

			Context("but the resource count is not zero", func() {
				BeforeEach(func() {
					result, _, rerr = controllertests.Roll(context.TODO(), ctrl, configuration, 0)
				})

				It("should try to destroy the configuration", func() {
					list := &batchv1.JobList{}
					Expect(cc.List(context.Background(), list)).To(Succeed())
					Expect(list.Items).ToNot(HaveLen(0))
				})
			})

			Context("but the configuration has no resources", func() {
				BeforeEach(func() {
					configuration.Status.Resources = ptr.To(0)
					Expect(cc.Status().Update(context.Background(), configuration)).To(Succeed())
					Expect(cc.Get(context.Background(), configuration.GetNamespacedName(), configuration)).To(Succeed())

					result, _, rerr = controllertests.Roll(context.TODO(), ctrl, configuration, 0)
				})

				It("should not create the destroy job", func() {
					list := &batchv1.JobList{}
					Expect(cc.List(context.Background(), list)).To(Succeed())
					Expect(list.Items).To(HaveLen(0))
				})

				It("should have created an event", func() {
					Expect(recorder.Events).To(HaveLen(1))
					Expect(recorder.Events[0]).To(Equal("(default/test) Normal DeletionSkipped: Configuration had zero resources, skipping terraform destroy"))
				})

				It("should have deleted the configuration", func() {
					list := &terraformv1alpha1.ConfigurationList{}
					Expect(cc.List(context.Background(), list)).To(Succeed())
					Expect(list.Items).To(HaveLen(0))
				})
			})
		})

		Context("and the provider is not ready", func() {
			BeforeEach(func() {
				provider.Status.GetCondition(corev1alpha1.ConditionReady).Status = metav1.ConditionFalse
				Expect(cc.Update(context.Background(), provider)).ToNot(HaveOccurred())

				result, _, rerr = controllertests.Roll(context.TODO(), ctrl, configuration, 0)
			})

			It("should requeue", func() {
				Expect(rerr).ToNot(HaveOccurred())
				Expect(result.Requeue).To(BeFalse())
				Expect(result.RequeueAfter).To(Equal(time.Second * 30))
			})

			It("should indicate the status in the conditions", func() {
				Expect(cc.Get(context.TODO(), configuration.GetNamespacedName(), configuration)).ToNot(HaveOccurred())

				cond := configuration.Status.GetCondition(terraformv1alpha1.ConditionProviderReady)
				Expect(cond.Status).To(Equal(metav1.ConditionFalse))
				Expect(cond.Reason).To(Equal(corev1alpha1.ReasonWarning))
				Expect(cond.Message).To(Equal("Provider is not ready"))
			})

			It("should not delete the configuration", func() {
				Expect(cc.Get(context.TODO(), configuration.GetNamespacedName(), configuration)).ToNot(HaveOccurred())
			})
		})

		Context("and the provider is missing", func() {
			BeforeEach(func() {
				Expect(cc.Delete(context.Background(), provider)).ToNot(HaveOccurred())

				result, _, rerr = controllertests.Roll(context.TODO(), ctrl, configuration, 0)
			})

			It("should indicate the status in the conditions", func() {
				Expect(cc.Get(context.TODO(), configuration.GetNamespacedName(), configuration)).ToNot(HaveOccurred())

				cond := configuration.Status.GetCondition(terraformv1alpha1.ConditionProviderReady)
				Expect(cond.Status).To(Equal(metav1.ConditionFalse))
				Expect(cond.Reason).To(Equal(corev1alpha1.ReasonActionRequired))
				Expect(cond.Message).To(Equal("Provider referenced \"aws\" does not exist"))
			})

			It("should not delete the configuration", func() {
				Expect(cc.Get(context.TODO(), configuration.GetNamespacedName(), configuration)).ToNot(HaveOccurred())
			})
		})

		Context("and we are using policy injected secrets", func() {
			var secret *v1.Secret

			BeforeEach(func() {
				policy := fixtures.NewPolicy("injected")
				policy.Spec.Defaults = []terraformv1alpha1.DefaultVariables{
					{
						Secrets: []string{"foo"},
					},
				}
				secret = &v1.Secret{}
				secret.Name = "foo"
				secret.Namespace = ctrl.ControllerNamespace
				secret.Data = map[string][]byte{"foo": []byte("bar")}

				Expect(cc.Create(context.Background(), policy)).ToNot(HaveOccurred())
				Expect(cc.Create(context.Background(), secret)).ToNot(HaveOccurred())
			})

			Context("and the secret is missing", func() {
				BeforeEach(func() {
					Expect(cc.Delete(context.Background(), secret)).ToNot(HaveOccurred())

					result, _, rerr = controllertests.Roll(context.TODO(), ctrl, configuration, 0)
				})

				It("should indicate the status in the conditions", func() {
					Expect(cc.Get(context.TODO(), configuration.GetNamespacedName(), configuration)).ToNot(HaveOccurred())

					cond := configuration.Status.GetCondition(corev1alpha1.ConditionReady)
					Expect(cond.Status).To(Equal(metav1.ConditionFalse))
					Expect(cond.Reason).To(Equal(corev1alpha1.ReasonActionRequired))
					Expect(cond.Message).To(Equal("Default secret (terraform-system/foo) does not exist, please contact administrator"))
				})

				It("should not delete the configuration", func() {
					Expect(cc.Get(context.Background(), configuration.GetNamespacedName(), configuration)).ToNot(HaveOccurred())
				})
			})

			Context("and the secret is available", func() {
				BeforeEach(func() {
					result, _, rerr = controllertests.Roll(context.TODO(), ctrl, configuration, 0)
				})

				It("should indicate the status in the conditions", func() {
					Expect(cc.Get(context.TODO(), configuration.GetNamespacedName(), configuration)).ToNot(HaveOccurred())

					cond := configuration.Status.GetCondition(corev1alpha1.ConditionReady)
					Expect(cond.Status).To(Equal(metav1.ConditionFalse))
					Expect(cond.Reason).To(Equal(corev1alpha1.ReasonInProgress))
					Expect(cond.Message).To(Equal("Terraform destroy is running"))
				})

				It("should have a secret included in the terraform destroy job ", func() {
					list := &batchv1.JobList{}
					Expect(cc.List(context.Background(), list,
						client.InNamespace(ctrl.ControllerNamespace),
						client.MatchingLabels(map[string]string{
							terraformv1alpha1.ConfigurationStageLabel: terraformv1alpha1.StageTerraformDestroy,
						},
						))).ToNot(HaveOccurred())

					Expect(list.Items).To(HaveLen(1))

					Expect(list.Items[0].Spec.Template.Spec.InitContainers[0].Name).To(Equal("setup"))
					Expect(list.Items[0].Spec.Template.Spec.InitContainers[0].EnvFrom).To(HaveLen(2))
					Expect(list.Items[0].Spec.Template.Spec.InitContainers[0].EnvFrom[0].SecretRef.Name).To(Equal(configuration.GetTerraformConfigSecretName()))
					Expect(list.Items[0].Spec.Template.Spec.InitContainers[0].EnvFrom[1].SecretRef.Name).To(Equal(secret.Name))

					Expect(list.Items[0].Spec.Template.Spec.InitContainers[1].Name).To(Equal("init"))
					Expect(list.Items[0].Spec.Template.Spec.InitContainers[1].EnvFrom).To(HaveLen(1))
					Expect(list.Items[0].Spec.Template.Spec.InitContainers[1].EnvFrom[0].SecretRef.Name).To(Equal(secret.Name))

					Expect(list.Items[0].Spec.Template.Spec.Containers[0].Name).To(Equal("terraform"))
					Expect(list.Items[0].Spec.Template.Spec.Containers[0].EnvFrom).To(HaveLen(2))
					Expect(list.Items[0].Spec.Template.Spec.Containers[0].EnvFrom[0].SecretRef.Name).To(Equal(provider.Name))
					Expect(list.Items[0].Spec.Template.Spec.Containers[0].EnvFrom[1].SecretRef.Name).To(Equal(secret.Name))
				})
			})
		})

		Context("and the configuration is using value from", func() {
			BeforeEach(func() {
				configuration.Spec.ValueFrom = []terraformv1alpha1.ValueFromSource{
					{
						Secret: pointer.StringPtr("secret"),
						Name:   "vpc_id",
						Key:    "vpc_id",
					},
				}
				Expect(cc.Update(context.Background(), configuration)).ToNot(HaveOccurred())
			})

			Context("which are missing", func() {
				BeforeEach(func() {
					result, _, rerr = controllertests.Roll(context.TODO(), ctrl, configuration, 0)
				})

				It("should requeue", func() {
					Expect(rerr).ToNot(HaveOccurred())
					Expect(result.Requeue).To(BeFalse())
					Expect(result.RequeueAfter).To(Equal(time.Minute * 5))
				})

				It("should indicate the status in the conditions", func() {
					Expect(cc.Get(context.TODO(), configuration.GetNamespacedName(), configuration)).ToNot(HaveOccurred())

					cond := configuration.Status.GetCondition(corev1alpha1.ConditionReady)
					Expect(cond.Status).To(Equal(metav1.ConditionFalse))
					Expect(cond.Reason).To(Equal(corev1alpha1.ReasonActionRequired))
					Expect(cond.Message).To(Equal("spec.valueFrom[0].secret (default/secret) does not exist"))
				})

				It("should not delete the configuration", func() {
					Expect(cc.Get(context.TODO(), configuration.GetNamespacedName(), configuration)).ToNot(HaveOccurred())
				})
			})

			Context("where the value is missing", func() {
				BeforeEach(func() {
					secret := &v1.Secret{}
					secret.Namespace = configuration.Namespace
					secret.Name = *configuration.Spec.ValueFrom[0].Secret
					secret.Data = map[string][]byte{"vpc_missing": []byte("test")}
					Expect(cc.Create(context.Background(), secret)).ToNot(HaveOccurred())

					result, _, rerr = controllertests.Roll(context.TODO(), ctrl, configuration, 0)
				})
				It("should requeue", func() {
					Expect(rerr).ToNot(HaveOccurred())
					Expect(result.Requeue).To(BeFalse())
					Expect(result.RequeueAfter).To(Equal(time.Minute * 5))
				})

				It("should indicate the status in the conditions", func() {
					Expect(cc.Get(context.TODO(), configuration.GetNamespacedName(), configuration)).ToNot(HaveOccurred())

					cond := configuration.Status.GetCondition(corev1alpha1.ConditionReady)
					Expect(cond.Status).To(Equal(metav1.ConditionFalse))
					Expect(cond.Reason).To(Equal(corev1alpha1.ReasonActionRequired))
					Expect(cond.Message).To(Equal("spec.valueFrom[0] (default/secret) does not contain key: \"vpc_id\""))
				})

				It("should not delete the configuration", func() {
					Expect(cc.Get(context.TODO(), configuration.GetNamespacedName(), configuration)).ToNot(HaveOccurred())
				})
			})
		})

		Context("and the configuration variables have changed", func() {
			BeforeEach(func() {
				configuration.Spec.Variables.Raw = []byte(`{"before": "changed"}`)
				Expect(cc.Update(context.Background(), configuration)).ToNot(HaveOccurred())

				// this is the secret before we reconcile
				secret := v1.Secret{}
				secret.Namespace = ctrl.ControllerNamespace
				secret.Name = configuration.GetTerraformConfigSecretName()
				secret.Data = map[string][]byte{
					"before": []byte("test"),
				}
				Expect(cc.Create(context.Background(), &secret)).ToNot(HaveOccurred())

				result, _, rerr = controllertests.Roll(context.TODO(), ctrl, configuration, 0)
			})

			It("should requeue", func() {
				Expect(rerr).ToNot(HaveOccurred())
				Expect(result.Requeue).To(BeFalse())
			})

			It("should indicate the status in the conditions", func() {
				Expect(cc.Get(context.TODO(), configuration.GetNamespacedName(), configuration)).ToNot(HaveOccurred())

				cond := configuration.Status.GetCondition(corev1alpha1.ConditionReady)
				Expect(cond.Status).To(Equal(metav1.ConditionFalse))
				Expect(cond.Reason).To(Equal(corev1alpha1.ReasonInProgress))
				Expect(cond.Message).To(Equal("Terraform destroy is running"))
			})

			It("should update configuration variables", func() {
				name := configuration.GetTerraformConfigSecretName()
				secret, found, err := kubernetes.GetSecretIfExists(context.Background(), cc, ctrl.ControllerNamespace, name)
				Expect(err).ToNot(HaveOccurred())
				Expect(found).To(BeTrue())
				Expect(secret.Data).To(HaveKey("variables.tfvars.json"))
				Expect(string(secret.Data["variables.tfvars.json"])).To(ContainSubstring(`{"before":"changed"}`))
			})
		})

		Context("and using authentication secret", func() {
			BeforeEach(func() {
				configuration.Spec.Auth = &v1.SecretReference{Name: "auth"}
				secret := fixtures.NewAuthenticationSecret(configuration.Namespace, configuration.Spec.Auth.Name)
				Expect(cc.Update(context.Background(), configuration)).ToNot(HaveOccurred())
				Expect(cc.Create(context.Background(), secret)).ToNot(HaveOccurred())
			})

			Context("which is missing", func() {
				BeforeEach(func() {
					secret := fixtures.NewAuthenticationSecret(configuration.Namespace, configuration.Spec.Auth.Name)
					Expect(cc.Delete(context.Background(), secret)).ToNot(HaveOccurred())

					result, _, rerr = controllertests.Roll(context.TODO(), ctrl, configuration, 0)
				})

				It("should requeue", func() {
					Expect(rerr).ToNot(HaveOccurred())
					Expect(result.Requeue).To(BeFalse())
					Expect(result.RequeueAfter).To(Equal(time.Minute * 5))
				})

				It("should indicate the status in the conditions", func() {
					Expect(cc.Get(context.TODO(), configuration.GetNamespacedName(), configuration)).ToNot(HaveOccurred())

					cond := configuration.Status.GetCondition(corev1alpha1.ConditionReady)
					Expect(cond.Status).To(Equal(metav1.ConditionFalse))
					Expect(cond.Reason).To(Equal(corev1alpha1.ReasonActionRequired))
					Expect(cond.Message).To(Equal("Authentication secret (spec.auth) does not exist"))
				})

				It("should not delete the configuration", func() {
					Expect(cc.Get(context.TODO(), configuration.GetNamespacedName(), configuration)).ToNot(HaveOccurred())
				})
			})
		})

		Context("and we are using a custom template", func() {
			BeforeEach(func() {
				ctrl.BackendTemplate = "template"
			})

			Context("which is missing", func() {
				BeforeEach(func() {
					result, _, rerr = controllertests.Roll(context.TODO(), ctrl, configuration, 0)
				})

				It("should not error", func() {
					Expect(rerr).ToNot(HaveOccurred())
					Expect(result.Requeue).To(BeFalse())
					Expect(result.RequeueAfter).To(BeZero())
				})

				It("should indicate the status in the conditions", func() {
					Expect(cc.Get(context.TODO(), configuration.GetNamespacedName(), configuration)).ToNot(HaveOccurred())

					cond := configuration.Status.GetCondition(corev1alpha1.ConditionReady)
					Expect(cond.Status).To(Equal(metav1.ConditionFalse))
					Expect(cond.Reason).To(Equal(corev1alpha1.ReasonActionRequired))
					Expect(cond.Message).To(Equal("Backend template secret \"terraform-system/template\" not found, contact administrator"))
				})

				It("should not delete the configuration", func() {
					Expect(cc.Get(context.TODO(), configuration.GetNamespacedName(), configuration)).ToNot(HaveOccurred())
				})
			})

			Context("which is not missing", func() {
				BeforeEach(func() {
					secret := fixtures.NewBackendTemplateSecret(ctrl.ControllerNamespace, ctrl.BackendTemplate)
					Expect(cc.Create(context.Background(), secret)).ToNot(HaveOccurred())

					result, _, rerr = controllertests.Roll(context.TODO(), ctrl, configuration, 0)
				})

				It("should not delete the configuration", func() {
					Expect(cc.Get(context.TODO(), configuration.GetNamespacedName(), configuration)).ToNot(HaveOccurred())
				})
			})
		})

		When("and no terraform configuration is job exists", func() {
			CommonChecks := func() {
				It("should not error", func() {
					Expect(rerr).ToNot(HaveOccurred())
					Expect(result.Requeue).To(BeFalse())
				})

				It("should requeue to wait for the destroy job", func() {
					Expect(result.RequeueAfter).To(Equal(time.Second * 5))
				})

				It("should have created a terraform destroy job in controller namespace", func() {
					list := &batchv1.JobList{}
					Expect(cc.List(context.Background(), list,
						client.InNamespace(ctrl.ControllerNamespace),
						client.MatchingLabels(map[string]string{
							terraformv1alpha1.ConfigurationStageLabel: terraformv1alpha1.StageTerraformDestroy,
						},
						))).ToNot(HaveOccurred())
					Expect(list.Items).To(HaveLen(1))
				})

				It("should indicate the status in the conditions", func() {
					Expect(cc.Get(context.TODO(), configuration.GetNamespacedName(), configuration)).ToNot(HaveOccurred())

					cond := configuration.Status.GetCondition(corev1alpha1.ConditionReady)
					Expect(cond.Status).To(Equal(metav1.ConditionFalse))
					Expect(cond.Reason).To(Equal(corev1alpha1.ReasonInProgress))
					Expect(cond.Message).To(Equal("Terraform destroy is running"))
				})

				It("should not delete the configuration", func() {
					Expect(cc.Get(context.TODO(), configuration.GetNamespacedName(), configuration)).ToNot(HaveOccurred())
				})
			}

			Context("and watchers are enabled for the configuration", func() {
				CommonChecks()

				BeforeEach(func() {
					ctrl.EnableWatchers = true

					result, _, rerr = controllertests.Roll(context.TODO(), ctrl, configuration, 0)
				})

				It("should have created a watcher", func() {
					list := &batchv1.JobList{}
					Expect(cc.List(context.Background(), list,
						client.InNamespace(configuration.Namespace),
						client.MatchingLabels(map[string]string{
							terraformv1alpha1.ConfigurationStageLabel: terraformv1alpha1.StageTerraformDestroy,
						},
						))).ToNot(HaveOccurred())

					Expect(list.Items).To(HaveLen(1))
					Expect(list.Items[0].Labels).To(Equal(map[string]string{
						terraformv1alpha1.ConfigurationGenerationLabel: strconv.FormatInt(configuration.Generation, 10),
						terraformv1alpha1.ConfigurationNameLabel:       configuration.Name,
						terraformv1alpha1.ConfigurationNamespaceLabel:  configuration.Namespace,
						terraformv1alpha1.ConfigurationStageLabel:      terraformv1alpha1.StageTerraformDestroy,
						terraformv1alpha1.ConfigurationUIDLabel:        string(configuration.UID),
					}))
					Expect(list.Items[0].Spec.Parallelism).ToNot(BeNil())
					Expect(*list.Items[0].Spec.Parallelism).To(Equal(int32(1)))
					Expect(list.Items[0].Spec.Completions).ToNot(BeNil())
					Expect(*list.Items[0].Spec.Completions).To(Equal(int32(1)))

					Expect(list.Items[0].Spec.Template.Spec.Containers).To(HaveLen(1))
					container := list.Items[0].Spec.Template.Spec.Containers[0]
					Expect(container.Name).To(Equal("watch"))
					Expect(container.Image).To(Equal(ctrl.ExecutorImage))
					Expect(container.Command).To(Equal([]string{"/watch_logs.sh"}))

					expectedURL := fmt.Sprintf("http://controller.%s.svc.cluster.local/v1/builds/%s/%s/logs?generation=%d&name=%s&namespace=%s&stage=%s&uid=%s",
						ctrl.ControllerNamespace,
						configuration.Namespace,
						configuration.Name,
						configuration.Generation,
						configuration.Name,
						configuration.Namespace,
						terraformv1alpha1.StageTerraformDestroy,
						string(configuration.UID),
					)
					Expect(container.Args).To(Equal([]string{"-e", expectedURL}))
				})
			})

			Context("and watchers are disabled for the configuration", func() {
				CommonChecks()

				BeforeEach(func() {
					ctrl.EnableWatchers = false

					result, _, rerr = controllertests.Roll(context.TODO(), ctrl, configuration, 0)
				})

				It("should not have created a watcher", func() {
					list := &batchv1.JobList{}
					Expect(cc.List(context.Background(), list,
						client.InNamespace(configuration.Namespace),
						client.MatchingLabels(map[string]string{
							terraformv1alpha1.ConfigurationStageLabel: terraformv1alpha1.StageTerraformDestroy,
						},
						))).ToNot(HaveOccurred())
					Expect(list.Items).To(HaveLen(0))
				})
			})
		})

		When("and watcher is already running", func() {
			BeforeEach(func() {
				watcher := fixtures.NewConfigurationPodWatcher(configuration, terraformv1alpha1.StageTerraformDestroy)

				Expect(cc.Create(context.Background(), watcher)).ToNot(HaveOccurred())
			})

			Context("and terraform destroy job has running", func() {
				BeforeEach(func() {
					job := fixtures.NewRunningTerraformJob(configuration, terraformv1alpha1.StageTerraformDestroy)
					Expect(cc.Create(context.Background(), job)).ToNot(HaveOccurred())

					result, _, rerr = controllertests.Roll(context.TODO(), ctrl, configuration, 0)
				})

				It("should requeue", func() {
					Expect(result.Requeue).To(BeFalse())
					Expect(result.RequeueAfter).To(Equal(5 * time.Second))
				})
			})

			Context("and the terraform destroy has failed", func() {
				BeforeEach(func() {
					job := fixtures.NewFailedTerraformJob(configuration, terraformv1alpha1.StageTerraformDestroy)
					job.Namespace = ctrl.ControllerNamespace
					Expect(cc.Create(context.Background(), job)).ToNot(HaveOccurred())

					result, _, rerr = controllertests.Roll(context.TODO(), ctrl, configuration, 0)
				})

				It("should not requeue", func() {
					Expect(result.Requeue).To(BeFalse())
					Expect(result.RequeueAfter).To(BeZero())
				})

				It("should have not created a new terraform destroy job", func() {
					list := &batchv1.JobList{}
					Expect(cc.List(context.Background(), list,
						client.InNamespace(ctrl.ControllerNamespace),
						client.MatchingLabels(map[string]string{
							terraformv1alpha1.ConfigurationStageLabel: terraformv1alpha1.StageTerraformDestroy,
						},
						))).ToNot(HaveOccurred())
					Expect(list.Items).To(HaveLen(1))
				})

				It("should have indicate status on conditions", func() {
					Expect(cc.Get(context.TODO(), configuration.GetNamespacedName(), configuration)).ToNot(HaveOccurred())
					Expect(configuration.Status.ResourceStatus).To(Equal(terraformv1alpha1.DestroyingResourcesFailed))

					cond := configuration.Status.GetCondition(corev1alpha1.ConditionReady)
					Expect(cond.Status).To(Equal(metav1.ConditionFalse))
					Expect(cond.Reason).To(Equal(corev1alpha1.ReasonError))
					Expect(cond.Message).To(Equal("Terraform destroy has failed"))
				})
			})

			Context("and terraform destroy job has succeeded", func() {
				BeforeEach(func() {
					job := fixtures.NewCompletedTerraformJob(configuration, terraformv1alpha1.StageTerraformDestroy)
					job.Namespace = ctrl.ControllerNamespace
					Expect(cc.Create(context.Background(), job)).ToNot(HaveOccurred())

					result, _, rerr = controllertests.Roll(context.TODO(), ctrl, configuration, 0)
				})

				It("should not requeue", func() {
					Expect(result.Requeue).To(BeFalse())
					Expect(result.RequeueAfter).To(BeZero())
				})

				It("should have deleted the configuration", func() {
					Expect(cc.Get(context.TODO(), configuration.GetNamespacedName(), configuration)).To(HaveOccurred())
				})

				It("should have deleted the configuration secrets", func() {
					secret := &v1.Secret{}
					secret.Namespace = ctrl.ControllerNamespace
					secret.Name = configuration.GetTerraformStateSecretName()

					Expect(cc.Get(context.TODO(), configuration.GetNamespacedName(), configuration)).To(HaveOccurred())
				})
			})
		})
	})
})
