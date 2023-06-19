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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	terraformv1alpha1 "github.com/appvia/terranetes-controller/pkg/apis/terraform/v1alpha1"
	"github.com/appvia/terranetes-controller/pkg/schema"
	"github.com/appvia/terranetes-controller/test/fixtures"
)

func TestHandler(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Running Test Suite")
}

var _ = Describe("Configuration Mutation", func() {
	var handler *mutator
	var configuration *terraformv1alpha1.Configuration
	var cc client.Client
	var err error

	When("creating a configuration", func() {
		BeforeEach(func() {
			ns := fixtures.NewNamespace("app")
			ns.Labels = map[string]string{"app": "test"}
			cc = fake.NewClientBuilder().WithRuntimeObjects(ns).WithScheme(schema.GetScheme()).Build()
			handler = &mutator{cc: cc}

			configuration = fixtures.NewValidBucketConfiguration("app", "test")
		})

		Context("and we no have provider reference", func() {
			BeforeEach(func() {
				configuration.Spec.ProviderRef = nil
			})

			Context("and no default provider", func() {
				BeforeEach(func() {
					err = handler.Default(context.Background(), configuration)
				})

				It("should not inject a provider reference", func() {
					Expect(err).ToNot(HaveOccurred())
					Expect(configuration.Spec.ProviderRef).To(BeNil())
				})
			})

			Context("and a provider configured", func() {
				BeforeEach(func() {
					secret := fixtures.NewValidAWSProviderSecret("terraform-system", "aws")
					provider := fixtures.NewValidAWSReadyProvider("aws", secret)
					provider.Annotations = map[string]string{terraformv1alpha1.DefaultProviderAnnotation: "true"}

					Expect(cc.Create(context.Background(), secret)).To(Succeed())
					Expect(cc.Create(context.Background(), provider)).To(Succeed())
				})

				Context("with a single provider configured as default", func() {
					It("should inject a provider reference", func() {
						err = handler.Default(context.Background(), configuration)

						Expect(err).ToNot(HaveOccurred())
						Expect(configuration.Spec.ProviderRef).ToNot(BeNil())
						Expect(configuration.Spec.ProviderRef.Name).To(Equal("aws"))
					})
				})

				Context("with multiple providers configured as default", func() {
					BeforeEach(func() {
						secret := fixtures.NewValidAWSProviderSecret("terraform-system", "aws1")
						provider := fixtures.NewValidAWSReadyProvider("aws1", secret)
						provider.Annotations = map[string]string{terraformv1alpha1.DefaultProviderAnnotation: "true"}

						Expect(cc.Create(context.Background(), secret)).To(Succeed())
						Expect(cc.Create(context.Background(), provider)).To(Succeed())
					})

					It("should fail with an error", func() {
						err = handler.Default(context.Background(), configuration)

						Expect(err).To(HaveOccurred())
						Expect(err.Error()).To(ContainSubstring("only one provider can be default, please contact your administrator"))
					})
				})
			})
		})

		When("and the configuration has a plan reference", func() {
			BeforeEach(func() {
				configuration.Spec.Plan = &terraformv1alpha1.PlanReference{
					Name:     "test",
					Revision: "v0.0.1",
				}

				err = handler.Default(context.Background(), configuration)
			})

			It("should inject the labels", func() {
				Expect(err).ToNot(HaveOccurred())
				Expect(configuration.Labels).To(HaveKeyWithValue(terraformv1alpha1.ConfigurationPlanLabel, "test"))
				Expect(configuration.Labels).To(HaveKeyWithValue(terraformv1alpha1.ConfigurationRevisionVersion, "v0.0.1"))
			})
		})

		When("and we may policies injected default variables", func() {
			var policy *terraformv1alpha1.Policy

			// these are the variables which we start with
			original := `{"foo": "bar", "list": ["a", "b", "c"]}`
			// these are the variables we are injecting in the policy
			injected := `{"inject": "me", "nested": {"foo": "bar"}}`
			// this is the combined result
			expected := `{"foo": "bar", "list": ["a", "b", "c"], "inject": "me", "nested": {"foo": "bar"}}`

			BeforeEach(func() {
				configuration.Spec.Variables.Raw = []byte(original)
				policy = fixtures.NewPolicy("defaults")
				policy.Spec.Defaults = []terraformv1alpha1.DefaultVariables{
					{
						Variables: runtime.RawExtension{Raw: []byte(injected)},
					},
				}
			})

			Context("but no policies currently defined", func() {
				It("should not inject any variables", func() {
					err = handler.Default(context.Background(), configuration)
					Expect(err).ToNot(HaveOccurred())
					Expect(configuration.Spec.Variables.Raw).To(Equal([]byte(original)))
				})
			})

			CommonChecks := func(variables string) {
				It("should not fail", func() {
					Expect(err).ToNot(HaveOccurred())
				})

				It("should have injected the variables from policy ", func() {
					Expect(configuration.Spec.Variables.Raw).To(MatchJSON(variables))
				})
			}

			Context("and a match all policy", func() {
				BeforeEach(func() {
					Expect(cc.Create(context.Background(), policy)).To(Succeed())

					err = handler.Default(context.Background(), configuration)
				})

				CommonChecks(expected)
			})

			Context("and a matching module selector", func() {
				BeforeEach(func() {
					policy.Spec.Defaults[0].Selector.Modules = []string{configuration.Spec.Module}
					Expect(cc.Create(context.Background(), policy)).To(Succeed())

					err = handler.Default(context.Background(), configuration)
				})

				CommonChecks(expected)
			})

			Context("and a matching module regexe", func() {
				BeforeEach(func() {
					policy.Spec.Defaults[0].Selector.Modules = []string{"^.*$"}
					Expect(cc.Create(context.Background(), policy)).To(Succeed())

					err = handler.Default(context.Background(), configuration)
				})

				CommonChecks(expected)
			})

			Context("and a matching label selector", func() {
				BeforeEach(func() {
					policy.Spec.Defaults[0].Selector.Namespace = &metav1.LabelSelector{
						MatchLabels: map[string]string{"app": "test"},
					}
					Expect(cc.Create(context.Background(), policy)).To(Succeed())

					err = handler.Default(context.Background(), configuration)
				})

				CommonChecks(expected)
			})

			Context("and a matching label expression", func() {
				BeforeEach(func() {
					policy.Spec.Defaults[0].Selector.Namespace = &metav1.LabelSelector{
						MatchExpressions: []metav1.LabelSelectorRequirement{
							{
								Key:      "app",
								Operator: metav1.LabelSelectorOpIn,
								Values:   []string{"test"},
							},
						},
					}
					Expect(cc.Create(context.Background(), policy)).To(Succeed())

					err = handler.Default(context.Background(), configuration)
				})

				CommonChecks(expected)
			})

			Context("and a matching label and module selector", func() {
				BeforeEach(func() {
					policy.Spec.Defaults[0].Selector.Modules = []string{configuration.Spec.Module}
					policy.Spec.Defaults[0].Selector.Namespace = &metav1.LabelSelector{
						MatchExpressions: []metav1.LabelSelectorRequirement{
							{
								Key:      "app",
								Operator: metav1.LabelSelectorOpIn,
								Values:   []string{"test"},
							},
						},
					}
					Expect(cc.Create(context.Background(), policy)).To(Succeed())

					err = handler.Default(context.Background(), configuration)
				})

				CommonChecks(expected)
			})

			Context("and no matching modules selector", func() {
				BeforeEach(func() {
					policy.Spec.Defaults[0].Selector.Modules = []string{"no_match"}
					Expect(cc.Create(context.Background(), policy)).To(Succeed())

					err = handler.Default(context.Background(), configuration)
				})

				CommonChecks(original)
			})

			Context("and module match but a label mismatch", func() {
				BeforeEach(func() {
					policy.Spec.Defaults[0].Selector.Modules = []string{configuration.Spec.Module}
					policy.Spec.Defaults[0].Selector.Namespace = &metav1.LabelSelector{
						MatchExpressions: []metav1.LabelSelectorRequirement{
							{
								Key:      "app",
								Operator: metav1.LabelSelectorOpIn,
								Values:   []string{"no_match"},
							},
						},
					}
					Expect(cc.Create(context.Background(), policy)).To(Succeed())

					err = handler.Default(context.Background(), configuration)
				})

				CommonChecks(original)
			})
		})
	})
})
