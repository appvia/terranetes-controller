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
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"

	load "github.com/appvia/terranetes-controller/pkg/utils/preload"
	"github.com/appvia/terranetes-controller/pkg/utils/preload/eks"
)

// preload is responsible for retrieving the data from the cloud vendor
func (c *Command) preload(ctx context.Context) (load.Data, error) {
	session, err := session.NewSession(&aws.Config{
		Region: aws.String(c.Region),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create aws session, error: %w", err)
	}

	// @step: create the preloader for eks clusters
	pe, err := eks.New(eks.Config{
		ClusterName: c.Cluster,
		Session:     session,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create preloader for cloud: %w", err)
	}

	return pe.Load(ctx)
}
