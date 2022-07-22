/*
 * Copyright (C) 2022 Appvia Ltd <info@appvia.io>
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

package provider

import (
	"context"

	log "github.com/sirupsen/logrus"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	terraformv1alphav1 "github.com/appvia/terranetes-controller/pkg/apis/terraform/v1alpha1"
	"github.com/appvia/terranetes-controller/pkg/controller"
)

// Reconcile is called to handle the reconciliation of the provider resource
func (c *Controller) Reconcile(ctx context.Context, request reconcile.Request) (reconcile.Result, error) {
	provider := &terraformv1alphav1.Provider{}

	// @step: retrieve the provider resource
	if err := c.cc.Get(ctx, request.NamespacedName, provider); err != nil {
		if kerrors.IsNotFound(err) {
			return reconcile.Result{}, nil
		}
		log.WithError(err).Error("failed to retrieve the provider resource")

		return reconcile.Result{}, err
	}
	// @step: ensure the provider has all the condition registered
	controller.EnsureConditionsRegistered(terraformv1alphav1.DefaultProviderConditions, provider)

	return controller.DefaultEnsureHandler.Run(ctx, c.cc, provider, []controller.EnsureFunc{
		c.ensureProviderSecret(provider),
	})
}
