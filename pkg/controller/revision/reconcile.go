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

package revision

import (
	"context"
	"time"

	log "github.com/sirupsen/logrus"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	terraformv1alpha1 "github.com/appvia/terranetes-controller/pkg/apis/terraform/v1alpha1"
	"github.com/appvia/terranetes-controller/pkg/controller"
)

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
	finalizer := controller.NewFinalizer(c.cc, controllerName)
	if finalizer.IsDeletionCandidate(revision) {
		result, err := controller.DefaultEnsureHandler.Run(ctx, c.cc, revision,
			[]controller.EnsureFunc{
				c.ensureRevisionDeleted(revision),
				finalizer.EnsureRemoved(revision),
			},
		)
		if err != nil {
			log.WithError(err).Error("failed to reconcile the revision resource")

			return reconcile.Result{}, err
		}

		return result, nil
	}

	// @step: ensure the conditions are registered
	controller.EnsureConditionsRegistered(terraformv1alpha1.DefaultRevisionConditions, revision)

	// @step: ensure the resource is reconciled
	result, err := controller.DefaultEnsureHandler.Run(ctx, c.cc, revision,
		[]controller.EnsureFunc{
			finalizer.EnsurePresent(revision),
			c.ensurePlanExists(revision),
			c.ensureInUseCount(revision),
		},
	)
	if err != nil {
		log.WithError(err).Error("failed to reconcile the revision resource")

		return reconcile.Result{}, err
	}

	return controller.RequeueUnless(result, err, 10*time.Minute)
}
