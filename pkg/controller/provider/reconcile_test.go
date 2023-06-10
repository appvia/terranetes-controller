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

package provider

import (
	"context"
	"io/ioutil"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/sirupsen/logrus"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	corev1alpha1 "github.com/appvia/terranetes-controller/pkg/apis/core/v1alpha1"
	terraformv1alpha1 "github.com/appvia/terranetes-controller/pkg/apis/terraform/v1alpha1"
	"github.com/appvia/terranetes-controller/pkg/schema"
	controllertests "github.com/appvia/terranetes-controller/test"
	"github.com/appvia/terranetes-controller/test/fixtures"
)

func TestController(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Running Test Suite")
}

var _ = Describe("Checking Provider Controller", func() {
	logrus.SetOutput(ioutil.Discard)

	var cc client.Client
	var result reconcile.Result
	var rerr error
	var ctrl *Controller
	var provider *terraformv1alpha1.Provider
	var secret *v1.Secret

	BeforeEach(func() {
		secret = fixtures.NewValidAWSProviderSecret("terraform-system", "aws")
		provider = fixtures.NewValidAWSProvider("aws", secret)
		cc = fake.NewClientBuilder().WithScheme(schema.GetScheme()).Build()
		ctrl = &Controller{cc: cc, ControllerNamespace: secret.Namespace}

		Expect(cc.Create(context.Background(), provider)).Should(Succeed())
		Expect(cc.Create(context.Background(), secret)).Should(Succeed())
	})

	When("provisioning a provider ", func() {
		Context("and we have an issue with google credentials", func() {
			BeforeEach(func() {
				secret = fixtures.NewValidAzureProviderSecret(ctrl.ControllerNamespace, "goole")
				provider.Spec.Provider = terraformv1alpha1.GCPProviderType
				provider.Spec.SecretRef.Name = secret.Name

				Expect(cc.Create(context.Background(), secret)).Should(Succeed())
				Expect(cc.Update(context.Background(), provider)).Should(Succeed())
			})

			Context("and the secret is missing GCLOUD_KEYFILE_JSON", func() {
				BeforeEach(func() {
					delete(secret.Data, "GCLOUD_KEYFILE_JSON")
					Expect(cc.Update(context.Background(), secret)).Should(Succeed())

					result, _, rerr = controllertests.Roll(context.TODO(), ctrl, provider, 0)
				})

				It("should fail", func() {
					expected := "Provider secret (terraform-system/goole) is missing the GOOGLE_APPLICATION_CREDENTIALS, GOOGLE_CREDENTIALS, GOOGLE_CLOUD_KEYFILE_JSON or GCLOUD_KEYFILE_JSON field"

					Expect(cc.Get(context.TODO(), provider.GetNamespacedName(), provider)).ToNot(HaveOccurred())

					Expect(provider.Status.Conditions[0].Type).To(Equal(corev1alpha1.ConditionReady))
					Expect(provider.Status.Conditions[0].Status).To(Equal(metav1.ConditionFalse))
					Expect(provider.Status.Conditions[0].Reason).To(Equal(corev1alpha1.ReasonActionRequired))
					Expect(provider.Status.Conditions[0].Message).To(Equal(expected))
				})
			})
		})

		Context("and we have an issue with azure credentials", func() {
			BeforeEach(func() {
				secret = fixtures.NewValidAzureProviderSecret(ctrl.ControllerNamespace, "azure")
				provider.Spec.Provider = terraformv1alpha1.AzureProviderType
				provider.Spec.SecretRef.Name = secret.Name

				Expect(cc.Create(context.Background(), secret)).Should(Succeed())
				Expect(cc.Update(context.Background(), provider)).Should(Succeed())
			})

			Context("and the secret is missing ARM_CLIENT_ID", func() {
				BeforeEach(func() {
					delete(secret.Data, "ARM_CLIENT_ID")
					Expect(cc.Update(context.Background(), secret)).Should(Succeed())

					result, _, rerr = controllertests.Roll(context.TODO(), ctrl, provider, 0)
				})

				It("should fail", func() {
					Expect(cc.Get(context.TODO(), provider.GetNamespacedName(), provider)).ToNot(HaveOccurred())

					Expect(provider.Status.Conditions[0].Type).To(Equal(corev1alpha1.ConditionReady))
					Expect(provider.Status.Conditions[0].Status).To(Equal(metav1.ConditionFalse))
					Expect(provider.Status.Conditions[0].Reason).To(Equal(corev1alpha1.ReasonActionRequired))
					Expect(provider.Status.Conditions[0].Message).To(Equal("Provider secret (terraform-system/azure) is missing the ARM_CLIENT_ID"))
				})
			})

			Context("and the secret is missing ARM_CLIENT_SECRET", func() {
				BeforeEach(func() {
					delete(secret.Data, "ARM_CLIENT_SECRET")
					Expect(cc.Update(context.Background(), secret)).Should(Succeed())

					result, _, rerr = controllertests.Roll(context.TODO(), ctrl, provider, 0)
				})

				It("should fail", func() {
					Expect(cc.Get(context.TODO(), provider.GetNamespacedName(), provider)).ToNot(HaveOccurred())

					Expect(provider.Status.Conditions[0].Type).To(Equal(corev1alpha1.ConditionReady))
					Expect(provider.Status.Conditions[0].Status).To(Equal(metav1.ConditionFalse))
					Expect(provider.Status.Conditions[0].Reason).To(Equal(corev1alpha1.ReasonActionRequired))
					Expect(provider.Status.Conditions[0].Message).To(Equal("Provider secret (terraform-system/azure) is missing the ARM_CLIENT_SECRET"))
				})
			})

			Context("and the secret is missing ARM_SUBSCRIPTION_ID", func() {
				BeforeEach(func() {
					delete(secret.Data, "ARM_SUBSCRIPTION_ID")
					Expect(cc.Update(context.Background(), secret)).Should(Succeed())

					result, _, rerr = controllertests.Roll(context.TODO(), ctrl, provider, 0)
				})

				It("should fail", func() {
					Expect(cc.Get(context.TODO(), provider.GetNamespacedName(), provider)).ToNot(HaveOccurred())

					Expect(provider.Status.Conditions[0].Type).To(Equal(corev1alpha1.ConditionReady))
					Expect(provider.Status.Conditions[0].Status).To(Equal(metav1.ConditionFalse))
					Expect(provider.Status.Conditions[0].Reason).To(Equal(corev1alpha1.ReasonActionRequired))
					Expect(provider.Status.Conditions[0].Message).To(Equal("Provider secret (terraform-system/azure) is missing the ARM_SUBSCRIPTION_ID"))
				})
			})

			Context("and the secret is missing ARM_TENANT_ID", func() {
				BeforeEach(func() {
					delete(secret.Data, "ARM_TENANT_ID")
					Expect(cc.Update(context.Background(), secret)).Should(Succeed())

					result, _, rerr = controllertests.Roll(context.TODO(), ctrl, provider, 0)
				})

				It("should fail", func() {
					Expect(cc.Get(context.TODO(), provider.GetNamespacedName(), provider)).ToNot(HaveOccurred())

					Expect(provider.Status.Conditions[0].Type).To(Equal(corev1alpha1.ConditionReady))
					Expect(provider.Status.Conditions[0].Status).To(Equal(metav1.ConditionFalse))
					Expect(provider.Status.Conditions[0].Reason).To(Equal(corev1alpha1.ReasonActionRequired))
					Expect(provider.Status.Conditions[0].Message).To(Equal("Provider secret (terraform-system/azure) is missing the ARM_TENANT_ID"))
				})
			})

		})

		Context("and we have an issue with aws credentials", func() {
			Context("secret is missing", func() {
				BeforeEach(func() {
					Expect(cc.Delete(context.Background(), secret)).Should(Succeed())

					result, _, rerr = controllertests.Roll(context.TODO(), ctrl, provider, 0)
				})

				It("should indicate the secret is missing", func() {
					Expect(cc.Get(context.TODO(), provider.GetNamespacedName(), provider)).ToNot(HaveOccurred())

					Expect(provider.Status.Conditions).To(HaveLen(2))
					Expect(provider.Status.Conditions[0].Type).To(Equal(corev1alpha1.ConditionReady))
					Expect(provider.Status.Conditions[0].Status).To(Equal(metav1.ConditionFalse))
					Expect(provider.Status.Conditions[0].Reason).To(Equal(corev1alpha1.ReasonActionRequired))
					Expect(provider.Status.Conditions[0].Message).To(Equal("Provider secret (terraform-system/aws) not found"))
				})

				It("should not requeue", func() {
					Expect(rerr).To(BeNil())
					Expect(result).To(Equal(reconcile.Result{}))
				})
			})

			Context("aws credentials are invalid", func() {
				Context("missing all keys", func() {
					BeforeEach(func() {
						secret.Data = nil
						Expect(cc.Update(context.Background(), secret)).Should(Succeed())

						result, _, rerr = controllertests.Roll(context.TODO(), ctrl, provider, 0)
					})

					It("should indicate missing keys", func() {
						Expect(cc.Get(context.TODO(), provider.GetNamespacedName(), provider)).ToNot(HaveOccurred())

						Expect(provider.Status.Conditions).To(HaveLen(2))
						Expect(provider.Status.Conditions[0].Type).To(Equal(corev1alpha1.ConditionReady))
						Expect(provider.Status.Conditions[0].Status).To(Equal(metav1.ConditionFalse))
						Expect(provider.Status.Conditions[0].Reason).To(Equal(corev1alpha1.ReasonActionRequired))
						Expect(provider.Status.Conditions[0].Message).To(Equal("Provider secret (terraform-system/aws) is missing the AWS_ACCESS_KEY_ID"))
					})
				})

				Context("missing access key", func() {
					BeforeEach(func() {
						secret = fixtures.NewValidAWSProviderSecret(ctrl.ControllerNamespace, provider.Spec.SecretRef.Name)
						secret.Data = map[string][]byte{
							"AWS_ACCESS_KEY_ID": []byte("test"),
						}
						Expect(cc.Update(context.Background(), secret)).Should(Succeed())

						result, _, rerr = controllertests.Roll(context.TODO(), ctrl, provider, 0)
					})

					It("should indicate missing a key", func() {
						Expect(cc.Get(context.TODO(), provider.GetNamespacedName(), provider)).ToNot(HaveOccurred())

						Expect(provider.Status.Conditions).To(HaveLen(2))
						Expect(provider.Status.Conditions[0].Type).To(Equal(corev1alpha1.ConditionReady))
						Expect(provider.Status.Conditions[0].Status).To(Equal(metav1.ConditionFalse))
						Expect(provider.Status.Conditions[0].Reason).To(Equal(corev1alpha1.ReasonActionRequired))
						Expect(provider.Status.Conditions[0].Message).To(Equal("Provider secret (terraform-system/aws) is missing the AWS_SECRET_ACCESS_KEY"))

					})
				})

				Context("missing aws access id", func() {
					BeforeEach(func() {
						secret = fixtures.NewValidAWSProviderSecret(ctrl.ControllerNamespace, provider.Spec.SecretRef.Name)
						secret.Data = map[string][]byte{
							"AWS_ACCESS_KEY_ID":     []byte(""),
							"AWS_SECRET_ACCESS_KEY": []byte("test"),
						}
						Expect(cc.Update(context.Background(), secret)).Should(Succeed())

						result, _, rerr = controllertests.Roll(context.TODO(), ctrl, provider, 0)
					})

					It("should indicate missing key", func() {
						Expect(cc.Get(context.TODO(), provider.GetNamespacedName(), provider)).ToNot(HaveOccurred())

						Expect(provider.Status.Conditions).To(HaveLen(2))
						Expect(provider.Status.Conditions[0].Type).To(Equal(corev1alpha1.ConditionReady))
						Expect(provider.Status.Conditions[0].Status).To(Equal(metav1.ConditionFalse))
						Expect(provider.Status.Conditions[0].Reason).To(Equal(corev1alpha1.ReasonActionRequired))
						Expect(provider.Status.Conditions[0].Message).To(Equal("Provider secret (terraform-system/aws) is missing the AWS_ACCESS_KEY_ID"))
					})
				})

				Context("aceess key bigger than secret", func() {
					BeforeEach(func() {
						secret.Data = map[string][]byte{
							"AWS_ACCESS_KEY_ID":     []byte("test111111"),
							"AWS_SECRET_ACCESS_KEY": []byte("111"),
						}
						Expect(cc.Update(context.Background(), secret)).Should(Succeed())

						result, _, rerr = controllertests.Roll(context.TODO(), ctrl, provider, 0)
					})

					It("should indicate key mistake", func() {
						Expect(cc.Get(context.TODO(), provider.GetNamespacedName(), provider)).ToNot(HaveOccurred())

						Expect(provider.Status.Conditions).To(HaveLen(2))
						Expect(provider.Status.Conditions[0].Type).To(Equal(corev1alpha1.ConditionReady))
						Expect(provider.Status.Conditions[0].Status).To(Equal(metav1.ConditionFalse))
						Expect(provider.Status.Conditions[0].Reason).To(Equal(corev1alpha1.ReasonActionRequired))
						Expect(provider.Status.Conditions[0].Message).To(Equal("provider secret (terraform-system/aws) aws access key is larger than secret"))
					})
				})
			})
		})

		Context("and the provider has preload enabled", func() {
			BeforeEach(func() {
				provider.Spec.Preload = &terraformv1alpha1.PreloadConfiguration{
					Cluster: "test",
					Context: "test",
					Enabled: pointer.BoolPtr(true),
					Region:  "test",
				}
			})

			Context("but is an unsupported provider", func() {
				BeforeEach(func() {
					provider.Spec.Provider = "unsupported"
					Expect(cc.Update(context.Background(), provider)).To(Succeed())

					result, _, rerr = controllertests.Roll(context.TODO(), ctrl, provider, 0)
				})

				It("should not error", func() {
					Expect(rerr).NotTo(HaveOccurred())
					Expect(result.Requeue).To(BeFalse())
				})

				It("should update the conditions to warning", func() {
					Expect(cc.Get(context.TODO(), provider.GetNamespacedName(), provider)).ToNot(HaveOccurred())

					cond := provider.GetCommonStatus().GetCondition(terraformv1alpha1.ConditionProviderPreload)
					Expect(cond.Type).To(Equal(terraformv1alpha1.ConditionProviderPreload))
					Expect(cond.Status).To(Equal(metav1.ConditionFalse))
					Expect(cond.Reason).To(Equal(corev1alpha1.ReasonWarning))
					Expect(cond.Message).To(Equal("Loading contextual is supported on AWS only"))
				})
			})
		})

		Context("and the provider has preload disabled", func() {
			BeforeEach(func() {
				result, _, rerr = controllertests.Roll(context.TODO(), ctrl, provider, 0)
			})

			Context("due to no preload configuration", func() {
				It("should not error", func() {
					Expect(rerr).NotTo(HaveOccurred())
					Expect(result.Requeue).To(BeFalse())
				})

				It("should update the conditions to disabled", func() {
					Expect(cc.Get(context.TODO(), provider.GetNamespacedName(), provider)).ToNot(HaveOccurred())

					cond := provider.GetCommonStatus().GetCondition(terraformv1alpha1.ConditionProviderPreload)
					Expect(cond.Type).To(Equal(terraformv1alpha1.ConditionProviderPreload))
					Expect(cond.Status).To(Equal(metav1.ConditionFalse))
					Expect(cond.Reason).To(Equal(corev1alpha1.ReasonDisabled))
					Expect(cond.Message).To(Equal("Loading contextual data is not enabled"))
				})
			})

			Context("due to preload configuration disabled", func() {
				BeforeEach(func() {
					provider.Spec.Preload = &terraformv1alpha1.PreloadConfiguration{
						Enabled: pointer.BoolPtr(false),
						Cluster: "test",
						Context: "test",
						Region:  "test",
					}

					result, _, rerr = controllertests.Roll(context.TODO(), ctrl, provider, 0)
				})

				It("should not error", func() {
					Expect(rerr).NotTo(HaveOccurred())
					Expect(result.Requeue).To(BeFalse())
				})

				It("should update the conditions to disabled", func() {
					Expect(cc.Get(context.TODO(), provider.GetNamespacedName(), provider)).ToNot(HaveOccurred())

					cond := provider.GetCommonStatus().GetCondition(terraformv1alpha1.ConditionProviderPreload)
					Expect(cond.Type).To(Equal(terraformv1alpha1.ConditionProviderPreload))
					Expect(cond.Status).To(Equal(metav1.ConditionFalse))
					Expect(cond.Reason).To(Equal(corev1alpha1.ReasonDisabled))
					Expect(cond.Message).To(Equal("Loading contextual data is not enabled"))
				})
			})
		})

		Context("and provider has valid configuration", func() {
			BeforeEach(func() {
				result, _, rerr = controllertests.Roll(context.TODO(), ctrl, provider, 0)
			})

			It("should not requeue", func() {
				Expect(rerr).To(BeNil())
				Expect(result).To(Equal(reconcile.Result{}))
			})

			It("should indicate the provider is ready", func() {
				Expect(cc.Get(context.TODO(), provider.GetNamespacedName(), provider)).ToNot(HaveOccurred())

				Expect(provider.Status.Conditions).To(HaveLen(2))
				Expect(provider.Status.Conditions[0].Type).To(Equal(corev1alpha1.ConditionReady))
				Expect(provider.Status.Conditions[0].Status).To(Equal(metav1.ConditionTrue))
				Expect(provider.Status.Conditions[0].Reason).To(Equal(corev1alpha1.ReasonReady))
				Expect(provider.Status.Conditions[0].Message).To(Equal("Resource ready"))
			})

			It("should indicate the preload is disabled", func() {
				Expect(cc.Get(context.TODO(), provider.GetNamespacedName(), provider)).ToNot(HaveOccurred())

				Expect(provider.Status.Conditions).To(HaveLen(2))
				Expect(provider.Status.Conditions[1].Type).To(Equal(terraformv1alpha1.ConditionProviderPreload))
				Expect(provider.Status.Conditions[1].Status).To(Equal(metav1.ConditionFalse))
				Expect(provider.Status.Conditions[1].Reason).To(Equal(corev1alpha1.ReasonDisabled))
				Expect(provider.Status.Conditions[1].Message).To(Equal("Loading contextual data is not enabled"))
			})
		})
	})
})
