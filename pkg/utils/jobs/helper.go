package jobs

import (
	batchv1 "k8s.io/api/batch/v1"
	v1 "k8s.io/api/core/v1"
)

// IsFailed returns true of the job has failed
func IsFailed(job *batchv1.Job) bool {
	for _, condition := range job.Status.Conditions {
		if condition.Type == batchv1.JobFailed && condition.Status == v1.ConditionTrue {
			return true
		}
	}

	return false
}

// IsComplete returns true if the job has succeeded
func IsComplete(job *batchv1.Job) bool {
	for _, condition := range job.Status.Conditions {
		if condition.Type == batchv1.JobComplete && condition.Status == v1.ConditionTrue {
			return true
		}
	}

	return false
}

// IsActive returns true if the job is active
func IsActive(job *batchv1.Job) bool {
	return !IsComplete(job) && !IsFailed(job)
}
