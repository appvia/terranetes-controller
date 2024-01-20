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
	"io"
	"testing"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/sirupsen/logrus"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	corev1alpha1 "github.com/appvia/terranetes-controller/pkg/apis/core/v1alpha1"
	terraformv1alpha1 "github.com/appvia/terranetes-controller/pkg/apis/terraform/v1alpha1"
	controllerutils "github.com/appvia/terranetes-controller/pkg/controller"
	"github.com/appvia/terranetes-controller/pkg/schema"
	controllertests "github.com/appvia/terranetes-controller/test"
	"github.com/appvia/terranetes-controller/test/fixtures"
)

func TestReconcile(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Running Test Suite")
}

var _ = Describe("Provider Controller", func() {
	logrus.SetOutput(io.Discard)

	var cc client.Client
	var result reconcile.Result
	var rerr error
	var controller *Controller
	var provider *terraformv1alpha1.Provider
	var secret *v1.Secret

	BeforeEach(func() {
		cc = fake.NewClientBuilder().
			WithScheme(schema.GetScheme()).
			WithStatusSubresource(&terraformv1alpha1.Provider{}).
			Build()

		controller = &Controller{cc: cc}

		secret = fixtures.NewValidAWSProviderSecret("default", "secret")
		provider = fixtures.NewValidAWSProvider("secret", secret)
		Expect(cc.Create(context.Background(), secret)).To(Succeed())
		Expect(cc.Create(context.Background(), provider)).To(Succeed())
	})

	When("reconciling a provider", func() {

		Context("which is configured with a static secret", func() {
			// CommonFunc is a collection of common assertions when credentials
			// are invalid in the provider secret
			CommonFunc := func(message string) {
				It("should have the conditions", func() {
					Expect(cc.Get(context.TODO(), provider.GetNamespacedName(), provider)).To(Succeed())
					Expect(provider.Status.Conditions).To(HaveLen(2))
				})

				It("should not have a finalizer", func() {
					Expect(cc.Get(context.Background(), provider.GetNamespacedName(), provider)).To(Succeed())
					Expect(provider.GetFinalizers()).To(BeEmpty())
				})

				It("should indicate the secret is missing", func() {
					Expect(cc.Get(context.TODO(), provider.GetNamespacedName(), provider)).To(Succeed())

					Expect(provider.Status.Conditions).To(HaveLen(2))
					Expect(provider.Status.Conditions[0].Type).To(Equal(corev1alpha1.ConditionReady))
					Expect(provider.Status.Conditions[0].Message).To(Equal(message))

					switch message {
					case controllerutils.ResourceReady:
						Expect(provider.Status.Conditions[0].Status).To(Equal(metav1.ConditionTrue))
						Expect(provider.Status.Conditions[0].Reason).To(Equal(corev1alpha1.ReasonReady))

					default:
						Expect(provider.Status.Conditions[0].Status).To(Equal(metav1.ConditionFalse))
						Expect(provider.Status.Conditions[0].Reason).To(Equal(corev1alpha1.ReasonActionRequired))
					}
				})

				It("should not requeue", func() {
					Expect(result.Requeue).To(BeFalse())

					if message == controllerutils.ResourceReady {
						Expect(result.RequeueAfter).To(Equal(0 * time.Second))
					} else {
						Expect(result.RequeueAfter).To(Equal(5 * time.Minute))
					}
				})
			}

			Context("provider secret is missing", func() {
				BeforeEach(func() {
					Expect(cc.Delete(context.Background(), secret)).To(Succeed())

					result, _, rerr = controllertests.Roll(context.TODO(), controller, provider, 0)
				})

				CommonFunc("Provider secret (default/secret) not found")
			})

			Context("and the provider is google", func() {
				BeforeEach(func() {
					provider.Spec.Provider = "google"
					Expect(cc.Update(context.Background(), provider)).To(Succeed())
				})

				Context("but it is missing the required keys", func() {
					BeforeEach(func() {
						secret.Data = map[string][]byte{}
						Expect(cc.Update(context.Background(), secret)).To(Succeed())

						result, _, rerr = controllertests.Roll(context.TODO(), controller, provider, 0)
					})

					CommonFunc("Provider secret (default/secret) is missing the GOOGLE_APPLICATION_CREDENTIALS, GOOGLE_CREDENTIALS, GOOGLE_CLOUD_KEYFILE_JSON or GCLOUD_KEYFILE_JSON field")
				})

				Context("and we have all the required keys", func() {
					BeforeEach(func() {
						secret.Data = map[string][]byte{"GOOGLE_APPLICATION_CREDENTIALS": []byte("foo")}
						Expect(cc.Update(context.Background(), secret)).To(Succeed())

						result, _, rerr = controllertests.Roll(context.TODO(), controller, provider, 0)
					})

					CommonFunc(controllerutils.ResourceReady)
				})
			})

			Context("and the provider is azurerm", func() {
				BeforeEach(func() {
					provider.Spec.Provider = "azurerm"

					Expect(cc.Update(context.Background(), provider)).To(Succeed())
				})

				Context("but missing client id", func() {
					BeforeEach(func() {
						secret.Data = nil
						Expect(cc.Update(context.Background(), secret)).To(Succeed())

						result, _, rerr = controllertests.Roll(context.TODO(), controller, provider, 0)
					})

					CommonFunc("Provider secret (default/secret) is missing the ARM_CLIENT_ID")
				})

				Context("but missing client secret", func() {
					BeforeEach(func() {
						secret.Data = map[string][]byte{"ARM_CLIENT_ID": []byte("foo")}
						Expect(cc.Update(context.Background(), secret)).To(Succeed())

						result, _, rerr = controllertests.Roll(context.TODO(), controller, provider, 0)
					})

					CommonFunc("Provider secret (default/secret) is missing the ARM_CLIENT_SECRET")
				})

				Context("but missing subscription id", func() {
					BeforeEach(func() {
						secret.Data = map[string][]byte{
							"ARM_CLIENT_ID":     []byte("foo"),
							"ARM_CLIENT_SECRET": []byte("foo"),
						}
						Expect(cc.Update(context.Background(), secret)).To(Succeed())

						result, _, rerr = controllertests.Roll(context.TODO(), controller, provider, 0)
					})

					CommonFunc("Provider secret (default/secret) is missing the ARM_SUBSCRIPTION_ID")
				})

				Context("but missing tenant id", func() {
					BeforeEach(func() {
						secret.Data = map[string][]byte{
							"ARM_CLIENT_ID":       []byte("foo"),
							"ARM_CLIENT_SECRET":   []byte("foo"),
							"ARM_SUBSCRIPTION_ID": []byte("foo"),
						}
						Expect(cc.Update(context.Background(), secret)).To(Succeed())

						result, _, rerr = controllertests.Roll(context.TODO(), controller, provider, 0)
					})

					CommonFunc("Provider secret (default/secret) is missing the ARM_TENANT_ID")
				})

				Context("and we have all the required keys", func() {
					BeforeEach(func() {
						secret.Data = map[string][]byte{
							"ARM_CLIENT_ID":       []byte("foo"),
							"ARM_CLIENT_SECRET":   []byte("foo"),
							"ARM_SUBSCRIPTION_ID": []byte("foo"),
							"ARM_TENANT_ID":       []byte("foo"),
						}
						Expect(cc.Update(context.Background(), secret)).To(Succeed())

						result, _, rerr = controllertests.Roll(context.TODO(), controller, provider, 0)
					})

					CommonFunc(controllerutils.ResourceReady)
				})
			})

			Context("and the provider is aws", func() {

				Context("but is missing the required keys", func() {
					BeforeEach(func() {
						secret.Data = map[string][]byte{}
						Expect(cc.Update(context.Background(), secret)).To(Succeed())

						result, _, rerr = controllertests.Roll(context.TODO(), controller, provider, 0)
					})

					CommonFunc("Provider secret (default/secret) is missing the AWS_ACCESS_KEY_ID")
				})

				Context("but it is missing the secret access key", func() {
					BeforeEach(func() {
						secret.Data = map[string][]byte{"AWS_ACCESS_KEY_ID": []byte("test")}
						Expect(cc.Update(context.Background(), secret)).To(Succeed())

						result, _, rerr = controllertests.Roll(context.TODO(), controller, provider, 0)
					})

					CommonFunc("Provider secret (default/secret) is missing the AWS_SECRET_ACCESS_KEY")
				})

				Context("but it has a empty access key", func() {
					BeforeEach(func() {
						secret.Data = map[string][]byte{
							"AWS_ACCESS_KEY_ID":     []byte(""),
							"AWS_SECRET_ACCESS_KEY": []byte("test"),
						}
						Expect(cc.Update(context.Background(), secret)).To(Succeed())

						result, _, rerr = controllertests.Roll(context.TODO(), controller, provider, 0)
					})

					CommonFunc("Provider secret (default/secret) is missing the AWS_ACCESS_KEY_ID")
				})

				Context("and the access key is bigger than the secret", func() {
					BeforeEach(func() {
						secret.Data = map[string][]byte{
							"AWS_ACCESS_KEY_ID":     []byte("bggger"),
							"AWS_SECRET_ACCESS_KEY": []byte("test"),
						}
						Expect(cc.Update(context.Background(), secret)).To(Succeed())

						result, _, rerr = controllertests.Roll(context.TODO(), controller, provider, 0)
					})

					CommonFunc("Provider secret (default/secret) aws access key is larger than secret")
				})

				Context("and we have all the required keys", func() {
					BeforeEach(func() {
						secret.Data = map[string][]byte{
							"AWS_ACCESS_KEY_ID":     []byte("test"),
							"AWS_SECRET_ACCESS_KEY": []byte("test"),
						}
						Expect(cc.Update(context.Background(), secret)).To(Succeed())

						result, _, rerr = controllertests.Roll(context.TODO(), controller, provider, 0)
					})

					CommonFunc(controllerutils.ResourceReady)
				})
			})
		})
	})

	Context("and the provider is configured with a secret", func() {

		Context("and provider is unknown", func() {
			BeforeEach(func() {
				provider.Spec.Provider = "not-supported"
				Expect(cc.Update(context.Background(), provider)).To(Succeed())

				result, _, rerr = controllertests.Roll(context.TODO(), controller, provider, 0)
			})

			It("should have the conditions", func() {
				Expect(cc.Get(context.TODO(), provider.GetNamespacedName(), provider)).To(Succeed())

				Expect(provider.Status.Conditions).To(HaveLen(2))
			})

			It("should indicate the provider is ready", func() {
				Expect(cc.Get(context.TODO(), provider.GetNamespacedName(), provider)).To(Succeed())

				Expect(provider.Status.Conditions).To(HaveLen(2))
				Expect(provider.Status.Conditions[0].Type).To(Equal(corev1alpha1.ConditionReady))
				Expect(provider.Status.Conditions[0].Status).To(Equal(metav1.ConditionTrue))
			})

			It("should not requeue", func() {
				Expect(rerr).To(BeNil())
				Expect(result).To(Equal(reconcile.Result{}))
			})
		})

		Context("and provider is setup correctly", func() {
			BeforeEach(func() {
				result, _, rerr = controllertests.Roll(context.TODO(), controller, provider, 0)
			})

			It("should have the conditions", func() {
				Expect(cc.Get(context.TODO(), provider.GetNamespacedName(), provider)).To(Succeed())

				Expect(provider.Status.Conditions).To(HaveLen(2))
			})

			It("should indicate the provider is ready", func() {
				Expect(cc.Get(context.TODO(), provider.GetNamespacedName(), provider)).To(Succeed())

				Expect(provider.Status.Conditions).To(HaveLen(2))
				Expect(provider.Status.Conditions[0].Type).To(Equal(corev1alpha1.ConditionReady))
				Expect(provider.Status.Conditions[0].Status).To(Equal(metav1.ConditionTrue))
				Expect(provider.Status.Conditions[0].Reason).To(Equal(corev1alpha1.ReasonReady))
				Expect(provider.Status.Conditions[0].Message).To(Equal("Resource ready"))
			})

			It("should not requeue", func() {
				Expect(rerr).To(BeNil())
				Expect(result).To(Equal(reconcile.Result{}))
			})
		})
	})
})
