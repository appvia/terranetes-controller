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

package provider

import (
	"context"
	"io/ioutil"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/sirupsen/logrus"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	corev1alphav1 "github.com/appvia/terranetes-controller/pkg/apis/core/v1alpha1"
	terraformv1alphav1 "github.com/appvia/terranetes-controller/pkg/apis/terraform/v1alpha1"
	"github.com/appvia/terranetes-controller/pkg/schema"
	controllertests "github.com/appvia/terranetes-controller/test"
	"github.com/appvia/terranetes-controller/test/fixtures"
)

func TestReconcile(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Running Test Suite")
}

var _ = Describe("Provider Controller", func() {
	logrus.SetOutput(ioutil.Discard)

	var cc client.Client
	var result reconcile.Result
	var rerr error
	var controller *Controller
	var provider *terraformv1alphav1.Provider

	validProvider := func() *terraformv1alphav1.Provider {
		return fixtures.NewValidAWSProvider("aws", fixtures.NewValidAWSProviderSecret("default", "aws"))
	}

	validProviderSecret := func() *v1.Secret {
		return fixtures.NewValidAWSProviderSecret("default", "aws")
	}

	When("provider secret is missing", func() {
		BeforeEach(func() {
			provider = validProvider()
			cc = fake.NewFakeClientWithScheme(schema.GetScheme(), provider)
			controller = &Controller{cc: cc}

			result, _, rerr = controllertests.Roll(context.TODO(), controller, provider, 3)
		})

		It("should have the conditions", func() {
			Expect(cc.Get(context.TODO(), provider.GetNamespacedName(), provider)).ToNot(HaveOccurred())

			Expect(provider.Status.Conditions).To(HaveLen(1))
		})

		It("should indicate the secret is missing", func() {
			Expect(cc.Get(context.TODO(), provider.GetNamespacedName(), provider)).ToNot(HaveOccurred())

			Expect(provider.Status.Conditions).To(HaveLen(1))
			Expect(provider.Status.Conditions[0].Type).To(Equal(corev1alphav1.ConditionReady))
			Expect(provider.Status.Conditions[0].Status).To(Equal(metav1.ConditionFalse))
			Expect(provider.Status.Conditions[0].Reason).To(Equal(corev1alphav1.ReasonActionRequired))
			Expect(provider.Status.Conditions[0].Message).To(Equal("Provider secret (default/aws) not found"))
		})

		It("should not requeue", func() {
			Expect(rerr).To(BeNil())
			Expect(result).To(Equal(reconcile.Result{}))
		})
	})

	When("aws provider secret is invalid", func() {
		BeforeEach(func() {
			provider = validProvider()
			secret := validProviderSecret()
			secret.Data = map[string][]byte{
				"AWS_ACCESS_KEY_ID": []byte("test"),
			}

			cc = fake.NewFakeClientWithScheme(schema.GetScheme(), provider, secret)
			controller = &Controller{cc: cc}
			result, _, rerr = controllertests.Roll(context.TODO(), controller, provider, 3)
		})

		It("should have the conditions", func() {
			Expect(cc.Get(context.TODO(), provider.GetNamespacedName(), provider)).ToNot(HaveOccurred())
			Expect(provider.Status.Conditions).To(HaveLen(1))
		})

		It("should indicate the provider is ready", func() {
			Expect(cc.Get(context.TODO(), provider.GetNamespacedName(), provider)).ToNot(HaveOccurred())
			Expect(provider.Status.Conditions).To(HaveLen(1))
			Expect(provider.Status.Conditions[0].Type).To(Equal(corev1alphav1.ConditionReady))
			Expect(provider.Status.Conditions[0].Status).To(Equal(metav1.ConditionFalse))
			Expect(provider.Status.Conditions[0].Reason).To(Equal(corev1alphav1.ReasonActionRequired))
			Expect(provider.Status.Conditions[0].Message).To(Equal("provider secret (default/aws) missing aws secrets"))
		})

		It("should not requeue", func() {
			Expect(rerr).To(BeNil())
			Expect(result).To(Equal(reconcile.Result{}))
		})
	})

	When("the cloud provider is not supported", func() {
		BeforeEach(func() {
			provider = validProvider()
			provider.Spec.Provider = "not-supported"
			secret := validProviderSecret()

			cc = fake.NewFakeClientWithScheme(schema.GetScheme(), provider, secret)
			controller = &Controller{cc: cc}
			result, _, rerr = controllertests.Roll(context.TODO(), controller, provider, 3)
		})

		It("should have the conditions", func() {
			Expect(cc.Get(context.TODO(), provider.GetNamespacedName(), provider)).ToNot(HaveOccurred())
			Expect(provider.Status.Conditions).To(HaveLen(1))
		})

		It("should indicate the provider is ready", func() {
			Expect(cc.Get(context.TODO(), provider.GetNamespacedName(), provider)).ToNot(HaveOccurred())
			Expect(provider.Status.Conditions).To(HaveLen(1))
			Expect(provider.Status.Conditions[0].Type).To(Equal(corev1alphav1.ConditionReady))
			Expect(provider.Status.Conditions[0].Status).To(Equal(metav1.ConditionFalse))
			Expect(provider.Status.Conditions[0].Reason).To(Equal(corev1alphav1.ReasonActionRequired))
			Expect(provider.Status.Conditions[0].Message).To(Equal("Provider type: not-supported is not supported"))
		})

		It("should not requeue", func() {
			Expect(rerr).To(BeNil())
			Expect(result).To(Equal(reconcile.Result{}))
		})
	})

	When("provider is setup correctly", func() {
		BeforeEach(func() {
			provider = validProvider()
			secret := validProviderSecret()

			cc = fake.NewFakeClientWithScheme(schema.GetScheme(), provider, secret)
			controller = &Controller{cc: cc}
			result, _, rerr = controllertests.Roll(context.TODO(), controller, provider, 3)
		})

		It("should have the conditions", func() {
			Expect(cc.Get(context.TODO(), provider.GetNamespacedName(), provider)).ToNot(HaveOccurred())
			Expect(provider.Status.Conditions).To(HaveLen(1))
		})

		It("should indicate the provider is ready", func() {
			Expect(cc.Get(context.TODO(), provider.GetNamespacedName(), provider)).ToNot(HaveOccurred())
			Expect(provider.Status.Conditions).To(HaveLen(1))
			Expect(provider.Status.Conditions[0].Type).To(Equal(corev1alphav1.ConditionReady))
			Expect(provider.Status.Conditions[0].Status).To(Equal(metav1.ConditionTrue))
			Expect(provider.Status.Conditions[0].Reason).To(Equal(corev1alphav1.ReasonReady))
			Expect(provider.Status.Conditions[0].Message).To(Equal("Resource ready"))
		})

		It("should not requeue", func() {
			Expect(rerr).To(BeNil())
			Expect(result).To(Equal(reconcile.Result{}))
		})

	})
})
