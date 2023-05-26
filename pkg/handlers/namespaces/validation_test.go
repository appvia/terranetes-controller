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

package namespaces

import (
	"context"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	terraformv1alpha1 "github.com/appvia/terranetes-controller/pkg/apis/terraform/v1alpha1"
	"github.com/appvia/terranetes-controller/pkg/schema"
	"github.com/appvia/terranetes-controller/test/fixtures"
)

func TestReconcile(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Running Test Suite")
}

var _ = Describe("Namespace Validation", func() {
	ctx := context.Background()
	var configuration *terraformv1alpha1.Configuration
	var cc client.Client
	var v *validator
	var err error
	var warnings admission.Warnings

	BeforeEach(func() {
		cc = fake.NewClientBuilder().WithScheme(schema.GetScheme()).WithRuntimeObjects(fixtures.NewNamespace("default")).Build()
		v = &validator{cc: cc, EnableNamespaceProtection: true}
	})

	When("attempting to delete a namespace", func() {
		Context("with a namespace that has no configuration resources", func() {
			It("should fail", func() {
				warnings, err = v.ValidateDelete(ctx, fixtures.NewNamespace("default"))
				Expect(err).To(Succeed())
				Expect(warnings).To(BeEmpty())
			})
		})

		Context("with a namespace that has configuration resources", func() {
			BeforeEach(func() {
				configuration = fixtures.NewValidBucketConfiguration("default", "test")
			})

			Context("and has provisioned resources", func() {
				BeforeEach(func() {
					Expect(cc.Create(ctx, configuration)).To(Succeed())
				})

				It("should fail due to the namespace being protected", func() {
					warnings, err = v.ValidateDelete(ctx, fixtures.NewNamespace("default"))

					Expect(err).To(HaveOccurred())
					Expect(err.Error()).To(Equal("deletion of namespace default is prevented, ensure Terranetes Configurations are deleted first"))
					Expect(warnings).To(BeEmpty())
				})
			})
		})
	})

	When("creating a namespace", func() {
		It("should succeed", func() {
			warnings, err = v.ValidateCreate(ctx, fixtures.NewNamespace("default"))
			Expect(err).To(Succeed())
			Expect(warnings).To(BeEmpty())
		})
	})

	When("updating a namespace", func() {
		It("should succeed", func() {
			warnings, err = v.ValidateUpdate(ctx, nil, fixtures.NewNamespace("default"))
			Expect(err).To(Succeed())
			Expect(warnings).To(BeEmpty())
		})
	})
})
