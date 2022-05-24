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

package policies

import (
	"context"
	"errors"
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	terraformv1alphav1 "github.com/appvia/terraform-controller/pkg/apis/terraform/v1alpha1"
	"github.com/appvia/terraform-controller/pkg/schema"
	"github.com/appvia/terraform-controller/test/fixtures"
)

func TestReconcile(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Running Test Suite")
}

var _ = Describe("Module Constraints", func() {
	var err error
	var v *validator
	var policy *terraformv1alphav1.Policy
	var cc client.Client

	BeforeEach(func() {
		cc = fake.NewClientBuilder().WithScheme(schema.GetScheme()).Build()
		v = &validator{cc: cc}
		policy = fixtures.NewPolicy("modules")
		policy.Spec.Constraints = &terraformv1alphav1.Constraints{}
		policy.Spec.Constraints.Modules = &terraformv1alphav1.ModuleConstraint{}
		policy.Spec.Constraints.Modules.Selector = &terraformv1alphav1.Selector{}
	})

	When("creating a module with constraints", func() {
		cases := []struct {
			Selector *terraformv1alphav1.Selector
			Expect   error
		}{
			{
				Selector: &terraformv1alphav1.Selector{
					Namespace: &metav1.LabelSelector{
						MatchExpressions: []metav1.LabelSelectorRequirement{
							{Key: ""},
						},
					},
				},
				Expect: errors.New("spec.constraints.modules.selector.namespace is invalid, \"\" is not a valid pod selector operator"),
			},
			{
				Selector: &terraformv1alphav1.Selector{
					Namespace: &metav1.LabelSelector{
						MatchExpressions: []metav1.LabelSelectorRequirement{
							{Key: "KEY", Operator: "BAD"},
						},
					},
				},
				Expect: errors.New("spec.constraints.modules.selector.namespace is invalid, \"BAD\" is not a valid pod selector operator"),
			},
			{
				Selector: &terraformv1alphav1.Selector{
					Resource: &metav1.LabelSelector{
						MatchExpressions: []metav1.LabelSelectorRequirement{
							{Key: "KEY", Operator: "BAD"},
						},
					},
				},
				Expect: errors.New("spec.constraints.modules.selector.resource is invalid, \"BAD\" is not a valid pod selector operator"),
			},
		}

		It("should error on invalid selectors", func() {
			for _, c := range cases {
				policy.Spec.Constraints.Modules.Selector = c.Selector
				err = v.ValidateCreate(context.TODO(), policy)
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(Equal(c.Expect.Error()))
			}
		})

		It("should fail on invalid regex", func() {
			policy.Spec.Constraints.Modules.Allowed = []string{"^^.[]$$"}
			err = v.ValidateCreate(context.TODO(), policy)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("spec.constraints.modules.allowed[0] is invalid"))
		})
	})
})

var _ = Describe("Policy Delete Validation", func() {
	var configurations []*terraformv1alphav1.Configuration
	var policy *terraformv1alphav1.Policy
	var err error

	JustBeforeEach(func() {
		if policy == nil {
			policy = fixtures.NewPolicy("test")
		}
		b := fake.NewClientBuilder().WithScheme(schema.GetScheme()).WithRuntimeObjects(fixtures.NewNamespace("default"))
		for _, x := range configurations {
			b.WithRuntimeObjects(x)
		}

		err = (&validator{cc: b.Build()}).ValidateDelete(context.TODO(), policy)
	})

	When("deleting the policy with no configurations present", func() {
		It("it should delete", func() {
			Expect(err).ToNot(HaveOccurred())
		})
	})

	When("deleting the policy with not configurations using it", func() {
		BeforeEach(func() {
			configurations = []*terraformv1alphav1.Configuration{
				fixtures.NewValidBucketConfiguration("default", "test"),
			}
		})

		It("it should delete", func() {
			Expect(err).ToNot(HaveOccurred())
		})
	})

	When("deleting the policy with configurations using it", func() {
		BeforeEach(func() {
			configuration := fixtures.NewValidBucketConfiguration("default", "test")
			configuration.Annotations = map[string]string{terraformv1alphav1.DefaultVariablesAnnotation: "test"}
			configurations = []*terraformv1alphav1.Configuration{configuration}
		})

		It("should throw a validation error", func() {
			Expect(err).To(HaveOccurred())
		})
	})

	When("deleting the policy with configurations using it but annotation to skip", func() {
		BeforeEach(func() {
			configuration := fixtures.NewValidBucketConfiguration("default", "test")
			configuration.Annotations = map[string]string{terraformv1alphav1.DefaultVariablesAnnotation: "test"}
			configurations = []*terraformv1alphav1.Configuration{configuration}

			policy = fixtures.NewPolicy("test")
			policy.Annotations = map[string]string{terraformv1alphav1.SkipDefaultsValidationCheck: "true"}
		})

		It("should throw a validation error", func() {
			Expect(err).ToNot(HaveOccurred())
		})
	})
})
