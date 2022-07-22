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

package terraform

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/appvia/terranetes-controller/pkg/cmd/search"
	"github.com/appvia/terranetes-controller/pkg/version"
)

type registry struct {
	// hc is the http client
	hc *http.Client
	// endpoint is the registry endpoint
	endpoint string
	// baseURL is the registry baseURL
	baseURL string
	// namespace scopes the requests to namespace
	namespace string
}

// New creates and returns a terraform registry lookup provider
func New(endpoint string) (search.Interface, error) {
	var namespace string

	u, err := url.Parse(endpoint)
	if err != nil {
		return nil, err
	}

	baseURL := fmt.Sprintf("https://%s", u.Host)

	if u.Path != "" {
		items := strings.Split(strings.TrimSuffix(u.Path, "/"), "/")

		switch {
		case len(items) != 3, items[1] != "namespaces":
			return nil, errors.New("invalid endpoint, supports only one path i.e. https://registry.terraform.io/namespaces/NAME")
		}
		namespace = items[2]
	}

	return &registry{
		baseURL:   baseURL,
		endpoint:  endpoint,
		hc:        &http.Client{},
		namespace: namespace,
	}, nil
}

//
// IsHandle returns true if we handle this source
func IsHandle(source string) bool {
	switch {
	case strings.HasPrefix(source, "terraform://"):
		return true

	case strings.HasPrefix(source, "https://registry.terraform.io"):
		return true
	}

	return false
}

// Source returns the source of the registry
func (r *registry) Source() string {
	return r.endpoint
}

// Versions returns a lists of version for a specific module
func (r *registry) Versions(ctx context.Context, module search.Module) ([]string, error) {
	location := fmt.Sprintf("%s/v1/modules/%s/%s/%s/versions", r.baseURL, module.Namespace, module.Name, module.Provider)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, location, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Accept-Language", "en-US,en;q=0.9")
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "terraform-controller/"+version.Version)

	resp, err := r.hc.Do(req)
	if err != nil {
		return nil, err
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	result := &versionsResult{}
	if err := json.NewDecoder(bytes.NewReader(body)).Decode(result); err != nil {
		return nil, err
	}

	var list []string
	for _, module := range result.Modules {
		for _, version := range module.Versions {
			list = append(list, version.Version)
		}
	}

	return list, nil
}

// ResolveSource is used to resolve the source of the module, this is only really required for
// terraform registries
func (r *registry) ResolveSource(ctx context.Context, module search.Module) (string, error) {
	location := fmt.Sprintf("%s/v1/modules/%s/%s/%s/%s/download",
		r.baseURL, module.Namespace, module.Name, module.Provider, module.Version)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, location, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Accept-Language", "en-US,en;q=0.9")
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "terraform-controller/"+version.Version)

	resp, err := r.hc.Do(req)
	if err != nil {
		return "", err
	}
	if resp.StatusCode != http.StatusNoContent {
		return "", fmt.Errorf("unexpected status code %d", resp.StatusCode)
	}

	source := resp.Header.Get("X-Terraform-Get")
	if source == "" {
		return "", fmt.Errorf("unexpected response, no X-Terraform-Get header")
	}

	return strings.Replace(source, "git::", "", -1), nil
}

// Find returns the terraform registry lookup provider
func (r *registry) Find(ctx context.Context, query search.Query) ([]search.Module, error) {
	var list []search.Module
	var offset int

	// @step: we override any namespace in the query
	if r.namespace != "" && query.Namespace == "" {
		query.Namespace = r.namespace
	}

	for {
		results, err := r.search(ctx, query, offset)
		if err != nil {
			return nil, err
		}

		for i := 0; i < len(results.Modules); i++ {
			// terraform registry is always public so we can remove the git protocol requirement
			source := strings.Replace(results.Modules[i].Source, "git::", "", -1)

			list = append(list, search.Module{
				ID:           results.Modules[i].ID,
				CreatedAt:    results.Modules[i].PublishedAt,
				Description:  results.Modules[i].Description,
				Downloads:    results.Modules[i].Downloads,
				Name:         results.Modules[i].Name,
				Namespace:    results.Modules[i].Namespace,
				Private:      false,
				Provider:     results.Modules[i].Provider,
				Registry:     r.endpoint,
				RegistryType: "TF",
				Source:       source,
				Version:      results.Modules[i].Version,
			})
		}

		if results.Meta.NextOffset != 0 && results.Meta.NextOffset >= offset {
			offset += results.Meta.NextOffset
		} else {
			break
		}
	}

	return list, nil
}

// search performs a search on a terraform registry
func (r *registry) search(ctx context.Context, query search.Query, offset int) (*searchResult, error) {
	location := fmt.Sprintf("%s/v1/modules", r.baseURL)
	if query.Query != "" {
		location = fmt.Sprintf("%s/search", location)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, location, nil)
	if err != nil {
		return nil, err
	}

	q := req.URL.Query()
	q.Set("offset", fmt.Sprintf("%d", offset))
	q.Set("limit", fmt.Sprintf("%d", 20))

	if query.Query != "" {
		q.Set("q", query.Query)
	}
	if query.Provider != "" {
		q.Set("provider", query.Provider)
	}
	if query.Namespace != "" {
		q.Set("namespace", query.Namespace)
	}
	req.URL.RawQuery = q.Encode()

	req.Header.Set("Accept", "application/json")
	req.Header.Set("Accept-Language", "en-US,en;q=0.9")
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "terraform-controller/"+version.Version)

	resp, err := r.hc.Do(req)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	results := new(searchResult)
	if err := json.NewDecoder(resp.Body).Decode(results); err != nil {
		return nil, err
	}

	return results, nil
}
