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

package filters

import (
	"testing"
	"time"

	batchv1 "k8s.io/api/batch/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/stretchr/testify/assert"

	terraformv1alpha1 "github.com/appvia/terranetes-controller/pkg/apis/terraform/v1alpha1"
)

func TestFilterLatest(t *testing.T) {
	list := batchv1.JobList{
		Items: []batchv1.Job{
			{
				ObjectMeta: metav1.ObjectMeta{
					CreationTimestamp: metav1.Time{Time: time.Now().Add(2 * -time.Hour)},
					Labels: map[string]string{
						terraformv1alpha1.ConfigurationGenerationLabel: "2",
						terraformv1alpha1.ConfigurationStageLabel:      "plan",
					},
				},
			},
			{
				ObjectMeta: metav1.ObjectMeta{
					CreationTimestamp: metav1.Time{Time: time.Now().Add(-time.Hour)},
					Labels: map[string]string{
						terraformv1alpha1.ConfigurationGenerationLabel: "1",
						terraformv1alpha1.ConfigurationStageLabel:      "plan",
					},
				},
			},
		},
	}

	job, found := Jobs(&list).WithStage("plan").Latest()
	assert.True(t, found)
	assert.NotNil(t, job)
	assert.Equal(t, "1", job.Labels[terraformv1alpha1.ConfigurationGenerationLabel])

	job, found = Jobs(&list).WithStage("plan").WithGeneration("3").Latest()
	assert.False(t, found)
	assert.Nil(t, job)
}

func TestFilterList(t *testing.T) {
	list := batchv1.JobList{
		Items: []batchv1.Job{
			{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						terraformv1alpha1.ConfigurationGenerationLabel: "1",
						terraformv1alpha1.ConfigurationStageLabel:      "plan",
					},
				},
			},
			{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						terraformv1alpha1.ConfigurationGenerationLabel: "2",
						terraformv1alpha1.ConfigurationStageLabel:      "plan",
					},
				},
			},
			{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						terraformv1alpha1.ConfigurationGenerationLabel: "2",
						terraformv1alpha1.ConfigurationStageLabel:      "apply",
					},
				},
			},
		},
	}

	one := "1"

	cases := []struct {
		Func     func() (*batchv1.JobList, bool)
		Expected int
	}{
		{
			Func:     Jobs(&list).WithStage("plan").List,
			Expected: 2,
		},
		{
			Func:     Jobs(&list).WithStage("plan").WithGeneration(one).List,
			Expected: 1,
		},
		{
			Func:     Jobs(&list).WithStage("plan").WithGeneration(one).List,
			Expected: 1,
		},
		{
			Func:     Jobs(&list).WithStage("none").WithGeneration(one).List,
			Expected: 0,
		},
		{
			Func:     Jobs(&list).WithGeneration(one).List,
			Expected: 1,
		},
		{
			Func:     Jobs(&list).WithStage("apply").List,
			Expected: 1,
		},
		{
			Func:     Jobs(&list).WithStage("apply").List,
			Expected: 1,
		},
	}
	for _, c := range cases {
		list, ok := c.Func()
		if c.Expected > 0 {
			assert.True(t, ok)
		} else {
			assert.False(t, ok)
		}
		assert.Equal(t, c.Expected, len(list.Items))
	}
}
