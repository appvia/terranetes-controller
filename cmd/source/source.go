/*
 * Copyright (C) 2022  Rohith Jayawardene <gambol99@gmail.com>
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

package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"sync"
	"time"

	"github.com/hashicorp/go-getter"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"

	"github.com/appvia/terraform-controller/pkg/server"
	"github.com/appvia/terraform-controller/pkg/version"
)

func init() {
	log.SetFormatter(&log.JSONFormatter{})
}

var config server.Config

func main() {
	cmd := &cobra.Command{
		Use:     "source <URL> <DIRECTORY>",
		Short:   "Used to retrieve the source code for the terraform controller",
		Version: version.Version,
		RunE: func(cmd *cobra.Command, args []string) error {
			if v, _ := cmd.Flags().GetBool("verbose"); v {
				log.SetLevel(log.DebugLevel)
			}

			if len(args) != 2 {
				return fmt.Errorf("invalid arguments, expected <URL> <DIRECTORY>")
			}

			return Run(context.Background(), args[0], args[1])
		},
	}

	flags := cmd.Flags()
	flags.Bool("verbose", false, "Enable verbose logging")
	flags.Bool("trace", false, "Enable trace logging")

	if err := cmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "[error] failed to run: %s", err)

		os.Exit(1)
	}
}

// Run is called to execute the action
func Run(ctx context.Context, source, dest string) error {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Minute)
	defer cancel()

	pwd, err := os.Getwd()
	if err != nil {
		return err
	}

	if strings.HasPrefix(source, "http") {
		source = strings.TrimPrefix(source, "http://")
		source = strings.TrimPrefix(source, "https://")
	}

	log.Infof("downloading the source: %s", source)

	// Build the client
	client := &getter.Client{
		Ctx:       ctx,
		Dir:       true,
		Dst:       dest,
		Detectors: goGetterDetectors,
		Mode:      getter.ClientModeAny,
		Options:   []getter.ClientOption{},
		Pwd:       pwd,
		Src:       source,
	}

	wg := sync.WaitGroup{}
	wg.Add(1)
	errChan := make(chan error, 2)
	go func() {
		defer wg.Done()
		defer cancel()
		if err := client.Get(); err != nil {
			errChan <- err
		}
	}()

	c := make(chan os.Signal)
	signal.Notify(c, os.Interrupt)

	select {
	case <-c:
		signal.Reset(os.Interrupt)
		wg.Wait()
	case <-ctx.Done():
		wg.Wait()
		log.Printf("Success downloaded the asset")
	case err := <-errChan:
		wg.Wait()
		log.Fatalf("Error downloading: %s", err)
	}

	return nil
}
