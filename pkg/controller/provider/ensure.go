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

	corev1alpha1 "github.com/appvia/terranetes-controller/pkg/apis/core/v1alpha1"
	terraformv1alpha1 "github.com/appvia/terranetes-controller/pkg/apis/terraform/v1alpha1"
	"github.com/appvia/terranetes-controller/pkg/controller"
	"github.com/appvia/terranetes-controller/pkg/utils/kubernetes"
)

// ensureProviderSecret is responsible for ensuring the provider secret exists
func (c *Controller) ensureProviderSecret(provider *terraformv1alpha1.Provider) controller.EnsureFunc {
	cond := controller.ConditionMgr(provider, corev1alpha1.ConditionReady, c.recorder)

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
		annotations := provider.GetAnnotations()
		if annotations == nil {
			annotations = map[string]string{}
		}

		switch annotations[terraformv1alpha1.ProviderSecretSkipChecks] {
		case "false", "False", "":
			switch provider.Spec.Provider {
			case terraformv1alpha1.AzureProviderType, terraformv1alpha1.AzureCloudStackProviderType:
				switch {
				case secret.Data["ARM_CLIENT_ID"] == nil:
					cond.ActionRequired("Provider secret (%s/%s) is missing the ARM_CLIENT_ID", secret.Namespace, secret.Name)
					return reconcile.Result{}, controller.ErrIgnore
				case secret.Data["ARM_CLIENT_SECRET"] == nil:
					cond.ActionRequired("Provider secret (%s/%s) is missing the ARM_CLIENT_SECRET", secret.Namespace, secret.Name)
					return reconcile.Result{}, controller.ErrIgnore
				case secret.Data["ARM_SUBSCRIPTION_ID"] == nil:
					cond.ActionRequired("Provider secret (%s/%s) is missing the ARM_SUBSCRIPTION_ID", secret.Namespace, secret.Name)
					return reconcile.Result{}, controller.ErrIgnore
				case secret.Data["ARM_TENANT_ID"] == nil:
					cond.ActionRequired("Provider secret (%s/%s) is missing the ARM_TENANT_ID", secret.Namespace, secret.Name)
					return reconcile.Result{}, controller.ErrIgnore
				}

			case terraformv1alpha1.GCPProviderType:
				switch {
				case
					secret.Data["GCLOUD_KEYFILE_JSON"] == nil &&
						secret.Data["GOOGLE_APPLICATION_CREDENTIALS"] == nil &&
						secret.Data["GOOGLE_CLOUD_KEYFILE_JSON"] == nil &&
						secret.Data["GOOGLE_CREDENTIALS"] == nil:
					cond.ActionRequired("Provider secret (%s/%s) is missing the GOOGLE_APPLICATION_CREDENTIALS, GOOGLE_CREDENTIALS, GOOGLE_CLOUD_KEYFILE_JSON or GCLOUD_KEYFILE_JSON field", secret.Namespace, secret.Name)

					return reconcile.Result{}, controller.ErrIgnore
				}

			case terraformv1alpha1.AWSProviderType:
				switch {
				case len(secret.Data["AWS_ACCESS_KEY_ID"]) == 0:
					cond.ActionRequired("Provider secret (%s/%s) is missing the AWS_ACCESS_KEY_ID", secret.Namespace, secret.Name)

					return reconcile.Result{}, controller.ErrIgnore

				case len(secret.Data["AWS_SECRET_ACCESS_KEY"]) == 0:
					cond.ActionRequired("Provider secret (%s/%s) is missing the AWS_SECRET_ACCESS_KEY", secret.Namespace, secret.Name)

					return reconcile.Result{}, controller.ErrIgnore
				case len(secret.Data["AWS_ACCESS_KEY_ID"]) > len(secret.Data["AWS_SECRET_ACCESS_KEY"]):
					cond.ActionRequired("provider secret (%s/%s) aws access key is larger than secret", secret.Namespace, secret.Name)

					return reconcile.Result{}, controller.ErrIgnore
				}
			}
		}

		return reconcile.Result{}, nil
	}
}

// ensurePreloadEnabled ensures that the provider is setup for preloading
func (c *Controller) ensurePreloadEnabled(provider *terraformv1alpha1.Provider) controller.EnsureFunc {
	cond := controller.ConditionMgr(provider, terraformv1alpha1.ConditionProviderPreload, c.recorder)

	return func(ctx context.Context) (reconcile.Result, error) {
		switch {
		case !provider.IsPreloadingEnabled():
			cond.Disabled("Loading contextual data is not enabled")

		case provider.Spec.Provider != terraformv1alpha1.AWSProviderType:
			cond.Warning("Loading contextual is supported on AWS only")
		}

		return reconcile.Result{}, nil
	}
}
