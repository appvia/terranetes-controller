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
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// Weights provides a convience components to weigthing objects
type Weights struct {
	items []item
}

type item struct {
	weight int
	object client.Object
}

// New returns a new weights
func New() *Weights {
	return &Weights{}
}

// Highest returns the items with the highest weight
func (w *Weights) Highest() []client.Object {
	var list []client.Object

	max := w.Max()
	for i := 0; i < len(w.items); i++ {
		if w.items[i].weight == max {
			list = append(list, w.items[i].object)
		}
	}

	return list
}

// Size returns the items in the list
func (w *Weights) Size() int {
	return len(w.items)
}

// HighestNames returns the names of the highest objects
func (w *Weights) HighestNames() []string {
	var list []string

	for _, x := range w.Highest() {
		list = append(list, x.GetName())
	}

	return list
}

// Max returns objects with the highest weight in the items
func (w *Weights) Max() int {
	weight := 0

	for i := 0; i < len(w.items); i++ {
		if w.items[i].weight > weight {
			weight = w.items[i].weight
		}
	}

	return weight
}

// Add is used to add a object with a weight to the list - if the item already exists, the
// weight is added together
func (w *Weights) Add(object client.Object, weight int) {
	for i := 0; i < len(w.items); i++ {
		if equalObjects(w.items[i].object, object) {
			w.items[i].weight += weight
			return
		}
	}
	w.items = append(w.items, item{weight: weight, object: object})
}

func equalObjects(a, b client.Object) bool {
	switch {
	case a.DeepCopyObject().GetObjectKind().GroupVersionKind().String() != b.GetObjectKind().GroupVersionKind().String():
		return false
	case a.GetNamespace() != b.GetNamespace():
		return false
	case a.GetName() != b.GetName():
		return false
	}

	return true
}
