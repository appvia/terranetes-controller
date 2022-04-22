/*
 * Copyright (C) 2022  Rohith Jayawardene <gambol99@gmail.com>
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

	"sigs.k8s.io/controller-runtime/pkg/client"
)

// CreateOrForceUpdate either creates or forces the update of the resource
func CreateOrForceUpdate(ctx context.Context, cc client.Client, resource client.Object) error {
	current := resource.DeepCopyObject().(client.Object)

	if found, err := GetIfExists(ctx, cc, current); err != nil {
		return err
	} else if !found {
		return cc.Create(ctx, resource)
	}
	resource.SetResourceVersion(current.GetResourceVersion())

	return cc.Update(ctx, resource)
}

// CreateOrPatch either creates or patches the resource
func CreateOrPatch(ctx context.Context, cc client.Client, resource client.Object) error {
	current := resource.DeepCopyObject().(client.Object)

	found, err := GetIfExists(ctx, cc, current)
	if err != nil {
		return err
	}
	if !found {
		return cc.Create(ctx, resource)
	}

	return cc.Patch(ctx, resource, client.MergeFrom(current))
}
