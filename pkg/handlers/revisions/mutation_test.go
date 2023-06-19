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

package revisions

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	terraformv1alpha1 "github.com/appvia/terranetes-controller/pkg/apis/terraform/v1alpha1"
	"github.com/appvia/terranetes-controller/pkg/schema"
	"github.com/appvia/terranetes-controller/test/fixtures"
)

var _ = Describe("Revision Mutation", func() {
	var handler *mutator
	var revision *terraformv1alpha1.Revision
	var cc client.Client
	var err error

	BeforeEach(func() {
		cc = fake.NewClientBuilder().WithScheme(schema.GetScheme()).Build()
		handler = &mutator{cc: cc}
	})

	When("creating a revision", func() {
		BeforeEach(func() {
			revision = fixtures.NewAWSBucketRevision("test")
			revision.Labels = map[string]string{"app": "test"}

			err = handler.Default(context.Background(), revision)
		})

		It("should not error", func() {
			Expect(err).ToNot(HaveOccurred())
		})

		It("should have a revision label mutated", func() {
			Expect(revision.Labels).To(HaveKeyWithValue(
				terraformv1alpha1.RevisionPlanNameLabel, revision.Spec.Plan.Name),
			)
			Expect(revision.Labels).To(HaveKeyWithValue(
				terraformv1alpha1.RevisionNameLabel, revision.Spec.Plan.Revision),
			)
			Expect(revision.Labels).To(HaveKeyWithValue("app", "test"))
		})
	})
})
