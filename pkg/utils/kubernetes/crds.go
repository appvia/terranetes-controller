/**
 * Copyright 2021 Appvia Ltd <info@appvia.io>
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package kubernetes

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"time"

	"github.com/ghodss/yaml"
	apiextv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	client "k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/rest"
)

// NewExtentionsAPIClient returns an extensions api client
func NewExtentionsAPIClient(cfg *rest.Config) (client.Interface, error) {
	return client.NewForConfig(cfg)
}

// ApplyCustomResourceRawDefinitions reads the definitions from the raw bytes
func ApplyCustomResourceRawDefinitions(ctx context.Context, cc client.Interface, raw []byte) error {
	var list []*apiextv1.CustomResourceDefinition

	for _, document := range regexp.MustCompile("(?m)^---\n").Split(string(raw), -1) {
		if document == "" {
			continue
		}

		encoded, err := yaml.YAMLToJSON([]byte(document))
		if err != nil {
			return err
		}

		crd := &apiextv1.CustomResourceDefinition{}
		if err := json.Unmarshal(encoded, crd); err != nil {
			return err
		}

		list = append(list, crd)
	}

	return ApplyCustomResourceDefinitions(ctx, cc, list)
}

// ApplyCustomResourceDefinitions s responsible for applying a collection of CRDs
func ApplyCustomResourceDefinitions(ctx context.Context, c client.Interface, list []*apiextv1.CustomResourceDefinition) error {
	for _, crd := range list {
		if err := ApplyCustomResourceDefinition(ctx, c, crd); err != nil {
			return err
		}
	}

	return nil
}

// ApplyCustomResourceDefinition is responsible for applying the CRD to the cluster
func ApplyCustomResourceDefinition(ctx context.Context, c client.Interface, crd *apiextv1.CustomResourceDefinition) error {
	err := func() error {
		current, err := c.ApiextensionsV1().CustomResourceDefinitions().Get(ctx, crd.Name, metav1.GetOptions{})
		if err != nil {
			if !errors.IsNotFound(err) {
				return err
			}

			_, err := c.ApiextensionsV1().CustomResourceDefinitions().Create(ctx, crd, metav1.CreateOptions{})

			return err
		}

		crd.SetGeneration(current.GetGeneration())
		crd.SetResourceVersion(current.GetResourceVersion())

		_, err = c.ApiextensionsV1().CustomResourceDefinitions().Update(ctx, crd, metav1.UpdateOptions{})

		return err
	}()
	if err != nil {
		return err
	}

	return CheckCustomResourceDefinition(ctx, c, crd)
}

// CheckCustomResourceDefinition ensures the CRD is ok to go
func CheckCustomResourceDefinition(ctx context.Context, c client.Interface, crd *apiextv1.CustomResourceDefinition) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	doneCh := make(chan struct{})

	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			default:
			}

			err := func() error {
				// @step: ensure the crd has been registered
				obj, err := c.ApiextensionsV1().CustomResourceDefinitions().Get(ctx, crd.Name, metav1.GetOptions{})
				if err != nil {
					return err
				}
				if len(obj.Status.Conditions) < 2 {
					return fmt.Errorf("waiting for crd conditions to reach 2")
				}
				for _, x := range obj.Status.Conditions {
					if x.Status != "True" {
						return fmt.Errorf("condition not met, reason: %s", x.Reason)
					}
				}
				time.Sleep(100 * time.Millisecond)

				return nil
			}()
			if err == nil {
				doneCh <- struct{}{}
				return
			}
		}
	}()

	select {
	case <-doneCh:
		return nil
	case <-time.After(10 * time.Second):
		return fmt.Errorf("failed to register the crd")
	}
}
