/*
 * Copyright (C) 2022 Rohith Jayawardene <gambol99@gmail.com>
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

	terraformv1alphav1 "github.com/appvia/terraform-controller/pkg/apis/terraform/v1alpha1"
	"github.com/appvia/terraform-controller/pkg/utils/jobs"
	"github.com/appvia/terraform-controller/pkg/utils/kubernetes"
)

// CreateWatcher is responsible for ensuring the logger is running in the application namespace
func (c Controller) CreateWatcher(ctx context.Context, configuration *terraformv1alphav1.Configuration, stage string) error {
	watcher := jobs.New(configuration, nil).NewJobWatch(c.JobNamespace, stage)

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
