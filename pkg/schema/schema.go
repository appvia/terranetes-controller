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

package schema

import (
	log "github.com/sirupsen/logrus"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/client-go/kubernetes/scheme"

	terraformv1alpha1 "github.com/appvia/terranetes-controller/pkg/apis/terraform/v1alpha1"
)

var (
	hs = runtime.NewScheme()
	// codec is the codec factory for the schema
	codec serializer.CodecFactory
)

func init() {
	builder := runtime.NewSchemeBuilder(
		scheme.AddToScheme,
		terraformv1alpha1.AddToScheme,
	)
	if err := builder.AddToScheme(hs); err != nil {
		log.WithError(err).Fatal("failed to build the scheme")
	}

	hs.AddKnownTypes(metav1.SchemeGroupVersion,
		&metav1.CreateOptions{},
		&metav1.DeleteOptions{},
		&metav1.GetOptions{},
		&metav1.ListOptions{},
		&metav1.PatchOptions{},
		&metav1.UpdateOptions{},
	)

	codec = serializer.NewCodecFactory(hs)
}

// GetScheme returns the schema
func GetScheme() *runtime.Scheme {
	return hs
}

// GetCodecFactory returns the codec factory
func GetCodecFactory() serializer.CodecFactory {
	return codec
}
