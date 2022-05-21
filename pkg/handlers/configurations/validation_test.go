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

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	terraformv1alphav1 "github.com/appvia/terraform-controller/pkg/apis/terraform/v1alpha1"
	"github.com/appvia/terraform-controller/pkg/schema"
	"github.com/appvia/terraform-controller/test/fixtures"
)

var _ = Describe("Configuration Validation", func() {
	ctx := context.Background()
	var cc client.Client
	var v *validator

	namespace := "default"
	name := "aws"

	When("creating a configuration", func() {
		BeforeEach(func() {
			cc = fake.NewClientBuilder().WithScheme(schema.GetScheme()).WithRuntimeObjects(fixtures.NewNamespace("default")).Build()
			v = &validator{cc: cc}
		})

		When("we have a module constraint", func() {
			BeforeEach(func() {
				provider := fixtures.NewValidAWSProvider(namespace, name)
				policy := fixtures.NewPolicy("block")
				policy.Spec.Constraints = &terraformv1alphav1.Constraints{}
				policy.Spec.Constraints.Modules = &terraformv1alphav1.ModuleConstraint{
					Allowed: []string{"does_not_match"},
				}

				Expect(cc.Create(ctx, policy)).To(Succeed())
				Expect(cc.Create(ctx, provider)).To(Succeed())
			})

			It("should deny the configuration of the module", func() {
				err := v.ValidateCreate(ctx, fixtures.NewValidBucketConfiguration(namespace, "test"))
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(Equal("the configuration has been denied by policy"))
			})
		})

		When("no module constraint passes", func() {
			BeforeEach(func() {
				provider := fixtures.NewValidAWSProvider(namespace, name)
				policy := fixtures.NewPolicy("block")
				policy.Spec.Constraints = &terraformv1alphav1.Constraints{}
				policy.Spec.Constraints.Modules = &terraformv1alphav1.ModuleConstraint{
					Allowed: []string{".*"},
				}

				Expect(cc.Create(ctx, policy)).To(Succeed())
				Expect(cc.Create(ctx, provider)).To(Succeed())
			})

			It("should allow the configuration of the module", func() {
				err := v.ValidateCreate(ctx, fixtures.NewValidBucketConfiguration(namespace, "test"))
				Expect(err).ToNot(HaveOccurred())
			})

			It("should allow the configuration of the module", func() {
				err := v.ValidateUpdate(ctx, nil, fixtures.NewValidBucketConfiguration(namespace, "test"))
				Expect(err).ToNot(HaveOccurred())
			})
		})

		When("provider namespace selectors do not match", func() {
			BeforeEach(func() {
				provider := fixtures.NewValidAWSProvider(namespace, name)
				provider.Spec.Selector = &terraformv1alphav1.Selector{
					Namespace: &metav1.LabelSelector{
						MatchLabels: map[string]string{
							"does_not_match": "true",
						},
					},
				}
				Expect(cc.Create(ctx, provider)).To(Succeed())
			})

			It("should deny the creation of the configuration", func() {
				err := v.ValidateCreate(ctx, fixtures.NewValidBucketConfiguration(namespace, "test"))
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("configuration has been denied by the provider policy"))
			})

			It("should deny the update of the configuration", func() {
				err := v.ValidateUpdate(ctx, nil, fixtures.NewValidBucketConfiguration(namespace, "test"))
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("configuration has been denied by the provider policy"))
			})
		})

		When("provider namespace selectors do match", func() {
			BeforeEach(func() {
				provider := fixtures.NewValidAWSProvider(namespace, name)
				provider.Spec.Selector = &terraformv1alphav1.Selector{
					Namespace: &metav1.LabelSelector{
						MatchLabels: map[string]string{
							"name": namespace,
						},
					},
				}
				Expect(cc.Create(ctx, provider)).To(Succeed())
			})

			It("should allow the creation of the configuration", func() {
				err := v.ValidateCreate(ctx, fixtures.NewValidBucketConfiguration(namespace, "test"))
				Expect(err).ToNot(HaveOccurred())
			})

			It("should allow the update of the configuration", func() {
				err := v.ValidateUpdate(ctx, nil, fixtures.NewValidBucketConfiguration(namespace, "test"))
				Expect(err).ToNot(HaveOccurred())
			})
		})

		When("provider resource selectors do not match", func() {
			BeforeEach(func() {
				provider := fixtures.NewValidAWSProvider(namespace, name)
				provider.Spec.Selector = &terraformv1alphav1.Selector{
					Resource: &metav1.LabelSelector{
						MatchLabels: map[string]string{
							"does_not_match": "true",
						},
					},
				}
				Expect(cc.Create(ctx, provider)).To(Succeed())
			})

			It("should deny the creation of the configuration", func() {
				err := v.ValidateCreate(ctx, fixtures.NewValidBucketConfiguration(namespace, "test"))
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("configuration has been denied by the provider policy"))
			})

			It("should deny the update of the configuration", func() {
				err := v.ValidateUpdate(ctx, nil, fixtures.NewValidBucketConfiguration(namespace, "test"))
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("configuration has been denied by the provider policy"))
			})
		})

		When("provider resource selectors match", func() {
			BeforeEach(func() {
				provider := fixtures.NewValidAWSProvider(namespace, name)
				provider.Spec.Selector = &terraformv1alphav1.Selector{
					Resource: &metav1.LabelSelector{
						MatchLabels: map[string]string{
							"does_match": "true",
						},
					},
				}
				Expect(cc.Create(ctx, provider)).To(Succeed())
			})

			It("should allow the creation of the configuration", func() {
				configuration := fixtures.NewValidBucketConfiguration(namespace, "test")
				configuration.Labels = map[string]string{"does_match": "true"}

				err := v.ValidateCreate(ctx, configuration)
				Expect(err).To(Succeed())
			})

			It("should allow the update of the configuration", func() {
				configuration := fixtures.NewValidBucketConfiguration(namespace, "test")
				configuration.Labels = map[string]string{"does_match": "true"}

				err := v.ValidateUpdate(ctx, nil, configuration)
				Expect(err).To(Succeed())
			})
		})
	})
})
