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

package controller

import (
	"context"
	"errors"
	"time"

	log "github.com/sirupsen/logrus"
	apiequality "k8s.io/apimachinery/pkg/api/equality"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	corev1alpha1 "github.com/appvia/terranetes-controller/pkg/apis/core/v1alpha1"
)

var (
	// ErrIgnore is used to used to stop the ensure chain
	ErrIgnore = errors.New("resource is being ignored")
	// DefaultEnsureHandler is the default sequential runner for a list of ensure functions
	DefaultEnsureHandler = EnsureRunner{}
)

const (
	// ResourceReady is the default message we use for the ready condition, if the
	// controller has not set a message
	ResourceReady = "Resource ready"
)

// EnsureFunc defines a method to ensure a state
type EnsureFunc func(ctx context.Context) (reconcile.Result, error)

// EnsureRunner provides a wrapper for running ensure funcs
type EnsureRunner struct{}

// RequeueImmediate should be used anywhere we wish an immediate / ASAP requeue to be performed
var RequeueImmediate = reconcile.Result{RequeueAfter: 5 * time.Millisecond}

// RequeueAfter is a helper function to return a requeue result with the given duration
func RequeueAfter(d time.Duration) EnsureFunc {
	return func(ctx context.Context) (reconcile.Result, error) {
		return reconcile.Result{RequeueAfter: d}, nil
	}
}

// RequeueUnless is a helper function to return a requeue result unless a requeue is already set or an error is present
func RequeueUnless(result reconcile.Result, err error, duration time.Duration) (reconcile.Result, error) {
	if err != nil {
		return reconcile.Result{}, err
	}
	if result.RequeueAfter > 0 {
		return result, nil
	}

	return reconcile.Result{RequeueAfter: duration}, nil
}

// Run is a generic handler for running the ensure methods
func (e *EnsureRunner) Run(ctx context.Context, cc client.Client, resource Object, ensures []EnsureFunc) (result reconcile.Result, rerr error) {
	original := resource.DeepCopyObject().(client.Object)
	status := resource.GetCommonStatus()

	// @here we are responsible for updating the transition times of the conditions where we
	// see a drift. And updating the status of the resource overall
	defer func() {
		status.LastReconcile = updateReconcileStatus(resource, original, status.LastReconcile)

		// @step: we need to update the status of the resource
		if err := cc.Status().Patch(ctx, resource, client.MergeFrom(original)); err != nil {
			if err := client.IgnoreNotFound(err); err != nil {
				log.WithError(err).Error("failed to update the status of resource")

				rerr = err
				result = reconcile.Result{}
			}
		}
	}()

	for _, x := range ensures {
		result, rerr = x(ctx)
		if rerr != nil {
			switch {
			case kerrors.IsConflict(rerr):
				rerr = nil
				result = RequeueImmediate
				return

			case errors.Is(rerr, ErrIgnore):
				rerr = nil
				result = reconcile.Result{}
				return
			}

			return reconcile.Result{}, rerr
		}
		if result.RequeueAfter > 0 {
			return result, nil
		}
	}

	cond := ConditionMgr(resource, corev1alpha1.ConditionReady, nil)
	cond.Success(ResourceReady)

	status.LastSuccess = updateReconcileStatus(resource, original, status.LastSuccess)

	return reconcile.Result{}, nil
}

// updateReconcileStatus ensures the last reconciliation timestamp is only
// updated if there are other changes to the resource. This ensures watchers
// won't be triggered for timestamp only changes.
func updateReconcileStatus(resource, original client.Object, last *corev1alpha1.LastReconcileStatus) *corev1alpha1.LastReconcileStatus {
	isUpToDate := last != nil &&
		last.Generation == resource.GetGeneration() &&
		apiequality.Semantic.DeepEqual(original, resource)
	if isUpToDate {
		return last
	}

	return &corev1alpha1.LastReconcileStatus{
		Generation: resource.GetGeneration(),
		Time:       metav1.Now(),
	}
}
