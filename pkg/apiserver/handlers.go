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

package apiserver

import (
	"bufio"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"
	"time"

	log "github.com/sirupsen/logrus"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	terraformv1alphav1 "github.com/appvia/terranetes-controller/pkg/apis/terraform/v1alpha1"
	"github.com/appvia/terranetes-controller/pkg/utils"
	"github.com/appvia/terranetes-controller/pkg/utils/filters"
	"github.com/appvia/terranetes-controller/pkg/utils/kubernetes"
)

var sanitizeRegEx = regexp.MustCompile(`^[a-zA-Z0-9\-\.\:]{1,64}$`)

// validateInput checks the inputs parameter is valid
func validateInput(name, value string) error {
	switch {
	case value == "":
		return fmt.Errorf("%s is empty", name)

	case !sanitizeRegEx.MatchString(value):
		return fmt.Errorf("%s does not match input regex: %s", name, sanitizeRegEx)
	}

	return nil
}

// handleHealth is http handler for the health endpoint
func (s *Server) handleHealth(w http.ResponseWriter, req *http.Request) {
	_, _ = w.Write([]byte("OK\n"))
}

// handleBuilds is http handler for the logs endpoint
//nolint:errcheck
func (s *Server) handleBuilds(w http.ResponseWriter, req *http.Request) {
	params := []string{
		"generation",
		"name",
		"namespace",
		"stage",
		"uid",
	}
	values := make(map[string]string)
	for _, key := range params {
		value := req.URL.Query().Get(key)

		if err := validateInput(key, value); err != nil {
			log.WithError(err).Error("received an invalid request")

			w.WriteHeader(http.StatusBadRequest)

			return
		}
		values[key] = value
	}

	fields := log.Fields{
		"generation": values["generation"],
		"name":       values["name"],
		"namespace":  values["namespace"],
		"stage":      values["stage"],
		"uid":        values["uid"],
	}
	log.WithFields(fields).Debug("received request for builds")

	labels := []string{
		terraformv1alphav1.ConfigurationGenerationLabel + "=" + values["generation"],
		terraformv1alphav1.ConfigurationNameLabel + "=" + values["name"],
		terraformv1alphav1.ConfigurationNamespaceLabel + "=" + values["namespace"],
		terraformv1alphav1.ConfigurationStageLabel + "=" + values["stage"],
		terraformv1alphav1.ConfigurationUIDLabel + "=" + values["uid"],
	}

	var pod *v1.Pod

	// @step: try and find the pod running the terraform job: We have to assume also
	// the pods hasn't been scheduled yet
	w.Header().Set("Content-Type", "text/plain")

	w.Write([]byte("[info] waiting for the job to be scheduled\n"))
	w.Write([]byte(fmt.Sprintf("[info] watching build: %s, generation: %s for the job to be scheduled\n", values["name"], values["generation"])))

	// @step: we query the jobs using the labels and find the latest job for the configuration at stage x, generation y. We then
	// find the associated pods and stream the logs back to the caller
	err := utils.RetryWithTimeout(req.Context(), 60*time.Second, 2*time.Second, func() (bool, error) {
		w.Write([]byte("."))

		// @step: find the matching job
		list, err := s.Client.BatchV1().Jobs(s.Namespace).List(req.Context(), metav1.ListOptions{
			LabelSelector: strings.Join(labels, ","),
		})
		if err != nil {
			log.WithFields(fields).WithError(err).Error("failed to list the jobs")

			return false, nil
		}
		if len(list.Items) == 0 {
			log.WithFields(fields).Warn("no jobs found")

			return false, nil
		}

		latest, found := filters.Jobs(list).
			WithGeneration(values["generation"]).
			WithName(values["name"]).
			WithNamespace(values["namespace"]).
			WithStage(values["stage"]).
			WithUID(values["uid"]).
			Latest()
		if !found || latest == nil {
			log.WithFields(fields).Debug("no matching job found")

			return false, nil
		}
		log.WithFields(fields).Warn("found zero matching jobs for the build")

		// @step: find the latest pod associated to the job
		pods, err := s.Client.CoreV1().Pods(s.Namespace).List(req.Context(), metav1.ListOptions{
			LabelSelector: "job-name=" + latest.Name,
		})
		if err != nil {
			log.WithFields(fields).WithError(err).Error("failed to list the pods")

			return false, nil
		}
		if len(pods.Items) == 0 {
			log.WithFields(fields).Warn("no pods associated to the job")

			return false, nil
		}

		pod = kubernetes.FindLatestPod(pods)
		switch pod.Status.Phase {
		case v1.PodRunning, v1.PodSucceeded:
			return true, nil
		}

		return false, nil
	})
	if err != nil {
		log.WithFields(fields).WithError(err).Error("failed to find the pod")
		w.Write([]byte("[error] failed to find associated pod in time\n"))

		return
	}
	log.WithFields(fields).WithField("pod", pod.Name).Debug("found the pod")

	err = func() error {
		for _, container := range append(pod.Spec.InitContainers, pod.Spec.Containers...) {
			stream, err := s.Client.CoreV1().Pods(s.Namespace).GetLogs(pod.Name, &v1.PodLogOptions{
				Container: container.Name,
				Follow:    true,
			}).Stream(req.Context())
			if err != nil {
				return err
			}

			// @step: we either copy or flush line by line the output
			if flush, ok := w.(http.Flusher); !ok {
				if _, err := io.Copy(w, stream); err != nil {
					return err
				}
			} else {
				scanner := bufio.NewScanner(stream)
				for scanner.Scan() {
					w.Write([]byte(fmt.Sprintf("%s\n", scanner.Text())))
					flush.Flush()
				}
			}
			stream.Close()
		}

		return nil
	}()
	if err != nil {
		log.WithFields(fields).WithError(err).Error("failed to stream the logs")

		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("[error] failed to retrieve the logs\n"))

		return
	}

	w.Write([]byte("[build] completed\n"))
}
