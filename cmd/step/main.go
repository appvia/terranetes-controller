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
	"errors"
	"fmt"
	"io"
	"math/rand"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	v1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager/signals"

	"github.com/appvia/terranetes-controller/pkg/utils"
	"github.com/appvia/terranetes-controller/pkg/utils/kubernetes"
	"github.com/appvia/terranetes-controller/pkg/version"
)

func init() {
	log.SetFormatter(&log.TextFormatter{})
}

func main() {
	var step Step

	cmd := &cobra.Command{
		Use:     "step [options] -- command",
		Short:   "Used to run a command in a structured order",
		Version: version.Version,
		RunE: func(cmd *cobra.Command, args []string) error {
			if v, _ := cmd.Flags().GetBool("verbose"); v {
				log.SetLevel(log.DebugLevel)
			}

			return Run(signals.SetupSignalHandler(), step)
		},
	}
	cmd.SilenceUsage = true

	flags := cmd.Flags()
	flags.DurationVar(&step.Timeout, "timeout", 30*time.Second, "Timeout for wait-on file to appear")
	flags.StringVar(&step.Comment, "comment", "", "Adds a banner before executing the step")
	flags.StringVar(&step.ErrorFile, "on-error", "", "The path to a file to indicate we have failed")
	flags.StringVar(&step.Namespace, "namespace", os.Getenv("KUBE_NAMESPACE"), "Namespace to upload any secrets")
	flags.StringVar(&step.SuccessFile, "on-success", "", "The path of the file used to indicate the step was successful")
	flags.StringVarP(&step.Shell, "shell", "s", "/bin/sh", "The shell to execute the command in")
	flags.StringVar(&step.FailureFile, "is-failure", "", "The path of the file used to indicate failure above")
	flags.StringSliceVarP(&step.UploadFile, "upload", "u", []string{}, "Upload file as a kubernetes secret")
	flags.StringVar(&step.WaitFile, "wait-on", "", "The path to a file to indicate this step can be run")
	flags.StringSliceVarP(&step.Commands, "command", "c", []string{}, "Command to execute")
	flags.IntVar(&step.RetryAttempts, "retry-attempts", 0, "Number of times to retry the commands")
	flags.DurationVar(&step.RetryMinBackoff, "retry-min-backoff", 0, "Minimum duration to wait between retry attempts")
	flags.DurationVar(&step.RetryMaxJitter, "retry-max-jitter", 2*time.Second, "Maximum random jitter to add to backoff time")
	if err := cmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "[Error] %s\n", err)

		os.Exit(1)
	}
}

// calculateBackoff returns a duration that includes the minimum backoff plus a random jitter
func calculateBackoff(minBackoff, maxJitter time.Duration) time.Duration {
	if maxJitter <= 0 {
		return minBackoff
	}
	jitter := time.Duration(rand.Int63n(int64(maxJitter)))
	return minBackoff + jitter
}

// Run is called to implement the action
func Run(ctx context.Context, step Step) error {
	if err := step.IsValid(); err != nil {
		return err
	}

	var cc client.Client
	if len(step.UploadFile) > 0 {
		ci, err := kubernetes.NewRuntimeClient(nil)
		if err != nil {
			return err
		}
		cc = ci
	}

	if step.Comment != "" {
		fmt.Printf(`
=======================================================
%s
=======================================================
`, strings.ToUpper(step.Comment))
	}

	if step.WaitFile != "" {
		log.WithFields(log.Fields{
			"is-failure": step.FailureFile,
			"on-error":   step.ErrorFile,
			"on-wait":    step.WaitFile,
			"timeout":    step.Timeout.String(),
		}).Info("waiting for signal to execute")

		err := utils.RetryWithTimeout(ctx, step.Timeout, time.Second, func() (bool, error) {
			if step.FailureFile != "" {
				if found, _ := utils.FileExists(step.ErrorFile); found {
					return false, errors.New("found error signal file, refusing to execute")
				}
			}
			if found, _ := utils.FileExists(step.WaitFile); found {
				return true, nil
			}

			return false, nil
		})
		if err != nil {
			return err
		}
	}

	for i, command := range step.Commands {
		attempt := 0
		var lastErr error

		for attempt <= step.RetryAttempts {
			if attempt > 0 {
				backoff := calculateBackoff(step.RetryMinBackoff, step.RetryMaxJitter)
				log.WithFields(log.Fields{
					"attempt": attempt,
					"command": i,
					"backoff": backoff,
				}).Info("retrying command")

				select {
				case <-ctx.Done():
					return ctx.Err()
				case <-time.After(backoff):
				}
			}

			//nolint:gosec
			cmd := exec.CommandContext(ctx, step.Shell, "-c", command)
			cmd.Env = os.Environ()

			logger := log.WithFields(log.Fields{
				"command": i,
				"attempt": attempt,
			})

			stdout, err := cmd.StdoutPipe()
			if err != nil {
				logger.WithError(err).Error("failed to acquire stdout pipe on command")
				return err
			}
			stderr, err := cmd.StderrPipe()
			if err != nil {
				logger.WithError(err).Error("failed to acquire stderr pipe on command")
				return err
			}

			//nolint:errcheck
			go io.Copy(os.Stdout, stdout)
			//nolint:errcheck
			go io.Copy(os.Stdout, stderr)

			if err := cmd.Start(); err != nil {
				logger.WithError(err).Error("failed to execute the command")
				lastErr = err
				attempt++
				continue
			}

			// @step: wait for the command to finish
			if err := cmd.Wait(); err != nil {
				logger.WithError(err).Error("command execution failed")
				lastErr = err
				attempt++
				continue
			}

			// Command succeeded, break the retry loop
			lastErr = nil
			break
		}

		// If we exhausted all retries and still have an error
		if lastErr != nil {
			if step.ErrorFile != "" {
				if err := utils.TouchFile(step.ErrorFile); err != nil {
					log.WithError(err).WithField("file", step.ErrorFile).Error("failed to create error file")
					return err
				}
			}
			return fmt.Errorf("command failed after %d attempts: %w", attempt, lastErr)
		}
	}
	log.Info("successfully executed the step")

	// @step: upload any files as kubernetes secrets
	for name, path := range step.UploadKeyPairs() {
		err := utils.Retry(ctx, 2, true, 5*time.Second, func() (bool, error) {
			err := uploadSecret(ctx, cc, step.Namespace, name, path)
			if err == nil {
				return true, nil
			}
			log.WithError(err).WithField("secret", name).Error("failed to upload secret")

			return false, nil
		})
		if err != nil {
			return err
		}
	}

	// @step: everything was good - lets touch the file
	if step.SuccessFile != "" {
		if err := utils.TouchFile(step.SuccessFile); err != nil {
			log.WithError(err).WithField("file", step.SuccessFile).Error("failed to create success file")

			return err
		}
	}

	return nil
}

// uploadSecret is used to create a kubernetes secret from a file
func uploadSecret(ctx context.Context, cc client.Client, namespace, name, path string) error {
	if found, err := utils.FileExists(path); err != nil {
		return err
	} else if !found {
		return fmt.Errorf("file %s does not exist", path)
	}

	// @step: read in the content of the file
	content, err := os.ReadFile(path)
	if err != nil {
		return err
	}

	secret := &v1.Secret{}
	secret.Namespace = namespace
	secret.Name = name

	found, err := kubernetes.GetIfExists(ctx, cc, secret)
	if err != nil {
		return err
	}
	// @step: define the secret
	if secret.Data == nil {
		secret.Data = make(map[string][]byte)
	}
	secret.Data[filepath.Base(path)] = content

	// @step: create or update the secret
	if !found {
		return cc.Create(ctx, secret)
	}

	return cc.Update(ctx, secret)
}
