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

	v1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	corev1alphav1 "github.com/appvia/terranetes-controller/pkg/apis/core/v1alpha1"
	terraformv1alpha1 "github.com/appvia/terranetes-controller/pkg/apis/terraform/v1alpha1"
	"github.com/appvia/terranetes-controller/pkg/controller"
	"github.com/appvia/terranetes-controller/pkg/utils/kubernetes"
)

// ensureProviderSecret is responsible for ensuring the provider secret exists
func (c *Controller) ensureProviderSecret(provider *terraformv1alpha1.Provider) controller.EnsureFunc {
	cond := controller.ConditionMgr(provider, corev1alphav1.ConditionReady, c.recorder)

	return func(ctx context.Context) (reconcile.Result, error) {
		if provider.Spec.SecretRef == nil {
			return reconcile.Result{}, nil
		}

		// @step: ensure the secret exists
		secret := &v1.Secret{}
		secret.Namespace = provider.Spec.SecretRef.Namespace
		secret.Name = provider.Spec.SecretRef.Name

		found, err := kubernetes.GetIfExists(ctx, c.cc, secret)
		if err != nil {
			cond.Failed(err, "Failed to retrieve the provider secret")

			return reconcile.Result{}, err
		}
		if !found {
			cond.ActionRequired("Provider secret (%s/%s) not found", secret.Namespace, secret.Name)

			return reconcile.Result{}, controller.ErrIgnore
		}

		// @step: ensure the format of the secret is correct
		switch provider.Spec.Provider {
		case terraformv1alpha1.AWSProviderType:
			switch {
			case secret.Data["AWS_ACCESS_KEY_ID"] == nil, secret.Data["AWS_SECRET_ACCESS_KEY"] == nil:
				cond.ActionRequired("provider secret (%s/%s) missing aws secrets", secret.Namespace, secret.Name)

				return reconcile.Result{}, controller.ErrIgnore
			}

		case terraformv1alpha1.AzureProviderType:
		case terraformv1alpha1.GCPProviderType:
		default:
			cond.ActionRequired("Provider type: %s is not supported", provider.Spec.Provider)

			return reconcile.Result{}, controller.ErrIgnore
		}

		return reconcile.Result{}, nil
	}
}
