/*
 * Copyright (C) 2023  Appvia Ltd <info@appvia.io>
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

package preload

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"time"

	batchv1 "k8s.io/api/batch/v1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/yaml"

	corev1alpha1 "github.com/appvia/terranetes-controller/pkg/apis/core/v1alpha1"
	terraformv1alpha1 "github.com/appvia/terranetes-controller/pkg/apis/terraform/v1alpha1"
	"github.com/appvia/terranetes-controller/pkg/assets"
	"github.com/appvia/terranetes-controller/pkg/controller"
	"github.com/appvia/terranetes-controller/pkg/utils/filters"
	"github.com/appvia/terranetes-controller/pkg/utils/jobs"
	"github.com/appvia/terranetes-controller/pkg/utils/kubernetes"
	"github.com/appvia/terranetes-controller/pkg/utils/template"
)

// ensurePreloadEnabled ensures that the provider is setup for preloading
func (c *Controller) ensurePreloadEnabled(provider *terraformv1alpha1.Provider) controller.EnsureFunc {
	cond := controller.ConditionMgr(provider, terraformv1alpha1.ConditionProviderPreload, c.recorder)

	return func(ctx context.Context) (reconcile.Result, error) {
		switch {
		case !provider.IsPreloadingEnabled(), provider.Spec.Provider != terraformv1alpha1.AWSProviderType:
			cond.Disabled("Loading contextual data is not enabled")

			return reconcile.Result{}, controller.ErrIgnore
		}

		return reconcile.Result{}, nil
	}
}

// ensureReady is responsible for checking the condition of the provider, we only move forward when
// the provider is ready
func (c *Controller) ensureReady(provider *terraformv1alpha1.Provider) controller.EnsureFunc {
	cond := controller.ConditionMgr(provider, terraformv1alpha1.ConditionProviderPreload, c.recorder)

	return func(ctx context.Context) (reconcile.Result, error) {
		switch {
		case provider.GetCommonStatus().GetCondition(corev1alpha1.ConditionReady) == nil:
			return reconcile.Result{RequeueAfter: 15 * time.Second}, nil

		case provider.GetCommonStatus().GetCondition(corev1alpha1.ConditionReady).Status != metav1.ConditionTrue:
			cond.InProgress("Waiting for provider to be ready")

			return reconcile.Result{RequeueAfter: 15 * time.Second}, nil
		}

		return reconcile.Result{}, nil
	}
}

// ensurePreloadNotRunning is responsible for ensuring we have no active jobs running
func (c *Controller) ensurePreloadNotRunning(provider *terraformv1alpha1.Provider, state *state) controller.EnsureFunc {
	cond := controller.ConditionMgr(provider, terraformv1alpha1.ConditionProviderPreload, c.recorder)
	cc := c.cc

	return func(ctx context.Context) (reconcile.Result, error) {
		list := &batchv1.JobList{}

		if err := cc.List(ctx, list,
			client.InNamespace(c.ControllerNamespace),
			client.MatchingLabels(map[string]string{
				terraformv1alpha1.PreloadJobLabel:      "true",
				terraformv1alpha1.PreloadProviderLabel: provider.Name,
			}),
		); err != nil {
			cond.Failed(err, "Failed to retrieve a list in the controller namespace")

			return reconcile.Result{}, err
		}

		// @step: grab the latest job and check if it's running
		job, found := filters.Jobs(list).Latest()
		if found && jobs.IsActive(job) {
			cond.InProgress("Contextual data is currently being loaded under job: %s", job.Name)

			return reconcile.Result{RequeueAfter: 5 * time.Second}, nil
		}
		state.jobs = list

		return reconcile.Result{}, nil
	}
}

// ensurePreloadStatus is responsible for updating the condition of the preloading
func (c *Controller) ensurePreloadStatus(provider *terraformv1alpha1.Provider, state *state) controller.EnsureFunc {
	cond := controller.ConditionMgr(provider, terraformv1alpha1.ConditionProviderPreload, c.recorder)
	cc := c.cc

	return func(ctx context.Context) (reconcile.Result, error) {
		job, found := filters.Jobs(state.jobs).Latest()
		if !found {
			return reconcile.Result{}, nil
		}

		// @step: what was the result of the job?
		switch {
		case jobs.IsFailed(job):
			cond.Failed(nil, "Contextual data failed to load, please check the logs")

		case jobs.IsComplete(job):
			txt := &terraformv1alpha1.Context{}
			txt.Name = provider.Spec.Preload.Context

			found, err := kubernetes.GetIfExists(ctx, cc, txt)
			if err != nil {
				cond.Failed(err, "Failed to retrieve the contextual data resource: %s", txt.Name)

				return reconcile.Result{}, err
			}
			if !found {
				cond.Failed(nil, "The contextual data resource: %s was not found, please check logs", txt.Name)

				return reconcile.Result{}, nil
			}

			// @step: ensure we have some data
			if len(txt.Spec.Variables) == 0 {
				cond.Failed(nil, "The contextual data resource: %s has no data, please check logs", txt.Name)

				return reconcile.Result{}, nil
			}
			cond.Success("Contextual data successfully loaded")
		}

		return reconcile.Result{}, nil
	}
}

// ensurePreload is responsible for checking when the last preloading job ran and if greater than
// the interval, we run a new job
func (c *Controller) ensurePreload(provider *terraformv1alpha1.Provider) controller.EnsureFunc {
	cc := c.cc
	cond := controller.ConditionMgr(provider, terraformv1alpha1.ConditionProviderPreload, c.recorder)

	return func(ctx context.Context) (reconcile.Result, error) {
		interval := provider.Spec.Preload.GetIntervalOrDefault(6 * time.Hour)

		switch {
		case provider.Status.LastPreloadTime == nil:
			break
		case provider.Status.LastPreloadTime.Add(interval).After(time.Now()):
			return reconcile.Result{RequeueAfter: 5 * time.Minute}, nil
		}
		c.recorder.Event(provider, v1.EventTypeNormal, "Preloading", "Starting contextual data preload")

		options := map[string]interface{}{
			"ContainerImage": c.ContainerImage,
			"Context": map[string]interface{}{
				"Name": provider.Spec.Preload.Context,
			},
			"Cluster": provider.Spec.Preload.Cluster,
			"Controller": map[string]interface{}{
				"Namespace": c.ControllerNamespace,
			},
			"GenerateName":    fmt.Sprintf("preload-%s", provider.Name),
			"ImagePullPolicy": "IfNotPresent",
			"Labels": map[string]string{
				terraformv1alpha1.PreloadJobLabel:      "true",
				terraformv1alpha1.PreloadProviderLabel: provider.Name,
			},
			"Provider": map[string]interface{}{
				"Cloud":          provider.Spec.Provider.String(),
				"Name":           provider.Name,
				"SecretRef":      provider.Spec.SecretRef,
				"ServiceAccount": pointer.StringDeref(provider.Spec.ServiceAccount, ""),
				"Source":         provider.Spec.Source,
			},
			"Region":         provider.Spec.Preload.Region,
			"ServiceAccount": jobs.DefaultServiceAccount,
			"Verbose":        true,
		}

		// @step: we build and run the job
		render, err := template.New(string(assets.MustAsset("preload.yaml.tpl")), options)
		if err != nil {
			cond.Failed(err, "Failed to render the job template")

			return reconcile.Result{}, err
		}
		// @step: parse into a batch job
		encoded, err := yaml.YAMLToJSON(render)
		if err != nil {
			cond.Failed(err, "Failed to parse the job template")

			return reconcile.Result{}, err
		}

		job := &batchv1.Job{}
		if err := json.NewDecoder(bytes.NewReader(encoded)).Decode(job); err != nil {
			cond.Failed(err, "Failed to decode the job template")

			return reconcile.Result{}, err
		}

		// @step: provision in the job in the controller namespace
		if err := cc.Create(ctx, job); err != nil {
			cond.Failed(err, "Failed to create the preloading job in controller namespace")

			return reconcile.Result{}, err
		}
		cond.InProgress("Contextual data is currently running under job: %s", job.Name)

		// @step: update the status
		provider.Status.LastPreloadTime = &metav1.Time{Time: time.Now()}

		return reconcile.Result{}, nil
	}
}
