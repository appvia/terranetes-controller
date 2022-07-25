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

package configuration

import (
	"context"
	"io"
	"regexp"
	"time"

	log "github.com/sirupsen/logrus"
	batchv1 "k8s.io/api/batch/v1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	corev1alphav1 "github.com/appvia/terranetes-controller/pkg/apis/core/v1alpha1"
	terraformv1alphav1 "github.com/appvia/terranetes-controller/pkg/apis/terraform/v1alpha1"
	"github.com/appvia/terranetes-controller/pkg/controller"
	"github.com/appvia/terranetes-controller/pkg/utils/jobs"
	"github.com/appvia/terranetes-controller/pkg/utils/kubernetes"
	"github.com/appvia/terranetes-controller/pkg/utils/terraform"
)

// ensureErrorDetection is helper used to try and detect by the configuration failed and
// report is back to the users via status
func (c *Controller) ensureErrorDetection(configuration *terraformv1alphav1.Configuration, job *batchv1.Job, state *state) controller.EnsureFunc {
	logger := log.WithFields(log.Fields{
		"job":       job.Name,
		"name":      configuration.Name,
		"namespace": configuration.Namespace,
	})
	provider := string(state.provider.Spec.Provider)
	cond := controller.ConditionMgr(configuration, corev1alphav1.ConditionReady, c.recorder)

	return func(ctx context.Context) (reconcile.Result, error) {
		// @step: we check if the logs for the configuration are available
		pods, err := c.kc.CoreV1().Pods(c.ControllerNamespace).List(ctx, metav1.ListOptions{
			LabelSelector: "job-name=" + job.Name,
		})
		if err != nil {
			logger.WithError(err).Error("failed to list pods for job")

			return reconcile.Result{}, controller.ErrIgnore
		}

		// @step: ensure we have at least one pod
		pod := kubernetes.FindLatestPod(pods)
		if pod == nil {
			logger.Error("no matching pod found for job, skipping the error checks")

			return reconcile.Result{}, controller.ErrIgnore
		}

		// @step: ensure the pod is has finished
		switch pod.Status.Phase {
		case v1.PodPending, v1.PodRunning:
			return reconcile.Result{RequeueAfter: 5 * time.Second}, nil
		case v1.PodSucceeded, v1.PodFailed:
			break
		default:
			// @step: anything else and it's not certain we can workout what went wrong
			// so we'll just ignore it and post as an error
			return reconcile.Result{}, controller.ErrIgnore
		}

		// @step: find the terraform container and retrieve the logs
		stream, err := c.kc.CoreV1().Pods(c.ControllerNamespace).GetLogs(pod.Name, &v1.PodLogOptions{
			Container: jobs.TerraformContainerName,
			Follow:    false,
		}).Stream(ctx)
		if err != nil {
			logger.WithError(err).Error("failed to retrieve logs for job")

			return reconcile.Result{}, controller.ErrIgnore
		}
		defer stream.Close()

		// @step: retrieve the logs from the terraform container
		logs, err := io.ReadAll(stream)
		if err != nil {
			logger.WithField("container", jobs.TerraformContainerName).
				WithError(err).Error("failed to read logs from pod while trying to detect errors")

			return reconcile.Result{}, controller.ErrIgnore
		}

		// @step: retrieve all the detection regexes for this configuration
		detectors := append(terraform.Detectors["*"], terraform.Detectors[provider]...)
		if len(detectors) == 0 {
			return reconcile.Result{}, controller.ErrIgnore
		}

		for i := 0; i < len(detectors); i++ {
			m, err := regexp.Compile(detectors[i].Regex)
			if err != nil {
				logger.WithError(err).Error("failed to compile regex")

				return reconcile.Result{}, controller.ErrIgnore
			}
			if m.MatchString(string(logs)) {
				cond.ActionRequired(detectors[i].Message)
			}
		}

		// if we've entered this method we cannot move forward in the reconciliation process
		return reconcile.Result{}, controller.ErrIgnore
	}
}
