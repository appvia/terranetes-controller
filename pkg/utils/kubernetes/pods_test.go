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
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestFindLatestPod(t *testing.T) {
	list := &v1.PodList{
		Items: []v1.Pod{
			{ObjectMeta: metav1.ObjectMeta{Name: "pod-1", CreationTimestamp: metav1.NewTime(time.Now().Add(-1 * time.Minute))}},
			{ObjectMeta: metav1.ObjectMeta{Name: "pod-2", CreationTimestamp: metav1.NewTime(time.Now().Add(-2 * time.Minute))}},
			{ObjectMeta: metav1.ObjectMeta{Name: "pod-3", CreationTimestamp: metav1.NewTime(time.Now().Add(-3 * time.Minute))}},
			{ObjectMeta: metav1.ObjectMeta{Name: "latest", CreationTimestamp: metav1.NewTime(time.Now().Add(-20 * time.Second))}},
			{ObjectMeta: metav1.ObjectMeta{Name: "pod-4", CreationTimestamp: metav1.NewTime(time.Now().Add(-1 * time.Minute))}},
		},
	}

	pod := FindLatestPod(list)
	assert.Equal(t, "latest", pod.Name)
}
