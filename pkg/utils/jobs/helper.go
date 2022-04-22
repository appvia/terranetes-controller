package jobs

import (
	"fmt"

	batchv1 "k8s.io/api/batch/v1"

	terraformv1alpha1 "github.com/appvia/terraform-controller/pkg/apis/terraform/v1alpha1"
)

// IsFailed returns true of the job has failed
func IsFailed(job *batchv1.Job) bool {
	switch {
	case job.Status.Failed > 0:
		return true
	case len(job.Status.Conditions) > 0:
		for _, condition := range job.Status.Conditions {
			if condition.Type == batchv1.JobFailed && condition.Status == "True" {
				return true
			}
		}
	}

	return false
}

// IsComplete returns true if the job has succeeded
func IsComplete(job *batchv1.Job) bool {
	return job.Status.Succeeded > 0
}

// IsActive returns true if the job is active
func IsActive(job *batchv1.Job) bool {
	return job.Status.Active > 0 || job.Status.Succeeded == 0 || job.Status.Failed == 0 || len(job.Status.Conditions) == 0
}

// GetTerraformImage returns the terraform image to use
func GetTerraformImage(configuration *terraformv1alpha1.Configuration, image, version string) string {
	if configuration.Spec.Terraform.Version == "" {
		return fmt.Sprintf("%s:%s", image, version)
	}

	return fmt.Sprintf("%s:%s", image, configuration.Spec.Terraform.Version)
}
