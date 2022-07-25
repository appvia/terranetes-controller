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

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	terraformv1alphav1 "github.com/appvia/terranetes-controller/pkg/apis/terraform/v1alpha1"
	"github.com/appvia/terranetes-controller/pkg/schema"
	"github.com/appvia/terranetes-controller/test/fixtures"
)

var _ = Describe("Configuration Validation", func() {
	ctx := context.Background()
	var cc client.Client
	var v *validator

	namespace := "default"
	name := "aws"

	When("we have a connection secret", func() {
		var configuration *terraformv1alphav1.Configuration

		BeforeEach(func() {
			cc = fake.NewClientBuilder().WithScheme(schema.GetScheme()).WithRuntimeObjects(fixtures.NewNamespace("default")).Build()
			v = &validator{cc: cc, versioning: true}

			Expect(cc.Create(ctx, fixtures.NewValidAWSReadyProvider(name, fixtures.NewValidAWSProviderSecret(namespace, name)))).To(Succeed())
		})

		When("the connection contains invalid key", func() {
			expected := "spec.writeConnectionSecretToRef.keys[0] contains invalid key: this:is:invalid, should be KEY:NEWNAME"

			BeforeEach(func() {
				configuration = fixtures.NewValidBucketConfiguration(namespace, name)
				configuration.Spec.WriteConnectionSecretToRef.Keys = []string{"this:is:invalid"}
			})

			It("should fail on creation", func() {
				err := v.ValidateCreate(ctx, configuration)
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(Equal(expected))
			})

			It("should fail on update", func() {
				err := v.ValidateUpdate(ctx, nil, configuration)
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(Equal(expected))
			})
		})

		When("the configuration keys are valid", func() {
			BeforeEach(func() {
				configuration = fixtures.NewValidBucketConfiguration(namespace, name)
				configuration.Spec.WriteConnectionSecretToRef.Keys = []string{"is:valid"}
			})

			It("should not faild", func() {
				err := v.ValidateCreate(ctx, configuration)
				Expect(err).NotTo(HaveOccurred())

				err = v.ValidateUpdate(ctx, nil, configuration)
				Expect(err).NotTo(HaveOccurred())
			})
		})

		When("we have no configuration keys", func() {
			BeforeEach(func() {
				configuration = fixtures.NewValidBucketConfiguration(namespace, name)
			})

			It("should not fail", func() {
				err := v.ValidateCreate(ctx, configuration)
				Expect(err).NotTo(HaveOccurred())

				err = v.ValidateUpdate(ctx, nil, configuration)
				Expect(err).NotTo(HaveOccurred())
			})
		})
	})

	When("updating an existing configuration", func() {
		BeforeEach(func() {
			cc = fake.NewClientBuilder().WithScheme(schema.GetScheme()).WithRuntimeObjects(fixtures.NewNamespace("default")).Build()
			v = &validator{cc: cc, versioning: true}

			Expect(cc.Create(ctx, fixtures.NewValidAWSReadyProvider(name, fixtures.NewValidAWSProviderSecret(namespace, name)))).To(Succeed())
		})

		When("versioning is disabled", func() {
			BeforeEach(func() {
				v.versioning = false
			})

			When("trying to change the version of existing configuration", func() {
				It("should failed to change the version of existing configuration", func() {
					before := fixtures.NewValidBucketConfiguration(namespace, "test")
					after := before.DeepCopy()
					after.Spec.TerraformVersion = "v1.1.0"

					err := v.ValidateUpdate(ctx, before, after)
					Expect(err).To(HaveOccurred())
					Expect(err.Error()).To(Equal("spec.terraformVersion has been disabled and cannot be changed"))
				})

				It("should not fail if version is being removed", func() {
					before := fixtures.NewValidBucketConfiguration(namespace, "test")
					after := before.DeepCopy()
					after.Spec.TerraformVersion = ""

					err := v.ValidateUpdate(ctx, before, after)
					Expect(err).To(Succeed())
				})

				It("should not fail if version in the same prior to versioning disabled", func() {
					before := fixtures.NewValidBucketConfiguration(namespace, "test")
					before.Spec.TerraformVersion = "v1.1.9"
					before.Spec.EnableAutoApproval = false

					after := before.DeepCopy()
					after.Spec.TerraformVersion = "v1.1.9"
					after.Spec.EnableAutoApproval = true

					err := v.ValidateUpdate(ctx, before, after)
					Expect(err).To(Succeed())
				})
			})
		})
	})

	When("creating a configuration", func() {
		BeforeEach(func() {
			cc = fake.NewClientBuilder().WithScheme(schema.GetScheme()).WithRuntimeObjects(fixtures.NewNamespace("default")).Build()
			v = &validator{cc: cc, versioning: true}
		})

		When("we have a module constraint", func() {
			BeforeEach(func() {
				provider := fixtures.NewValidAWSProvider(name, fixtures.NewValidAWSProviderSecret(namespace, name))
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
				Expect(err.Error()).To(Equal("configuration has been denied by policy"))
			})
		})

		When("no module constraint passes", func() {
			BeforeEach(func() {
				provider := fixtures.NewValidAWSProvider(name, fixtures.NewValidAWSProviderSecret(namespace, name))
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

		When("we have two module constraints", func() {
			BeforeEach(func() {
				provider := fixtures.NewValidAWSProvider(name, fixtures.NewValidAWSProviderSecret(namespace, name))
				all := fixtures.NewPolicy("all")
				all.Spec.Constraints = &terraformv1alphav1.Constraints{}
				all.Spec.Constraints.Modules = &terraformv1alphav1.ModuleConstraint{Allowed: []string{"default.*"}}

				allow := fixtures.NewPolicy("allow")
				allow.Spec.Constraints = &terraformv1alphav1.Constraints{}
				allow.Spec.Constraints.Modules = &terraformv1alphav1.ModuleConstraint{Allowed: []string{"allow.*"}}

				Expect(cc.Create(ctx, all)).To(Succeed())
				Expect(cc.Create(ctx, allow)).To(Succeed())
				Expect(cc.Create(ctx, provider)).To(Succeed())
			})

			It("should fail with a constraint violation", func() {
				configuration := fixtures.NewValidBucketConfiguration(namespace, "test")
				configuration.Spec.Module = "allow"

				Expect(cc.Delete(ctx, fixtures.NewPolicy("allow"))).To(Succeed())

				err := v.ValidateCreate(ctx, fixtures.NewValidBucketConfiguration(namespace, "test"))
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(Equal("configuration has been denied by policy"))
			})

			It("should be allowed by the second policy", func() {
				configuration := fixtures.NewValidBucketConfiguration(namespace, "test")
				configuration.Spec.Module = "allow"
				err := v.ValidateCreate(ctx, configuration)

				Expect(err).ToNot(HaveOccurred())
			})
		})

		When("provider namespace selectors do not match", func() {
			BeforeEach(func() {
				provider := fixtures.NewValidAWSProvider(name, fixtures.NewValidAWSProviderSecret(namespace, name))
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
				provider := fixtures.NewValidAWSProvider(name, fixtures.NewValidAWSProviderSecret(namespace, name))
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
				provider := fixtures.NewValidAWSProvider(name, fixtures.NewValidAWSProviderSecret(namespace, name))
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
				provider := fixtures.NewValidAWSProvider(name, fixtures.NewValidAWSProviderSecret(namespace, name))
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

		When("versioning is disabled on configurations", func() {
			BeforeEach(func() {
				v.versioning = false
				Expect(cc.Create(ctx, fixtures.NewValidAWSReadyProvider(name, fixtures.NewValidAWSProviderSecret(namespace, name)))).To(Succeed())
			})

			It("should be denied due to versioning", func() {
				configuration := fixtures.NewValidBucketConfiguration(namespace, "test")
				configuration.Spec.TerraformVersion = "bad"
				err := v.ValidateCreate(ctx, configuration)

				Expect(err).ToNot(Succeed())
				Expect(err.Error()).To(Equal("spec.terraformVersion changes have been disabled"))
			})
		})
	})
})
