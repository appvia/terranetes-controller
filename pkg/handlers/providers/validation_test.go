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

package providers

import (
	"context"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/utils/pointer"
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

var _ = Describe("Provider Validation", func() {
	ctx := context.Background()
	var cc client.Client
	var v *validator

	namespace := "default"
	name := "test"

	BeforeEach(func() {
		cc = fake.NewClientBuilder().WithScheme(schema.GetScheme()).WithRuntimeObjects(fixtures.NewNamespace(namespace)).Build()
		v = &validator{cc: cc, jobNamespace: namespace}
	})

	When("creating a provider", func() {
		It("should not error when creating a valid provider", func() {
			err := v.ValidateCreate(ctx, fixtures.NewValidAWSProvider(name, fixtures.NewValidAWSProviderSecret(namespace, name)))
			Expect(err).ToNot(HaveOccurred())
		})

		It("should not error when updating a valid provider", func() {
			err := v.ValidateUpdate(ctx, nil, fixtures.NewValidAWSProvider(name, fixtures.NewValidAWSProviderSecret(namespace, name)))
			Expect(err).ToNot(HaveOccurred())
		})
	})

	When("creating a provider with an unknown provider", func() {
		It("should throw error", func() {
			policy := fixtures.NewValidAWSProvider(name, fixtures.NewValidAWSProviderSecret(namespace, name))
			policy.Spec.Provider = "unknown"

			err := v.ValidateCreate(ctx, policy)
			Expect(err).ToNot(HaveOccurred())

			err = v.ValidateUpdate(ctx, nil, policy)
			Expect(err).ToNot(HaveOccurred())
		})
	})

	When("creating a provider with a secret", func() {
		It("should throw error when no secret reference", func() {
			policy := fixtures.NewValidAWSProvider(name, fixtures.NewValidAWSProviderSecret(namespace, name))
			policy.Spec.SecretRef = nil
			msg := "spec.secretRef: secret is required when source is secret"

			err := v.ValidateCreate(ctx, policy)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(Equal(msg))

			err = v.ValidateUpdate(ctx, nil, policy)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(Equal(msg))
		})

		It("should throw error when no secret name in reference", func() {
			policy := fixtures.NewValidAWSProvider(name, fixtures.NewValidAWSProviderSecret(namespace, name))
			policy.Spec.SecretRef.Name = ""
			msg := "spec.secretRef.name: name is required when source is secret"

			err := v.ValidateCreate(ctx, policy)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(Equal(msg))

			err = v.ValidateUpdate(ctx, nil, policy)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(Equal(msg))
		})

		It("should thrown a error when the namespace is not within the job namespace", func() {
			policy := fixtures.NewValidAWSProvider(name, fixtures.NewValidAWSProviderSecret(namespace, name))
			policy.Spec.SecretRef.Namespace = "not_controller_namespace"
			msg := "spec.secretRef.namespace: must be in same namespace as the controller"

			err := v.ValidateCreate(ctx, policy)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(Equal(msg))

			err = v.ValidateUpdate(ctx, nil, policy)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(Equal(msg))
		})

		It("should throw error when no secret namespace in reference", func() {
			policy := fixtures.NewValidAWSProvider(name, fixtures.NewValidAWSProviderSecret(namespace, name))
			policy.Spec.SecretRef.Namespace = ""
			msg := "spec.secretRef.namespace: namespace is required when source is secret"

			err := v.ValidateCreate(ctx, policy)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(Equal(msg))

			err = v.ValidateUpdate(ctx, nil, policy)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(Equal(msg))
		})
	})

	When("creating a provider with a injected identity", func() {
		It("should throw error when no service account", func() {
			policy := fixtures.NewValidAWSProvider(name, fixtures.NewValidAWSProviderSecret(namespace, name))
			policy.Spec.Source = "injected"
			policy.Spec.ServiceAccount = nil
			msg := "spec.serviceAccount: serviceAccount is required when source is injected"

			err := v.ValidateCreate(ctx, policy)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(Equal(msg))

			err = v.ValidateUpdate(ctx, nil, policy)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(Equal(msg))
		})

		It("should not throw an error when service account defined", func() {
			policy := fixtures.NewValidAWSProvider(name, fixtures.NewValidAWSProviderSecret(namespace, name))
			policy.Spec.SecretRef.Name = ""
			policy.Spec.Source = "injected"
			policy.Spec.ServiceAccount = pointer.String(name)

			err := v.ValidateCreate(ctx, policy)
			Expect(err).ToNot(HaveOccurred())
			err = v.ValidateUpdate(ctx, nil, policy)
			Expect(err).ToNot(HaveOccurred())
		})
	})

	When("with a preloading configuration", func() {
		var provider *terraformv1alpha1.Provider

		Context("and the context name is invalid", func() {
			expected := "spec.preload.context: is required"

			BeforeEach(func() {
				provider = fixtures.NewValidAWSProvider("other", fixtures.NewValidAWSProviderSecret(namespace, "other"))
				provider.Spec.Preload = &terraformv1alpha1.PreloadConfiguration{}
			})

			It("should throw an error on creation", func() {
				err := v.ValidateCreate(ctx, provider)
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(Equal(expected))
			})

			It("should throw an error on update", func() {
				err := v.ValidateCreate(ctx, provider)
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(Equal(expected))
			})
		})

		Context("and the cluster name is not defined", func() {
			expected := "spec.preload.cluster: is required"

			BeforeEach(func() {
				provider = fixtures.NewValidAWSProvider("other", fixtures.NewValidAWSProviderSecret(namespace, "other"))
				provider.Spec.Preload = &terraformv1alpha1.PreloadConfiguration{
					Context: "test",
				}
			})

			It("should throw an error on creation", func() {
				err := v.ValidateCreate(ctx, provider)
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(Equal(expected))
			})

			It("should throw an error on update", func() {
				err := v.ValidateCreate(ctx, provider)
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(Equal(expected))
			})
		})

		Context("and the cloud region is not defined", func() {
			expected := "spec.preload.region: is required"

			BeforeEach(func() {
				provider = fixtures.NewValidAWSProvider("other", fixtures.NewValidAWSProviderSecret(namespace, "other"))
				provider.Spec.Preload = &terraformv1alpha1.PreloadConfiguration{
					Context: "test",
					Cluster: "test",
				}
			})

			It("should throw an error on creation", func() {
				err := v.ValidateCreate(ctx, provider)
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(Equal(expected))
			})

			It("should throw an error on update", func() {
				err := v.ValidateCreate(ctx, provider)
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(Equal(expected))
			})
		})
	})

	When("creating a provider with default annotation defined", func() {
		var provider *terraformv1alpha1.Provider

		BeforeEach(func() {
			provider = fixtures.NewValidAWSProvider(name, fixtures.NewValidAWSProviderSecret(namespace, name))
			provider.Annotations = map[string]string{
				terraformv1alpha1.DefaultProviderAnnotation: "true",
			}
		})

		Context("we have not other providers", func() {
			It("should not throw an error on creation", func() {
				err := v.ValidateCreate(ctx, provider)
				Expect(err).ToNot(HaveOccurred())
			})

			It("should not throw an error on update", func() {
				err := v.ValidateUpdate(ctx, nil, provider)
				Expect(err).ToNot(HaveOccurred())
			})
		})

		Context("we have other providers, but none set to default", func() {
			BeforeEach(func() {
				o := fixtures.NewValidAWSProvider("other", fixtures.NewValidAWSProviderSecret(namespace, "other"))
				Expect(cc.Create(ctx, o)).To(Succeed())
			})

			It("should not throw an error on creation", func() {
				err := v.ValidateCreate(ctx, provider)
				Expect(err).ToNot(HaveOccurred())
			})

			It("should not throw an error on update", func() {
				err := v.ValidateUpdate(ctx, nil, provider)
				Expect(err).ToNot(HaveOccurred())
			})
		})

		Context("we have other providers, and one set to default", func() {
			expected := "only one provider can be set as the default provider"

			BeforeEach(func() {
				o := fixtures.NewValidAWSProvider("other", fixtures.NewValidAWSProviderSecret(namespace, "other"))
				o.Annotations = map[string]string{
					terraformv1alpha1.DefaultProviderAnnotation: "true",
				}
				Expect(cc.Create(ctx, o)).To(Succeed())
			})

			It("should throw an error on creation", func() {
				err := v.ValidateCreate(ctx, provider)
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(Equal(expected))
			})

			It("should throw an error on update", func() {
				err := v.ValidateUpdate(ctx, nil, provider)
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(Equal(expected))
			})
		})

	})
})
