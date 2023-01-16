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

package utils

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/hashicorp/go-getter"
)

// Download retrieves a source assets using the go-getter library
func Download(ctx context.Context, source, destination string) error {
	pwd, err := os.Getwd()
	if err != nil {
		return err
	}

	if strings.HasPrefix(source, "http") {
		source = strings.TrimPrefix(source, "http://")
		source = strings.TrimPrefix(source, "https://")
	}

	client := &getter.Client{
		Ctx: ctx,
		Dst: destination,
		Detectors: []getter.Detector{
			new(getter.GitHubDetector),
			new(getter.GitLabDetector),
			new(getter.GitDetector),
			new(getter.BitBucketDetector),
			new(getter.GCSDetector),
			new(getter.S3Detector),
			new(getter.FileDetector),
		},
		Mode:    getter.ClientModeAny,
		Options: []getter.ClientOption{},
		Pwd:     pwd,
		Src:     source,
	}

	doneCh := make(chan struct{})
	errCh := make(chan error, 1)
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)

	go func() {
		switch err := client.Get(); err {
		case nil:
			doneCh <- struct{}{}
		default:
			errCh <- err
		}
	}()

	return func() error {
		for {
			select {
			case <-sigCh:
				return errors.New("received a signal, cancelling the download")
			case <-ctx.Done():
				return errors.New("download has timed out or cancelled the download")
			case <-doneCh:
				return nil
			case err := <-errCh:
				return fmt.Errorf("failed to download the source: %w", err)
			}
		}
	}()
}
