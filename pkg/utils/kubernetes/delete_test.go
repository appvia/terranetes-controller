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

	"github.com/stretchr/testify/assert"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestDelete(t *testing.T) {
	cases := []struct {
		Expected error
		Object   client.Object
		Existing []client.Object
	}{
		{
			Expected: nil,
			Object:   &v1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "test"}},
		},
	}
	for _, c := range cases {
		cc := fake.NewClientBuilder().WithObjects(c.Existing...).Build()
		err := DeleteIfExists(context.Background(), cc, c.Object)
		assert.Equal(t, c.Expected, err)
	}
}
