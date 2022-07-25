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

	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/stretchr/testify/assert"

	"github.com/appvia/terranetes-controller/pkg/schema"
	"github.com/appvia/terranetes-controller/test/fixtures"
)

func TestCreateOrForceUpdate(t *testing.T) {
	namespace := "default"
	name := "test"

	cc := fake.NewClientBuilder().WithScheme(schema.GetScheme()).Build()
	configuration := fixtures.NewValidBucketConfiguration(namespace, name)

	found, err := GetIfExists(context.TODO(), cc, configuration.DeepCopy())
	assert.NoError(t, err)
	assert.False(t, found)

	err = CreateOrForceUpdate(context.TODO(), cc, configuration)
	assert.NoError(t, err)
	assert.Equal(t, "https://github.com/terraform-aws-modules/terraform-aws-s3-bucket.git", configuration.Spec.Module)

	found, err = GetIfExists(context.TODO(), cc, configuration.DeepCopy())
	assert.NoError(t, err)
	assert.True(t, found)

	update := configuration.DeepCopy()
	update.Spec.Module = "updated"
	err = CreateOrForceUpdate(context.TODO(), cc, update)
	assert.NoError(t, err)

	found, err = GetIfExists(context.TODO(), cc, configuration)
	assert.NoError(t, err)
	assert.True(t, found)
	assert.Equal(t, "updated", configuration.Spec.Module)
}

func TestCreateOfPatch(t *testing.T) {
	namespace := "default"
	name := "test"

	cc := fake.NewClientBuilder().WithScheme(schema.GetScheme()).Build()
	configuration := fixtures.NewValidBucketConfiguration(namespace, name)

	// first we create
	assert.NoError(t, CreateOrPatch(context.TODO(), cc, configuration))
	found, err := GetIfExists(context.TODO(), cc, configuration.DeepCopy())
	assert.NoError(t, err)
	assert.True(t, found)

	// then we update
	updated := configuration.DeepCopy()
	updated.Spec.Module = "updated"
	assert.NoError(t, CreateOrPatch(context.TODO(), cc, updated))

	found, err = GetIfExists(context.TODO(), cc, configuration)
	assert.NoError(t, err)
	assert.True(t, found)
	assert.Equal(t, "updated", configuration.Spec.Module)
}
