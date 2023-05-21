/*
 * Copyright (C) 2023  Appvia Ltd <info@appvia.io>
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

package preload

import (
	"bytes"
	"context"
	"encoding/json"

	log "github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	terraformv1alpha1 "github.com/appvia/terranetes-controller/pkg/apis/terraform/v1alpha1"
	"github.com/appvia/terranetes-controller/pkg/schema"
	"github.com/appvia/terranetes-controller/pkg/utils/kubernetes"
	"github.com/appvia/terranetes-controller/pkg/utils/preload"
)

// makeContext is responsible for creating a context for the preload command
func (c *Command) makeContext(ctx context.Context, data preload.Data) error {
	c.logger.Info("attempting to provision the preload context in the cluster")

	// @step: we need to create a runtime client
	cc, err := kubernetes.NewRuntimeClient(schema.GetScheme())
	if err != nil {
		return err
	}

	// @step: create the new context resource
	encoded := &bytes.Buffer{}
	if err := data.MarshalTo(encoded); err != nil {
		return err
	}
	txt := &terraformv1alpha1.Context{}
	if err := json.NewDecoder(encoded).Decode(&txt.Spec.Variables); err != nil {
		return err
	}
	txt.Name = c.Context
	txt.Labels = map[string]string{
		terraformv1alpha1.PreloadProviderLabel: c.Provider,
	}

	// @step: next we need to grab the current context if any
	current := &terraformv1alpha1.Context{}
	current.Name = txt.Name

	// @step: check if the context exists
	found, err := kubernetes.GetIfExists(ctx, cc, current)
	if err != nil {
		return err
	}
	switch {
	case !found:
		c.logger.Info("no existing context found, creating a new one")

		return cc.Create(ctx, txt)

	case found && c.EnableOverride:
		log.Info("existing context found, updating all values")
		// @step: we are permitted to override the entire context
		txt.ResourceVersion = current.ResourceVersion

		return cc.Update(ctx, txt)
	}

	c.logger.Info("existing context found, updating only new values")

	// @step: else we only add values which aren't there already
	original := current.DeepCopy()

	for _, key := range data.Keys() {
		if _, found := current.Spec.Variables[key]; !found {
			encoded, err := data.Get(key).Marshal()
			if err != nil {
				return err
			}
			current.Spec.Variables[key] = runtime.RawExtension{Raw: encoded}
		}
	}

	if err := cc.Patch(ctx, current, client.MergeFrom(original)); err != nil {
		return err
	}
	c.logger.Info("successfully the updated context")

	return nil
}
