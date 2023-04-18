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

package server

import (
	"bytes"
	"context"
	"fmt"
	"os"

	admissionv1 "k8s.io/api/admissionregistration/v1"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/appvia/terranetes-controller/pkg/register"
	"github.com/appvia/terranetes-controller/pkg/schema"
	"github.com/appvia/terranetes-controller/pkg/utils"
	"github.com/appvia/terranetes-controller/pkg/utils/kubernetes"
)

// registerWebhooks is responsible for registering the webhooks
func (s *Server) registerWebhooks(ctx context.Context) error {
	cc, err := client.New(s.cfg, client.Options{Scheme: schema.GetScheme()})
	if err != nil {
		return err
	}

	// @step: read the certificate authority
	ca, err := os.ReadFile(s.config.TLSAuthority)
	if err != nil {
		return fmt.Errorf("failed to read the certificate authority file, %w", err)
	}

	documents, err := utils.YAMLDocuments(bytes.NewReader(register.MustAsset("webhooks/manifests.yaml")))
	if err != nil {
		return fmt.Errorf("failed to decode the webhooks manifests, %w", err)
	}

	// @step: register the validating webhooks
	for _, x := range documents {
		o, err := schema.DecodeYAML([]byte(x))
		if err != nil {
			return fmt.Errorf("failed to decode the webhook, %w", err)
		}

		switch o := o.(type) {
		case *admissionv1.ValidatingWebhookConfiguration:
			for i := 0; i < len(o.Webhooks); i++ {
				o.Webhooks[i].ClientConfig.CABundle = ca
				o.Webhooks[i].ClientConfig.Service.Namespace = os.Getenv("KUBE_NAMESPACE")
				o.Webhooks[i].ClientConfig.Service.Name = "controller"
				o.Webhooks[i].ClientConfig.Service.Port = pointer.Int32(443)
			}

		case *admissionv1.MutatingWebhookConfiguration:
			for i := 0; i < len(o.Webhooks); i++ {
				o.Webhooks[i].ClientConfig.CABundle = ca
				o.Webhooks[i].ClientConfig.Service.Namespace = os.Getenv("KUBE_NAMESPACE")
				o.Webhooks[i].ClientConfig.Service.Name = "controller"
				o.Webhooks[i].ClientConfig.Service.Port = pointer.Int32(443)
			}

		default:
			return fmt.Errorf("expected a validating or mutating webhook, got %T", o)
		}

		if err := kubernetes.CreateOrForceUpdate(ctx, cc, o); err != nil {
			return fmt.Errorf("failed to create / update the webhook, %w", err)
		}
	}

	// @step: create a webhook for intercepting the namespaces
	decision := admissionv1.Fail
	sideEffects := admissionv1.SideEffectClassNone

	wh := &admissionv1.ValidatingWebhookConfiguration{}
	wh.Name = "validating-webhook-namespace"
	wh.Webhooks = []admissionv1.ValidatingWebhook{
		{
			AdmissionReviewVersions: []string{"v1"},
			ClientConfig: admissionv1.WebhookClientConfig{
				Service: &admissionv1.ServiceReference{
					Name:      "controller",
					Namespace: os.Getenv("KUBE_NAMESPACE"),
					Path:      pointer.String("/validate/terraform.appvia.io/namespaces"),
					Port:      pointer.Int32(443),
				},
				CABundle: ca,
			},
			FailurePolicy: &decision,
			Name:          "namespaces.terraform.appvia.io",
			Rules: []admissionv1.RuleWithOperations{
				{
					Operations: []admissionv1.OperationType{"DELETE"},
					Rule: admissionv1.Rule{
						APIGroups:   []string{""},
						APIVersions: []string{"v1"},
						Resources:   []string{"namespaces"},
					},
				},
			},
			SideEffects: &sideEffects,
		},
	}

	switch s.config.EnableNamespaceProtection {
	case true:
		if err := kubernetes.CreateOrForceUpdate(ctx, cc, wh); err != nil {
			return fmt.Errorf("failed to create / update the namespace webhook, %w", err)
		}
	default:
		if err := kubernetes.DeleteIfExists(ctx, cc, wh); err != nil {
			return fmt.Errorf("failed to delete the namespace webhook, %w", err)
		}
	}

	return nil
}
