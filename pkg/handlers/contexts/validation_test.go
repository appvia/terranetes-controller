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

package contexts

import (
	"context"
	"fmt"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/runtime"
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

var _ = Describe("Context Validation", func() {
	var c *terraformv1alpha1.Context
	var cc client.Client
	var v *validator
	var err error

	BeforeEach(func() {
		cc = fake.NewClientBuilder().WithScheme(schema.GetScheme()).WithRuntimeObjects(fixtures.NewNamespace("default")).Build()
		v = &validator{cc: cc}
		c = fixtures.NewTerranettesContext("default")
	})

	When("creating or updating a terraform context", func() {
		Context("and there are no variables", func() {
			BeforeEach(func() {
				c.Spec.Variables = nil

				err = v.ValidateCreate(context.Background(), c)
			})

			It("should not return an error", func() {
				Expect(err).To(BeNil())
			})
		})

		Context("and there are variables", func() {
			Context("but the value is empty", func() {
				BeforeEach(func() {
					c.Spec.Variables = map[string]runtime.RawExtension{
						"foo": runtime.RawExtension{},
					}
					err = v.ValidateCreate(context.Background(), c)
				})

				It("should return an error", func() {
					err = v.ValidateCreate(context.Background(), c)
					Expect(err).ToNot(BeNil())
				})

				It("should return an error with the correct message", func() {
					Expect(err.Error()).To(Equal("spec.variable[\"foo\"] must have a value"))
				})
			})

			Context("and the variables are invalid", func() {
				BeforeEach(func() {
					c.Spec.Variables = map[string]runtime.RawExtension{
						"foo": runtime.RawExtension{Raw: []byte("invalid")},
					}

					err = v.ValidateCreate(context.Background(), c)
				})

				It("should return an error", func() {
					Expect(err).ToNot(BeNil())
					Expect(err.Error()).To(Equal("spec.variable[\"foo\"] invalid input"))
				})
			})

			Context("but it does not contain a description", func() {
				BeforeEach(func() {
					c.Spec.Variables = map[string]runtime.RawExtension{
						"foo": runtime.RawExtension{
							Raw: []byte(`{"value": "bar"}`),
						},
					}
					err = v.ValidateCreate(context.Background(), c)
				})

				It("should return an error", func() {
					err = v.ValidateCreate(context.Background(), c)
					Expect(err).ToNot(BeNil())
				})

				It("should return an error with the correct message", func() {
					Expect(err.Error()).To(Equal("spec.variables[\"foo\"].description is required"))
				})
			})

			Context("but it does not contain a value", func() {
				BeforeEach(func() {
					c.Spec.Variables = map[string]runtime.RawExtension{
						"foo": runtime.RawExtension{
							Raw: []byte(`{"description": "bar"}`),
						},
					}
					err = v.ValidateCreate(context.Background(), c)
				})

				It("should return an error", func() {
					err = v.ValidateCreate(context.Background(), c)
					Expect(err).ToNot(BeNil())
				})

				It("should return an error with the correct message", func() {
					Expect(err.Error()).To(Equal("spec.variables[\"foo\"].value is required"))
				})
			})

			Context("and contains all required fields", func() {
				BeforeEach(func() {
					c.Spec.Variables = map[string]runtime.RawExtension{
						"foo": runtime.RawExtension{
							Raw: []byte(`{"description": "bar", "value": "baz"}`),
						},
					}
					err = v.ValidateCreate(context.Background(), c)
				})

				It("should not return an error", func() {
					err = v.ValidateCreate(context.Background(), c)
					Expect(err).To(BeNil())
				})
			})
		})
	})

	When("deleting a context", func() {
		BeforeEach(func() {
			for i := 0; i < 2; i++ {
				cr := fixtures.NewValidBucketConfiguration("default", fmt.Sprintf("test-%d", i))
				cr.Spec.ValueFrom = []terraformv1alpha1.ValueFromSource{
					{
						Context: pointer.String(c.Name),
						Key:     "bar",
						Name:    "baz",
					},
				}
				Expect(cc.Create(context.Background(), cr)).To(Succeed())
			}
			cr := fixtures.NewValidBucketConfiguration("default", "ignore")
			cr.Spec.ValueFrom = []terraformv1alpha1.ValueFromSource{
				{
					Context: pointer.String("ignore"),
					Key:     "bar",
					Name:    "baz",
				},
			}
			Expect(cc.Create(context.Background(), cr)).To(Succeed())
		})

		Context("and the annotation is set", func() {
			BeforeEach(func() {
				c.Annotations = map[string]string{
					terraformv1alpha1.OrphanAnnotation: "true",
				}

				err = v.ValidateDelete(context.Background(), c)
			})

			It("should not return an error", func() {
				Expect(err).To(BeNil())
			})
		})

		Context("and the orphan annotation is not set", func() {

			Context("but we have configurations referencing the context", func() {
				It("should return an error", func() {
					err = v.ValidateDelete(context.Background(), c)
					Expect(err).ToNot(BeNil())
					Expect(err.Error()).To(Equal("resource in use by configuration(s): default/test-0, default/test-1"))
				})
			})

			Context("but we have configurations referencing the context", func() {
				BeforeEach(func() {
					c = fixtures.NewTerranettesContext("not_referenced")
				})

				It("should not return an error", func() {
					err = v.ValidateDelete(context.Background(), c)
					Expect(err).To(BeNil())
				})
			})
		})

		Context("and the orphan annotation is set to true", func() {
			BeforeEach(func() {
				c.Annotations = map[string]string{terraformv1alpha1.OrphanAnnotation: "true"}
				cr := fixtures.NewValidBucketConfiguration("default", "test")
				cr.Spec.ValueFrom = []terraformv1alpha1.ValueFromSource{
					{
						Context: pointer.String(c.Name),
						Key:     "bar",
						Name:    "baz",
					},
				}
				Expect(cc.Create(context.Background(), cr)).To(Succeed())
			})

			It("should not return an error, regardless of reference", func() {
				err = v.ValidateDelete(context.Background(), c)
				Expect(err).To(BeNil())
			})
		})
	})
})
