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
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	terraformv1alpha1 "github.com/appvia/terranetes-controller/pkg/apis/terraform/v1alpha1"
	"github.com/appvia/terranetes-controller/pkg/schema"
	"github.com/appvia/terranetes-controller/pkg/utils"
	"github.com/appvia/terranetes-controller/test/fixtures"
)

func TestReconcile(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Running Test Suite")
}

var _ = Describe("Revision Validation", func() {
	ctx := context.Background()
	var err error
	var cc client.Client
	var v *validator
	var revision *terraformv1alpha1.Revision
	var warnings admission.Warnings

	BeforeEach(func() {
		cc = fake.NewClientBuilder().WithScheme(schema.GetScheme()).Build()
		v = &validator{cc: cc}
	})

	When("creating a revision", func() {
		BeforeEach(func() {
			revision = fixtures.NewAWSBucketRevision("test")
		})

		It("should fail when no plan name", func() {
			revision.Spec.Plan.Name = ""
			warnings, err := v.ValidateCreate(ctx, revision)
			Expect(err).To(HaveOccurred())
			Expect(warnings).To(BeEmpty())
			Expect(err.Error()).To(Equal("spec.plan.name is required"))
		})

		It("should fail when no plan description", func() {
			revision.Spec.Plan.Description = ""

			warnings, err := v.ValidateCreate(ctx, revision)
			Expect(err).To(HaveOccurred())
			Expect(warnings).To(BeEmpty())
			Expect(err.Error()).To(Equal("spec.plan.description is required"))
		})

		It("should fail when no plan version", func() {
			revision.Spec.Plan.Revision = ""

			warnings, err := v.ValidateCreate(ctx, revision)
			Expect(err).To(HaveOccurred())
			Expect(warnings).To(BeEmpty())
			Expect(err.Error()).To(Equal("spec.plan.version is required"))
		})

		It("should fail when invalid semver", func() {
			revision.Spec.Plan.Revision = "BAD"

			warnings, err := v.ValidateCreate(ctx, revision)
			Expect(err).To(HaveOccurred())
			Expect(warnings).To(BeEmpty())
			Expect(err.Error()).To(Equal("spec.plan.version is not a valid semver"))
		})

		It("should fail if dependencies added by nothing to depend on", func() {
			revision.Spec.Dependencies = []terraformv1alpha1.RevisionDependency{{}}

			warnings, err := v.ValidateCreate(ctx, revision)
			Expect(err).To(HaveOccurred())
			Expect(warnings).To(BeEmpty())
			Expect(err.Error()).To(Equal("spec.plan.dependencies[0] is missing a context, provider or terranetes"))
		})

		It("should fail when context dependency name not set", func() {
			revision.Spec.Dependencies = []terraformv1alpha1.RevisionDependency{{
				Context: &terraformv1alpha1.RevisionContextDependency{
					Name: "",
				},
			}}

			warnings, err := v.ValidateCreate(ctx, revision)
			Expect(err).To(HaveOccurred())
			Expect(warnings).To(BeEmpty())
			Expect(err.Error()).To(Equal("spec.plan.dependencies[0].context.name is required"))
		})

		It("should fail when terranetes dependency version not set", func() {
			revision.Spec.Dependencies = []terraformv1alpha1.RevisionDependency{{
				Terranetes: &terraformv1alpha1.RevisionTerranetesDependency{
					Version: "",
				},
			}}

			warnings, err := v.ValidateCreate(ctx, revision)
			Expect(err).To(HaveOccurred())
			Expect(warnings).To(BeEmpty())
			Expect(err.Error()).To(Equal("spec.plan.dependencies[0].terranetes.version is required"))
		})

		It("should fail when provider dependency cloud not set", func() {
			revision.Spec.Dependencies = []terraformv1alpha1.RevisionDependency{{
				Provider: &terraformv1alpha1.RevisionProviderDependency{
					Cloud: "",
				},
			}}

			warnings, err := v.ValidateCreate(ctx, revision)
			Expect(err).To(HaveOccurred())
			Expect(warnings).To(BeEmpty())
			Expect(err.Error()).To(Equal("spec.plan.dependencies[0].provider.cloud is required"))
		})

		It("should fail if input name not set", func() {
			revision.Spec.Inputs = []terraformv1alpha1.RevisionInput{{
				Key: "", Description: "test",
			}}

			warnings, err := v.ValidateCreate(ctx, revision)
			Expect(err).To(HaveOccurred())
			Expect(warnings).To(BeEmpty())
			Expect(err.Error()).To(Equal("spec.plan.inputs[0].key is required"))
		})

		It("should fail if inputs description not set", func() {
			revision.Spec.Inputs = []terraformv1alpha1.RevisionInput{{
				Key: "test", Description: "",
			}}

			warnings, err := v.ValidateCreate(ctx, revision)
			Expect(err).To(HaveOccurred())
			Expect(warnings).To(BeEmpty())
			Expect(err.Error()).To(Equal("spec.plan.inputs[0].description is required"))
		})

		It("should success if inputs type not set", func() {
			revision.Spec.Inputs = []terraformv1alpha1.RevisionInput{{
				Key: "test", Description: "test",
			}}

			warnings, err := v.ValidateCreate(ctx, revision)
			Expect(err).To(Succeed())
			Expect(warnings).To(BeEmpty())
		})

		It("should if inputs value does not contain anything", func() {
			revision.Spec.Inputs = []terraformv1alpha1.RevisionInput{{
				Key: "test", Description: "test", Default: &runtime.RawExtension{},
			}}

			warnings, err := v.ValidateCreate(ctx, revision)
			Expect(err).To(HaveOccurred())
			Expect(warnings).To(BeEmpty())
			Expect(err.Error()).To(Equal("spec.plan.inputs[0].default.value is required"))
		})

		It("should fail if inputs value does not contain field", func() {
			revision.Spec.Inputs = []terraformv1alpha1.RevisionInput{{
				Key: "test", Description: "test", Default: &runtime.RawExtension{
					Raw: []byte(`{"test": "test"}`),
				},
			}}

			warnings, err := v.ValidateCreate(ctx, revision)
			Expect(err).To(HaveOccurred())
			Expect(warnings).To(BeEmpty())
			Expect(err.Error()).To(Equal("spec.plan.inputs[0].default.value is required"))
		})

		It("should not fail when field is defined", func() {
			revision.Spec.Inputs = []terraformv1alpha1.RevisionInput{{
				Key: "test", Description: "test", Default: &runtime.RawExtension{
					Raw: []byte(`{"value": "test"}`),
				},
			}}
			warnings, err := v.ValidateCreate(ctx, revision)
			Expect(err).To(Succeed())
			Expect(warnings).To(BeEmpty())
		})
	})

	When("update protection is enabled", func() {
		var cloudresource *terraformv1alpha1.CloudResource

		BeforeEach(func() {
			cloudresource = fixtures.NewCloudResource("default", "test")
			cloudresource.Labels = map[string]string{
				terraformv1alpha1.CloudResourcePlanNameLabel: revision.Spec.Plan.Name,
				terraformv1alpha1.CloudResourceRevisionLabel: revision.Spec.Plan.Revision,
			}
			cloudresource.Spec.Plan.Name = revision.Spec.Plan.Name
			cloudresource.Spec.Plan.Revision = revision.Spec.Plan.Revision
			Expect(cc.Create(ctx, cloudresource)).To(Succeed())

			v.EnableUpdateProtection = true
		})

		Context("but no cloud resource is referencing the revision", func() {
			BeforeEach(func() {
				cloudresource.Labels = map[string]string{
					terraformv1alpha1.CloudResourcePlanNameLabel: revision.Spec.Plan.Name,
					terraformv1alpha1.CloudResourceRevisionLabel: "another_revision",
				}
				Expect(cc.Update(ctx, cloudresource)).To(Succeed())
				warnings, err = v.ValidateUpdate(ctx, revision, revision)
			})

			It("should not fail", func() {
				Expect(err).To(Succeed())
				Expect(warnings).To(BeEmpty())
			})
		})

		Context("and a cloud resource references the revision, and a change in the spec", func() {
			var updated *terraformv1alpha1.Revision

			BeforeEach(func() {
				updated = revision.DeepCopy()
				updated.Spec.Configuration.EnableAutoApproval = true
			})

			It("should fail", func() {
				warnings, err = v.ValidateUpdate(ctx, revision, updated)
				Expect(err).ToNot(Succeed())
				Expect(err.Error()).To(Equal("in use by cloudresource/s, update denied (use terraform.appvia.io/revision.skip-update-protection to override)"))
				Expect(warnings).To(BeEmpty())
			})
		})

		Context("and a cloud resource references the revision, but no change to spec", func() {
			var updated *terraformv1alpha1.Revision

			BeforeEach(func() {
				updated = revision.DeepCopy()
				updated.Annotations = utils.MergeStringMaps(updated.Annotations, map[string]string{
					terraformv1alpha1.RevisionSkipUpdateProtectionAnnotation: "true",
				})
			})

			It("should not fail", func() {
				warnings, err = v.ValidateUpdate(ctx, revision, updated)
				Expect(err).To(Succeed())
				Expect(warnings).To(BeEmpty())
			})
		})

		Context("and a cloud resource is referencing the revision but with skip check annotation", func() {
			BeforeEach(func() {
				revision.Annotations = map[string]string{
					terraformv1alpha1.RevisionSkipUpdateProtectionAnnotation: "true",
				}
			})

			It("should not fail", func() {
				warnings, err = v.ValidateUpdate(ctx, revision, revision)
				Expect(err).To(Succeed())
				Expect(warnings).To(BeEmpty())
			})
		})

		Context("and we are creating not updating", func() {
			It("should not fail", func() {
				warnings, err = v.ValidateCreate(ctx, revision)
				Expect(err).To(Succeed())
				Expect(warnings).To(BeEmpty())
			})
		})
	})

	When("the revision already exists", func() {
		BeforeEach(func() {
			revision = fixtures.NewAWSBucketRevision("test")
			another := fixtures.NewAWSBucketRevision("another")
			another1 := fixtures.NewAWSBucketRevision("another1")

			Expect(cc.Create(ctx, another)).To(Succeed())
			Expect(cc.Create(ctx, another1)).To(Succeed())
			Expect(cc.Create(ctx, revision)).To(Succeed())
		})

		It("should fail with an error", func() {
			warnings, err = v.ValidateCreate(ctx, revision)
			Expect(err).ToNot(Succeed())
			Expect(warnings).To(BeEmpty())
		})

		It("should indicate the duplicate revision", func() {
			warnings, err = v.ValidateCreate(ctx, revision)
			Expect(err).ToNot(Succeed())
			Expect(warnings).To(BeEmpty())
			Expect(err.Error()).To(Equal("spec.plan.revision same version already exists on revision/s: another,another1"))
		})
	})

	It("should not fail creating", func() {
		warnings, err = v.ValidateCreate(ctx, revision)
		Expect(err).To(Succeed())
		Expect(warnings).To(BeEmpty())
	})

	It("should not fail updated", func() {
		warnings, err = v.ValidateUpdate(ctx, revision, revision)
		Expect(err).To(Succeed())
		Expect(warnings).To(BeEmpty())
	})
})
