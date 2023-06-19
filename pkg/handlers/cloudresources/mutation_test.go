/*
 * Copyright (C) 2023  Appvia Ltd <info@appvia.io>
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

package cloudresources

import (
	"context"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
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
	var cloudresource *terraformv1alpha1.CloudResource
	var plan *terraformv1alpha1.Plan
	var cc client.Client
	var err error

	When("creating a cloud resource", func() {
		BeforeEach(func() {
			cloudresource = fixtures.NewCloudResource("default", "test")
			cloudresource.Spec.Plan = terraformv1alpha1.PlanReference{
				Name:     "test",
				Revision: "v0.0.1",
			}
			cloudresource.Spec.ProviderRef = &terraformv1alpha1.ProviderReference{Name: "aws"}

			plan = fixtures.NewPlan(cloudresource.Spec.Plan.Name)
			plan.Status.Latest.Version = "v1.0.0"

			cc = fake.NewClientBuilder().WithScheme(schema.GetScheme()).Build()
			handler = &mutator{cc: cc}

			Expect(cc.Create(context.Background(), plan)).To(Succeed())
		})

		It("should have the labels mutated", func() {
			err = handler.Default(context.Background(), cloudresource)
			Expect(err).ToNot(HaveOccurred())
			Expect(cloudresource.Labels).To(HaveKeyWithValue("terraform.appvia.io/plan", "test"))
			Expect(cloudresource.Labels).To(HaveKeyWithValue("terraform.appvia.io/revision", "v0.0.1"))
		})

		Context("when no provider is specified", func() {
			BeforeEach(func() {
				secret := fixtures.NewValidAWSProviderSecret("terraform-system", "aws")
				provider := fixtures.NewValidAWSReadyProvider("default", secret)
				provider.Annotations = map[string]string{
					terraformv1alpha1.DefaultProviderAnnotation: "true",
				}
				cloudresource.Spec.ProviderRef = nil

				Expect(cc.Create(context.Background(), secret)).To(Succeed())
				Expect(cc.Create(context.Background(), provider)).To(Succeed())

				err = handler.Default(context.Background(), cloudresource)
			})

			It("should have injected the provider", func() {
				Expect(err).ToNot(HaveOccurred())
				Expect(cloudresource.Spec.ProviderRef).ToNot(BeNil())
				Expect(cloudresource.Spec.ProviderRef.Name).To(Equal("default"))
			})
		})

		Context("when the revision is not defined", func() {
			BeforeEach(func() {
				cloudresource.Spec.Plan.Revision = ""
			})

			Context("when the plan is not found", func() {
				BeforeEach(func() {
					Expect(cc.Delete(context.Background(), plan)).To(Succeed())

					err = handler.Default(context.Background(), cloudresource)
				})

				It("should return an error", func() {
					Expect(err).To(HaveOccurred())
					Expect(err.Error()).To(Equal("spec.plan.name resource \"test\" not found"))
				})
			})

			Context("when the plan does not have a latest version", func() {
				BeforeEach(func() {
					plan.Status.Latest.Version = ""
					Expect(cc.Update(context.Background(), plan)).To(Succeed())

					err = handler.Default(context.Background(), cloudresource)
				})

				It("should return an error", func() {
					Expect(err).To(HaveOccurred())
					Expect(err.Error()).To(Equal("spec.plan.name resource \"test\" does not have a latest revision"))
				})
			})

			Context("when the plan has a latest version", func() {
				It("should have the revision mutated", func() {
					err = handler.Default(context.Background(), cloudresource)
					Expect(err).ToNot(HaveOccurred())
					Expect(cloudresource.Spec.Plan.Revision).To(Equal("v1.0.0"))
				})
			})
		})
	})
})
