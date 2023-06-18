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

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	corev1alpha1 "github.com/appvia/terranetes-controller/pkg/apis/core/v1alpha1"
	terraformv1alpha1 "github.com/appvia/terranetes-controller/pkg/apis/terraform/v1alpha1"
	"github.com/appvia/terranetes-controller/pkg/schema"
	"github.com/appvia/terranetes-controller/test/fixtures"
)

var _ = Describe("Checking CloudResource Validation", func() {
	var v *validator
	var cloudresource *terraformv1alpha1.CloudResource
	var plan *terraformv1alpha1.Plan
	var revision *terraformv1alpha1.Revision
	var cc client.Client

	BeforeEach(func() {
		cc = fake.NewClientBuilder().WithScheme(schema.GetScheme()).Build()
		cloudresource = fixtures.NewCloudResource("default", "test")
		cloudresource.Spec.Plan.Name = "test"
		cloudresource.Spec.Plan.Revision = "v0.0.1"
		cloudresource.Spec.WriteConnectionSecretToRef = &terraformv1alpha1.WriteConnectionSecret{
			Name: "test",
		}
		cloudresource.Spec.Auth = &v1.SecretReference{Name: "auth"}

		revision = fixtures.NewAWSBucketRevision("revision")
		revision.Spec.Plan.Name = cloudresource.Spec.Plan.Name
		revision.Spec.Plan.Revision = cloudresource.Spec.Plan.Revision
		plan = fixtures.NewPlan(cloudresource.Spec.Plan.Name, revision)

		Expect(cc.Create(context.Background(), revision)).To(Succeed())
		Expect(cc.Create(context.Background(), plan)).To(Succeed())

		v = &validator{cc: cc}
	})

	When("creating a validator", func() {
		It("should not be nil", func() {
			Expect(v).ToNot(BeNil())
		})
	})

	When("resource not a cloud resource", func() {
		It("should return an error", func() {
			err := v.ValidateCreate(context.Background(), &terraformv1alpha1.Revision{})
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(Equal("expected a CloudResource, but got: *v1alpha1.Revision"))

			err = v.ValidateUpdate(context.Background(), &terraformv1alpha1.Revision{}, &terraformv1alpha1.Revision{})
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(Equal("expected a CloudResource, but got: *v1alpha1.Revision"))
		})
	})

	When("creating a cloud resource", func() {
		It("should fail when no plan name is provided", func() {
			cloudresource.Spec.Plan.Name = ""

			err := v.ValidateCreate(context.Background(), cloudresource)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(Equal("spec.plan.name is required"))
		})

		It("should fail when no plan revision", func() {
			cloudresource.Spec.Plan.Revision = ""

			err := v.ValidateCreate(context.Background(), cloudresource)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(Equal("spec.plan.revision is required"))
		})

		It("should fail when connection secret is empty", func() {
			cloudresource.Spec.WriteConnectionSecretToRef.Name = ""

			err := v.ValidateCreate(context.Background(), cloudresource)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(Equal("spec.writeConnectionSecretToRef.name is required"))

			err = v.ValidateUpdate(context.Background(), cloudresource, cloudresource)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(Equal("spec.writeConnectionSecretToRef.name is required"))
		})

		It("should fail when auth secret is empty", func() {
			cloudresource.Spec.Auth.Name = ""

			err := v.ValidateCreate(context.Background(), cloudresource)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(Equal("spec.auth.name is required"))
		})

		It("should failed when connection secret keys are invalid", func() {
			expected := "spec.writeConnectionSecretToRef.keys[0] contains invalid key: this:is:invalid, should be KEY:NEWNAME"

			cloudresource.Spec.WriteConnectionSecretToRef.Keys = []string{"this:is:invalid"}

			err := v.ValidateCreate(context.Background(), cloudresource)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(Equal(expected))

			err = v.ValidateUpdate(context.Background(), cloudresource, cloudresource)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(Equal(expected))
		})

		It("should fail when plan does not exist", func() {
			Expect(cc.Delete(context.Background(), plan)).To(Succeed())

			err := v.ValidateCreate(context.Background(), cloudresource)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(Equal("spec.plan.name does not exist"))
		})

		It("should fail when plan not ready", func() {
			plan.Status.GetCondition(corev1alpha1.ConditionReady).Status = metav1.ConditionFalse
			Expect(cc.Status().Update(context.Background(), plan)).To(Succeed())

			err := v.ValidateCreate(context.Background(), cloudresource)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(Equal("spec.plan.name is not is a ready state"))
		})

		It("should fail when revision does not exist", func() {
			Expect(cc.Delete(context.Background(), revision)).To(Succeed())

			err := v.ValidateCreate(context.Background(), cloudresource)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(Equal("spec.plan.revision: does not exist"))
		})

		It("should fail when revisions in plan", func() {
			plan.Spec.Revisions = nil
			Expect(cc.Update(context.Background(), plan)).To(Succeed())

			err := v.ValidateCreate(context.Background(), cloudresource)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(Equal("spec.plan.revision: v0.0.1 does not exist in plan"))
		})

		It("should fail when revision not present in plan", func() {
			plan.Spec.Revisions[0].Version = "v0.0.2"
			Expect(cc.Update(context.Background(), plan)).To(Succeed())

			err := v.ValidateCreate(context.Background(), cloudresource)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(Equal("spec.plan.revision: v0.0.1 does not exist in plan"))
		})

		It("should fail when no provider reference and provider not present in revision", func() {
			cloudresource.Spec.ProviderRef = nil
			revision.Spec.Configuration.ProviderRef = nil
			Expect(cc.Update(context.Background(), revision)).To(Succeed())

			err := v.ValidateCreate(context.Background(), cloudresource)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(Equal("spec.providerRef is required"))
		})

		It("should fail when no provider reference name present", func() {
			cloudresource.Spec.ProviderRef = &terraformv1alpha1.ProviderReference{}
			Expect(cc.Update(context.Background(), revision)).To(Succeed())

			err := v.ValidateCreate(context.Background(), cloudresource)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(Equal("spec.providerRef.name is required"))
		})

		Context("and the revision has inputs", func() {
			BeforeEach(func() {
				revision.Spec.Inputs = []terraformv1alpha1.RevisionInput{
					{
						Key:         "database_name",
						Description: "Database name",
						Required:    pointer.Bool(true),
					},
				}
				Expect(cc.Update(context.Background(), revision)).To(Succeed())
			})

			It("should fail cloud resource input not permitted in variables", func() {
				cloudresource.Spec.Variables = &runtime.RawExtension{
					Raw: []byte(`{"not_permitted": "mydb"}`),
				}
				expected := "spec.variables.not_permitted is not permitted by revision: v0.0.1"

				err := v.ValidateCreate(context.Background(), cloudresource)
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(Equal(expected))

				err = v.ValidateUpdate(context.Background(), cloudresource, cloudresource)
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(Equal(expected))
			})

			It("should fail cloud resource input not permitted in value from", func() {
				cloudresource.Spec.ValueFrom = []terraformv1alpha1.ValueFromSource{
					{
						Name:   "my_name",
						Key:    "not_permitted",
						Secret: pointer.String("mysecret"),
					},
				}
				expected := "spec.valueFrom[0] input is not permitted by revision: v0.0.1"

				err := v.ValidateCreate(context.Background(), cloudresource)
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(Equal(expected))

				err = v.ValidateUpdate(context.Background(), cloudresource, cloudresource)
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(Equal(expected))
			})
		})

		It("should not fail", func() {
			err := v.ValidateCreate(context.Background(), cloudresource)
			Expect(err).ToNot(HaveOccurred())

			err = v.ValidateUpdate(context.Background(), cloudresource, cloudresource)
			Expect(err).ToNot(HaveOccurred())
		})
	})
})
