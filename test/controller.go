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

package controllertests

import (
	"context"

	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

// Roll is called to run the reconciler
func Roll(ctx context.Context, ctrl reconcile.Reconciler, o client.Object, _ int) (reconcile.Result, int, error) {
	req := reconcile.Request{}
	req.Namespace = o.GetNamespace()
	req.Name = o.GetName()

	var result reconcile.Result
	var err error

	for i := 0; i < 10; i++ {
		result, err = ctrl.Reconcile(ctx, req)
		switch {
		case err != nil:
			return result, i, err
		case result.RequeueAfter > 0:
		default:
			return result, i, nil
		}
	}

	return result, 0, err
}
