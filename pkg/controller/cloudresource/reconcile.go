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

package cloudresource

import (
	"context"

	log "github.com/sirupsen/logrus"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	terraformv1alpha1 "github.com/appvia/terranetes-controller/pkg/apis/terraform/v1alpha1"
	"github.com/appvia/terranetes-controller/pkg/controller"
)

type state struct {
	// configuration is the current configuration
	configuration *terraformv1alpha1.Configuration
	// plan is the plan definition we are using
	plan *terraformv1alpha1.Plan
	// revision is the plan revision we are using
	revision *terraformv1alpha1.Revision
}

// Reconcile is called to handle the reconciliation of the resource
func (c *Controller) Reconcile(ctx context.Context, request reconcile.Request) (reconcile.Result, error) {
	cloudresource := &terraformv1alpha1.CloudResource{}

	if err := c.cc.Get(ctx, request.NamespacedName, cloudresource); err != nil {
		if kerrors.IsNotFound(err) {
			return reconcile.Result{}, nil
		}
		log.WithError(err).Error("failed to retrieve the cloudresource resource")

		return reconcile.Result{}, err
	}

	state := &state{}

	finalizer := controller.NewFinalizer(c.cc, controllerName)
	if finalizer.IsDeletionCandidate(cloudresource) {
		result, err := controller.DefaultEnsureHandler.Run(ctx, c.cc, cloudresource,
			[]controller.EnsureFunc{
				c.ensureConfigurationRemoved(cloudresource),
				finalizer.EnsureRemoved(cloudresource),
			})
		if err != nil {
			log.WithError(err).Error("failed to delete the cloudresource resource")

			return reconcile.Result{}, err
		}

		return result, err
	}

	// @step: ensure the conditions are registered
	controller.EnsureConditionsRegistered(terraformv1alpha1.DefaultCloudResourceConditions, cloudresource)

	result, err := controller.DefaultEnsureHandler.Run(ctx, c.cc, cloudresource,
		[]controller.EnsureFunc{
			finalizer.EnsurePresent(cloudresource),
			c.ensurePlanExists(cloudresource, state),
			c.ensureRevisionExists(cloudresource, state),
			c.ensureConfigurationExists(cloudresource, state),
			c.ensureUpdateStatus(cloudresource, state),
			c.ensureConfigurationStatus(cloudresource, state),
		})
	if err != nil {
		log.WithError(err).Error("failed to reconcile the cloudresource resource")

		return reconcile.Result{}, err
	}

	return result, err
}
