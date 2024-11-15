/*
 * Copyright (C) 2023  Appvia Ltd <info@appvia.io>
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

package verify

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/enescakir/emoji"
	"github.com/spf13/cobra"
	"github.com/tidwall/gjson"
	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/utils/ptr"

	terraformv1alpha1 "github.com/appvia/terranetes-controller/pkg/apis/terraform/v1alpha1"
	"github.com/appvia/terranetes-controller/pkg/cmd"
	"github.com/appvia/terranetes-controller/pkg/cmd/tnctl/convert"
	"github.com/appvia/terranetes-controller/pkg/utils"
	"github.com/appvia/terranetes-controller/pkg/utils/kubernetes"
	"github.com/appvia/terranetes-controller/pkg/utils/policies"
	"github.com/appvia/terranetes-controller/pkg/utils/terraform"
)

var longDescription = `
Performs a series of checks against the Revision to ensure the configuration is
valid and it will work with the within the cluster. This command uses the current
kubeconfig context to retrieve details such as Provider/s, Policies and Contexts.

Verify the revision will work in the cluster
$ tnctl verify revision revision.yaml

We can also include additional files such as Contexts, Policies and Plans. This can
be useful if you want to test a revision against a specific context or policy, before
applying it to the cluster.
$ tnctl verify revision revision.yaml --source-dir /path/to/files

When validating the module against the Checkov security policy, by default you
scan the module rather than the terraform plan. While the module scan does pick
many issues some validation errors will only appear during the plan stage. You
should consider using the '--use-terraform-plan' flag. Note, this requires
you have the appropriate cloud credentials configured within your terminal
environment.
$ tnctl verify revision revision.yaml --use-terraform-plan

To speed up multiple iterations of this command it's useful to use the --directory
flag. This instructs the command to reuse the directory, rather then creating a
an ephemeral one each time (and downloading, terraform provider, if --use-terraform-plan
is enabled, and so forth). Note, the --directory flag will create files in the
directory, so ensure there's no terraform files already there.
$ tnctl verify revision revision.yaml --directory /path/to/directory

Once verification has completed, you can continue to assure the Revision by running
it against terraform itself
$ tnctl convert revision revision.yaml | terraform plan -out plan.out
`

// RevisionCommand are the options for the command
type RevisionCommand struct {
	cmd.Factory
	// File is the path to the file to verify
	File string
	// SourceDir is the directory used to include additional files
	SourceDir string
	// CheckovImage is the version of checkov image to use when validating the security policy
	CheckovImage string
	// TerraformImage is the version of terraform to use when validating the security policy
	TerraformImage string
	// Directory is the temporary directory used to store the converted files
	Directory string
	// EnableCluster indicates we should not retrieve configuration from the current kubeconfig
	EnableCluster bool
	// EnableTerraformPlan indicates we should use a terraform plan to verify the security policy.
	// Note, this does require credentials to be configured
	EnableTerraformPlan bool
	// ShowGuidelines indicates we should show the guidelines in the output 
	ShowGuidelines bool
	// Contexts is a list of contexts from the cluster
	Contexts *terraformv1alpha1.ContextList
	// Policies is a list of policies from the cluster
	Policies *terraformv1alpha1.PolicyList
	// Providers is a collection of providers in the cluster
	Providers *terraformv1alpha1.ProviderList
	// Check is a collection of checks we performed
	Verify *CheckResult
	// KeepTempDir indicates we should not remove the temporary directory
	KeepTempDir bool
}

// NewRevisionCommand creates a new command
func NewRevisionCommand(factory cmd.Factory) *cobra.Command {
	o := &RevisionCommand{Factory: factory}

	command := &cobra.Command{
		Use:          "revision [OPTIONS] FILE",
		Short:        "Performs a series of checks against a Revision to ensure it is ready for use",
		Long:         longDescription,
		Args:         cobra.MinimumNArgs(1),
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			o.File = args[0]

			if cmd.Flags().Changed("directory") {
				o.KeepTempDir = true
			}

			return o.Run(cmd.Context())
		},
	}

	flags := command.Flags()
	flags.BoolVar(&o.EnableCluster, "use-cluster", true, "Indicates if we should retrieve configuration from the current kubeconfig")
	flags.BoolVar(&o.EnableTerraformPlan, "use-terraform-plan", false, "Indicates if we should use a terraform plan to verify the security policy")
	flags.BoolVar(&o.KeepTempDir, "keep-temp-dir", false, "Indicates if we should keep the temporary directory")
	flags.BoolVar(&o.ShowGuidelines, "show-guidelines", true, "Indicates if we should show the guidelines in the output")
	flags.StringVar(&o.CheckovImage, "checkov-image", "", "The docker image of checkov to use when validating the security policy")
	flags.StringVar(&o.TerraformImage, "terraform-image", "", "The docker image of terraform to use when generating a plan")
	flags.StringVarP(&o.Directory, "directory", "d", "", "Path to a directory to store temporary files")
	flags.StringVarP(&o.SourceDir, "source-dir", "s", "", "Path to a directory containing additional (or overrides) files i.e. Contexts, Policies, Plans etc")

	_ = flags.MarkHidden("keep-temp-dir")

	return command
}

// Run runs the command
func (o *RevisionCommand) Run(ctx context.Context) error {
	o.Verify = NewCheckResult(o.Stdout())

	revision := &terraformv1alpha1.Revision{}
	// @step: load the cloudresource from the file
	if err := utils.LoadYAML(o.File, revision); err != nil {
		return err
	}

	// @step: we need to source any additional files
	if err := o.sourceFiles(); err != nil {
		return err
	}
	// @step: retrieve configuration from the current cluster
	if err := o.sourceFromCluster(ctx); err != nil {
		return err
	}
	// @step: we need to convert the revision to a configuration so we can
	// validate as a later stage
	if err := o.convertRevision(ctx, revision); err != nil {
		return err
	}
	defer func() {
		if !o.KeepTempDir {
			_ = os.RemoveAll(o.Directory)
		} else {
			o.Println("Keeping temporary directory: %s", o.Directory)
		}
	}()

	_ = o.Verify.Check("Validating Revision Syntax", func(o CheckInterface) error {
		o.Passed("The Revision CRD is syntactically correct with no errors found")
		return nil
	})

	// @step: check the revision spec
	if err := o.checkRevisionSpec(revision); err != nil {
		return err
	}
	// @step: check the revision inputs
	if err := o.checkRevisionInputs(revision); err != nil {
		return err
	}
	// @step: check for the checkov policy version
	if err := o.retrieveCheckovVersion(ctx); err != nil {
		return err
	}
	// @step: check for the terraform version
	if err := o.retrieveTerraformVersion(ctx); err != nil {
		return err
	}
	// @step: check if the cloudresource is permitted by the policy
	if err := o.checkModuleSecurityPolicy(revision); err != nil {
		return err
	}
	// @step: check if the resource has a provider
	if err := o.checkProvider(revision); err != nil {
		return err
	}
	// @step: check if any value from exists in cluster
	if err := o.checkValueFromReferences(revision); err != nil {
		return err
	}
	// @step: generate the terraform plan if requested
	if err := o.checkTerraformPlan(ctx, revision); err != nil {
		return err
	}
	// @step: check if policy is defined for, it will pass
	if err := o.checkSecurityPolicy(ctx); err != nil {
		return err
	}
	// @step: print a summary of the checks
	if err := o.retrieveSummary(); err != nil {
		return err
	}

	if o.Verify.FailedCount() > 0 {
		return errors.New("revision failed verification checks")
	}

	return nil
}

// retrieveSummary returns a summary of the checks performed
// nolint:unparam
func (o *RevisionCommand) retrieveSummary() error {
	o.Println("\n%v Passed: %d, Warning: %d",
		emoji.GreenCircle, o.Verify.PassedCount(), o.Verify.WarningCount())

	if o.Verify.FailedCount() > 0 {
		o.Println("%v Failed: %d",
			emoji.RedCircle,
			o.Verify.FailedCount(),
		)
	}

	return nil
}

// checkRevisionSpec is responsible for checks on the revision spec
func (o *RevisionCommand) checkRevisionSpec(revision *terraformv1alpha1.Revision) error {
	return o.Verify.Check("Validating Revision Specification", func(o CheckInterface) error {
		if len(revision.Spec.Plan.Categories) == 0 {
			o.Warning("The Revision does not have any categories defined")
		} else {
			o.Passed("The Revision has categories defined")
		}

		if len(revision.Spec.Plan.Description) == 0 {
			o.Failed("The Revision does not have a description defined")
		} else {
			o.Passed("The Revision has a description defined")
		}

		if revision.Spec.Plan.Description == "ADD DESCRIPTION" {
			o.Warning("The Revision has the default description defined")
		}

		if len(revision.Spec.Plan.ChangeLog) == 0 {
			o.Warning("You should consider adding a changelog to the spec.plan definition")
		} else {
			o.Passed("The Revision has a changelog defined")
		}
		if revision.Spec.Plan.ChangeLog == "ADD CHANGELOG" {
			o.Warning("The Revision has the default changelog defined")
		}

		if len(revision.Spec.Plan.Name) == 0 {
			o.Failed("The Revision does not have a spec.plan.name defined")
		}

		return nil
	})
}

// retrieveCheckovVersion attempts to retrieve the version of checkov being used in the cluster, else
// if the cluster is unavailable, we default to latest
// nolint:dupl
func (o *RevisionCommand) retrieveCheckovVersion(ctx context.Context) error {
	if o.CheckovImage != "" {
		return nil
	}
	o.CheckovImage = "latest"

	return o.Verify.Check("Retrieving Checkov Version", func(v CheckInterface) error {
		latest := "Unable to discover Checkov version from cluster, defaulting to latest"

		// @step: retrieve a client
		cc, err := o.GetClient()
		if err != nil {
			v.Warning(latest)

			return nil
		}

		// @step: attempt to retrieve the checkov version
		controller := &appsv1.Deployment{}
		controller.Namespace = "terraform-system"
		controller.Name = "terranetes-controller"

		if found, err := kubernetes.GetIfExists(ctx, cc, controller); err != nil || !found {
			v.Warning(latest)
			return nil
		}
		if len(controller.Spec.Template.Spec.Containers) == 0 {
			v.Warning(latest)
			return nil
		}

		var found bool
		for _, x := range controller.Spec.Template.Spec.Containers[0].Args {
			switch {
			case !strings.Contains(x, "--policy-image="):
				continue
			case len(strings.Split(x, "=")) != 2:
				continue
			}
			o.CheckovImage = strings.Split(x, "=")[1]
			found = true
		}
		if !found {
			v.Info("Unable to discover Checkov version from cluster, using: %q", o.CheckovImage)
		} else {
			v.Passed("Discovered Checkov version: %q", o.CheckovImage)
		}

		return nil
	})
}

// retrieveTerraformVersion is responsible for retrieving the terraform version from the cluster
// nolint:dupl
func (o *RevisionCommand) retrieveTerraformVersion(ctx context.Context) error {
	if o.TerraformImage != "" {
		return nil
	}
	o.TerraformImage = "latest"

	return o.Verify.Check("Retrieving Terraform Version", func(v CheckInterface) error {
		latest := "Unable discover Terraform version from cluster, defaulting to latest"

		// @step: retrieve a client
		cc, err := o.GetClient()
		if err != nil {
			v.Warning(latest)

			return nil
		}

		// @step: attempt to retrieve the checkov version
		controller := &appsv1.Deployment{}
		controller.Namespace = "terraform-system"
		controller.Name = "terranetes-controller"

		if found, err := kubernetes.GetIfExists(ctx, cc, controller); err != nil || !found {
			v.Warning(latest)
			return nil
		}
		if len(controller.Spec.Template.Spec.Containers) == 0 {
			v.Warning(latest)
			return nil
		}

		var found bool
		for _, x := range controller.Spec.Template.Spec.Containers[0].Args {
			switch {
			case !strings.Contains(x, "--terraform-image="):
				continue
			case len(strings.Split(x, "=")) != 2:
				continue
			}
			o.TerraformImage = strings.Split(x, "=")[1]
			found = true
		}
		if !found {
			v.Info("Unable to discover Terraform version from cluster, using: %q", o.TerraformImage)
		} else {
			v.Passed("Discovered Terraform version: %q", o.TerraformImage)
		}

		return nil
	})
}

// checkRevisionInputs is responsible for checking the inputs
func (o *RevisionCommand) checkRevisionInputs(revision *terraformv1alpha1.Revision) error {
	return o.Verify.Check("Validating Revision Inputs", func(v CheckInterface) error {
		for i, input := range revision.Spec.Inputs {
			v.Info("Checking input: %s", input.Key)

			if input.Type == nil {
				v.Warning("Input (spec.inputs[%d]) does not have a type defined", i)
			}
			if input.Description == "" {
				v.Failed("Input (spec.inputs[%d].description) does not have a description defined", i)
			}
			if input.Key == "" {
				v.Failed("Input (spec.inputs[%d].key) does not have a key defined", i)
			}
			if input.Default == nil && ptr.Deref(input.Required, false) {
				v.Warning("Input (spec.inputs[%d].default) does not have a default defined", i)
			}
		}

		return nil
	})
}

// checkTerraformPlan is responsible for producing a terraform plan for checkov to
// validate against. Plans provide a better way to validate what will actually happen.
// The issue with using a plan is that it requires credentials to the cloud vendor, as
// there is no consistent way to produce a plan without them.
// AWS https://stackoverflow.com/questions/54269578/terraform-run-plan-without-aws-credentials
// But can't be done for GCP
func (o *RevisionCommand) checkTerraformPlan(ctx context.Context, revision *terraformv1alpha1.Revision) error {
	switch {
	case !o.EnableTerraformPlan:
		return nil
	case revision.Spec.Configuration.ProviderRef == nil:
		return nil
	case revision.Spec.Configuration.ProviderRef.Name == "":
		return nil
	case o.Providers == nil || len(o.Providers.Items) == 0:
		return nil
	}

	return o.Verify.Check("Validating Security Policies against Terraform Plan", func(v CheckInterface) error {
		v.Info("Checking security policies against terraform plan")

		// @step: we need to inject the environment variables for the provider
		provider, found := o.Providers.GetItem(revision.Spec.Configuration.ProviderRef.Name)
		if !found {
			v.Failed("Unable to find the provider: %s, cannot verify security policy", revision.Spec.Configuration.ProviderRef.Name)

			return nil
		}

		// @step: first we need to perform a terraform init
		options := []string{
			"run", "--interactive",
			"--user", fmt.Sprintf("%d", os.Getuid()),
			"--volume", fmt.Sprintf("%s:/source", o.Directory),
			"--workdir", "/source",
			o.TerraformImage,
			"init", "-lock=false",
		}
		cmd := exec.CommandContext(ctx, "docker", options...)

		combined, err := cmd.CombinedOutput()
		if err != nil {
			v.Failed("Unable to perform terraform init: %s", string(combined))

			return nil
		}
		v.Info("Successfully performed terraform init on the source")

		v.Info("Attempting to generate terraform plan from Revision")
		// @step: now we need to perform a terraform plan
		options = []string{"run", "--interactive", "--rm"}
		switch provider.Spec.Provider {
		case terraformv1alpha1.AWSProviderType:
			for _, x := range []string{"AWS_ACCESS_KEY_ID", "AWS_SECRET_ACCESS_KEY"} {
				if value := os.Getenv(x); value == "" {
					v.Failed("Missing: %s, in environment", x)

					return nil
				}
				options = append(options, fmt.Sprintf("--env=%s", x))
			}
			for _, x := range []string{"AWS_DEFAULT_REGION", "AWS_REGION", "AWS_SESSION_TOKEN"} {
				if value := os.Getenv(x); value != "" {
					options = append(options, fmt.Sprintf("--env=%s", x))
				}
			}

		case terraformv1alpha1.GCPProviderType:
			optional := []string{
				"GCLOUD_KEYFILE_JSON",
				"GOOGLE_APPLICATION_CREDENTIALS",
				"GOOGLE_CLOUD_KEYFILE_JSON",
				"GOOGLE_CREDENTIALS",
			}
			for _, x := range optional {
				if value := os.Getenv(x); value != "" {
					options = append(options, fmt.Sprintf("--env=%s", x))
				}
			}

		case terraformv1alpha1.AzureProviderType:
		default:
			mandatory := []string{"ARM_CLIENT_ID", "ARM_CLIENT_SECRET", "ARM_SUBSCRIPTION_ID", "ARM_TENANT_ID"}
			for _, x := range mandatory {
				if value := os.Getenv(x); value == "" {
					v.Failed("Missing: %s, in environment", x)
					return nil
				}
				options = append(options, fmt.Sprintf("--env=%s", x))
			}

			return nil
		}

		options = append(options, []string{
			"--user", fmt.Sprintf("%d", os.Getuid()),
			"--volume", fmt.Sprintf("%s:/source", o.Directory),
			"--workdir", "/source",
			o.TerraformImage,
			"plan", "-no-color", "-out=plan.tfplan", "-refresh=false", "-lock=false",
		}...)

		cmd = exec.CommandContext(ctx, "docker", options...)
		cmd.Env = os.Environ()
		combined, err = cmd.CombinedOutput()
		if err != nil {
			v.Failed("Unable to generate terraform plan, error: %s, output: \n%s", err, string(combined))

			return nil
		}
		v.Info("Successfully generated a terraform plan from Revision")

		v.Info("Converting terraform plan to json for Checkov to verify")
		options = []string{
			"run", "--interactive", "--rm",
			"--user", fmt.Sprintf("%d", os.Getuid()),
			"--volume", fmt.Sprintf("%s:/source", o.Directory),
			"--workdir", "/source",
			"--entrypoint", "sh",
			o.TerraformImage,
			"-c", "terraform show -json plan.tfplan > plan.json",
		}

		cmd = exec.CommandContext(ctx, "docker", options...)
		combined, err = cmd.CombinedOutput()
		if err != nil {
			v.Failed("Unable to convert terraform plan, error: %s, output: %s", err, string(combined))

			return nil
		}

		return nil
	})
}

// checkSecurityPolicy checks if the revision is permitted by the policy
func (o *RevisionCommand) checkSecurityPolicy(ctx context.Context) error {
	return o.Verify.Check("Validating against Checkov Security Policy", func(v CheckInterface) error {
		switch {
		case o.Policies == nil:
			fallthrough
		case len(policies.FindSecurityPolicyConstraints(o.Policies)) == 0:
			fallthrough
		case len(o.Policies.Items) == 0:
			v.Warning("No Checkov Security Policies found")

			return nil
		}
		constraints := policies.FindSecurityPolicyConstraints(o.Policies)

		// @step: check we can find docker binary
		if path, err := exec.LookPath("docker"); err != nil {
			v.Warning("Unable to check for docker binary: %w, used for checkov policy validation", err)

			return nil
		} else if path == "" {
			v.Warning("Unable to find docker binary, used for checkov policy validation")
		}

		// @step: are we running against a plan or source
		framework := "terraform"
		if o.EnableTerraformPlan {
			framework = "terraform_plan"
		} else {
			v.Warning("Checkov is using the code, not the plan, consider --use-terraform-plan")
		}

		v.Info("Found %d security policies to validate against", len(constraints))
		for _, x := range constraints {
			// @step: we need to generate the checkov configuration
			checkov, err := terraform.NewCheckovPolicy(map[string]interface{}{
				"Framework": framework,
				"Policy":    x.Spec.Constraints.Checkov,
			})
			if err != nil {
				return err
			}

			// @step: we need write the checkov configuration
			err = os.WriteFile(filepath.Join(o.Directory, ".checkov.yml"), []byte(checkov), 0600)
			if err != nil {
				return err
			}

			options := []string{
				"run", "--interactive",
				"--user", fmt.Sprintf("%d", os.Getuid()),
				"--volume", fmt.Sprintf("%s:/source", o.Directory),
				"--workdir", "/source",
				o.CheckovImage,
				"--directory", "/source",
				"--framework", framework,
				"--download-external-module", "true",
				"--repo-root-for-plan-enrichment", "/source",
				"--output", "json",
				"--output-file-path", "/source",
			}
			cmd := exec.CommandContext(ctx, "docker", options...)

			combined, err := cmd.CombinedOutput()
			if err != nil {
				return fmt.Errorf("failed to run checkov: %s, error: %w", string(combined), err)
			}
			filename := filepath.Join(o.Directory, "results_json.json")

			// @step: read in the results from checkov
			results, err := os.ReadFile(filename)
			if err != nil {
				return err
			}

			// @step: lets start by processing the passed results
			passed := gjson.GetBytes(results, "results.passed_checks")
			if passed.Exists() && passed.IsArray() {
				v.Passed("Revision has passed %d checks in policy: %q", len(passed.Array()), x.Name)
			}

			// @step: lets start by processing the failed results
			failed := gjson.GetBytes(results, "results.failed_checks")
			if failed.Exists() && failed.IsArray() {
				if len(failed.Array()) > 0 {
					v.Info("Checks: https://www.checkov.io/5.Policy%%20Index/all.html")
				}

				for _, check := range failed.Array() {
					v.Failed("%v", utils.MaxChars(check.Get("check_name").String(), 69))
					if check.Get("check_id").String() != "" {
						v.Additional("Check ID: %v", check.Get("check_id"))
					}
					if check.Get("resource").String() != "" {
						v.Additional("Resource: %v", check.Get("resource"))
					}
		      if o.ShowGuidelines && check.Get("guideline").String() != "" {
						v.Additional("Guideline: %v", check.Get("guideline"))
					}
				}
			}

			if len(failed.Array()) > 0 {
				v.Failed("Revision will fail on security policy: %q", x.Name)
			} else {
				v.Passed("Revision is permitted by the policy: %q", x.Name)
			}
		}

		return nil
	})
}

// checkValueFromReferences checks if any valueFrom references exist in the cluster
func (o *RevisionCommand) checkValueFromReferences(revision *terraformv1alpha1.Revision) error {
	return o.Verify.Check("Validating of Context References", func(v CheckInterface) error {
		switch {
		case revision.Spec.Configuration.ValueFrom == nil:
			fallthrough
		case len(revision.Spec.Configuration.ValueFrom) == 0:
			v.Passed("Revision does not reference any values from context/s")

			return nil
		}

		for _, x := range revision.Spec.Configuration.ValueFrom {
			if o.Contexts == nil {
				v.Warning("Revision references a context: %q, key: %q, but none available to check against", *x.Context, x.Key)

				continue
			}

			txt, found := o.Contexts.GetItem(*x.Context)
			if !found {
				v.Failed("Revision references a context %q which does not exist", *x.Context)
			} else {
				v.Passed("Revision references a context %q, key %q which exists", txt.Name, x.Key)

				if _, found := txt.Spec.Variables[x.Key]; !found {
					v.Failed("Revision references key %q, in Context %q which does not exist", x.Key, *x.Context)
				}
			}
		}

		return nil
	})
}

// checkProvider checks if the provider is defined in the cluster
func (o *RevisionCommand) checkProvider(revision *terraformv1alpha1.Revision) error {
	return o.Verify.Check("Validating Cloud Credentials Provider", func(v CheckInterface) error {
		v.Info("Checking if we providers associated with the revision")

		switch {
		case revision.Spec.Configuration.ProviderRef == nil:
			v.Skipped("Revision does not have a provider defined")

			return nil

		case o.Providers == nil:
			v.Warning("No providers were found in the cluster or sources directory")

		case !o.Providers.HasItem(revision.Spec.Configuration.ProviderRef.Name):
			v.Warning("Provider %q not found or skipped (use-cluster)", revision.Spec.Configuration.ProviderRef.Name)

		default:
			v.Passed("Provider referenced exists in cluster")
		}

		return nil
	})
}

// checkModuleSecurityPolicy is responsible for checking if the cloudresource is permitted by the policy
func (o *RevisionCommand) checkModuleSecurityPolicy(revision *terraformv1alpha1.Revision) error {
	return o.Verify.Check("Validating Module Policy permits Revision", func(c CheckInterface) error {
		switch {
		case o.Policies == nil:
			fallthrough

		case len(o.Policies.Items) == 0:
			c.Warning("No module constraint policies found, the Revision will be permitted")

			return nil

		case utils.ContainsPrefix(revision.Spec.Configuration.Module, []string{"/", "."}):
			c.Warning("Revision is using a local directory, skipping policy check")
		}

		policies := policies.FindModuleConstraints(o.Policies)
		if len(policies) == 0 {
			c.Warning("No module constraint policies found, the Revision will be permitted")

			return nil
		}
		c.Info("Found %d module constraint policies", len(policies))

		// @step: do we have any policies related to module enforcement?
		for _, policy := range policies {
			permitted, err := policy.Spec.Constraints.Modules.Matches(revision.Spec.Configuration.Module)
			if err == nil && permitted {
				c.Passed("Revision is permitted by policy constraint %q", policy.Name)

				return nil
			}
		}
		c.Failed("Revision is not permitted by any policy")

		return nil
	})
}

// convertRevision is responsible for converting the revision to a terraform plan so we can
// validate the plan
func (o *RevisionCommand) convertRevision(ctx context.Context, revision *terraformv1alpha1.Revision) error {
	switch {
	case o.Directory == "":
		temp, err := os.MkdirTemp(os.TempDir(), "revision-*")
		if err != nil {
			return fmt.Errorf("failed to create temporary directory, error: %w", err)
		}

		o.Directory = temp

	case o.Directory == ".":
		path, err := os.Getwd()
		if err != nil {
			return fmt.Errorf("failed to get the current working directory, error: %w", err)
		}
		o.Directory = path

	case !filepath.IsAbs(o.Directory):
		path, err := os.Getwd()
		if err != nil {
			return fmt.Errorf("failed to get the current working directory, error: %w", err)
		}
		o.Directory = filepath.Join(path, o.Directory)
	}

	// we convert the revision we've loaded into terraform code
	if err := (&convert.RevisionCommand{
		Factory:          o.Factory,
		Contexts:         o.Contexts,
		Directory:        o.Directory,
		IncludeCheckov:   false,
		IncludeProvider:  (len(o.Providers.Items) > 0),
		IncludeTerraform: true,
		Policies:         o.Policies,
		Providers:        o.Providers,
		Revision:         revision,
	}).Run(ctx); err != nil {
		return fmt.Errorf("failed to convert revision to terraform code, error: %w", err)
	}

	return nil
}

// sourceFromCluster is responsible for sourcing the contexts, policies and providers from
// the cluster
func (o *RevisionCommand) sourceFromCluster(ctx context.Context) error {
	if !o.EnableCluster {
		return nil
	}

	// @step: retrieve a client
	cc, err := o.GetClient()
	if err != nil {
		return fmt.Errorf("failed to create client on current kubeconfig: %w", err)
	}

	// @step: retrieve the various items from the cluster and merge, or set if required
	{
		list := &terraformv1alpha1.ContextList{}
		if err := cc.List(ctx, list); err != nil {
			return fmt.Errorf("failed to retrieves contexts from cluster: %w", err)
		}
		if o.Contexts == nil {
			o.Contexts = list
		} else {
			o.Contexts.Merge(list.Items)
		}
	}
	{
		list := &terraformv1alpha1.ProviderList{}
		if err := cc.List(ctx, list); err != nil {
			return fmt.Errorf("failed to retrieves providers from cluster: %w", err)
		}
		if o.Providers == nil {
			o.Providers = list
		} else {
			o.Providers.Merge(list.Items)
		}
	}
	{
		list := &terraformv1alpha1.PolicyList{}
		if err := cc.List(ctx, list); err != nil {
			return fmt.Errorf("failed to retrieves policies from cluster: %w", err)
		}
		if o.Policies == nil {
			o.Policies = list
		} else {
			o.Policies.Merge(list.Items)
		}
	}

	return nil
}

// sourceFiles is responsible for sourcing any additional files
func (o *RevisionCommand) sourceFiles() error {
	// @step: we start by setting the defaults
	if o.Contexts == nil {
		o.Contexts = &terraformv1alpha1.ContextList{}
	}
	if o.Policies == nil {
		o.Policies = &terraformv1alpha1.PolicyList{}
	}
	if o.Providers == nil {
		o.Providers = &terraformv1alpha1.ProviderList{}
	}

	if o.SourceDir == "" {
		return nil
	}

	// @step: is the source directory a directory?
	if found, err := utils.DirExists(o.SourceDir); err != nil {
		return err
	} else if !found {
		return fmt.Errorf("source directory: %q does not exist", o.SourceDir)
	}

	return o.Verify.Check("Validating Sources Directory", func(c CheckInterface) error {
		c.Info("Loading sources from directory: %s", o.SourceDir)

		// @step: we walk all the files in the directory, load the yaml files and merge
		// any contexts, policies and providers into the current lists
		return filepath.Walk(o.SourceDir,
			func(path string, info os.FileInfo, err error) error {
				switch {
				case err != nil:
					return err
				case info.IsDir():
					return nil
				case !utils.Contains(filepath.Ext(path), []string{".yaml", ".yml"}):
					return nil
				}

				file, err := os.Open(path)
				if err != nil {
					return err
				}

				documents, err := utils.YAMLDocuments(file)
				if err != nil {
					return err
				}
				for _, doc := range documents {
					switch {
					case !strings.Contains(doc, "kind"):
						continue

					case strings.Contains(doc, "kind: Context"):
						c.Info("Sourcing additional Context from %s", path)
						resource := &terraformv1alpha1.Context{}
						if err := utils.LoadYAMLFromReader(strings.NewReader(doc), resource); err != nil {
							return err
						}
						o.Contexts.Items = append(o.Contexts.Items, *resource)

					case strings.Contains(doc, "kind: Policy"):
						c.Info("Sourcing additional Policy from %s", path)
						resource := &terraformv1alpha1.Policy{}
						if err := utils.LoadYAMLFromReader(strings.NewReader(doc), resource); err != nil {
							return err
						}
						o.Policies.Items = append(o.Policies.Items, *resource)

					case strings.Contains(doc, "kind: Provider"):
						c.Info("Sourcing additional Provider from %s", path)
						resource := &terraformv1alpha1.Provider{}
						if err := utils.LoadYAMLFromReader(strings.NewReader(doc), resource); err != nil {
							return err
						}
						o.Providers.Items = append(o.Providers.Items, *resource)
					}
				}

				return nil
			})
	})
}
