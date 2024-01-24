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

package expire

import (
	"context"
	"time"

	log "github.com/sirupsen/logrus"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	terraformv1alpha1 "github.com/appvia/terranetes-controller/pkg/apis/terraform/v1alpha1"
	"github.com/appvia/terranetes-controller/pkg/controller"
	"github.com/appvia/terranetes-controller/pkg/utils"
)

// ensureExpiration is responsible ensuring any revisions which need expiring are removed from the system
func (c *Controller) ensureExpiration(revision *terraformv1alpha1.Revision) controller.EnsureFunc {
	logger := log.WithFields(log.Fields{
		"revision": revision.Name,
		"plan":     revision.Spec.Plan.Name,
	})

	return func(ctx context.Context) (reconcile.Result, error) {
		// @step: is the revision older than the expiration time?
		if time.Since(revision.CreationTimestamp.Time) < c.RevisionExpiration {
			logger.Debug("revision is not older than the expiration time, ignoring")

			return reconcile.Result{}, controller.ErrIgnore
		}

		return reconcile.Result{}, nil
	}
}

// ensureMultipleRevisions is responsible for ensuring we have multiple revisions, else we can quit
func (c *Controller) ensureMultipleRevisions(revision *terraformv1alpha1.Revision, state *state) controller.EnsureFunc {
	cc := c.cc
	logger := log.WithFields(log.Fields{
		"revision": revision.Name,
		"plan":     revision.Spec.Plan.Name,
	})

	return func(ctx context.Context) (reconcile.Result, error) {
		// @step: do we have mutiple revisions of the same plans?
		state.revisions = &terraformv1alpha1.RevisionList{}

		if err := cc.List(ctx, state.revisions, client.MatchingLabels(map[string]string{
			terraformv1alpha1.RevisionPlanNameLabel: revision.Spec.Plan.Name,
		})); err != nil {
			logger.WithError(err).Error("failed to retrieve the list of revisions")

			return reconcile.Result{}, err
		}

		// @check do we have more than 1 revision?
		if len(state.revisions.Items) <= 1 {
			logger.Debug("there is only 1 revision of the plan, ignoring")

			return reconcile.Result{}, controller.ErrIgnore
		}

		return reconcile.Result{}, nil
	}
}

// ensureNotLatest is responsible for ensuring we are not the latest revision of the plan
func (c *Controller) ensureNotLatest(revision *terraformv1alpha1.Revision, state *state) controller.EnsureFunc {
	logger := log.WithFields(log.Fields{
		"revision": revision.Name,
		"plan":     revision.Spec.Plan.Name,
	})

	return func(ctx context.Context) (reconcile.Result, error) {
		// @step: are we older than the other revisions
		var versions []string
		for _, x := range state.revisions.Items {
			versions = append(versions, x.Spec.Plan.Revision)
		}

		latest, err := utils.LatestSemverVersion(versions)
		if err != nil {
			logger.WithError(err).Error("failed to sort the revisions")

			return reconcile.Result{}, err
		}
		if latest == revision.Spec.Plan.Revision {
			logger.Debug("revision is the latest, ignoring")

			return reconcile.Result{}, controller.ErrIgnore
		}

		return reconcile.Result{}, nil
	}
}

// enaureZeroCloudResources is responsible for ensuring there are no cloud resources for the revision
func (c *Controller) ensureZeroCloudResources(revision *terraformv1alpha1.Revision) controller.EnsureFunc {
	cc := c.cc
	logger := log.WithFields(log.Fields{
		"revision": revision.Name,
		"plan":     revision.Spec.Plan.Name,
	})

	return func(ctx context.Context) (reconcile.Result, error) {
		// @step: if the revision referenced by any cloud resources, if so we cannot delete it
		resources := &terraformv1alpha1.CloudResourceList{}
		if err := cc.List(ctx, resources, client.MatchingLabels(map[string]string{
			terraformv1alpha1.CloudResourcePlanNameLabel: revision.Spec.Plan.Name,
			terraformv1alpha1.CloudResourceRevisionLabel: revision.Spec.Plan.Revision,
		})); err != nil {
			logger.WithError(err).Error("failed to retrieve the list of cloud resources")

			return reconcile.Result{}, err
		}

		if len(resources.Items) > 0 {
			logger.Debug("revision has cloud resources, ignoring")

			return reconcile.Result{}, controller.ErrIgnore
		}

		return reconcile.Result{}, nil
	}
}

// ensureDeletion is responsible for triggering the deletion of the revision
func (c *Controller) ensureDeletion(revision *terraformv1alpha1.Revision) controller.EnsureFunc {
	cc := c.cc
	logger := log.WithFields(log.Fields{
		"revision": revision.Name,
		"plan":     revision.Spec.Plan.Name,
	})

	return func(ctx context.Context) (reconcile.Result, error) {
		// @step: raise an event indicating we are deleting the revision
		c.recorder.Eventf(revision, "Normal", "ExpiringRevision", "Expiring the revision %s", revision.Spec.Plan.Revision)

		if err := cc.Delete(ctx, revision); err != nil {
			logger.WithError(err).Error("failed to delete the revision")

			return reconcile.Result{}, err
		}

		return reconcile.Result{}, nil
	}
}
