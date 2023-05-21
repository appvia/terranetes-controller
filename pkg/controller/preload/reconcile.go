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
	"context"

	log "github.com/sirupsen/logrus"
	batchv1 "k8s.io/api/batch/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	terraformv1alpha1 "github.com/appvia/terranetes-controller/pkg/apis/terraform/v1alpha1"
	"github.com/appvia/terranetes-controller/pkg/controller"
)

type state struct {
	// is the jobs currently found
	jobs *batchv1.JobList
}

// Reconcile is called to handle the reconciliation of the resource
func (c *Controller) Reconcile(ctx context.Context, request reconcile.Request) (reconcile.Result, error) {
	provider := &terraformv1alpha1.Provider{}

	if err := c.cc.Get(ctx, request.NamespacedName, provider); err != nil {
		if kerrors.IsNotFound(err) {
			return reconcile.Result{}, nil
		}
		log.WithError(err).Error("failed to retrieve the preload resource")

		return reconcile.Result{}, err
	}

	state := &state{}

	result, err := controller.DefaultEnsureHandler.Run(ctx, c.cc, provider,
		[]controller.EnsureFunc{
			c.ensurePreloadEnabled(provider),
			c.ensureReady(provider),
			c.ensurePreloadNotRunning(provider, state),
			c.ensurePreloadStatus(provider, state),
			c.ensurePreload(provider),
		})
	if err != nil {
		log.WithError(err).Error("failed to reconcile the provider for preload data")

		return reconcile.Result{}, err
	}

	return result, err
}
