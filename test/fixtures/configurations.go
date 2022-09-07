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

	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"

	terraformv1alphav1 "github.com/appvia/terranetes-controller/pkg/apis/terraform/v1alpha1"
)

// NewValidBucketConfiguration returns a valid configuration for aws bucket
func NewValidBucketConfiguration(namespace, name string) *terraformv1alphav1.Configuration {
	config := &terraformv1alphav1.Configuration{}
	config.Namespace = namespace
	config.UID = types.UID("1234-122-1234-1234")
	config.Name = name
	config.Spec = terraformv1alphav1.ConfigurationSpec{
		Module:      "https://github.com/terraform-aws-modules/terraform-aws-s3-bucket.git",
		ProviderRef: &terraformv1alphav1.ProviderReference{Name: "aws"},
		Variables: &runtime.RawExtension{
			Raw: []byte(`{"name": "test"}`),
		},
		WriteConnectionSecretToRef: &terraformv1alphav1.WriteConnectionSecret{
			Name: "aws-secret",
		},
	}

	return config
}

// NewConfigurationPodWatcher returns a new configuration pod
func NewConfigurationPodWatcher(configuration *terraformv1alphav1.Configuration, stage string) *v1.Pod {
	pod := &v1.Pod{}
	pod.Namespace = configuration.Namespace
	pod.Name = "test-configuration-1234"
	pod.Labels = map[string]string{
		terraformv1alphav1.ConfigurationGenerationLabel: fmt.Sprintf("%d", configuration.GetGeneration()),
		terraformv1alphav1.ConfigurationNameLabel:       configuration.Name,
		terraformv1alphav1.ConfigurationStageLabel:      stage,
		terraformv1alphav1.ConfigurationUIDLabel:        string(configuration.UID),
	}
	pod.Status.Phase = v1.PodSucceeded
	pod.Spec.Containers = []v1.Container{{Name: "terraform"}}

	return pod
}
