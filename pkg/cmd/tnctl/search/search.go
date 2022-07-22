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

package search

import (
	"context"
	"errors"
	"os"
	"strings"
	"time"

	"github.com/AlecAivazis/survey/v2"
	"github.com/Masterminds/sprig/v3"
	"github.com/fatih/color"
	"github.com/manifoldco/promptui"
	"github.com/spf13/cobra"

	"github.com/appvia/terranetes-controller/pkg/cmd"
	"github.com/appvia/terranetes-controller/pkg/cmd/search"
	"github.com/appvia/terranetes-controller/pkg/cmd/search/github"
	"github.com/appvia/terranetes-controller/pkg/cmd/search/terraform"
	"github.com/appvia/terranetes-controller/pkg/cmd/tnctl/build"
	"github.com/appvia/terranetes-controller/pkg/utils"
)

var longSearchHelp = `
Searches the sources, determined by the configuration file (tnctl config view)
for modules which match the required terms. Once selected the command will
generate the Configuration CRD required to use the module as a source.

At present we support using the Terraform registry and GitHub user / organizations
as a source for terraform modules.

Add the terraform registry
$ tnctl config sources add https://registry.terraform.io

Scope the terraform registry searches to a specific namespace
$ tnctl config sources add https://registry.terraform.io/namespaces/appvia

Adding a GitHub user or organization
$ tnctl config sources add https://github.com/appvia

For private repositories on Github you will need to export your token
to the environment variable GITHUB_TOKEN.
$ export GITHUB_TOKEN=TOKEN

This command assumes credentials have already been setup. For the Terraform registry,
nothing is required, but for private repositories on Github your environment must
already be setup to git clone the repository.
`

// Command returns the cobra command for the "build" sub-command.
type Command struct {
	cmd.Factory
	// EnableDefaults indicates if any defaults with values from the terraform module are included
	EnableDefaults bool
	// Provider is the module provider
	Provider string
	// Name is the name of the resource
	Name string
	// Source is the registry source
	Source string
	// SourceNamespace is the module namespace
	SourceNamespace string
	// Query is the registry query string
	Query string
}

// NewCommand returns a new instance of the get command
func NewCommand(factory cmd.Factory) *cobra.Command {
	o := &Command{Factory: factory}

	c := &cobra.Command{
		Use:   "search [OPTIONS]",
		Short: "Searches for cloud resources to consume",
		Long:  longSearchHelp,
		RunE: func(cmd *cobra.Command, args []string) error {
			o.Query = strings.Join(args, " ")

			return o.Run(cmd.Context())
		},
	}

	flags := c.Flags()
	flags.BoolVar(&o.EnableDefaults, "enable-defaults", false, "Indicates any defaults with values from the terraform module are included")
	flags.StringVar(&o.Name, "name", "", "Is the name of the resource to create")
	flags.StringVar(&o.SourceNamespace, "source-namespace", "", "The namespace within the source registry to scope the search")
	flags.StringVarP(&o.Provider, "provider", "p", "", "Limit the search only to modules with the given provider")
	flags.StringVarP(&o.Source, "source", "s", "", "Limit the scope of the search to a specific source")

	cmd.RegisterFlagCompletionFunc(c, "provider", cmd.AutoCompleteWithList([]string{"aws", "azurerm", "google", "vsphere"}))
	cmd.RegisterFlagCompletionFunc(c, "source", func(cmd *cobra.Command, args []string, flagValue string) ([]string, cobra.ShellCompDirective) {
		config, found, err := o.GetConfig()
		if !found || err != nil {
			return []string{"https://registry.terraform.io"}, cobra.ShellCompDirectiveDefault
		}

		return config.Sources, cobra.ShellCompDirectiveDefault
	})

	return c
}

// Run implements the command action
func (o *Command) Run(ctx context.Context) error {
	config, found, err := o.GetConfig()
	if err != nil {
		return err
	}
	if !found || len(config.Sources) == 0 {
		config.Sources = []string{"https://registry.terraform.io"}
	}

	// @step: ensure we have something to search for
	if len(o.Query) == 0 {
		if err := survey.AskOne(&survey.Input{
			Message: "What resource are you looking to provision?",
			Help:    "The terms are used to match the description of the terraform module",
			Suggest: func(toComplete string) []string {
				return []string{"bucket", "database", "elasticsearch", "network", "rds", "registry"}
			},
		}, &o.Query, survey.WithKeepFilter(true)); err != nil {
			return err
		}
	}

	if o.Provider == "" {
		if err := survey.AskOne(&survey.Input{
			Message: "What cloud provider should we scope the search to?",
			Help:    "Scopes the search to providers of the specified type e.g aws, google, azure, etc",
			Suggest: func(toComplete string) []string {
				return []string{"aws", "azurerm", "google", "vsphere"}
			},
		}, &o.Provider, survey.WithKeepFilter(true)); err != nil {
			return err
		}
	}

	// @step: initialize and retrieve handlers for our configured sources
	handlers, err := o.makeSourceHandlers(ctx, config.Sources)
	if err != nil {
		return err
	}

	// @step: retrieve the results from each of our sources
	nctx, cancel := context.WithTimeout(ctx, time.Second*30)
	defer cancel()
	responses, err := o.findModules(nctx, handlers)
	if err != nil {
		return err
	}

	// @step: allow the user to choose the module
	module, found, err := o.chooseModule(nctx, responses)
	if err != nil {
		if err == promptui.ErrAbort || err == promptui.ErrInterrupt {
			return nil
		}
		return err
	}
	if !found {
		o.Println("No modules found")

		return nil
	}

	handler := handlers[module.Registry]

	if o.Name == "" {
		if err := survey.AskOne(&survey.Input{
			Message: "What should the name of the configuration resource be?",
			Help:    "This is the name of the configuration resource itself",
			Default: module.Name,
		}, &o.Name, survey.WithKeepFilter(true)); err != nil {
			return err
		}
	}

	// @step: allow the user to choose the version of the module
	module, err = o.chooseModuleVersion(ctx, module, handler)
	if err != nil {
		return err
	}

	// @step: we need to resolve the source of the module
	reference, err := handler.ResolveSource(ctx, module)
	if err != nil {
		return err
	}

	o.Println("%s Using terraform module: %s", cmd.IconGood, color.CyanString(reference))
	o.Println("%s Source: %s", cmd.IconGood, color.CyanString(module.Registry))

	provider := module.Provider
	if provider == "" {
		provider = o.Provider
	}

	return (&build.Command{
		Factory:        o.Factory,
		EnableDefaults: o.EnableDefaults,
		Name:           o.Name,
		Provider:       provider,
		Source:         reference,
	}).Run(ctx)
}

// chooseModule prompts the users to select the module to use
func (o *Command) chooseModule(_ context.Context, responses []search.Response) (search.Module, bool, error) {
	var modules []search.Module

	for _, x := range responses {
		if x.Error != nil {
			return search.Module{}, false, x.Error
		}
		modules = append(modules, x.Modules...)
	}

	if len(modules) == 0 {
		return search.Module{}, false, nil
	}

	searcher := func(input string, index int) bool {
		lower := strings.ToLower(input)
		module := modules[index]

		switch {
		case strings.Contains(strings.ToLower(module.Name), lower):
			return true
		case strings.Contains(strings.ToLower(module.Description), lower):
			return true
		case strings.Contains(strings.ToLower(module.Namespace), lower):
			return true
		}

		return false
	}

	methods := sprig.TxtFuncMap()
	for name, fn := range promptui.FuncMap {
		methods[name] = fn
	}

	// @step: allow the user to select the module
	templates := &promptui.SelectTemplates{
		Label:    promptui.IconInitial + " Which resource do you want to provision?",
		Active:   promptui.IconSelect + "[{{ .RegistryType }}] {{ .Namespace }}/{{ .Name | cyan }}",
		FuncMap:  methods,
		Inactive: " [{{ .RegistryType }}] {{ .Namespace }}/{{ .Name | cyan | faint }}",
		Details: `
{{ "Name:      " | faint | cyan }} {{ .Name }}
{{ "Namespace: " | faint | cyan }} {{ .Namespace }}
{{ "Module:    " | faint | cyan }} {{ .Source }}
{{ "Source:    " | faint | cyan }} {{ .Registry }}
{{ "Created:   " | faint | cyan }} {{ .CreatedAt }}
{{- if gt .Downloads 0 }}
{{ "Downloads: " | faint | cyan }} {{ .Downloads }}
{{- end }}
{{- if gt .Stars 0 }}
{{ "Stars:     " | faint | cyan }} {{ .Stars }}
{{- end }}

{{ .Description | wrap 80 }}
  `}

	prompt := promptui.Select{
		Label:        "Which module would you like to use?",
		HideSelected: true,
		HideHelp:     true,
		Items:        modules,
		Searcher:     searcher,
		Size:         15,
		Templates:    templates,
	}

	index, _, err := prompt.RunCursorAt(0, 0)
	if err != nil {
		return search.Module{}, false, err
	}

	return modules[index], true, nil
}

// chooseModuleVersion prompts the users to select the module version to use
func (o *Command) chooseModuleVersion(ctx context.Context, module search.Module, handler search.Interface) (search.Module, error) {
	versions, err := handler.Versions(ctx, module)
	if err != nil {
		return search.Module{}, err
	}

	switch {
	case len(versions) == 0:
		return search.Module{}, errors.New("no versions found for module")
	case len(versions) == 1:
		module.Version = versions[0]

		return module, nil
	}

	// @note: not all versions can be sorted - we will do best effort to sort them
	sorted, err := utils.SortSemverVersions(versions)
	if err == nil {
		versions = sorted
	}

	question := &survey.Select{
		Message:  "Which version of the module do you want to use?",
		Help:     "Select the tagged release of the module to use, note we default to the last release",
		Default:  versions[len(versions)-1],
		Options:  versions,
		PageSize: 15,
	}
	if err := survey.AskOne(question, &module.Version, nil); err != nil {
		return search.Module{}, err
	}

	return module, nil
}

// findModules is responsible for searching the sources for a collection of modules
func (o *Command) findModules(ctx context.Context, handlers map[string]search.Interface) ([]search.Response, error) {
	doneCh := make(chan search.Response)
	results := make([]search.Response, 0)

	query := search.Query{
		Namespace: o.SourceNamespace,
		Provider:  o.Provider,
		Query:     o.Query,
	}

	searchForModules := func(handler search.Interface) {
		now := time.Now()
		modules, err := handler.Find(ctx, query)
		doneCh <- search.Response{Modules: modules, Error: err, Time: time.Since(now)}
	}

	for _, x := range handlers {
		go searchForModules(x)
	}

	for {
		select {
		case <-ctx.Done():
			return nil, errors.New("context cancelled waiting for response")

		case response := <-doneCh:
			results = append(results, response)
			if len(results) == len(handlers) {
				return results, nil
			}
		}
	}
}

// makeSourceHandlers creates and returns a set of module handlers for use to use based on the
// users configuration file
func (o *Command) makeSourceHandlers(_ context.Context, sources []string) (map[string]search.Interface, error) {
	searches := make(map[string]search.Interface)

	for _, source := range sources {
		switch {
		case o.Source != "" && o.Source != source:
			continue

		case terraform.IsHandle(source):
			h, err := terraform.New(source)
			if err != nil {
				return nil, err
			}
			searches[source] = h

		case github.IsHandle(source):
			h, err := github.New(source, os.Getenv("GITHUB_TOKEN"))
			if err != nil {
				return nil, err
			}
			searches[source] = h
		}
	}

	return searches, nil
}
