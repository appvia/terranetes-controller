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
	"errors"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/hashicorp/go-getter"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"

	"github.com/appvia/terraform-controller/pkg/server"
	"github.com/appvia/terraform-controller/pkg/utils"
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
		Args:    cobra.ExactArgs(2),
		Version: version.Version,
		RunE: func(cmd *cobra.Command, args []string) error {
			timeout, _ := cmd.Flags().GetDuration("timeout")

			return Run(context.Background(), args[0], args[1], timeout)
		},
	}

	flags := cmd.Flags()
	flags.Duration("timeout", 10*time.Minute, "The timeout for the operation")

	if err := cmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "[error] failed to run: %s", err)

		os.Exit(1)
	}
}

// Run is called to execute the action
func Run(ctx context.Context, source, dest string, timeout time.Duration) error {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	pwd, err := os.Getwd()
	if err != nil {
		return err
	}

	if strings.HasPrefix(source, "http") {
		source = strings.TrimPrefix(source, "http://")
		source = strings.TrimPrefix(source, "https://")
	}

	log.WithFields(log.Fields{
		"source": source,
		"dest":   dest,
	}).Info("downloading the assets")

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

	doneCh := make(chan struct{})
	errCh := make(chan error, 1)
	sigCh := make(chan os.Signal)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)

	go func() {
		switch err := client.Get(); err {
		case nil:
			doneCh <- struct{}{}
		default:
			errCh <- err
		}
	}()

	ticker := time.NewTicker(3 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			if size, err := utils.DirSize(dest); err == nil {
				log.WithFields(log.Fields{
					"bytes": utils.ByteCountSI(size),
				}).Info("continuing downloaded the assets")
			}
		case <-sigCh:
			return errors.New("received a signal, cancelling the download")
		case <-ctx.Done():
			return errors.New("download has timed out, cancelling the download")
		case <-doneCh:
			log.WithField("source", source).Info("successfully downloaded the source")
			return nil
		case err := <-errCh:
			return fmt.Errorf("failed to download the source: %s", err)
		}
	}
}
