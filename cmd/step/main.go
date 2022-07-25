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
	"os"
	"os/exec"
	"strings"
	"time"

	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"sigs.k8s.io/controller-runtime/pkg/manager/signals"

	"github.com/appvia/terranetes-controller/pkg/utils"
	"github.com/appvia/terranetes-controller/pkg/version"
)

// Step represents a stage to run
type Step struct {
	// Commands is the commands and arguments to run
	Commands []string
	// Comment adds a banner to the stage
	Comment string
	// UploadFile is file to upload on success of the command

	// ErrorFile is the path to a file which is created when the command failed
	ErrorFile string
	// FailureFile is the path to a file indicating failure
	FailureFile string
	// Shell is the shell to execute the command in
	Shell string
	// SuccessFile is the path to a file which is created when the command ran successfully
	SuccessFile string
	// Timeout is the max time to wait on file before considering the run a failure
	Timeout time.Duration
	// WaitFile is the path to a file which is wait for to run
	WaitFile string
}

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
	flags.StringVar(&step.SuccessFile, "on-success", "", "The path of the file used to indicate the step was successful")
	flags.StringVarP(&step.Shell, "shell", "s", "/bin/sh", "The shell to execute the command in")
	flags.StringVar(&step.FailureFile, "is-failure", "", "The path of the file used to indicate failure above")
	flags.StringVar(&step.WaitFile, "wait-on", "", "The path to a file to indicate this step can be run")
	flags.StringSliceVarP(&step.Commands, "command", "c", []string{}, "Command to execute")

	if err := cmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "[Error] %s\n", err)

		os.Exit(1)
	}
}

// Run is called to implement the action
func Run(ctx context.Context, step Step) error {
	if len(step.Commands) == 0 {
		return errors.New("no commands defined")
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
		cmd := exec.CommandContext(ctx, step.Shell, "-c", command)
		cmd.Env = os.Environ()

		logger := log.WithField("command", i)

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

			return err
		}

		// @step: wait for the command to finish
		if err := cmd.Wait(); err != nil {
			logger.WithError(err).Error("failed to execute command successfully")

			if step.ErrorFile != "" {
				if err := utils.TouchFile(step.ErrorFile); err != nil {
					logger.WithError(err).WithField("file", step.ErrorFile).Error("failed to create error file")

					return err
				}
			}

			return err
		}
	}
	log.Info("successfully executed the step")

	// @step: everything was good - lets touch the file
	if step.SuccessFile != "" {
		if err := utils.TouchFile(step.SuccessFile); err != nil {
			log.WithError(err).WithField("file", step.SuccessFile).Error("failed to create success file")

			return err
		}
	}

	return nil
}
