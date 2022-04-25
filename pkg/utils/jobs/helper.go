package jobs

import (
	batchv1 "k8s.io/api/batch/v1"
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
