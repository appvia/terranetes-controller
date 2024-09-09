package jobs_test

import (
	"testing"

	batchv1 "k8s.io/api/batch/v1"

	"github.com/appvia/terranetes-controller/pkg/apis/terraform/v1alpha1"
	"github.com/appvia/terranetes-controller/pkg/assets"
	"github.com/appvia/terranetes-controller/pkg/utils/jobs"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewTerraformPlan(t *testing.T) {
	cases := []struct {
		name     string
		conf     *v1alpha1.Configuration
		provider *v1alpha1.Provider
		opts     jobs.Options
		checkJob func(*testing.T, *batchv1.Job)
	}{
		{
			name: "When TFVars specified, variables.tfvars is mounted and included in args",
			conf: &v1alpha1.Configuration{
				Spec: v1alpha1.ConfigurationSpec{
					TFVars: `horse = "strong"`,
				},
			},
			provider: &v1alpha1.Provider{},
			opts: jobs.Options{
				Template: assets.MustAsset("job.yaml.tpl"),
			},
			checkJob: func(t *testing.T, job *batchv1.Job) {
				require.GreaterOrEqual(t, len(job.Spec.Template.Spec.Volumes), 3, "Expected at least 3 volumes")
				require.GreaterOrEqual(t, len(job.Spec.Template.Spec.Volumes[2].Secret.Items), 3, "Expected at least 3 entries in volume mount")
				assert.Equal(t, "variables.tfvars", job.Spec.Template.Spec.Volumes[2].Secret.Items[2].Key)
				assert.Contains(t, job.Spec.Template.Spec.Containers[0].Args[1], "--var-file variables.tfvars")
			},
		},
		{
			name: "When TFVars unspecified, variables.tfvars is not mounted and not included in args",
			conf: &v1alpha1.Configuration{
				Spec: v1alpha1.ConfigurationSpec{},
			},
			provider: &v1alpha1.Provider{},
			opts: jobs.Options{
				Template: assets.MustAsset("job.yaml.tpl"),
			},
			checkJob: func(t *testing.T, job *batchv1.Job) {
				require.GreaterOrEqual(t, len(job.Spec.Template.Spec.Volumes), 3, "Expected at least 3 volumes")
				require.GreaterOrEqual(t, len(job.Spec.Template.Spec.Volumes[2].Secret.Items), 2, "Expected only 2 entries in volume mount")
				assert.NotContains(t, job.Spec.Template.Spec.Containers[0].Args[1], "--var-file variables.tfvars")
			},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			job, err := jobs.New(tc.conf, tc.provider).NewTerraformPlan(tc.opts)
			if err != nil {
				t.Error(err)
				t.Fail()
			}
			if tc.checkJob != nil {
				tc.checkJob(t, job)
			}
		})
	}
}
