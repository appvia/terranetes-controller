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

package main

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"os/signal"
	"path"
	"strings"
	"syscall"
	"time"

	"github.com/hashicorp/go-getter"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"

	"github.com/appvia/terranetes-controller/pkg/utils"
	"github.com/appvia/terranetes-controller/pkg/version"
)

func init() {
	log.SetFormatter(&log.TextFormatter{})
}

func main() {
	var source, destination string
	var timeout time.Duration
	var tmpDirectory bool

	cmd := &cobra.Command{
		Use:     "source [options]",
		Short:   "Used to retrieve the source code for the terraform controller",
		Version: version.Version,
		RunE: func(cmd *cobra.Command, args []string) error {
			return Run(context.Background(), source, destination, timeout, tmpDirectory)
		},
	}

	flags := cmd.Flags()
	flags.DurationVarP(&timeout, "timeout", "t", 10*time.Minute, "The timeout for the operation")
	flags.StringVarP(&source, "source", "s", "", "Source which needs to be downloaded")
	flags.StringVarP(&destination, "dest", "d", "", "Directory where the source code to be saved")
	flags.BoolVar(&tmpDirectory, "tmpdir", true, "Use a temporary directory to download the assets")

	if err := cmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "[error] failed to run: %s", err)

		os.Exit(1)
	}
}

// Run is called to execute the action
func Run(ctx context.Context, source, destination string, timeout time.Duration, tmpdir bool) error {
	if source == "" {
		return errors.New("no source defined")
	}
	if destination == "" {
		return errors.New("no destination directory defined")
	}
	if timeout < 0 {
		return errors.New("timeout can not be less than zero")
	}

	uri, err := url.Parse(source)
	if err != nil {
		return fmt.Errorf("failed to parse source url: %v", err)
	}
	location := source

	// @step: check for an ssh key in the environment variables and provision a configuration
	switch {
	case os.Getenv("SSH_AUTH_KEYFILE") != "":
		data, err := os.ReadFile(os.Getenv("SSH_AUTH_KEYFILE"))
		if err != nil {
			return fmt.Errorf("failed to read ssh key file: %v", err)
		}
		encoded := base64.StdEncoding.EncodeToString(data)
		switch strings.Contains(source, "?") {
		case true:
			location = fmt.Sprintf("%s&sshkey=%s", source, encoded)
		default:
			location = fmt.Sprintf("%s?sshkey=%s", source, encoded)
		}

	case os.Getenv("SSH_AUTH_KEY") != "":
		encoded := base64.StdEncoding.EncodeToString([]byte(os.Getenv("SSH_AUTH_KEY")))
		switch strings.Contains(source, "?") {
		case true:
			location = fmt.Sprintf("%s&sshkey=%s", source, encoded)
		default:
			location = fmt.Sprintf("%s?sshkey=%s", source, encoded)
		}

	case os.Getenv("GIT_USERNAME") != "" && os.Getenv("GIT_PASSWORD") != "":
		filename := path.Join("${HOME}", ".git", "config")

		found, err := utils.FileExists(os.ExpandEnv(filename))
		if err != nil {
			return err
		}
		if found {
			log.WithField("path", filename).Warn("git configuration file already found, skipping")

			break
		}

		// @step: update the gitconfig to overload the URL
		// git config --global url."https://token:$GIT_TOKEN@github.com/example/".insteadOf "ssh://git@github.com/example/"
		args := []string{
			"config", "--global",
			fmt.Sprintf("url.\"https://%s:%s@%s/%s\".insteadOf \"%s\"",
				os.Getenv("GIT_USERNAME"),
				os.Getenv("GIT_PASSWORD"),
				uri.Hostname(), uri.Path, source),
		}
		if err := exec.Command("git", args...).Run(); err != nil {
			return fmt.Errorf("failed tp update the git configuration: %v", err)
		}

	}

	// @step: retrieve the working directory
	pwd, err := os.Getwd()
	if err != nil {
		return err
	}

	if strings.HasPrefix(location, "http") {
		location = strings.TrimPrefix(location, "http://")
		location = strings.TrimPrefix(location, "https://")
	}

	log.WithFields(log.Fields{
		"source": source,
		"dest":   destination,
	}).Info("downloading the assets")

	// @step: create a temporary directory
	dest := destination
	if tmpdir {
		dest = "/tmp/source"

		if err := os.RemoveAll(dest); err != nil {
			return fmt.Errorf("failed to remove temporary directory: %v", err)
		}
	}

	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	client := &getter.Client{
		Ctx: ctx,
		Dst: dest,
		Detectors: []getter.Detector{
			new(getter.GitHubDetector),
			new(getter.GitLabDetector),
			new(getter.GitDetector),
			new(getter.BitBucketDetector),
			new(getter.GCSDetector),
			new(getter.S3Detector),
		},
		Mode:    getter.ClientModeAny,
		Options: []getter.ClientOption{},
		Pwd:     pwd,
		Src:     location,
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

	ticker := time.NewTicker(3 * time.Second)
	defer ticker.Stop()

	err = func() error {
		for {
			select {
			case <-ticker.C:
				if size, err := utils.DirSize(dest); err == nil {
					log.WithFields(log.Fields{
						"bytes": utils.ByteCountSI(size),
					}).Info("continuing to download the assets")
				}
			case <-sigCh:
				return errors.New("received a signal, cancelling the download")
			case <-ctx.Done():
				return errors.New("download has timed out, cancelling the download")
			case <-doneCh:
				return nil
			case err := <-errCh:
				return fmt.Errorf("failed to download the source: %s", err)
			}
		}
	}()
	if err != nil {
		return err
	}
	log.WithField("source", source).Info("successfully downloaded the source")

	// @step: if we were using a temporary directory we need to copy the files over
	if !tmpdir {
		return nil
	}

	return exec.Command("cp", []string{"-rT", "/tmp/source/", destination}...).Run()
}
