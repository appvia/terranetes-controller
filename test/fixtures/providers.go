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

package fixtures

import (
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	corev1alphav1 "github.com/appvia/terranetes-controller/pkg/apis/core/v1alpha1"
	terraformv1alphav1 "github.com/appvia/terranetes-controller/pkg/apis/terraform/v1alpha1"
	"github.com/appvia/terranetes-controller/pkg/controller"
)

// NewValidAWSProvider returns a valid provider for aws
func NewValidAWSProvider(name string, secret *v1.Secret) *terraformv1alphav1.Provider {
	provider := &terraformv1alphav1.Provider{
		ObjectMeta: metav1.ObjectMeta{Name: name},
		Spec: terraformv1alphav1.ProviderSpec{
			Source:   terraformv1alphav1.SourceSecret,
			Provider: "aws",
		},
	}

	if secret != nil {
		provider.Spec.SecretRef = &v1.SecretReference{Name: secret.Name, Namespace: secret.Namespace}
	}

	return provider
}

// NewValidAWSNotReadyProvider returns a ready aws provider
func NewValidAWSNotReadyProvider(name string, secret *v1.Secret) *terraformv1alphav1.Provider {
	provider := NewValidAWSProvider(name, secret)
	controller.EnsureConditionsRegistered(terraformv1alphav1.DefaultProviderConditions, provider)
	provider.Status.GetCondition(corev1alphav1.ConditionReady).Status = metav1.ConditionFalse

	return provider
}

// NewValidAWSReadyProvider returns a ready aws provider
func NewValidAWSReadyProvider(name string, secret *v1.Secret) *terraformv1alphav1.Provider {
	provider := NewValidAWSProvider(name, secret)
	controller.EnsureConditionsRegistered(terraformv1alphav1.DefaultProviderConditions, provider)
	provider.Status.GetCondition(corev1alphav1.ConditionReady).Status = metav1.ConditionTrue

	return provider
}

// NewValidAWSProviderSecret returns a valid provider secret for aws
func NewValidAWSProviderSecret(namespace, name string) *v1.Secret {
	secret := &v1.Secret{}
	secret.Namespace = namespace
	secret.Name = name
	secret.Data = map[string][]byte{
		"AWS_ACCESS_KEY_ID":     []byte("test"),
		"AWS_SECRET_ACCESS_KEY": []byte("test"),
	}

	return secret
}
