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
	"fmt"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// FakeRecorder is a fake recorder
type FakeRecorder struct {
	Events []string
}

// NewFakeRecorder returns a fake recorder
func NewFakeRecorder() record.EventRecorder {
	return &FakeRecorder{}
}

// Event is just like Eventf, but withouts formatting
func (f *FakeRecorder) Event(object runtime.Object, eventtype, reason, message string) {
	o := object.(client.Object)

	f.Events = append(f.Events, fmt.Sprintf("(%s/%s) %s %s: %s", o.GetNamespace(), o.GetName(), eventtype, reason, message))
}

// Eventf is just like Event, but with Sprintf-style formatting
func (f *FakeRecorder) Eventf(object runtime.Object, eventtype, reason, messageFmt string, args ...interface{}) {
	o := object.(client.Object)

	f.Events = append(f.Events, fmt.Sprintf("(%s/%s) %s %s: %s", o.GetNamespace(), o.GetName(), eventtype, reason, fmt.Sprintf(messageFmt, args...)))
}

// AnnotatedEventf is just like Eventf, but adds the annotations to the event
func (f *FakeRecorder) AnnotatedEventf(object runtime.Object, annotations map[string]string, eventtype, reason, messageFmt string, args ...interface{}) {
	o := object.(client.Object)

	f.Events = append(f.Events, fmt.Sprintf("(%s/%s) %s %s: %s", o.GetNamespace(), o.GetName(), eventtype, reason, fmt.Sprintf(messageFmt, args...)))
}
