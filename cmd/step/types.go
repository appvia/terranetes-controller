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
	"errors"
	"fmt"
	"strings"
	"time"
)

// Step represents a stage to run
type Step struct {
	// Commands is the commands and arguments to run
	Commands []string
	// RetryBackoff is the backoff time to retry ANY of the commands
	RetryBackoff time.Duration
	// RetryAttempts is the number of times to retry the commands before giving up
	RetryAttempts int
	// Comment adds a banner to the stage
	Comment string
	// ErrorFile is the path to a file which is created when the command failed
	ErrorFile string
	// FailureFile is the path to a file indicating failure
	FailureFile string
	// Namespace is the namespace to upload any files to as a secret
	Namespace string
	// Shell is the shell to execute the command in
	Shell string
	// SuccessFile is the path to a file which is created when the command ran successfully
	SuccessFile string
	// Timeout is the max time to wait on file before considering the run a failure
	Timeout time.Duration
	// UploadFile is file to upload on success of the command
	UploadFile []string
	// WaitFile is the path to a file which is wait for to run
	WaitFile string
}

// IsValid returns an error if the skip configuation is invalid
func (s Step) IsValid() error {
	switch {
	case len(s.Commands) == 0:
		return errors.New("no commands specified")

	case s.Timeout < 0:
		return errors.New("timeout must be greater than 0")

	case s.RetryAttempts < 0:
		return errors.New("retry attempts must be greater than or equal to 0")

	case s.RetryBackoff < 0:
		return errors.New("retry backoff must be greater than or equal to 0")

	case len(s.UploadFile) > 0 && s.Namespace == "":
		return errors.New("namespace must be specified when uploading files")

	case len(s.UploadFile) > 0:
		for _, x := range s.UploadFile {
			if e := strings.Split(x, "="); len(e) != 2 {
				return fmt.Errorf("upload file must be in the format 'key=path'")
			}
		}
	}

	return nil
}

// UploadKeyPairs returns a map of key pairs to upload
func (s Step) UploadKeyPairs() map[string]string {
	if len(s.UploadFile) == 0 {
		return nil
	}

	keys := make(map[string]string)
	for _, x := range s.UploadFile {
		if e := strings.Split(x, "="); len(e) == 2 {
			keys[e[0]] = e[1]
		}
	}

	return keys
}
