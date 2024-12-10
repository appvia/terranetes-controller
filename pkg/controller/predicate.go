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
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	"github.com/appvia/terranetes-controller/pkg/utils"
)

// NamespacePredicate is a predicate that matches namespaces
type NamespacePredicate struct {
	// Namespaces is a list of namespaces to match
	Namespaces []string
}

// NewNamespacePredicate creates a new namespace predicate
func NewNamespacePredicate(namespaces []string) predicate.Funcs {
	filter := func(object client.Object) bool {
		if len(namespaces) == 0 {
			return true
		}

		return utils.Contains(object.GetNamespace(), namespaces)
	}

	return predicate.NewPredicateFuncs(filter)
}
