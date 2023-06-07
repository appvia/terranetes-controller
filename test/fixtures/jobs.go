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

package fixtures

import (
	"fmt"

	batchv1 "k8s.io/api/batch/v1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	terraformv1alpha1 "github.com/appvia/terranetes-controller/pkg/apis/terraform/v1alpha1"
)

// NewRunningPreloadJob returns a new running preload job
func NewRunningPreloadJob(namespace, provider string) *batchv1.Job {
	job := &batchv1.Job{}
	job.Name = fmt.Sprintf("preload-%s", "test")
	job.Namespace = namespace
	job.Labels = map[string]string{
		terraformv1alpha1.PreloadJobLabel:      "true",
		terraformv1alpha1.PreloadProviderLabel: provider,
	}
	job.Status.Active = 1
	job.Status.Failed = 0
	job.Status.Conditions = []batchv1.JobCondition{
		{
			Type:   batchv1.JobComplete,
			Status: v1.ConditionFalse,
		},
		{
			Type:   batchv1.JobFailed,
			Status: v1.ConditionFalse,
		},
	}

	return job
}

// NewCompletedPreloadJob returns a new running preload job
func NewCompletedPreloadJob(namespace, provider string) *batchv1.Job {
	job := &batchv1.Job{}
	job.Name = fmt.Sprintf("preload-%s", "test")
	job.Namespace = namespace
	job.Labels = map[string]string{
		terraformv1alpha1.PreloadJobLabel:      "true",
		terraformv1alpha1.PreloadProviderLabel: provider,
	}
	job.Status.Active = 0
	job.Status.Failed = 0
	job.Status.Conditions = []batchv1.JobCondition{
		{
			Type:   batchv1.JobFailed,
			Status: v1.ConditionFalse,
		},
		{
			Type:   batchv1.JobComplete,
			Status: v1.ConditionTrue,
		},
	}

	return job
}

// NewFailedPreloadJob returns a new running preload job
func NewFailedPreloadJob(namespace, provider string) *batchv1.Job {
	job := &batchv1.Job{}
	job.Name = fmt.Sprintf("preload-%s", "test")
	job.Namespace = namespace
	job.Labels = map[string]string{
		terraformv1alpha1.PreloadJobLabel:      "true",
		terraformv1alpha1.PreloadProviderLabel: provider,
	}
	job.Status.Active = 0
	job.Status.Failed = 1
	job.Status.Conditions = []batchv1.JobCondition{
		{
			Type:   batchv1.JobFailed,
			Status: v1.ConditionTrue,
		},
		{
			Type:   batchv1.JobComplete,
			Status: v1.ConditionFalse,
		},
	}

	return job
}

// NewTerraformJob returns a new terraform job
func NewTerraformJob(configuration *terraformv1alpha1.Configuration, namespace, stage string) *batchv1.Job {
	job := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      configuration.Name + "-" + stage + "-1234",
			Namespace: namespace,
			Labels: map[string]string{
				terraformv1alpha1.ConfigurationNameLabel:       configuration.Name,
				terraformv1alpha1.ConfigurationNamespaceLabel:  configuration.Namespace,
				terraformv1alpha1.ConfigurationGenerationLabel: fmt.Sprintf("%d", configuration.Generation),
				terraformv1alpha1.ConfigurationUIDLabel:        string(configuration.UID),
				terraformv1alpha1.ConfigurationStageLabel:      stage,
			},
		},
		Spec: batchv1.JobSpec{},
	}

	if configuration.GetAnnotations()[terraformv1alpha1.DriftAnnotation] != "" {
		job.Labels[terraformv1alpha1.DriftAnnotation] = configuration.GetAnnotations()[terraformv1alpha1.DriftAnnotation]
	}

	return job
}

// NewCompletedTerraformJob returns a new completed terraform job
func NewCompletedTerraformJob(configuration *terraformv1alpha1.Configuration, stage string) *batchv1.Job {
	job := NewTerraformJob(configuration, configuration.Namespace, stage)
	job.Status.Active = 0
	job.Status.Failed = 0
	job.Status.Conditions = []batchv1.JobCondition{
		{
			Type:   batchv1.JobFailed,
			Status: v1.ConditionFalse,
		},
		{
			Type:   batchv1.JobComplete,
			Status: v1.ConditionTrue,
		},
	}

	return job
}

// NewFailedTerraformJob returns a new failed terraform job
func NewFailedTerraformJob(configuration *terraformv1alpha1.Configuration, stage string) *batchv1.Job {
	job := NewTerraformJob(configuration, configuration.Namespace, stage)
	job.Status.Conditions = []batchv1.JobCondition{
		{
			Type:   batchv1.JobFailed,
			Status: v1.ConditionTrue,
		},
		{
			Type:   batchv1.JobComplete,
			Status: v1.ConditionFalse,
		},
	}

	return job
}

// NewRunningTerraformJob returns a running terraform job
func NewRunningTerraformJob(configuration *terraformv1alpha1.Configuration, stage string) *batchv1.Job {
	job := NewTerraformJob(configuration, configuration.Namespace, stage)
	job.Status.Active = 1
	job.Status.Failed = 0
	job.Status.Conditions = []batchv1.JobCondition{
		{
			Type:   batchv1.JobComplete,
			Status: v1.ConditionFalse,
		},
		{
			Type:   batchv1.JobFailed,
			Status: v1.ConditionFalse,
		},
	}

	return job
}
