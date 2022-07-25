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

package controller

import (
	"context"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/appvia/terranetes-controller/pkg/utils"
)

// Finalizable is an kubernetes resource api object that supports finalizers
type Finalizable interface {
	client.Object
	GetFinalizers() []string
	SetFinalizers(finalizers []string)
	GetDeletionTimestamp() *metav1.Time
}

// Finalizer manages the finalizers for resources in kubernetes
type Finalizer struct {
	driver client.Client
	value  string
}

// NewFinalizer constructs a new finalizer manager
func NewFinalizer(driver client.Client, finalizerValue string) *Finalizer {
	return &Finalizer{
		driver: driver,
		value:  finalizerValue,
	}
}

// Add adds a finalizer to an object
func (c *Finalizer) Add(resource Finalizable) error {
	finalizers := append(resource.GetFinalizers(), c.value)
	resource.SetFinalizers(finalizers)

	return c.driver.Update(context.Background(), resource.DeepCopyObject().(client.Object), client.FieldOwner(c.value))
}

// Remove removes a finalizer from an object
func (c *Finalizer) Remove(resource Finalizable) error {
	allFinalizers := resource.GetFinalizers()

	if !utils.Contains(c.value, allFinalizers) {
		return nil
	}

	finalizers := []string{}
	for _, finalizer := range allFinalizers {
		if finalizer != c.value {
			finalizers = append(finalizers, finalizer)
		}
	}
	resource.SetFinalizers(finalizers)

	return c.driver.Update(context.Background(), resource)
}

// IsDeletionCandidate checks if the resource is a candidate for deletion
// @note: this method should not be used by a dependent - they should simply
// check resource.GetDeletionTimestamp() != nil
func (c *Finalizer) IsDeletionCandidate(resource Finalizable) bool {
	if resource.GetDeletionTimestamp() == nil {
		return false
	}
	length := len(resource.GetFinalizers())

	switch {
	case length == 0:
		return true

	case length == 1 && utils.Contains(c.value, resource.GetFinalizers()):
		return true

	case length == 2 && utils.Contains(c.value, resource.GetFinalizers()) && utils.Contains(metav1.FinalizerDeleteDependents, resource.GetFinalizers()):
		return true

	default:
		return false
	}
}

// NeedToAdd checks if the resource should have but does not have the finalizer
func (c *Finalizer) NeedToAdd(resource Finalizable) bool {
	return resource.GetDeletionTimestamp().IsZero() && !utils.Contains(c.value, resource.GetFinalizers())
}

// EnsureEmpty finalizer only includes us - so we can continue and delete the resource
func (c *Finalizer) EnsureEmpty(resource Finalizable) EnsureFunc {
	return func(ctx context.Context) (reconcile.Result, error) {
		if c.IsDeletionCandidate(resource) {
			return reconcile.Result{}, nil
		}

		return reconcile.Result{RequeueAfter: 30 * time.Second}, nil
	}
}

// EnsurePresent ensures the finalizer is present on the resource
func (c *Finalizer) EnsurePresent(resource Finalizable) func(context.Context) (reconcile.Result, error) {
	return func(context.Context) (reconcile.Result, error) {
		if !c.NeedToAdd(resource) {
			return reconcile.Result{}, nil
		}

		if err := c.Add(resource); err != nil {
			return reconcile.Result{}, err
		}

		return RequeueImmediate, nil
	}
}

// EnsureRemoved is called to ensure we have removed ourself from the finalizers
func (c *Finalizer) EnsureRemoved(resource Finalizable) func(context.Context) (reconcile.Result, error) {
	return func(context.Context) (reconcile.Result, error) {
		allFinalizers := resource.GetFinalizers()

		if !utils.Contains(c.value, allFinalizers) {
			return reconcile.Result{}, nil
		}

		finalizers := []string{}
		for _, finalizer := range allFinalizers {
			if finalizer != c.value {
				finalizers = append(finalizers, finalizer)
			}
		}
		resource.SetFinalizers(finalizers)

		return reconcile.Result{}, c.driver.Update(context.Background(), resource)
	}
}
