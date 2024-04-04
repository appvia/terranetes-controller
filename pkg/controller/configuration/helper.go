/*
 * Copyright (C) 2022 Appvia Ltd <info@appvia.io>
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

package configuration

import (
	"context"
	"fmt"
	"strings"

	log "github.com/sirupsen/logrus"
	v1 "k8s.io/api/core/v1"

	terraformv1alpha1 "github.com/appvia/terranetes-controller/pkg/apis/terraform/v1alpha1"
	"github.com/appvia/terranetes-controller/pkg/utils/jobs"
	"github.com/appvia/terranetes-controller/pkg/utils/kubernetes"
)

// GetTerraformImage is called to return the terraform image to use, or the image plus version
// override
func GetTerraformImage(configuration *terraformv1alpha1.Configuration, image string) string {
	if configuration.Spec.TerraformVersion == "" {
		return image
	}
	e := strings.Split(image, ":")

	return fmt.Sprintf("%s:%s", e[0], configuration.Spec.TerraformVersion)
}

// getNamespaceFromCache is responsible for retrieving the namespace from the cache, or deferring
// to a direct lookup if it's not found
func (c *Controller) getNamespaceFromCache(ctx context.Context, name string) (*v1.Namespace, error) {
	item, found := c.cache.Get(name)
	if found {
		return item.(*v1.Namespace), nil
	}

	log.WithFields(log.Fields{
		"namespace": name,
	}).Warn("namespace not found in the cache, deferring to a direct lookup")

	namespace := &v1.Namespace{}
	namespace.Name = name

	if found, err := kubernetes.GetIfExists(ctx, c.cc, namespace); err != nil {
		return nil, err
	} else if !found {
		return nil, fmt.Errorf("namespace %s not found", name)
	}

	c.cache.SetDefault(name, namespace)

	return namespace, nil
}

// CreateWatcher is responsible for ensuring the logger is running in the application namespace
func (c Controller) CreateWatcher(ctx context.Context, configuration *terraformv1alpha1.Configuration, stage string) error {
	watcher := jobs.New(configuration, nil).NewJobWatch(c.ControllerNamespace, stage, c.ExecutorImage)

	// @step: check if the logger has been created
	found, err := kubernetes.GetIfExists(ctx, c.cc, watcher.DeepCopy())
	if err != nil {
		return err
	}
	if found {
		return nil
	}

	return c.cc.Create(ctx, watcher)
}
