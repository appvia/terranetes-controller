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
	"os"

	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"sigs.k8s.io/controller-runtime/pkg/manager/signals"

	"github.com/appvia/terranetes-controller/pkg/utils"
	"github.com/appvia/terranetes-controller/pkg/version"
)

var description = `
Preload is provided a Kubernetes cluster name and region, along with credentials. The purpose
of the binary is to retrieve data related to the cluster; network ids, subnets, security groups,
routing tables and so forth and use the information to populate a Terranetes Context resource.
This data can be referenced from Configuration resources in order to provide local context to the
module`

// Command are the options for the preload command
type Command struct {
	Config
	// logger is the logger for the command
	logger *log.Entry
}

func init() {
	log.SetFormatter(&log.JSONFormatter{})
}

// New creates a new preload command
func New() *cobra.Command {
	o := &Command{}

	cmd := &cobra.Command{
		Use:     "preload [options]",
		Long:    description,
		Short:   "Used to retrieve data from the cloud provider and store in context",
		Version: version.Version,
		RunE: func(cmd *cobra.Command, args []string) error {
			if v, _ := cmd.Flags().GetBool("verbose"); v {
				log.SetLevel(log.DebugLevel)
			}

			return o.Run(signals.SetupSignalHandler())
		},
	}

	flags := cmd.Flags()
	flags.BoolVar(&o.Config.EnableOverride, "enable-overrides", true, "Indicates as values change, the context should be updated")
	flags.StringVar(&o.Config.Cloud, "cloud", os.Getenv("CLOUD"), "Is the cloud vendor we are retrieving data from")
	flags.StringVar(&o.Config.Cluster, "cluster", os.Getenv("CLUSTER"), "Is the cluster name we are retrieving data from")
	flags.StringVar(&o.Config.Context, "context", os.Getenv("CONTEXT"), "Is the context name we will provision the data into")
	flags.StringVar(&o.Config.Provider, "provider", os.Getenv("PROVIDER"), "Is the provider name which triggered the preloading")
	flags.StringVar(&o.Config.Region, "region", os.Getenv("REGION"), "Is the region we are retrieving data from")
	flags.Bool("verbose", true, "Enable verbose logging")

	return cmd
}

// Run executes the preload command
func (c *Command) Run(ctx context.Context) error {
	switch {
	case c.Cloud == "":
		return fmt.Errorf("cloud is required")
	case c.Cluster == "":
		return fmt.Errorf("cluster is required")
	case c.Provider == "":
		return fmt.Errorf("provider is required")
	case c.Region == "":
		return fmt.Errorf("region is required")
	case c.Cloud != "aws":
		return fmt.Errorf("%s cloud is not supported", c.Cloud)
	}

	c.logger = log.WithFields(log.Fields{
		"cloud":    c.Cloud,
		"cluster":  c.Cluster,
		"context":  c.Context,
		"provider": c.Provider,
		"region":   c.Region,
	})
	c.logger.Info("retrieve contextual data from the cloud vendor")

	// @step: retrieve the data from the provider
	data, err := c.preload(ctx)
	if err != nil {
		return err
	}
	for _, key := range utils.Sorted(data.Keys()) {
		log.Debugf("key: %s, value: %v", key, data[key].Value)
	}

	// @step: if we have no context we can exit here
	if c.Context == "" {
		return nil
	}

	return c.makeContext(ctx, data)
}
