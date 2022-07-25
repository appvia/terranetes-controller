/*
 * Copyright (C) 2022 Appvia Ltd <info@appvia.io>
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

package configuration

import (
	"context"
	"fmt"
	"strings"

	terraformv1alphav1 "github.com/appvia/terranetes-controller/pkg/apis/terraform/v1alpha1"
	"github.com/appvia/terranetes-controller/pkg/utils/jobs"
	"github.com/appvia/terranetes-controller/pkg/utils/kubernetes"
)

// checkovPolicyTemplate is the default template used to produce a checkov configuration
var checkovPolicyTemplate = `framework:
  - terraform_plan
soft-fail: true
compact: true
{{- if .Policy.Checks }}
check:
	{{- range $check := .Policy.Checks }}
  - {{ $check }}
  {{- end }}
{{- end }}
{{- if .Policy.SkipChecks }}
skip-check:
  {{- range .Policy.SkipChecks }}
  - {{ . }}
  {{- end }}
{{- end }}
{{- if .Policy.External }}
external-checks-dir:
  {{- range .Policy.External }}
  - /run/policy/{{ .Name }}
  {{- end }}
{{- end }}`

// GetTerraformImage is called to return the terraform image to use, or the image plus version
// override
func GetTerraformImage(configuration *terraformv1alphav1.Configuration, image string) string {
	if configuration.Spec.TerraformVersion == "" {
		return image
	}
	e := strings.Split(image, ":")

	return fmt.Sprintf("%s:%s", e[0], configuration.Spec.TerraformVersion)
}

// CreateWatcher is responsible for ensuring the logger is running in the application namespace
func (c Controller) CreateWatcher(ctx context.Context, configuration *terraformv1alphav1.Configuration, stage string) error {
	watcher := jobs.New(configuration, nil).NewJobWatch(c.ControllerNamespace, stage)

	// @step: check if the logger has been created
	found, err := kubernetes.GetIfExists(ctx, c.cc, watcher.DeepCopy())
	if err != nil {
		return err
	}
	if found {
		return nil
	}

	return c.cc.Create(ctx, watcher)
}
