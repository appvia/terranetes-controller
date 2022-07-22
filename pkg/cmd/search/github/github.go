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

package github

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"strings"

	githubcc "github.com/google/go-github/v45/github"
	"golang.org/x/oauth2"

	"github.com/appvia/terranetes-controller/pkg/cmd"
	"github.com/appvia/terranetes-controller/pkg/cmd/search"
	"github.com/appvia/terranetes-controller/pkg/utils"
)

type ghClient struct {
	// endpoint is the github endpoint
	endpoint string
	// token is the github token
	token string
	// gc is the github client
	gc *githubcc.Client
	// namespace is the github namespace if any
	namespace string
}

var filter = regexp.MustCompile(`^terraform\-[\w]+\-[\w]+`)

// IsHandle returns true if the given string is a valid github handle
func IsHandle(source string) bool {
	switch {
	case strings.HasPrefix(source, "github.com/"), strings.HasPrefix(source, "https://github.com/"):
		return true
	}

	return false
}

// New creates and returns a github client
func New(endpoint, token string) (search.Interface, error) {
	switch {
	case endpoint == "":
		return nil, cmd.ErrMissingArgument("endpoint")
	}

	// @step: parse the endpoint
	u, err := url.Parse(endpoint)
	if err != nil {
		return nil, err
	}
	if u.Path == "" {
		return nil, errors.New("must must a user or organizations /user or /org")
	}
	if u.Scheme == "" {
		endpoint = fmt.Sprintf("https://%s", endpoint)
	}

	// @step: create the http client
	var hc *http.Client

	if token != "" {
		auth := oauth2.StaticTokenSource(&oauth2.Token{AccessToken: token})
		hc = oauth2.NewClient(context.Background(), auth)
	}

	return &ghClient{
		endpoint:  endpoint,
		gc:        githubcc.NewClient(hc),
		namespace: strings.TrimPrefix(u.Path, "/"),
		token:     token,
	}, nil
}

// Source returns the source of the given module
func (r *ghClient) Source() string {
	return r.endpoint
}

// ResolveSource returns the source of the given module
func (r *ghClient) ResolveSource(ctx context.Context, module search.Module) (string, error) {
	source := module.Source
	if module.Private {
		source = fmt.Sprintf("git::ssh://git@%s", strings.TrimPrefix(module.Source, "https://"))
	}

	return fmt.Sprintf("%s?ref=%s", source, module.Version), nil
}

// Find returns git repositories that match the given search term
func (r *ghClient) Find(ctx context.Context, query search.Query) ([]search.Module, error) {
	var modules []search.Module

	// @step: we need to check if the user is an organization or user
	user, resp, err := r.gc.Users.Get(ctx, r.namespace)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode == http.StatusNotFound {
		return nil, fmt.Errorf("%q not found", r.namespace)
	}

	var list []*githubcc.Repository

	switch user.GetType() == "Organization" {
	case true:
		list, err = r.searchByOrganization(ctx, query)
	default:
		list, err = r.searchByUser(ctx, query)
	}
	if err != nil {
		return nil, err
	}

	for i := 0; i < len(list); i++ {
		switch {
		case query.Namespace != "" && list[i].GetOwner().GetLogin() != query.Namespace:
			continue
		case !filter.MatchString(list[i].GetName()):
			continue
		case query.Query != "" && !containsTerms(query.Query, list[i]):
			continue
		}

		module := search.Module{
			CreatedAt:    list[i].GetCreatedAt().Time,
			Description:  list[i].GetDescription(),
			Downloads:    0,
			ID:           fmt.Sprintf("%d", list[i].GetID()),
			Name:         list[i].GetName(),
			Namespace:    list[i].GetOwner().GetLogin(),
			Private:      list[i].GetPrivate(),
			Registry:     r.endpoint,
			RegistryType: "GH",
			Source:       list[i].GetHTMLURL(),
			Stars:        list[i].GetStargazersCount(),
		}

		modules = append(modules, module)
	}

	return modules, nil
}

// Versions returns a list of tags from the module
func (r *ghClient) Versions(ctx context.Context, module search.Module) ([]string, error) {
	options := githubcc.ListOptions{
		PerPage: 100,
	}
	var list []*githubcc.RepositoryTag

	for {
		results, pager, err := r.gc.Repositories.ListTags(ctx, module.Namespace, module.Name, &options)
		if err != nil {
			return nil, err
		}
		list = append(list, results...)

		if pager.NextPage == 0 {
			break
		}
		options.Page = pager.NextPage
	}
	if len(list) == 0 {
		return nil, errors.New("no tags found in source repository")
	}

	var versions []string
	for i := 0; i < len(list); i++ {
		versions = append(versions, list[i].GetName())
	}

	return versions, nil
}

// searchByUser returns a list of repositories for the user
func (r *ghClient) searchByUser(ctx context.Context, _ search.Query) ([]*githubcc.Repository, error) {
	var list []*githubcc.Repository

	options := &githubcc.RepositoryListOptions{
		ListOptions: githubcc.ListOptions{PerPage: 100},
	}

	for {
		results, pager, err := r.gc.Repositories.List(ctx, r.namespace, options)
		if err != nil {
			return nil, err
		}
		list = append(list, results...)

		if pager.NextPage == 0 {
			break
		}
		options.Page = pager.NextPage
	}

	return list, nil
}

// searchByOrganization returns a list of repositories in the organization
func (r *ghClient) searchByOrganization(ctx context.Context, _ search.Query) ([]*githubcc.Repository, error) {
	var list []*githubcc.Repository

	options := &githubcc.RepositoryListByOrgOptions{
		ListOptions: githubcc.ListOptions{PerPage: 100},
		Type:        "all",
	}

	for {
		results, pager, err := r.gc.Repositories.ListByOrg(ctx, r.namespace, options)
		if err != nil {
			return nil, err
		}
		list = append(list, results...)

		if pager.NextPage == 0 {
			break
		}
		options.Page = pager.NextPage
	}

	return list, nil
}

// containsTerms returns true if the given string is contained in the repository terms
func containsTerms(query string, repostory *githubcc.Repository) bool {
	terms := strings.ToLower(strings.ReplaceAll(repostory.GetDescription(), ",", " "))
	terms = terms + " " + strings.ToLower(strings.Join(repostory.Topics, " "))
	lower := strings.ToLower(query)

	return utils.ContainsList(strings.Split(lower, " "), strings.Split(terms, " "))
}
