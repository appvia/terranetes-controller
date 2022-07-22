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

package drift

import (
	"fmt"
	"reflect"
	"time"

	log "github.com/sirupsen/logrus"
	"k8s.io/client-go/tools/record"

	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	terraformv1alphav1 "github.com/appvia/terranetes-controller/pkg/apis/terraform/v1alpha1"
)

const controllerName = "drift.terraform.appvia.io"

// Controller handles the reconciliation of the configuration resource
type Controller struct {
	// cc is the kubernetes client to the cluster
	cc client.Client
	// recorder is the kubernetes event recorder
	recorder record.EventRecorder
	// CheckInterval is the interval the controller checks to trigger drift on the configurations
	CheckInterval time.Duration
	// DriftInterval is the minimum time before triggering drift detection on a configuration
	DriftInterval time.Duration
	// DriftThreshold is the maximum number of drift checks to run concurrently
	DriftThreshold float64
}

// Add is called to setup the manager for the controller
func (c *Controller) Add(mgr manager.Manager) error {
	log.WithFields(log.Fields{
		"drift":     c.DriftInterval.String(),
		"interval":  c.CheckInterval.String(),
		"threshold": c.DriftThreshold,
	}).Info("adding the drift controller")

	switch {
	case c.CheckInterval <= 0:
		return fmt.Errorf("interval must be greater than 0")
	case c.DriftInterval <= 0:
		return fmt.Errorf("drift interval must be greater than 0")
	case c.DriftThreshold < 0:
		return fmt.Errorf("max running must be greater than or equal to 0")
	}

	c.cc = mgr.GetClient()
	c.recorder = mgr.GetEventRecorderFor(controllerName)

	return ctrl.NewControllerManagedBy(mgr).
		For(&terraformv1alphav1.Configuration{}).
		Named(controllerName).
		WithOptions(controller.Options{MaxConcurrentReconciles: 1}).
		WithEventFilter(&predicate.Funcs{
			UpdateFunc: func(e event.UpdateEvent) bool {
				switch {
				case !e.ObjectNew.(*terraformv1alphav1.Configuration).Spec.EnableDriftDetection:
					return false
				case !e.ObjectNew.GetDeletionTimestamp().IsZero():
					return false
				case !reflect.DeepEqual(e.ObjectNew.GetAnnotations(), e.ObjectOld.GetAnnotations()):
					return false
				}

				return true
			},
			CreateFunc: func(e event.CreateEvent) bool {
				return false
			},
			DeleteFunc: func(e event.DeleteEvent) bool {
				return false
			},
		}).
		Complete(c)
}
