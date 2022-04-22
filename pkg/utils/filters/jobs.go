/*
 * Copyright (C) 2022 Rohith Jayawardene <gambol99@gmail.com>
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
	batchv1 "k8s.io/api/batch/v1"

	terraformv1alpha1 "github.com/appvia/terraform-controller/pkg/apis/terraform/v1alpha1"
)

// Filter provides a filter for jobs
type Filter struct {
	namespace  string
	name       string
	generation string
	uid        string
	list       *batchv1.JobList
	stage      string
}

// Jobs providers a filter for jobs
func Jobs(list *batchv1.JobList) *Filter {
	return &Filter{list: list}
}

// WithName filters on the configuration name
func (j *Filter) WithName(name string) *Filter {
	j.name = name

	return j
}

// WithNamespace filters on the configuration namespace
func (j *Filter) WithNamespace(namespace string) *Filter {
	j.namespace = namespace

	return j
}

// WithStage sets the stage for the filter
func (j *Filter) WithStage(stage string) *Filter {
	j.stage = stage

	return j
}

// WithGeneration sets the generation for the filter
func (j *Filter) WithGeneration(generation string) *Filter {
	j.generation = generation

	return j
}

// WithUID filters on the configuration uid
func (j *Filter) WithUID(uid string) *Filter {
	j.uid = uid

	return j
}

// Latest returns the latest job
func (j *Filter) Latest() (*batchv1.Job, bool) {
	list, found := j.List()
	if list == nil || !found {
		return nil, false
	}

	// @step: find the latest item in the list
	latest := &list.Items[0]
	for i := 0; i < len(list.Items); i++ {
		if list.Items[i].CreationTimestamp.After(latest.CreationTimestamp.Time) {
			latest = &list.Items[i]
		}
	}

	return latest, true
}

// List returns the filtered list
func (j *Filter) List() (*batchv1.JobList, bool) {
	list := &batchv1.JobList{}

	for i := 0; i < len(j.list.Items); i++ {
		switch {
		case j.stage != "" && j.list.Items[i].Labels[terraformv1alpha1.ConfigurationStageLabel] != j.stage:
			continue

		case j.generation != "" && j.list.Items[i].Labels[terraformv1alpha1.ConfigurationGenerationLabel] != j.generation:
			continue

		case j.uid != "" && j.list.Items[i].Labels[terraformv1alpha1.ConfigurationUID] != j.uid:
			continue
		}

		list.Items = append(list.Items, j.list.Items[i])
	}

	return list, len(list.Items) > 0
}
