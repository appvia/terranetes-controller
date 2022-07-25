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
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	terraformv1alphav1 "github.com/appvia/terranetes-controller/pkg/apis/terraform/v1alpha1"
	"github.com/appvia/terranetes-controller/pkg/schema"
	"github.com/appvia/terranetes-controller/test/fixtures"
)

func TestReconcile(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Running Test Suite")
}

var _ = Describe("Configuration Mutation", func() {
	var policies *terraformv1alphav1.PolicyList
	var m *mutator
	var before, after *terraformv1alphav1.Configuration
	var err error

	ns := &v1.Namespace{}
	ns.Name = "test"
	ns.Labels = map[string]string{"app": "test"}

	JustBeforeEach(func() {
		after = before.DeepCopy()
		b := fake.NewClientBuilder().
			WithRuntimeObjects(ns.DeepCopy(), fixtures.NewNamespace("default")).
			WithScheme(schema.GetScheme())

		if policies != nil {
			for _, x := range policies.Items {
				b = b.WithRuntimeObjects(&x)
			}
		}
		m = &mutator{cc: b.Build()}
		err = m.Default(context.Background(), after)
	})

	When("we have not policies", func() {
		BeforeEach(func() {
			policies = nil
			before = fixtures.NewValidBucketConfiguration("default", "test")
		})

		It("should remain unchanged", func() {
			Expect(before).To(Equal(after))
			Expect(err).ToNot(HaveOccurred())
		})
	})

	When("we have policies but zero matches", func() {
		BeforeEach(func() {
			before = fixtures.NewValidBucketConfiguration("default", "test")
			policies = &terraformv1alphav1.PolicyList{}
			policy := fixtures.NewPolicy("test")
			policy.Spec.Defaults = []terraformv1alphav1.DefaultVariables{
				{
					Selector: terraformv1alphav1.DefaultVariablesSelector{
						Namespace: &metav1.LabelSelector{
							MatchLabels: map[string]string{"app": "no_match"},
						},
					},
					Variables: runtime.RawExtension{
						Raw: []byte(`{"foo": "bar"}`),
					},
				},
			}
			policies.Items = append(policies.Items, *policy)
		})

		It("should remain unchanged", func() {
			Expect(before).To(Equal(after))
			Expect(err).ToNot(HaveOccurred())
		})
	})

	When("we have a match on the namespace selector", func() {
		BeforeEach(func() {
			before = fixtures.NewValidBucketConfiguration("test", "test")
			before.Spec.Variables = &runtime.RawExtension{}

			policies = &terraformv1alphav1.PolicyList{}
			policy := fixtures.NewPolicy("test")
			policy.Spec.Defaults = []terraformv1alphav1.DefaultVariables{
				{
					Selector: terraformv1alphav1.DefaultVariablesSelector{
						Namespace: &metav1.LabelSelector{
							MatchLabels: map[string]string{"app": "test"},
						},
					},
					Variables: runtime.RawExtension{
						Raw: []byte(`{"is": "changed"}`),
					},
				},
			}
			policies.Items = append(policies.Items, *policy)
		})

		It("should have changed", func() {
			Expect(before).ToNot(Equal(after))
			Expect(after.Spec.Variables.Raw).To(Equal([]byte(`{"is":"changed"}`)))
			Expect(err).ToNot(HaveOccurred())
		})
	})

	When("we have a matching namespace policy and existing variables", func() {
		BeforeEach(func() {
			before = fixtures.NewValidBucketConfiguration("test", "test")
			before.Spec.Variables.Raw = []byte(`{"name":"existing"}`)

			policies = &terraformv1alphav1.PolicyList{}
			policy := fixtures.NewPolicy("test")
			policy.Spec.Defaults = []terraformv1alphav1.DefaultVariables{
				{
					Selector: terraformv1alphav1.DefaultVariablesSelector{
						Namespace: &metav1.LabelSelector{
							MatchLabels: map[string]string{"app": "test"},
						},
					},
					Variables: runtime.RawExtension{
						Raw: []byte(`{"foo": "bar"}`),
					},
				},
			}
			policies.Items = append(policies.Items, *policy)
		})

		It("should have changed", func() {
			Expect(before).ToNot(Equal(after))
			Expect(after.Spec.Variables.Raw).To(Equal([]byte(`{"foo":"bar","name":"existing"}`)))
			Expect(err).ToNot(HaveOccurred())
		})
	})

	When("we have a module policy", func() {
		BeforeEach(func() {
			before = fixtures.NewValidBucketConfiguration("test", "test")
			before.Spec.Variables.Raw = []byte(`{"name":"existing"}`)

			policies = &terraformv1alphav1.PolicyList{}
			policy := fixtures.NewPolicy("test")
			policy.Spec.Defaults = []terraformv1alphav1.DefaultVariables{
				{
					Selector: terraformv1alphav1.DefaultVariablesSelector{
						Modules: []string{before.Spec.Module},
					},
					Variables: runtime.RawExtension{
						Raw: []byte(`{"foo": "bar"}`),
					},
				},
			}
			policies.Items = append(policies.Items, *policy)
		})

		It("should have changed", func() {
			Expect(before).ToNot(Equal(after))
			Expect(after.Spec.Variables.Raw).To(Equal([]byte(`{"foo":"bar","name":"existing"}`)))
			Expect(err).ToNot(HaveOccurred())
		})
	})

	When("we have have multiple selectors", func() {
		BeforeEach(func() {
			before = fixtures.NewValidBucketConfiguration("test", "test")
			before.Spec.Variables.Raw = []byte(`{"name":"existing"}`)

			policies = &terraformv1alphav1.PolicyList{}
			policy := fixtures.NewPolicy("test")
			policy.Spec.Defaults = []terraformv1alphav1.DefaultVariables{
				{
					Selector: terraformv1alphav1.DefaultVariablesSelector{
						Modules: []string{before.Spec.Module},
						Namespace: &metav1.LabelSelector{
							MatchLabels: map[string]string{"app": "test"},
						},
					},
					Variables: runtime.RawExtension{
						Raw: []byte(`{"foo": "bar"}`),
					},
				},
			}
			policies.Items = append(policies.Items, *policy)
		})

		It("should have changed", func() {
			Expect(before).ToNot(Equal(after))
			Expect(after.Spec.Variables.Raw).To(Equal([]byte(`{"foo":"bar","name":"existing"}`)))
			Expect(err).ToNot(HaveOccurred())
		})
	})

	When("we have have multiple selectors and does not match", func() {
		BeforeEach(func() {
			before = fixtures.NewValidBucketConfiguration("test", "test")
			before.Spec.Variables.Raw = []byte(`{"name":"existing"}`)

			policies = &terraformv1alphav1.PolicyList{}
			policy := fixtures.NewPolicy("test")
			policy.Spec.Defaults = []terraformv1alphav1.DefaultVariables{
				{
					Selector: terraformv1alphav1.DefaultVariablesSelector{
						Modules: []string{before.Spec.Module},
						Namespace: &metav1.LabelSelector{
							MatchLabels: map[string]string{"app": "no_match"},
						},
					},
					Variables: runtime.RawExtension{
						Raw: []byte(`{"foo": "bar"}`),
					},
				},
			}
			policies.Items = append(policies.Items, *policy)
		})

		It("should have changed", func() {
			Expect(before).To(Equal(after))
			Expect(after.Spec.Variables.Raw).To(Equal([]byte(`{"name":"existing"}`)))
			Expect(err).ToNot(HaveOccurred())
		})
	})
})
