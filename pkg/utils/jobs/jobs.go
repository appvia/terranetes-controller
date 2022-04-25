/*
 * Copyright (C) 2022  Rohith Jayawardene <gambol99@gmail.com>
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

package jobs

import (
	"fmt"
	"strings"

	batchv1 "k8s.io/api/batch/v1"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/pointer"

	terraformv1alphav1 "github.com/appvia/terraform-controller/pkg/apis/terraform/v1alpha1"
)

// Options is the configuration for the render
type Options struct {
	// CostAnalysisSecret is the name of the secret contain the infracost token and url
	CostAnalysisSecret string
	// EnableCostAnalysis is the flag to enable cost analysis
	EnableCostAnalysis bool
	// EnableVerification is the flag to enable verification
	EnableVerification bool
	// ExecutorImage is the image to use for the terraform jobs
	ExecutorImage string
	// GitImage is the image to use for the git jobs
	GitImage string
	// Namespace is the location of the jobs
	Namespace string
}

// Render is responsible for rendering the terraform configuration
type Render struct {
	// configuration is the configuration that we are rendering
	configuration *terraformv1alphav1.Configuration
	// provider is the provider that we are rendering
	provider *terraformv1alphav1.Provider
}

// New returns a new render job
func New(configuration *terraformv1alphav1.Configuration, provider *terraformv1alphav1.Provider) *Render {
	return &Render{
		configuration: configuration,
		provider:      provider,
	}
}

// NewJobWatch is responsible for creating a job watch pod
func (r *Render) NewJobWatch(namespace, stage string) *batchv1.Job {
	query := []string{
		"generation=" + fmt.Sprintf("%d", r.configuration.GetGeneration()),
		"name=" + r.configuration.Name,
		"namespace=" + r.configuration.Namespace,
		"stage=" + stage,
		"uid=" + string(r.configuration.UID),
	}

	endpoint := fmt.Sprintf("http://controller.%s.svc.cluster.local/builds?%s", namespace, strings.Join(query, "&"))
	buildLog := "/tmp/build.log"

	return &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("%s-%d-%s", r.configuration.Name, r.configuration.GetGeneration(), stage),
			Namespace: r.configuration.Namespace,
			Labels: map[string]string{
				terraformv1alphav1.ConfigurationGenerationLabel: fmt.Sprintf("%d", r.configuration.GetGeneration()),
				terraformv1alphav1.ConfigurationNameLabel:       r.configuration.Name,
				terraformv1alphav1.ConfigurationNamespaceLabel:  r.configuration.Namespace,
				terraformv1alphav1.ConfigurationStageLabel:      stage,
			},
			OwnerReferences: []metav1.OwnerReference{
				*metav1.NewControllerRef(r.configuration, r.configuration.GroupVersionKind()),
			},
		},
		Spec: batchv1.JobSpec{
			BackoffLimit:            pointer.Int32Ptr(3),
			Completions:             pointer.Int32Ptr(1),
			Parallelism:             pointer.Int32Ptr(1),
			TTLSecondsAfterFinished: pointer.Int32Ptr(3600),
			Template: v1.PodTemplateSpec{
				Spec: v1.PodSpec{
					RestartPolicy: v1.RestartPolicyOnFailure,
					SecurityContext: &v1.PodSecurityContext{
						RunAsGroup:   pointer.Int64(65534),
						RunAsNonRoot: pointer.Bool(true),
						RunAsUser:    pointer.Int64(65534),
					},
					Containers: []v1.Container{
						{
							Name:            "watch",
							Image:           "curlimages/curl:7.82.0",
							ImagePullPolicy: v1.PullIfNotPresent,
							Command:         []string{"sh"},
							Args:            []string{"-c", fmt.Sprintf("curl --no-buffer --silent '%s' | tee -a %[2]s && grep -qi 'build.*complete' %[2]s", endpoint, buildLog)},
							SecurityContext: &v1.SecurityContext{
								AllowPrivilegeEscalation: pointer.Bool(false),
								Capabilities:             &v1.Capabilities{Drop: []v1.Capability{"ALL"}},
								Privileged:               pointer.Bool(false),
							},
						},
					},
				},
			},
		},
	}
}

// NewTerraformPlan is responsible for creating a batch job to run terraform plan
func (r *Render) NewTerraformPlan(options Options) (*batchv1.Job, error) {
	o := r.createTerraformJob(options, terraformv1alphav1.StageTerraformPlan)
	o.Namespace = options.Namespace
	o.GenerateName = fmt.Sprintf("%s-plan-", r.configuration.Name)
	o.Labels[terraformv1alphav1.ConfigurationStageLabel] = terraformv1alphav1.StageTerraformPlan

	return o, nil
}

// NewTerraformApply is responsible for creating a batch job to run terraform apply
func (r *Render) NewTerraformApply(options Options) (*batchv1.Job, error) {
	o := r.createTerraformJob(options, terraformv1alphav1.StageTerraformApply)
	o.GenerateName = fmt.Sprintf("%s-apply-", r.configuration.Name)
	o.Labels[terraformv1alphav1.ConfigurationStageLabel] = terraformv1alphav1.StageTerraformApply

	return o, nil
}

// NewTerraformDestroy is responsible for creating a batch job to run terraform destroy
func (r *Render) NewTerraformDestroy(options Options) (*batchv1.Job, error) {
	o := r.createTerraformJob(options, terraformv1alphav1.StageTerraformDestroy)
	o.Namespace = options.Namespace
	o.GenerateName = fmt.Sprintf("%s-destroy-", r.configuration.Name)
	o.Labels[terraformv1alphav1.ConfigurationStageLabel] = terraformv1alphav1.StageTerraformDestroy

	return o, nil
}

// createTerraformJob is responsible for creating a terraform job from the configuration
func (r *Render) createTerraformJob(options Options, stage string) *batchv1.Job {
	o := &batchv1.Job{}
	o.Namespace = options.Namespace
	o.Labels = map[string]string{
		terraformv1alphav1.ConfigurationGenerationLabel: fmt.Sprintf("%d", r.configuration.GetGeneration()),
		terraformv1alphav1.ConfigurationNameLabel:       r.configuration.GetName(),
		terraformv1alphav1.ConfigurationNamespaceLabel:  r.configuration.GetNamespace(),
		terraformv1alphav1.ConfigurationUID:             string(r.configuration.GetUID()),
	}

	// @step: construct the arguments for the executor
	arguments := []string{fmt.Sprintf("-%s", stage)}
	if options.EnableCostAnalysis {
		arguments = append(arguments, []string{"-enable-costs"}...)
	}
	if options.EnableVerification {
		arguments = append(arguments, []string{"-enable-checkov", "-checkov-url", "bad"}...)
	}
	if r.configuration.HasVariables() {
		arguments = append(arguments, []string{"-var-file", terraformv1alphav1.TerraformVariablesConfigMapKey}...)
	}

	o.Spec = batchv1.JobSpec{
		BackoffLimit: pointer.Int32(3),
		Completions:  pointer.Int32(1),
		Parallelism:  pointer.Int32(1),
		Template: v1.PodTemplateSpec{
			Spec: v1.PodSpec{
				RestartPolicy:      v1.RestartPolicyOnFailure,
				ServiceAccountName: "terraform-executor",
				InitContainers: []v1.Container{
					{
						Name:            "source",
						Image:           options.GitImage,
						ImagePullPolicy: v1.PullIfNotPresent,
						Command:         []string{"sh"},
						Args:            []string{"-c", fmt.Sprintf("/bin/source %s /tmp/source && cp -r /tmp/source/* /data", r.configuration.Spec.Module)},
						VolumeMounts: []v1.VolumeMount{
							{
								Name:      "source",
								MountPath: "/data",
							},
						},
					},
					{
						Name:            "config",
						Image:           options.GitImage,
						ImagePullPolicy: v1.PullIfNotPresent,
						Command:         []string{"sh"},
						Args:            []string{"-c", "cp /tf/config/* /data"},
						VolumeMounts: []v1.VolumeMount{
							{
								Name:      "config",
								MountPath: "/tf/config",
								ReadOnly:  true,
							},
							{
								Name:      "source",
								MountPath: "/data",
							},
						},
					},
					{
						Name:            "init",
						Image:           options.ExecutorImage,
						ImagePullPolicy: v1.PullIfNotPresent,
						Args:            []string{"-init"},
						WorkingDir:      "/data",
						VolumeMounts: []v1.VolumeMount{
							{
								Name:      "source",
								MountPath: "/data",
							},
						},
					},
				},
				Containers: []v1.Container{
					{
						Name:            "terraform",
						Image:           options.ExecutorImage,
						ImagePullPolicy: v1.PullIfNotPresent,
						Args:            arguments,
						SecurityContext: &v1.SecurityContext{
							Capabilities: &v1.Capabilities{
								Drop: []v1.Capability{"ALL"},
							},
							Privileged:   &[]bool{true}[0],
							RunAsNonRoot: &[]bool{true}[0],
							RunAsUser:    &[]int64{65534}[0],
						},
						Env: []v1.EnvVar{
							{
								Name:  "CONFIGURATION_NAME",
								Value: r.configuration.GetName(),
							},
							{
								Name:  "CONFIGURATION_NAMESPACE",
								Value: r.configuration.GetNamespace(),
							},
							{
								Name:  "CONFIGURATION_UID",
								Value: string(r.configuration.GetUID()),
							},
							{
								Name:  "COST_REPORT_NAME",
								Value: r.configuration.GetTerraformCostSecretName(),
							},
							{
								Name: "KUBE_NAMESPACE",
								ValueFrom: &v1.EnvVarSource{
									FieldRef: &v1.ObjectFieldSelector{
										FieldPath: "metadata.namespace",
									},
								},
							},
						},
						WorkingDir: "/data",
						VolumeMounts: []v1.VolumeMount{
							{
								Name:      "source",
								MountPath: "/data",
							},
						},
						Resources: v1.ResourceRequirements{
							Limits: v1.ResourceList{
								v1.ResourceCPU:    resource.MustParse("1"),
								v1.ResourceMemory: resource.MustParse("1Gi"),
							},
							Requests: v1.ResourceList{
								v1.ResourceCPU:    resource.MustParse("5m"),
								v1.ResourceMemory: resource.MustParse("32Mi"),
							},
						},
					},
				},
				Volumes: []v1.Volume{
					{
						Name: "source",
						VolumeSource: v1.VolumeSource{
							EmptyDir: &v1.EmptyDirVolumeSource{},
						},
					},
				},
			},
		},
	}

	// @step: add the volumes to the job - this will always have a backend.tf, but the
	// variables.tf is optional, as is the provider.tf
	items := []v1.KeyToPath{
		{Key: terraformv1alphav1.TerraformBackendConfigMapKey, Path: "backend.tf"},
		{Key: terraformv1alphav1.TerraformProviderConfigMapKey, Path: "provider.tf"},
	}
	// @step: if we have any variables in the configuration, we add the variables.tfvars file
	if r.configuration.HasVariables() {
		items = append(items, v1.KeyToPath{
			Key:  terraformv1alphav1.TerraformVariablesConfigMapKey,
			Path: terraformv1alphav1.TerraformVariablesConfigMapKey,
		})
	}

	// @step: add the volume to the job
	o.Spec.Template.Spec.Volumes = append(o.Spec.Template.Spec.Volumes, v1.Volume{
		Name: "config",
		VolumeSource: v1.VolumeSource{
			ConfigMap: &v1.ConfigMapVolumeSource{
				Items: items,
				LocalObjectReference: v1.LocalObjectReference{
					Name: string(r.configuration.GetUID()),
				},
			},
		},
	})

	// @step: add any additional secrets to the job
	if options.CostAnalysisSecret != "" {
		o.Spec.Template.Spec.Containers[0].EnvFrom = append(o.Spec.Template.Spec.Containers[0].EnvFrom, v1.EnvFromSource{
			SecretRef: &v1.SecretEnvSource{
				LocalObjectReference: v1.LocalObjectReference{
					Name: options.CostAnalysisSecret,
				},
			},
		})
	}

	// @step: add the credentials to the job
	switch r.provider.Spec.Source {
	case terraformv1alphav1.SourceSecret:
		environment := []v1.EnvFromSource{
			{
				SecretRef: &v1.SecretEnvSource{
					LocalObjectReference: v1.LocalObjectReference{
						Name: r.provider.Spec.SecretRef.Name,
					},
				},
			},
		}
		o.Spec.Template.Spec.Containers[0].EnvFrom = append(o.Spec.Template.Spec.Containers[0].EnvFrom, environment...)
		o.Spec.Template.Spec.InitContainers[2].EnvFrom = append(o.Spec.Template.Spec.InitContainers[2].EnvFrom, environment...)

	case terraformv1alphav1.SourceInjected:
		o.Spec.Template.Spec.ServiceAccountName = *r.provider.Spec.ServiceAccount
	}

	return o
}
