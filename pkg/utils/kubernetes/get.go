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

package kubernetes

import (
	"context"

	v1 "k8s.io/api/core/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// GetSecretIfExists retrieves a secret if it exists
func GetSecretIfExists(ctx context.Context, cc client.Client, namespace, name string) (*v1.Secret, bool, error) {
	secret := &v1.Secret{}
	secret.Namespace = namespace
	secret.Name = name

	exists, err := GetIfExists(ctx, cc, secret)
	if err != nil {
		return nil, false, err
	}
	if !exists {
		return nil, false, nil
	}

	return secret, exists, nil
}

// GetIfExists retrieves an object if it exists
func GetIfExists(ctx context.Context, cc client.Client, object client.Object) (bool, error) {
	if err := cc.Get(ctx, client.ObjectKeyFromObject(object), object); err != nil {
		if !kerrors.IsNotFound(err) {
			return false, err
		}

		return false, nil
	}

	return true, nil
}
