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
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	terraformv1alpha1 "github.com/appvia/terranetes-controller/pkg/apis/terraform/v1alpha1"
	"github.com/appvia/terranetes-controller/pkg/schema"
	"github.com/appvia/terranetes-controller/test/fixtures"
)

var _ = Describe("Checking Configuration Validation", func() {
	ctx := context.Background()
	var cc client.Client
	var v *validator
	var err error
	var warnings admission.Warnings
	var configuration *terraformv1alpha1.Configuration

	namespace := "default"
	name := "aws"

	BeforeEach(func() {
		cc = fake.NewClientBuilder().WithScheme(schema.GetScheme()).WithRuntimeObjects(fixtures.NewNamespace("default")).Build()
		v = &validator{cc: cc, enableVersions: true}
		configuration = fixtures.NewValidBucketConfiguration(namespace, name)
	})

	When("creating a validator", func() {
		It("should not be nil", func() {
			v := NewValidator(cc, true)
			Expect(v).ToNot(BeNil())
		})
	})

	When("not passing a configuration", func() {
		It("should fail", func() {
			warnings, err = v.ValidateCreate(ctx, &v1.Namespace{})
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(Equal("expected a Configuration, but got: *v1.Namespace"))
			Expect(warnings).To(BeEmpty())
		})
	})

	When("and the configuration has a plan reference", func() {
		BeforeEach(func() {
			configuration.Spec.Variables = nil
			configuration.Spec.ValueFrom = nil

			configuration.Spec.Plan = &terraformv1alpha1.PlanReference{
				Name:     "test",
				Revision: "1.0.0",
			}
		})

		It("should fail when no plan name", func() {
			configuration.Spec.Plan.Name = ""

			warnings, err = v.ValidateCreate(ctx, configuration)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(Equal("spec.plan.name is required"))
			Expect(warnings).To(BeEmpty())
		})

		It("should fail when no plan revision", func() {
			configuration.Spec.Plan.Revision = ""
			warnings, err := v.ValidateCreate(ctx, configuration)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(Equal("spec.plan.revision is required"))
			Expect(warnings).To(BeEmpty())
		})
	})

	When("we have authentication", func() {
		Context("and the name is missing", func() {
			BeforeEach(func() {
				configuration.Spec.Auth = &v1.SecretReference{}

				warnings, err = v.ValidateCreate(ctx, configuration)
			})

			It("should fail", func() {
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(Equal("spec.auth.name is required"))
				Expect(warnings).To(BeEmpty())
			})
		})
	})

	When("we have a connection secret", func() {
		BeforeEach(func() {
			Expect(cc.Create(ctx, fixtures.NewValidAWSReadyProvider(name, fixtures.NewValidAWSProviderSecret(namespace, name)))).To(Succeed())
		})

		Context("the connection secret is empty", func() {
			expected := "spec.writeConnectionSecretToRef.name is required"

			BeforeEach(func() {
				configuration.Spec.WriteConnectionSecretToRef.Name = ""
			})

			It("should fail on creating", func() {
				warnings, err = v.ValidateCreate(ctx, configuration)
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(Equal(expected))
				Expect(warnings).To(BeEmpty())
			})

			It("should fail on updating", func() {
				warnings, err = v.ValidateUpdate(ctx, nil, configuration)
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(Equal(expected))
				Expect(warnings).To(BeEmpty())
			})
		})

		Context("the connection contains invalid key", func() {
			expected := "spec.writeConnectionSecretToRef.keys[0] contains invalid key: this:is:invalid, should be KEY:NEWNAME"

			BeforeEach(func() {
				configuration.Spec.WriteConnectionSecretToRef.Keys = []string{"this:is:invalid"}
			})

			It("should fail on creation", func() {
				warnings, err := v.ValidateCreate(ctx, configuration)
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(Equal(expected))
				Expect(warnings).To(BeEmpty())
			})

			It("should fail on update", func() {
				warnings, err := v.ValidateUpdate(ctx, nil, configuration)
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(Equal(expected))
				Expect(warnings).To(BeEmpty())
			})
		})

		Context("the configuration keys are valid", func() {
			BeforeEach(func() {
				configuration.Spec.WriteConnectionSecretToRef.Keys = []string{"is:valid"}
			})

			It("should not fail", func() {
				warnings, err := v.ValidateCreate(ctx, configuration)
				Expect(err).NotTo(HaveOccurred())
				Expect(warnings).To(BeEmpty())

				warnings, err = v.ValidateUpdate(ctx, nil, configuration)
				Expect(err).NotTo(HaveOccurred())
				Expect(warnings).To(BeEmpty())
			})
		})

		Context("we have no configuration keys", func() {
			It("should not fail", func() {
				warnings, err := v.ValidateCreate(ctx, configuration)
				Expect(err).NotTo(HaveOccurred())
				Expect(warnings).To(BeEmpty())

				warnings, err = v.ValidateUpdate(ctx, nil, configuration)
				Expect(err).NotTo(HaveOccurred())
				Expect(warnings).To(BeEmpty())
			})
		})
	})

	When("updating an existing configuration", func() {
		BeforeEach(func() {
			Expect(cc.Create(ctx, fixtures.NewValidAWSReadyProvider(name, fixtures.NewValidAWSProviderSecret(namespace, name)))).To(Succeed())
		})

		When("versioning is disabled", func() {
			BeforeEach(func() {
				v.enableVersions = false
			})

			When("trying to change the version of existing configuration", func() {

				It("should failed to change the version of existing configuration", func() {
					before := fixtures.NewValidBucketConfiguration(namespace, "test")
					after := before.DeepCopy()
					after.Spec.TerraformVersion = "v1.1.0"

					warnings, err := v.ValidateUpdate(ctx, before, after)
					Expect(err).To(HaveOccurred())
					Expect(err.Error()).To(Equal("spec.terraformVersion has been disabled and cannot be changed"))
					Expect(warnings).To(BeEmpty())
				})

				It("should not fail if version is being removed", func() {
					before := fixtures.NewValidBucketConfiguration(namespace, "test")
					after := before.DeepCopy()
					after.Spec.TerraformVersion = ""

					warnings, err := v.ValidateUpdate(ctx, before, after)
					Expect(err).To(Succeed())
					Expect(warnings).To(BeEmpty())
				})

				It("should not fail if version in the same prior to versioning disabled", func() {
					before := fixtures.NewValidBucketConfiguration(namespace, "test")
					before.Spec.TerraformVersion = "v1.1.9"
					before.Spec.EnableAutoApproval = false

					after := before.DeepCopy()
					after.Spec.TerraformVersion = "v1.1.9"
					after.Spec.EnableAutoApproval = true

					warnings, err := v.ValidateUpdate(ctx, before, after)
					Expect(err).To(Succeed())
					Expect(warnings).To(BeEmpty())
				})
			})
		})
	})

	When("creating a configuration", func() {
		BeforeEach(func() {
			configuration = fixtures.NewValidBucketConfiguration(namespace, name)
		})

		It("should fail when no provider is found", func() {
			configuration.Spec.ProviderRef = nil

			warnings, err := v.ValidateCreate(ctx, configuration)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(Equal("spec.providerRef is required"))
			Expect(warnings).To(BeEmpty())
		})

		It("should fail when no provider name is found", func() {
			configuration.Spec.ProviderRef.Name = ""

			warnings, err := v.ValidateCreate(ctx, configuration)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(Equal("spec.providerRef.name is required"))
			Expect(warnings).To(BeEmpty())
		})

		It("should fail when no module is found", func() {
			configuration.Spec.Module = ""

			warnings, err := v.ValidateCreate(ctx, configuration)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(Equal("spec.module is required"))
			Expect(warnings).To(BeEmpty())
		})

		Context("specifying value from inputs", func() {
			It("should fail when no inputs are found", func() {
				configuration.Spec.ValueFrom = []terraformv1alpha1.ValueFromSource{{}}
				warnings, err = v.ValidateCreate(ctx, configuration)

				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(Equal("spec.valueFrom[0] requires either context or secret"))
				Expect(warnings).To(BeEmpty())
			})

			It("should fail when both context and secret are found", func() {
				configuration = fixtures.NewValidBucketConfiguration(namespace, name)
				configuration.Spec.ValueFrom = []terraformv1alpha1.ValueFromSource{
					{
						Secret:  pointer.String("secret"),
						Context: pointer.String("context"),
					},
				}
				warnings, err = v.ValidateCreate(ctx, configuration)
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(Equal("spec.valueFrom[0] requires either context or secret, not both"))
				Expect(warnings).To(BeEmpty())
			})
		})

		Context("we have a module constraint", func() {
			BeforeEach(func() {
				provider := fixtures.NewValidAWSProvider(name, fixtures.NewValidAWSProviderSecret(namespace, name))
				policy := fixtures.NewPolicy("block")
				policy.Spec.Constraints = &terraformv1alpha1.Constraints{}
				policy.Spec.Constraints.Modules = &terraformv1alpha1.ModuleConstraint{
					Allowed: []string{"does_not_match"},
				}

				Expect(cc.Create(ctx, policy)).To(Succeed())
				Expect(cc.Create(ctx, provider)).To(Succeed())
			})

			It("should deny the configuration of the module", func() {
				warnings, err := v.ValidateCreate(ctx, fixtures.NewValidBucketConfiguration(namespace, "test"))
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(Equal("spec.module: source has been denied by module policy, contact an administrator"))
				Expect(warnings).To(BeEmpty())
			})
		})

		Context("no module constraint passes", func() {
			BeforeEach(func() {
				provider := fixtures.NewValidAWSProvider(name, fixtures.NewValidAWSProviderSecret(namespace, name))
				policy := fixtures.NewPolicy("block")
				policy.Spec.Constraints = &terraformv1alpha1.Constraints{}
				policy.Spec.Constraints.Modules = &terraformv1alpha1.ModuleConstraint{
					Allowed: []string{".*"},
				}

				Expect(cc.Create(ctx, policy)).To(Succeed())
				Expect(cc.Create(ctx, provider)).To(Succeed())
			})

			It("should allow the configuration of the module", func() {
				warnings, err := v.ValidateCreate(ctx, fixtures.NewValidBucketConfiguration(namespace, "test"))
				Expect(err).ToNot(HaveOccurred())
				Expect(warnings).To(BeEmpty())
			})

			It("should allow the configuration of the module", func() {
				warnings, err := v.ValidateUpdate(ctx, nil, fixtures.NewValidBucketConfiguration(namespace, "test"))
				Expect(err).ToNot(HaveOccurred())
				Expect(warnings).To(BeEmpty())
			})
		})

		Context("we have two module constraints", func() {
			BeforeEach(func() {
				provider := fixtures.NewValidAWSProvider(name, fixtures.NewValidAWSProviderSecret(namespace, name))
				all := fixtures.NewPolicy("all")
				all.Spec.Constraints = &terraformv1alpha1.Constraints{}
				all.Spec.Constraints.Modules = &terraformv1alpha1.ModuleConstraint{Allowed: []string{"default.*"}}

				allow := fixtures.NewPolicy("allow")
				allow.Spec.Constraints = &terraformv1alpha1.Constraints{}
				allow.Spec.Constraints.Modules = &terraformv1alpha1.ModuleConstraint{Allowed: []string{"allow.*"}}

				Expect(cc.Create(ctx, all)).To(Succeed())
				Expect(cc.Create(ctx, allow)).To(Succeed())
				Expect(cc.Create(ctx, provider)).To(Succeed())
			})

			It("should fail with a constraint violation", func() {
				configuration := fixtures.NewValidBucketConfiguration(namespace, "test")
				configuration.Spec.Module = "allow"

				Expect(cc.Delete(ctx, fixtures.NewPolicy("allow"))).To(Succeed())

				warnings, err := v.ValidateCreate(ctx, fixtures.NewValidBucketConfiguration(namespace, "test"))
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(Equal("spec.module: source has been denied by module policy, contact an administrator"))
				Expect(warnings).To(BeEmpty())
			})

			It("should be allowed by the second policy", func() {
				configuration := fixtures.NewValidBucketConfiguration(namespace, "test")
				configuration.Spec.Module = "allow"
				warnings, err := v.ValidateCreate(ctx, configuration)

				Expect(err).ToNot(HaveOccurred())
				Expect(warnings).To(BeEmpty())
			})
		})

		Context("provider namespace selectors do not match", func() {
			BeforeEach(func() {
				provider := fixtures.NewValidAWSProvider(name, fixtures.NewValidAWSProviderSecret(namespace, name))
				provider.Spec.Selector = &terraformv1alpha1.Selector{
					Namespace: &metav1.LabelSelector{
						MatchLabels: map[string]string{
							"does_not_match": "true",
						},
					},
				}
				Expect(cc.Create(ctx, provider)).To(Succeed())
			})

			It("should deny the creation of the configuration", func() {
				warnings, err := v.ValidateCreate(ctx, fixtures.NewValidBucketConfiguration(namespace, "test"))
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("configuration has been denied by the provider policy"))
				Expect(warnings).To(BeEmpty())
			})

			It("should deny the update of the configuration", func() {
				warnings, err := v.ValidateUpdate(ctx, nil, fixtures.NewValidBucketConfiguration(namespace, "test"))
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("configuration has been denied by the provider policy"))
				Expect(warnings).To(BeEmpty())
			})
		})

		Context("provider namespace selectors do match", func() {
			BeforeEach(func() {
				provider := fixtures.NewValidAWSProvider(name, fixtures.NewValidAWSProviderSecret(namespace, name))
				provider.Spec.Selector = &terraformv1alpha1.Selector{
					Namespace: &metav1.LabelSelector{
						MatchLabels: map[string]string{
							"name": namespace,
						},
					},
				}
				Expect(cc.Create(ctx, provider)).To(Succeed())
			})

			It("should allow the creation of the configuration", func() {
				warnings, err := v.ValidateCreate(ctx, fixtures.NewValidBucketConfiguration(namespace, "test"))
				Expect(err).ToNot(HaveOccurred())
				Expect(warnings).To(BeEmpty())
			})

			It("should allow the update of the configuration", func() {
				warnings, err := v.ValidateUpdate(ctx, nil, fixtures.NewValidBucketConfiguration(namespace, "test"))
				Expect(err).ToNot(HaveOccurred())
				Expect(warnings).To(BeEmpty())
			})
		})

		Context("provider resource selectors do not match", func() {
			BeforeEach(func() {
				provider := fixtures.NewValidAWSProvider(name, fixtures.NewValidAWSProviderSecret(namespace, name))
				provider.Spec.Selector = &terraformv1alpha1.Selector{
					Resource: &metav1.LabelSelector{
						MatchLabels: map[string]string{
							"does_not_match": "true",
						},
					},
				}
				Expect(cc.Create(ctx, provider)).To(Succeed())
			})

			It("should deny the creation of the configuration", func() {
				warnings, err := v.ValidateCreate(ctx, fixtures.NewValidBucketConfiguration(namespace, "test"))
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("configuration has been denied by the provider policy"))
				Expect(warnings).To(BeEmpty())
			})

			It("should deny the update of the configuration", func() {
				warnings, err := v.ValidateUpdate(ctx, nil, fixtures.NewValidBucketConfiguration(namespace, "test"))
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("configuration has been denied by the provider policy"))
				Expect(warnings).To(BeEmpty())
			})
		})

		Context("provider resource selectors match", func() {
			BeforeEach(func() {
				provider := fixtures.NewValidAWSProvider(name, fixtures.NewValidAWSProviderSecret(namespace, name))
				provider.Spec.Selector = &terraformv1alpha1.Selector{
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

				warnings, err := v.ValidateCreate(ctx, configuration)
				Expect(err).To(Succeed())
				Expect(warnings).To(BeEmpty())
			})

			It("should allow the update of the configuration", func() {
				configuration := fixtures.NewValidBucketConfiguration(namespace, "test")
				configuration.Labels = map[string]string{"does_match": "true"}

				warnings, err := v.ValidateUpdate(ctx, nil, configuration)
				Expect(err).To(Succeed())
				Expect(warnings).To(BeEmpty())
			})
		})

		Context("versioning is disabled on configurations", func() {
			BeforeEach(func() {
				v.enableVersions = false
				Expect(cc.Create(ctx, fixtures.NewValidAWSReadyProvider(name, fixtures.NewValidAWSProviderSecret(namespace, name)))).To(Succeed())
			})

			It("should be denied due to versioning", func() {
				configuration := fixtures.NewValidBucketConfiguration(namespace, "test")
				configuration.Spec.TerraformVersion = "bad"

				warnings, err := v.ValidateCreate(ctx, configuration)
				Expect(err).ToNot(Succeed())
				Expect(err.Error()).To(Equal("spec.terraformVersion changes have been disabled"))
				Expect(warnings).To(BeEmpty())
			})
		})
	})

	When("deleting a configuration", func() {
		It("should not error", func() {
			warnings, err := v.ValidateDelete(ctx, fixtures.NewValidBucketConfiguration(namespace, "test"))
			Expect(err).ToNot(HaveOccurred())
			Expect(warnings).To(BeEmpty())
		})
	})
})
