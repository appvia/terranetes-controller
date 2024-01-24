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

package expire

import (
	"context"
	"time"

	log "github.com/sirupsen/logrus"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	terraformv1alpha1 "github.com/appvia/terranetes-controller/pkg/apis/terraform/v1alpha1"
	"github.com/appvia/terranetes-controller/pkg/controller"
)

type state struct {
	// revision is a list of all the revisions for the plan
	revisions *terraformv1alpha1.RevisionList
}

// Reconcile is called to handle the reconciliation of the resource
func (c *Controller) Reconcile(ctx context.Context, request reconcile.Request) (reconcile.Result, error) {
	revision := &terraformv1alpha1.Revision{}

	// @step: retrieve the resource from the cache
	if err := c.cc.Get(ctx, request.NamespacedName, revision); err != nil {
		if kerrors.IsNotFound(err) {
			return reconcile.Result{}, nil
		}
		log.WithError(err).Error("failed to retrieve the revision resource")

		return reconcile.Result{}, err
	}

	// @step: is the resource marked for deletion?
	if revision.DeletionTimestamp != nil {
		return reconcile.Result{RequeueAfter: 60 * time.Minute}, nil
	}

	state := &state{}

	// @step: ensure the resource is reconciled
	result, err := controller.DefaultEnsureHandler.Run(ctx, c.cc, revision,
		[]controller.EnsureFunc{
			c.ensureExpiration(revision),
			c.ensureMultipleRevisions(revision, state),
			c.ensureNotLatest(revision, state),
			c.ensureZeroCloudResources(revision),
			c.ensureDeletion(revision),
		},
	)
	if err != nil {
		log.WithError(err).Error("failed to reconcile the revision resource")

		return reconcile.Result{}, err
	}

	return controller.RequeueUnless(result, err, 60*time.Minute)
}
