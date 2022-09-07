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
	"testing"

	v1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/stretchr/testify/assert"

	"github.com/appvia/terranetes-controller/pkg/schema"
)

func TestCreateOrForceUpdate(t *testing.T) {
	namespace := "default"
	name := "test"

	cc := fake.NewClientBuilder().WithScheme(schema.GetScheme()).Build()
	pod := &v1.Pod{}
	pod.Name = name
	pod.Namespace = namespace
	pod.Labels = map[string]string{"hello": "world"}

	found, err := GetIfExists(context.TODO(), cc, pod.DeepCopy())
	assert.NoError(t, err)
	assert.False(t, found)

	err = CreateOrForceUpdate(context.TODO(), cc, pod)
	assert.NoError(t, err)
	assert.Equal(t, map[string]string{"hello": "world"}, pod.Labels)

	found, err = GetIfExists(context.TODO(), cc, pod.DeepCopy())
	assert.NoError(t, err)
	assert.True(t, found)

	update := pod.DeepCopy()
	update.Labels["is"] = "updated"
	err = CreateOrForceUpdate(context.TODO(), cc, update)
	assert.NoError(t, err)

	found, err = GetIfExists(context.TODO(), cc, pod)
	assert.NoError(t, err)
	assert.True(t, found)
	assert.Equal(t, map[string]string{"hello": "world", "is": "updated"}, pod.Labels)
}

func TestCreateOfPatch(t *testing.T) {
	namespace := "default"
	name := "test"

	cc := fake.NewClientBuilder().WithScheme(schema.GetScheme()).Build()
	pod := &v1.Pod{}
	pod.Name = name
	pod.Namespace = namespace
	pod.Labels = map[string]string{"hello": "world"}

	// first we create
	assert.NoError(t, CreateOrPatch(context.TODO(), cc, pod))
	found, err := GetIfExists(context.TODO(), cc, pod.DeepCopy())
	assert.NoError(t, err)
	assert.True(t, found)

	// then we update
	updated := pod.DeepCopy()
	updated.Labels["hello"] = "updated"
	assert.NoError(t, CreateOrPatch(context.TODO(), cc, updated))

	found, err = GetIfExists(context.TODO(), cc, pod)
	assert.NoError(t, err)
	assert.True(t, found)
	assert.Equal(t, updated.Labels["hello"], "updated")
}
