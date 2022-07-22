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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	terraformv1alphav1 "github.com/appvia/terranetes-controller/pkg/apis/terraform/v1alpha1"
)

// NewTerraformJob returns a new terraform job
func NewTerraformJob(configuration *terraformv1alphav1.Configuration, namespace, stage string) *batchv1.Job {
	job := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      configuration.Name + "-" + stage + "-1234",
			Namespace: namespace,
			Labels: map[string]string{
				terraformv1alphav1.ConfigurationNameLabel:       configuration.Name,
				terraformv1alphav1.ConfigurationNamespaceLabel:  configuration.Namespace,
				terraformv1alphav1.ConfigurationGenerationLabel: fmt.Sprintf("%d", configuration.Generation),
				terraformv1alphav1.ConfigurationUIDLabel:        string(configuration.UID),
				terraformv1alphav1.ConfigurationStageLabel:      stage,
			},
		},
		Spec: batchv1.JobSpec{},
	}

	if configuration.GetAnnotations()[terraformv1alphav1.DriftAnnotation] != "" {
		job.Labels[terraformv1alphav1.DriftAnnotation] = configuration.GetAnnotations()[terraformv1alphav1.DriftAnnotation]
	}

	return job
}
