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

package jobs

import (
	"bytes"
	"crypto/sha256"
	"encoding/base32"
	"encoding/json"
	"fmt"
	"strings"

	batchv1 "k8s.io/api/batch/v1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/yaml"

	terraformv1alpha1 "github.com/appvia/terranetes-controller/pkg/apis/terraform/v1alpha1"
	"github.com/appvia/terranetes-controller/pkg/utils"
	"github.com/appvia/terranetes-controller/pkg/utils/terraform"
)

// DefaultServiceAccount is the default service account to use for the job if no override is given
const DefaultServiceAccount = "terranetes-executor"

// TerraformContainerName is the default name for the main terraform container
const TerraformContainerName = "terraform"

// Options is the configuration for the render
type Options struct {
	// AdditionalJobAnnotations are additional annotations added to the job
	AdditionalJobAnnotations map[string]string
	// AdditionalJobSecret is a collection of secrets which should be mounted into the job.
	AdditionalJobSecrets []string
	// AdditionalJobLabels are additional labels added to the job
	AdditionalJobLabels map[string]string
	// BackoffLimit is the number of times we are willing to allow a job to fail
	// before we give up
	BackoffLimit int
	// BinaryPath is the name of the binary to use to run the terraform commands
	BinaryPath string
	// DefaultExecutorMemoryRequest is the default memory request for the executor
	DefaultExecutorMemoryRequest string
	// DefaultExecutorMemoryLimit is the default memory limit for the executor
	DefaultExecutorMemoryLimit string
	// DefaultExecutorCPURequest is the default CPU request for the executor
	DefaultExecutorCPURequest string
	// DefaultExecutorCPULimit is the default CPU limit for the executor
	DefaultExecutorCPULimit string
	// EnableInfraCosts is the flag to enable cost analysis
	EnableInfraCosts bool
	// ExecutorImage is the image to use for the terraform jobs
	ExecutorImage string
	// ExecutorSecrets is a list of additional secrets to add to the job
	ExecutorSecrets []string
	// InfracostsImage is the image to use for infracosts
	InfracostsImage string
	// InfracostsSecret is the name of the secret contain the infracost token and url
	InfracostsSecret string
	// Namespace is the location of the jobs
	Namespace string
	// PolicyConstraint is a matching constraint for this policy
	PolicyConstraint *terraformv1alpha1.PolicyConstraint
	// PolicyImage is image to use for checkov
	PolicyImage string
	// SaveTerraformState indicates we should save the terraform state in a secret
	SaveTerraformState bool
	// Template is the source for the job template if overridden by the controller
	Template []byte
	// Image is the image to use for the terraform jobs
	Image string
}

// Render is responsible for rendering the terraform configuration
type Render struct {
	// configuration is the configuration that we are rendering
	configuration *terraformv1alpha1.Configuration
	// provider is the provider that we are rendering
	provider *terraformv1alpha1.Provider
}

// New returns a new render job
func New(configuration *terraformv1alpha1.Configuration, provider *terraformv1alpha1.Provider) *Render {
	return &Render{
		configuration: configuration,
		provider:      provider,
	}
}

// NewJobWatch is responsible for creating a job watch pod
func (r *Render) NewJobWatch(namespace, stage string, executorImage string) *batchv1.Job {
	query := []string{
		"generation=" + fmt.Sprintf("%d", r.configuration.GetGeneration()),
		"name=" + r.configuration.Name,
		"namespace=" + r.configuration.Namespace,
		"stage=" + stage,
		"uid=" + string(r.configuration.UID),
	}

	endpoint := fmt.Sprintf("http://controller.%s.svc.cluster.local/v1/builds/%s/%s/logs?%s",
		namespace, r.configuration.Namespace, r.configuration.Name, strings.Join(query, "&"))

	job := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("%s-%d-%s", r.configuration.Name, r.configuration.GetGeneration(), stage),
			Namespace: r.configuration.Namespace,
			Labels: utils.MergeStringMaps(r.configuration.Labels, map[string]string{
				terraformv1alpha1.ConfigurationGenerationLabel: fmt.Sprintf("%d", r.configuration.GetGeneration()),
				terraformv1alpha1.ConfigurationNameLabel:       r.configuration.Name,
				terraformv1alpha1.ConfigurationNamespaceLabel:  r.configuration.Namespace,
				terraformv1alpha1.ConfigurationStageLabel:      stage,
				terraformv1alpha1.ConfigurationUIDLabel:        string(r.configuration.GetUID()),
			}),
			OwnerReferences: []metav1.OwnerReference{
				*metav1.NewControllerRef(r.configuration, r.configuration.GroupVersionKind()),
			},
		},
		Spec: batchv1.JobSpec{
			BackoffLimit:            ptr.To(int32(0)),
			Completions:             ptr.To(int32(1)),
			Parallelism:             ptr.To(int32(1)),
			TTLSecondsAfterFinished: ptr.To(int32(3600)),
			Template: v1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: utils.MergeStringMaps(r.configuration.GetLabels(), map[string]string{
						terraformv1alpha1.ConfigurationGenerationLabel: fmt.Sprintf("%d", r.configuration.GetGeneration()),
						terraformv1alpha1.ConfigurationNameLabel:       r.configuration.Name,
						terraformv1alpha1.ConfigurationStageLabel:      stage,
						terraformv1alpha1.ConfigurationUIDLabel:        string(r.configuration.GetUID()),
					}),
				},
				Spec: v1.PodSpec{
					RestartPolicy: v1.RestartPolicyNever,
					SecurityContext: &v1.PodSecurityContext{
						RunAsGroup:   ptr.To(int64(65534)),
						RunAsNonRoot: ptr.To(true),
						RunAsUser:    ptr.To(int64(65534)),
					},
					Containers: []v1.Container{
						{
							Name:            "watch",
							Image:           executorImage,
							ImagePullPolicy: v1.PullIfNotPresent,
							Command:         []string{"/watch_logs.sh"},
							Args:            []string{"-e", endpoint},
							SecurityContext: &v1.SecurityContext{
								AllowPrivilegeEscalation: ptr.To(false),
								Capabilities:             &v1.Capabilities{Drop: []v1.Capability{"ALL"}},
								Privileged:               ptr.To(false),
							},
						},
					},
				},
			},
		},
	}

	if r.configuration.HasRetryableAnnotation() {
		job.Labels[terraformv1alpha1.RetryAnnotation] = r.configuration.GetAnnotations()[terraformv1alpha1.RetryAnnotation]
		job.Spec.Template.Labels[terraformv1alpha1.RetryAnnotation] = r.configuration.GetAnnotations()[terraformv1alpha1.RetryAnnotation]
	}

	return job
}

// NewTerraformPlan is responsible for creating a batch job to run terraform plan
func (r *Render) NewTerraformPlan(options Options) (*batchv1.Job, error) {
	return r.createTerraformFromTemplate(options, terraformv1alpha1.StageTerraformPlan)
}

// NewTerraformApply is responsible for creating a batch job to run terraform apply
func (r *Render) NewTerraformApply(options Options) (*batchv1.Job, error) {
	return r.createTerraformFromTemplate(options, terraformv1alpha1.StageTerraformApply)
}

// NewTerraformDestroy is responsible for creating a batch job to run terraform destroy
func (r *Render) NewTerraformDestroy(options Options) (*batchv1.Job, error) {
	return r.createTerraformFromTemplate(options, terraformv1alpha1.StageTerraformDestroy)
}

// createTerraformFromTemplate is used to render the terraform job from the parameters and the template
func (r *Render) createTerraformFromTemplate(options Options, stage string) (*batchv1.Job, error) {
	var arguments string

	if r.configuration.Spec.HasVariables() {
		arguments = fmt.Sprintf("--var-file %s", terraformv1alpha1.TerraformVariablesConfigMapKey)
	}
	if r.configuration.Spec.TFVars != "" {
		arguments = fmt.Sprintf("--var-file %s %s", terraformv1alpha1.TerraformTFVarsConfigMapKey, arguments)
	}

	params := map[string]interface{}{
		"GenerateName": fmt.Sprintf("%s-%s-", r.configuration.Name, stage),
		"Namespace":    options.Namespace,
		"Annotations":  options.AdditionalJobAnnotations,
		"Labels": utils.MergeStringMaps(options.AdditionalJobLabels, map[string]string{
			terraformv1alpha1.ConfigurationGenerationLabel: fmt.Sprintf("%d", r.configuration.GetGeneration()),
			terraformv1alpha1.ConfigurationNameLabel:       r.configuration.GetName(),
			terraformv1alpha1.ConfigurationNamespaceLabel:  r.configuration.GetNamespace(),
			terraformv1alpha1.ConfigurationStageLabel:      stage,
			terraformv1alpha1.ConfigurationUIDLabel:        string(r.configuration.GetUID()),
		}),
		"BinaryPath":                   options.BinaryPath,
		"DefaultExecutorMemoryRequest": options.DefaultExecutorMemoryRequest,
		"DefaultExecutorMemoryLimit":   options.DefaultExecutorMemoryLimit,
		"DefaultExecutorCPURequest":    options.DefaultExecutorCPURequest,
		"DefaultExecutorCPULimit":      options.DefaultExecutorCPULimit,
		"Provider": map[string]interface{}{
			"Name":           r.provider.Name,
			"Namespace":      r.provider.Namespace,
			"SecretRef":      r.provider.Spec.SecretRef,
			"ServiceAccount": ptr.Deref(r.provider.Spec.ServiceAccount, ""),
			"Source":         string(r.provider.Spec.Source),
		},
		"EnableInfraCosts":       options.EnableInfraCosts,
		"EnableVariables":        r.configuration.Spec.HasVariables(),
		"EnableTFVars":           r.configuration.Spec.TFVars != "",
		"ExecutorSecrets":        options.ExecutorSecrets,
		"ImagePullPolicy":        "IfNotPresent",
		"Policy":                 options.PolicyConstraint,
		"SaveTerraformState":     options.SaveTerraformState,
		"ServiceAccount":         DefaultServiceAccount,
		"Stage":                  stage,
		"TerraformArguments":     arguments,
		"TerraformContainerName": TerraformContainerName,
		"Configuration": map[string]interface{}{
			"Generation": fmt.Sprintf("%d", r.configuration.GetGeneration()),
			"Module":     r.configuration.Spec.Module,
			"Name":       r.configuration.Name,
			"Namespace":  r.configuration.Namespace,
			"UUID":       string(r.configuration.GetUID()),
			"Variables":  r.configuration.Spec.Variables,
		},
		"Images": map[string]interface{}{
			"Executor":   options.ExecutorImage,
			"Infracosts": options.InfracostsImage,
			"Image":      options.Image,
			"Policy":     options.PolicyImage,
		},
		"Secrets": map[string]interface{}{
			"AdditionalSecrets": options.AdditionalJobSecrets,
			"Config":            r.configuration.GetTerraformConfigSecretName(),
			"Infracosts":        options.InfracostsSecret,
			"InfracostsReport":  r.configuration.GetTerraformCostSecretName(),
			"PolicyReport":      r.configuration.GetTerraformPolicySecretName(),
			"TerraformPlanJSON": r.configuration.GetTerraformPlanJSONSecretName(),
			"TerraformPlanOut":  r.configuration.GetTerraformPlanOutSecretName(),
			"TerraformState":    r.configuration.GetTerraformStateSecretName(),
		},
	}

	// @step: create the template and render
	render, err := terraform.Template(string(options.Template), params)
	if err != nil {
		return nil, err
	}

	// @step: parse into a batch job
	encoded, err := yaml.YAMLToJSON(render)
	if err != nil {
		return nil, err
	}

	job := &batchv1.Job{}
	if err := json.NewDecoder(bytes.NewReader(encoded)).Decode(job); err != nil {
		return nil, err
	}

	return job, nil
}

func TemplateHash(data []byte) (string, error) {
	hash := sha256.New()
	_, err := hash.Write(data)
	if err != nil {
		return "", err
	}
	return strings.ToLower(base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString(hash.Sum(nil))), nil
}
