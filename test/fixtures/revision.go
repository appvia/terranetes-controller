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

package fixtures

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/utils/ptr"

	corev1alpha1 "github.com/appvia/terranetes-controller/pkg/apis/core/v1alpha1"
	terraformv1alpha1 "github.com/appvia/terranetes-controller/pkg/apis/terraform/v1alpha1"
	"github.com/appvia/terranetes-controller/pkg/controller"
)

// NewPlan returns a new Plan object
func NewPlan(name string, revisions ...*terraformv1alpha1.Revision) *terraformv1alpha1.Plan {
	plan := terraformv1alpha1.NewPlan(name)
	controller.EnsureConditionsRegistered(terraformv1alpha1.DefaultPlanConditions, plan)
	plan.Status.GetCondition(corev1alpha1.ConditionReady).Status = metav1.ConditionTrue

	for _, revision := range revisions {
		plan.Spec.Revisions = append(plan.Spec.Revisions, terraformv1alpha1.PlanRevision{
			Name:     revision.Name,
			Revision: revision.Spec.Plan.Revision,
		})
	}

	return plan
}

// NewCloudResource returns a new CloudResource object
func NewCloudResource(namespace, name string) *terraformv1alpha1.CloudResource {
	cr := terraformv1alpha1.NewCloudResource(namespace, name)
	controller.EnsureConditionsRegistered(terraformv1alpha1.DefaultCloudResourceConditions, cr)

	return cr
}

// NewCloudResourceWithRevision returns a new CloudResource object
func NewCloudResourceWithRevision(namespace, name string, revision *terraformv1alpha1.Revision) *terraformv1alpha1.CloudResource {
	cr := terraformv1alpha1.NewCloudResource(namespace, name)
	controller.EnsureConditionsRegistered(terraformv1alpha1.DefaultCloudResourceConditions, cr)

	cr.Spec.Plan.Name = revision.Spec.Plan.Name
	cr.Spec.Plan.Revision = revision.Spec.Plan.Revision

	return cr
}

// NewAWSBucketRevision returns a new Revision object
func NewAWSBucketRevision(name string) *terraformv1alpha1.Revision {
	revision := terraformv1alpha1.NewRevision(name)
	revision.Spec.Plan.Name = "bucket"
	revision.Spec.Plan.Revision = "1.0.0"
	revision.Spec.Plan.Categories = []string{"aws", "s3", "bucket"}
	revision.Spec.Plan.Description = "Creates an S3 bucket"
	revision.Spec.Configuration = NewValidBucketConfiguration("test", "test").Spec
	revision.Spec.Dependencies = []terraformv1alpha1.RevisionDependency{
		{
			Provider: &terraformv1alpha1.RevisionProviderDependency{
				Cloud: "aws",
			},
		},
	}
	revision.Spec.Inputs = []terraformv1alpha1.RevisionInput{
		{
			Key:         "bucket_name",
			Description: "The name of the bucket",
			Required:    ptr.To(true),
			Default: &runtime.RawExtension{
				Raw: []byte(`{"value": "test"}`),
			},
		},
	}

	return revision
}
