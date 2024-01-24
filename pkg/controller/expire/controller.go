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

package expire

import (
	"time"

	log "github.com/sirupsen/logrus"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/manager"

	terraformv1alpha1 "github.com/appvia/terranetes-controller/pkg/apis/terraform/v1alpha1"
)

var controllerName = "expire.terraform.appvia.io"

// Controller handles the reconciliation of the resource
type Controller struct {
	// cc is the kubernetes client to the cluster
	cc client.Client
	// recorder is used to record events
	recorder record.EventRecorder
	// RevisionExpiration indicates the amount of time multiple revisions should be kept for
	// which are not being used.
	RevisionExpiration time.Duration
}

// Add is called to setup the manager for the controller
func (c *Controller) Add(mgr manager.Manager) error {
	log.WithFields(log.Fields{
		"enabled":    c.RevisionExpiration != 0,
		"expiration": c.RevisionExpiration,
	}).Info("adding the expiration controller")

	// @step: we can ignore running this controller if the expiration is zero
	if c.RevisionExpiration == 0 {
		return nil
	}

	c.cc = mgr.GetClient()
	c.recorder = mgr.GetEventRecorderFor(controllerName)

	return ctrl.NewControllerManagedBy(mgr).
		For(&terraformv1alpha1.Revision{}).
		Named(controllerName).
		WithOptions(controller.Options{MaxConcurrentReconciles: 1}).
		Complete(c)
}
