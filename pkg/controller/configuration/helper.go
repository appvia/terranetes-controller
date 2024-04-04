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

// CreateWatcher is responsible for ensuring the logger is running in the application namespace
func (c Controller) CreateWatcher(ctx context.Context, configuration *terraformv1alpha1.Configuration, stage string) error {
	watcher := jobs.New(configuration, nil).NewJobWatch(c.ControllerNamespace, stage, c.ExecutorImage)

	// @step: check if the logger has been created
	if found, err := kubernetes.GetIfExists(ctx, c.cc, watcher.DeepCopy()); err != nil {
		return err
	} else if found {
		return nil
	}

	return c.cc.Create(ctx, watcher)
}
