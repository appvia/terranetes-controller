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

package drift

import (
	"context"
	"fmt"
	"io/ioutil"
	"testing"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/sirupsen/logrus"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	corev1alphav1 "github.com/appvia/terranetes-controller/pkg/apis/core/v1alpha1"
	terraformv1alphav1 "github.com/appvia/terranetes-controller/pkg/apis/terraform/v1alpha1"
	"github.com/appvia/terranetes-controller/pkg/controller"
	"github.com/appvia/terranetes-controller/pkg/schema"
	controllertests "github.com/appvia/terranetes-controller/test"
	"github.com/appvia/terranetes-controller/test/fixtures"
)

func TestReconcile(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Running Test Suite")
}

var _ = Describe("Drift Controller", func() {
	logrus.SetOutput(ioutil.Discard)

	ctx := context.TODO()
	namespace := "default"

	When("configuration is reconciled", func() {
		cases := []struct {
			Name        string
			Before      func(ctrl *Controller)
			Check       func(configuration *terraformv1alphav1.Configuration)
			ShouldDrift bool
		}{
			{
				Name: "drift detection is not enabled",
				Check: func(configuration *terraformv1alphav1.Configuration) {
					configuration.Spec.EnableDriftDetection = false
				},
			},
			{
				Name: "configuration is deleting",
				Check: func(configuration *terraformv1alphav1.Configuration) {
					now := metav1.NewTime(time.Now())
					configuration.DeletionTimestamp = &now
					configuration.Finalizers = []string{"do-not-delete"}
				},
			},
			{
				Name: "terraform plan has not been run yet",
				Check: func(configuration *terraformv1alphav1.Configuration) {
					cond := configuration.Status.GetCondition(terraformv1alphav1.ConditionTerraformPlan)
					cond.Reason = corev1alphav1.ReasonNotDetermined
					cond.Status = metav1.ConditionFalse

					cond = configuration.Status.GetCondition(terraformv1alphav1.ConditionTerraformApply)
					cond.Reason = corev1alphav1.ReasonNotDetermined
					cond.Status = metav1.ConditionFalse

				},
			},
			{
				Name: "terraform apply has not been run yet",
				Check: func(configuration *terraformv1alphav1.Configuration) {
					cond := configuration.Status.GetCondition(terraformv1alphav1.ConditionTerraformPlan)
					cond.Reason = corev1alphav1.ReasonComplete
					cond.Status = metav1.ConditionTrue

					cond = configuration.Status.GetCondition(terraformv1alphav1.ConditionTerraformApply)
					cond.Reason = corev1alphav1.ReasonNotDetermined
					cond.Status = metav1.ConditionFalse
				},
			},
			{
				Name: "terraform plan has failed",
				Check: func(configuration *terraformv1alphav1.Configuration) {
					cond := configuration.Status.GetCondition(terraformv1alphav1.ConditionTerraformPlan)
					cond.Reason = corev1alphav1.ReasonError
					cond.Status = metav1.ConditionFalse
				},
			},
			{
				Name: "terraform apply has failed",
				Check: func(configuration *terraformv1alphav1.Configuration) {
					cond := configuration.Status.GetCondition(terraformv1alphav1.ConditionTerraformApply)
					cond.Reason = corev1alphav1.ReasonError
					cond.Status = metav1.ConditionFalse
				},
			},
			{
				Name: "terraform plan in progress",
				Check: func(configuration *terraformv1alphav1.Configuration) {
					cond := configuration.Status.GetCondition(terraformv1alphav1.ConditionTerraformPlan)
					cond.Reason = corev1alphav1.ReasonInProgress
					cond.Status = metav1.ConditionFalse
				},
			},
			{
				Name: "terraform apply in progress",
				Check: func(configuration *terraformv1alphav1.Configuration) {
					cond := configuration.Status.GetCondition(terraformv1alphav1.ConditionTerraformApply)
					cond.Reason = corev1alphav1.ReasonInProgress
					cond.Status = metav1.ConditionFalse
				},
			},
			{
				Name: "terraform plan occurred recently",
				Check: func(configuration *terraformv1alphav1.Configuration) {
					cond := configuration.Status.GetCondition(terraformv1alphav1.ConditionTerraformPlan)
					cond.Reason = corev1alphav1.ReasonComplete
					cond.LastTransitionTime = metav1.NewTime(time.Now())
					cond.Status = metav1.ConditionTrue
				},
			},
			{
				Name: "terraform apply occurred recently",
				Check: func(configuration *terraformv1alphav1.Configuration) {
					cond := configuration.Status.GetCondition(terraformv1alphav1.ConditionTerraformPlan)
					cond.Reason = corev1alphav1.ReasonComplete
					cond.LastTransitionTime = metav1.NewTime(time.Now().Add(-24 * time.Hour))
					cond.Status = metav1.ConditionTrue

					cond = configuration.Status.GetCondition(terraformv1alphav1.ConditionTerraformApply)
					cond.Reason = corev1alphav1.ReasonComplete
					cond.LastTransitionTime = metav1.NewTime(time.Now().Add(-5 * time.Minute))
					cond.Status = metav1.ConditionTrue
				},
			},
			{
				Name: "we have multiple configuration in drift already",
				Before: func(ctrl *Controller) {
					for i := 0; i < 20; i++ {
						configuration := fixtures.NewValidBucketConfiguration(namespace, fmt.Sprintf("test%d-config", i))
						configuration.Annotations = map[string]string{terraformv1alphav1.DriftAnnotation: "true"}
						configuration.Spec.EnableDriftDetection = true

						controller.EnsureConditionsRegistered(terraformv1alphav1.DefaultConfigurationConditions, configuration)
						cond := configuration.Status.GetCondition(terraformv1alphav1.ConditionTerraformPlan)
						cond.Reason = corev1alphav1.ReasonComplete
						cond.LastTransitionTime = metav1.NewTime(time.Now().Add(-5 * time.Hour))
						cond.ObservedGeneration = configuration.GetGeneration()
						cond.Status = metav1.ConditionTrue

						cond = configuration.Status.GetCondition(terraformv1alphav1.ConditionTerraformApply)
						cond.Reason = corev1alphav1.ReasonComplete
						cond.LastTransitionTime = metav1.NewTime(time.Now().Add(-5 * time.Hour))
						cond.ObservedGeneration = configuration.GetGeneration()
						cond.Status = metav1.ConditionTrue

						ctrl.cc.Create(ctx, configuration)
					}
				},
			},
			{
				Name:        "configuration should trigger a drift detection",
				ShouldDrift: true,
			},
		}

		for _, c := range cases {
			When(c.Name, func() {
				events := &controllertests.FakeRecorder{}
				ctrl := &Controller{
					CheckInterval:  5 * time.Minute,
					DriftInterval:  2 * time.Hour,
					DriftThreshold: 0.2,
					cc:             fake.NewFakeClientWithScheme(schema.GetScheme()),
					recorder:       events,
				}

				configuration := fixtures.NewValidBucketConfiguration(namespace, "test")
				configuration.Spec.EnableDriftDetection = true
				controller.EnsureConditionsRegistered(terraformv1alphav1.DefaultConfigurationConditions, configuration)

				// @step: set the conditions to true
				conditions := []corev1alphav1.ConditionType{
					terraformv1alphav1.ConditionTerraformPlan,
					terraformv1alphav1.ConditionTerraformApply,
				}
				for _, name := range conditions {
					cond := configuration.Status.GetCondition(name)
					cond.Reason = corev1alphav1.ReasonComplete
					cond.LastTransitionTime = metav1.NewTime(time.Now().Add(-5 * time.Hour))
					cond.ObservedGeneration = configuration.GetGeneration()
					cond.Status = metav1.ConditionTrue
				}

				cond := configuration.Status.GetCondition(corev1alphav1.ConditionReady)
				cond.Reason = corev1alphav1.ReasonReady
				cond.LastTransitionTime = metav1.NewTime(time.Now().Add(-5 * time.Hour))
				cond.ObservedGeneration = configuration.GetGeneration()
				cond.Status = metav1.ConditionTrue

				if c.Before != nil {
					c.Before(ctrl)
				}
				if c.Check != nil {
					c.Check(configuration)
				}
				Expect(ctrl.cc.Create(ctx, configuration)).To(Succeed())

				It("should not return an error", func() {
					result, _, rerr := controllertests.Roll(ctx, ctrl, configuration, 1)

					Expect(rerr).To(BeNil())
					Expect(result.RequeueAfter).To(Equal(ctrl.CheckInterval))
				})

				switch c.ShouldDrift {
				case true:
					It("should have a drift detection annotation", func() {
						Expect(ctrl.cc.Get(ctx, configuration.GetNamespacedName(), configuration)).ToNot(HaveOccurred())
						Expect(configuration.GetAnnotations()).ToNot(BeEmpty())
						Expect(configuration.GetAnnotations()[terraformv1alphav1.DriftAnnotation]).ToNot(BeEmpty())
					})

					It("should have raised a event indicating the trigger", func() {
						Expect(events.Events).ToNot(BeEmpty())
						Expect(events.Events).To(HaveLen(1))
						Expect(events.Events[0]).To(Equal("(default/test) Normal DriftDetection: Triggered drift detection on configuration"))
					})

				default:
					It("should not have a drift annotation", func() {
						Expect(ctrl.cc.Get(ctx, configuration.GetNamespacedName(), configuration)).ToNot(HaveOccurred())
						Expect(configuration.GetAnnotations()).To(BeEmpty())
					})
				}
			})
		}
	})
})
