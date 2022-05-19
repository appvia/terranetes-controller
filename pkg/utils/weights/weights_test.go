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

package weights

import (
	"testing"

	"github.com/stretchr/testify/assert"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestNew(t *testing.T) {
	assert.NotNil(t, New())
}

func TestMax(t *testing.T) {
	w := New()
	w.Add(&v1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "test1"}}, 1)
	w.Add(&v1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "test2"}}, 1)
	w.Add(&v1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "test3"}}, 11)
	w.Add(&v1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "test4"}}, 10)

	assert.Equal(t, 11, w.Max())
}

func TestSize(t *testing.T) {
	w := New()
	w.Add(&v1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "test1"}}, 1)
	w.Add(&v1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "test2"}}, 1)

	assert.Equal(t, 2, w.Size())
}

func TestMaxAggregate(t *testing.T) {
	w := New()
	w.Add(&v1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "test1"}}, 1)
	w.Add(&v1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "test2"}}, 1)
	w.Add(&v1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "test3"}}, 11)
	w.Add(&v1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "test3"}}, 10)

	assert.Equal(t, 21, w.Max())
}

func TestHightestNames(t *testing.T) {
	w := New()
	w.Add(&v1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "test1"}}, 1)
	w.Add(&v1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "test2"}}, 11)

	items := w.HighestNames()
	assert.Len(t, items, 1)
	assert.Equal(t, "test2", items[0])

	w = New()
	w.Add(&v1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "test1"}}, 11)
	w.Add(&v1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "test2"}}, 11)

	items = w.HighestNames()
	assert.Len(t, items, 2)
	assert.Equal(t, []string{"test1", "test2"}, items)
}

func TestHightest(t *testing.T) {
	w := New()
	w.Add(&v1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "test1"}}, 1)
	w.Add(&v1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "test2"}}, 1)
	w.Add(&v1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "test3"}}, 11)
	w.Add(&v1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "test4"}}, 10)

	items := w.Highest()
	assert.Len(t, items, 1)
	assert.Equal(t, "test3", items[0].GetName())

	w = New()
	w.Add(&v1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "test1"}}, 1)
	w.Add(&v1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "test2"}}, 1)
	w.Add(&v1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "test2"}}, 11)
	w.Add(&v1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "test4"}}, 10)

	items = w.Highest()
	assert.Len(t, items, 1)
	assert.Equal(t, "test2", items[0].GetName())
}

func TestHightestMultiple(t *testing.T) {
	w := New()
	w.Add(&v1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "test1"}}, 2)
	w.Add(&v1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "test2"}}, 2)
	w.Add(&v1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "test3"}}, 1)

	assert.Equal(t, 2, w.Max())
	assert.Len(t, w.Highest(), 2)
	assert.Equal(t, "test1", w.Highest()[0].GetName())
	assert.Equal(t, "test2", w.Highest()[1].GetName())
}
