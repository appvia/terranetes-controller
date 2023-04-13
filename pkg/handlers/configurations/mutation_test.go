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

package configurations

import (
	"context"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	terraformv1alpha1 "github.com/appvia/terranetes-controller/pkg/apis/terraform/v1alpha1"
	"github.com/appvia/terranetes-controller/pkg/schema"
	"github.com/appvia/terranetes-controller/test/fixtures"
)

func TestReconcile(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Running Test Suite")
}

var _ = Describe("Configuration Mutation", func() {

	var handler *mutator
	var before, after *terraformv1alpha1.Configuration
	var cc client.Client
	var err error

	When("creating a configuration", func() {
		BeforeEach(func() {
			ns := fixtures.NewNamespace("app")
			ns.Labels = map[string]string{"app": "test"}
			cc = fake.NewClientBuilder().WithRuntimeObjects(ns).WithScheme(schema.GetScheme()).Build()
			handler = &mutator{cc: cc}
			before = fixtures.NewValidBucketConfiguration("app", "test")
			after = before.DeepCopy()
		})

		Context("and we have no policies", func() {
			BeforeEach(func() {
				err = handler.Default(context.Background(), after)
			})

			It("should not throw an error", func() {
				Expect(err).ToNot(HaveOccurred())
			})

			It("should remain unchanged", func() {
				Expect(before).To(Equal(after))
				Expect(err).ToNot(HaveOccurred())
			})
		})

		Context("and we have zero matching policies", func() {
			BeforeEach(func() {
				policy := fixtures.NewPolicy("mutate")
				policy.Spec.Defaults = []terraformv1alpha1.DefaultVariables{
					{
						Selector: terraformv1alpha1.DefaultVariablesSelector{
							Namespace: &metav1.LabelSelector{
								MatchLabels: map[string]string{"app": "no_match"},
							},
						},
						Variables: runtime.RawExtension{
							Raw: []byte(`{"foo": "bar"}`),
						},
					},
				}
				Expect(cc.Create(context.Background(), policy)).To(Succeed())

				err = handler.Default(context.Background(), after)
			})

			It("should not throw an error", func() {
				Expect(err).ToNot(HaveOccurred())
			})

			It("should remain unchanged", func() {
				Expect(before).To(Equal(after))
				Expect(err).ToNot(HaveOccurred())
			})
		})

		Context("and we have a matching namespace selector", func() {
			BeforeEach(func() {
				policy := fixtures.NewPolicy("mutate")
				policy.Spec.Defaults = []terraformv1alpha1.DefaultVariables{
					{
						Selector: terraformv1alpha1.DefaultVariablesSelector{
							Namespace: &metav1.LabelSelector{
								MatchLabels: map[string]string{"app": "test"},
							},
						},
						Variables: runtime.RawExtension{
							Raw: []byte(`{"is": "changed"}`),
						},
					},
				}
				Expect(cc.Create(context.Background(), policy)).To(Succeed())

				err = handler.Default(context.Background(), after)
			})

			It("should not throw an error", func() {
				Expect(err).ToNot(HaveOccurred())
			})

			It("should have injected the default variables", func() {
				Expect(before).ToNot(Equal(after))
				Expect(after.Spec.Variables.Raw).To(Equal([]byte(`{"is":"changed","name":"test"}`)))
			})
		})

		Context("and we have matching namespace selector but no initial variables", func() {
			BeforeEach(func() {
				before.Spec.Variables.Raw = []byte("")
				after = before.DeepCopy()

				policy := fixtures.NewPolicy("test")
				policy.Spec.Defaults = []terraformv1alpha1.DefaultVariables{
					{
						Selector: terraformv1alpha1.DefaultVariablesSelector{
							Namespace: &metav1.LabelSelector{
								MatchLabels: map[string]string{"app": "test"},
							},
						},
						Variables: runtime.RawExtension{
							Raw: []byte(`{"foo": "bar"}`),
						},
					},
				}
				Expect(cc.Create(context.Background(), policy)).To(Succeed())

				err = handler.Default(context.Background(), after)
			})

			It("should not throw an error", func() {
				Expect(err).ToNot(HaveOccurred())
			})

			It("should have changed", func() {
				Expect(before).ToNot(Equal(after))
				Expect(after.Spec.Variables.Raw).To(Equal([]byte(`{"foo":"bar"}`)))
			})
		})

		Context("and we have module selectors", func() {
			var policy *terraformv1alpha1.Policy

			BeforeEach(func() {
				before.Spec.Variables.Raw = []byte(`{"name":"existing"}`)
				after = before.DeepCopy()

				policy = fixtures.NewPolicy("test")
				policy.Spec.Defaults = []terraformv1alpha1.DefaultVariables{
					{
						Selector: terraformv1alpha1.DefaultVariablesSelector{
							Modules: []string{before.Spec.Module},
						},
						Variables: runtime.RawExtension{
							Raw: []byte(`{"foo": "bar"}`),
						},
					},
				}
			})

			Context("which is not matching", func() {
				BeforeEach(func() {
					policy.Spec.Defaults[0].Selector.Modules = []string{"not-matching"}
					Expect(cc.Create(context.Background(), policy)).To(Succeed())

					err = handler.Default(context.Background(), after)
				})

				It("should not throw an error", func() {
					Expect(err).ToNot(HaveOccurred())
				})

				It("should not have changed", func() {
					Expect(before).To(Equal(after))
					Expect(after.Spec.Variables.Raw).To(Equal([]byte(`{"name":"existing"}`)))
				})
			})

			Context("which is matching", func() {
				BeforeEach(func() {
					Expect(cc.Create(context.Background(), policy)).To(Succeed())

					err = handler.Default(context.Background(), after)
				})

				It("should not throw an error", func() {
					Expect(err).ToNot(HaveOccurred())
				})

				It("should have changed", func() {
					Expect(before).ToNot(Equal(after))
					Expect(after.Spec.Variables.Raw).To(Equal([]byte(`{"foo":"bar","name":"existing"}`)))
				})
			})
		})

		Context("and we are using multiple selectors", func() {
			var policy *terraformv1alpha1.Policy

			BeforeEach(func() {
				before.Spec.Variables.Raw = []byte(`{"name":"existing"}`)
				after = before.DeepCopy()

				policy = fixtures.NewPolicy("test")
				policy.Spec.Defaults = []terraformv1alpha1.DefaultVariables{
					{
						Selector: terraformv1alpha1.DefaultVariablesSelector{
							Modules: []string{before.Spec.Module},
							Namespace: &metav1.LabelSelector{
								MatchLabels: map[string]string{"app": "test"},
							},
						},
						Variables: runtime.RawExtension{
							Raw: []byte(`{"foo": "bar"}`),
						},
						Secrets: []string{"test"},
					},
				}
			})

			Context("which is not matching", func() {
				BeforeEach(func() {
					policy.Spec.Defaults[0].Selector.Modules = []string{"which_does_not_match"}
					Expect(cc.Create(context.Background(), policy)).To(Succeed())

					err = handler.Default(context.Background(), after)
				})

				It("should not throw an error", func() {
					Expect(err).ToNot(HaveOccurred())
				})

				It("should not have changed", func() {
					Expect(before).To(Equal(after))
					Expect(before.Spec.Variables.Raw).To(Equal([]byte(`{"name":"existing"}`)))
					Expect(after.Spec.Variables.Raw).To(Equal([]byte(`{"name":"existing"}`)))
				})
			})

			Context("which is matching", func() {
				BeforeEach(func() {
					Expect(cc.Create(context.Background(), policy)).To(Succeed())

					err = handler.Default(context.Background(), after)
				})

				It("should not throw an error", func() {
					Expect(err).ToNot(HaveOccurred())
				})

				It("should have changed", func() {
					Expect(before).ToNot(Equal(after))
					Expect(before.Spec.Variables.Raw).To(Equal([]byte(`{"name":"existing"}`)))
					Expect(after.Spec.Variables.Raw).To(Equal([]byte(`{"foo":"bar","name":"existing"}`)))
				})
			})
		})

		Context("with a policy defining default secrets", func() {
			BeforeEach(func() {
				before.Spec.Variables.Raw = []byte(`{"name":"existing"}`)
				after = before.DeepCopy()

				policy := fixtures.NewPolicy("test")
				policy.Spec.Defaults = []terraformv1alpha1.DefaultVariables{
					{
						Secrets: []string{"test"},
					},
				}

				err = handler.Default(context.Background(), after)
			})

			It("should not throw an error", func() {
				Expect(err).ToNot(HaveOccurred())
			})

			It("should not have changed", func() {
				Expect(before).To(Equal(after))
				Expect(after.Spec.Variables.Raw).To(Equal([]byte(`{"name":"existing"}`)))
				Expect(err).ToNot(HaveOccurred())
			})
		})

		Context("with no provider reference defined in the configuration", func() {
			var provider *terraformv1alpha1.Provider

			BeforeEach(func() {
				before.Spec.ProviderRef.Name = ""
				after = before.DeepCopy()

				secret := fixtures.NewValidAWSProviderSecret("terraform-system", "default")
				provider = fixtures.NewValidAWSReadyProvider("default", secret)
				provider.Annotations = map[string]string{
					terraformv1alpha1.DefaultProviderAnnotation: "true",
				}

				Expect(cc.Create(context.Background(), secret)).To(Succeed())
			})

			Context("and no default provider defined", func() {
				BeforeEach(func() {
					provider.Annotations = map[string]string{}
					Expect(cc.Create(context.Background(), provider)).To(Succeed())

					err = handler.Default(context.Background(), after)
				})

				It("should not throw an error", func() {
					Expect(err).ToNot(HaveOccurred())
				})

				It("should not have changed", func() {
					Expect(before).To(Equal(after))
					Expect(after.Spec.ProviderRef.Name).To(Equal(""))
				})
			})

			Context("and a default provider defined", func() {
				BeforeEach(func() {
					Expect(cc.Create(context.Background(), provider)).To(Succeed())

					err = handler.Default(context.Background(), after)
				})

				It("should not throw an error", func() {
					Expect(err).ToNot(HaveOccurred())
				})

				It("should have changed", func() {
					Expect(before).ToNot(Equal(after))
					Expect(after.Spec.ProviderRef.Name).To(Equal(provider.Name))
					Expect(before.Spec.ProviderRef.Name).To(BeEmpty())
				})
			})

			Context("and multiple default providers defined", func() {
				BeforeEach(func() {
					additional := fixtures.NewValidAWSReadyProvider("additional", &v1.Secret{})
					additional.Annotations = map[string]string{
						terraformv1alpha1.DefaultProviderAnnotation: "true",
					}
					Expect(cc.Create(context.Background(), provider)).To(Succeed())
					Expect(cc.Create(context.Background(), additional)).To(Succeed())

					err = handler.Default(context.Background(), after)
				})

				It("should throw an error", func() {
					Expect(err).To(HaveOccurred())
					Expect(err.Error()).To(Equal("only one provider can be default, please contact your administrator"))
				})

				It("should have not changed", func() {
					Expect(before).To(Equal(after))
				})
			})
		})
	})
})
