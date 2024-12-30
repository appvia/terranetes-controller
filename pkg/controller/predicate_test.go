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
	"testing"

	"github.com/stretchr/testify/assert"
	v1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
)

func TestNewNamespacePredicate(t *testing.T) {
	assert.NotNil(t, NewNamespacePredicate([]string{"test"}))
}

func TestNewNamespacePredicateCreate(t *testing.T) {
	filter := NewNamespacePredicate([]string{"test"})
	assert.NotNil(t, filter)

	pod := &v1.Pod{}
	pod.Namespace = "test"

	assert.True(t, filter.Create(event.TypedCreateEvent[client.Object]{Object: pod}))
}
