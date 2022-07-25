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
	"encoding/json"
	"fmt"
	"strings"
	"text/template"

	batchv1 "k8s.io/api/batch/v1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/yaml"

	terraformv1alphav1 "github.com/appvia/terranetes-controller/pkg/apis/terraform/v1alpha1"
	"github.com/appvia/terranetes-controller/pkg/utils"
)

// DefaultServiceAccount is the default service account to use for the job if no override is given
const DefaultServiceAccount = "terraform-executor"

// TerraformContainerName is the default name for the main terraform container
const TerraformContainerName = "terraform"

// Options is the configuration for the render
type Options struct {
	// AdditionalLabels are additional labels added to the job
	AdditionalLabels map[string]string
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
	PolicyConstraint *terraformv1alphav1.PolicyConstraint
	// PolicyImage is image to use for checkov
	PolicyImage string
	// Template is the source for the job template if overridden by the controller
	Template []byte
	// TerraformImage is the image to use for the terraform jobs
	TerraformImage string
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

	endpoint := fmt.Sprintf("http://controller.%s.svc.cluster.local/v1/builds/%s/%s/logs?%s",
		namespace, r.configuration.Namespace, r.configuration.Name, strings.Join(query, "&"))

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
				terraformv1alphav1.ConfigurationUIDLabel:        string(r.configuration.GetUID()),
			},
			OwnerReferences: []metav1.OwnerReference{
				*metav1.NewControllerRef(r.configuration, r.configuration.GroupVersionKind()),
			},
		},
		Spec: batchv1.JobSpec{
			BackoffLimit:            pointer.Int32Ptr(5),
			Completions:             pointer.Int32Ptr(1),
			Parallelism:             pointer.Int32Ptr(1),
			TTLSecondsAfterFinished: pointer.Int32Ptr(3600),
			Template: v1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						terraformv1alphav1.ConfigurationGenerationLabel: fmt.Sprintf("%d", r.configuration.GetGeneration()),
						terraformv1alphav1.ConfigurationNameLabel:       r.configuration.Name,
						terraformv1alphav1.ConfigurationStageLabel:      stage,
						terraformv1alphav1.ConfigurationUIDLabel:        string(r.configuration.GetUID()),
					},
				},
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
	return r.createTerraformFromTemplate(options, terraformv1alphav1.StageTerraformPlan)
}

// NewTerraformApply is responsible for creating a batch job to run terraform apply
func (r *Render) NewTerraformApply(options Options) (*batchv1.Job, error) {
	return r.createTerraformFromTemplate(options, terraformv1alphav1.StageTerraformApply)
}

// NewTerraformDestroy is responsible for creating a batch job to run terraform destroy
func (r *Render) NewTerraformDestroy(options Options) (*batchv1.Job, error) {
	return r.createTerraformFromTemplate(options, terraformv1alphav1.StageTerraformDestroy)
}

// createTerraformFromTemplate is used to render the terraform job from the parameters and the template
func (r *Render) createTerraformFromTemplate(options Options, stage string) (*batchv1.Job, error) {
	var arguments string

	if r.configuration.HasVariables() {
		arguments = fmt.Sprintf("--var-file %s", terraformv1alphav1.TerraformVariablesConfigMapKey)
	}

	params := map[string]interface{}{
		"GenerateName": fmt.Sprintf("%s-%s-", r.configuration.Name, stage),
		"Namespace":    options.Namespace,
		"Labels": utils.MergeStringMaps(map[string]string{
			terraformv1alphav1.ConfigurationGenerationLabel: fmt.Sprintf("%d", r.configuration.GetGeneration()),
			terraformv1alphav1.ConfigurationNameLabel:       r.configuration.GetName(),
			terraformv1alphav1.ConfigurationNamespaceLabel:  r.configuration.GetNamespace(),
			terraformv1alphav1.ConfigurationStageLabel:      stage,
			terraformv1alphav1.ConfigurationUIDLabel:        string(r.configuration.GetUID()),
		}, options.AdditionalLabels),
		"Provider": map[string]interface{}{
			"Name":           r.provider.Name,
			"Namespace":      r.provider.Namespace,
			"SecretRef":      r.provider.Spec.SecretRef,
			"ServiceAccount": pointer.StringPtrDerefOr(r.provider.Spec.ServiceAccount, ""),
			"Source":         string(r.provider.Spec.Source),
		},
		"EnableInfraCosts":       options.EnableInfraCosts,
		"EnableVariables":        r.configuration.HasVariables(),
		"ExecutorSecrets":        options.ExecutorSecrets,
		"ImagePullPolicy":        "IfNotPresent",
		"Policy":                 options.PolicyConstraint,
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
			"Terraform":  options.TerraformImage,
			"Policy":     options.PolicyImage,
		},
		"Secrets": map[string]interface{}{
			"Config":           r.configuration.GetTerraformConfigSecretName(),
			"Infracosts":       options.InfracostsSecret,
			"InfracostsReport": r.configuration.GetTerraformCostSecretName(),
			"PolicyReport":     r.configuration.GetTerraformPolicySecretName(),
		},
	}

	// @step: create the template and render
	render := &bytes.Buffer{}
	tmpl, err := template.New("main").Funcs(utils.GetTxtFunc()).Parse(string(options.Template))
	if err != nil {
		return nil, err
	}

	if err = tmpl.Execute(render, params); err != nil {
		return nil, err
	}

	// @step: parse into a batch job
	encoded, err := yaml.YAMLToJSON(render.Bytes())
	if err != nil {
		return nil, err
	}

	job := &batchv1.Job{}
	if err := json.NewDecoder(bytes.NewReader(encoded)).Decode(job); err != nil {
		return nil, err
	}

	return job, nil
}
