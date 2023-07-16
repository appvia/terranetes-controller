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

package create

import (
	"bytes"
	"context"
	"io/ioutil"
	"os"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	terraformv1alpha1 "github.com/appvia/terranetes-controller/pkg/apis/terraform/v1alpha1"
	"github.com/appvia/terranetes-controller/pkg/cmd"
	"github.com/appvia/terranetes-controller/pkg/schema"
	"github.com/appvia/terranetes-controller/test/fixtures"
)

func TestCreate(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Running Test Suite")
}

var _ = Describe("Create CloudResource", func() {
	logrus.SetOutput(ioutil.Discard)

	var cc client.Client
	var streams genericclioptions.IOStreams
	var command *cobra.Command
	var stdout *bytes.Buffer
	var err error

	BeforeEach(func() {
		cc = fake.NewClientBuilder().WithScheme(schema.GetScheme()).Build()
		streams, _, stdout, _ = genericclioptions.NewTestIOStreams()
		factory, _ := cmd.NewFactory(
			cmd.WithClient(cc),
			cmd.WithStreams(streams),
		)
		command = NewCloudResourceCommand(factory)
	})

	When("no revisions exists", func() {
		BeforeEach(func() {
			os.Args = []string{"cloudresource"}

			err = command.ExecuteContext(context.Background())
		})

		It("should fail with no revisions", func() {
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(Equal("no plans found to create a cloud resource"))
		})
	})

	When("revisions exists", func() {
		var revision *terraformv1alpha1.Revision

		BeforeEach(func() {
			revision = fixtures.NewAWSBucketRevision("test")

			Expect(cc.Create(context.Background(), revision)).To(Succeed())
		})

		Context("but the plan does not exist", func() {
			BeforeEach(func() {
				os.Args = []string{"cloudresource", "--plan", "dosntexist"}

				err = command.ExecuteContext(context.Background())
			})

			It("should fail with no plans", func() {
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(Equal("the plan dosntexist does not exist"))
			})
		})

		Context("but the revision in the plan does not exist", func() {
			BeforeEach(func() {
				os.Args = []string{"cloudresource", "--plan", revision.Spec.Plan.Name, "--revision", "dosntexist"}

				err = command.ExecuteContext(context.Background())
			})

			It("should fail with no plans", func() {
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(Equal("the revision dosntexist does not exist"))
			})
		})

		Context("and revision exists", func() {
			BeforeEach(func() {
				os.Args = []string{"cloudresource", "--plan", revision.Spec.Plan.Name, "--revision", revision.Spec.Plan.Revision}

				err = command.ExecuteContext(context.Background())
			})

			It("should not fail", func() {
				Expect(err).NotTo(HaveOccurred())
			})

			It("should have created a cloud resource", func() {
				expected := `apiVersion: terraform.appvia.io/v1alpha1
kind: CloudResource
metadata:
  creationTimestamp: null
  name: test
spec:
  plan:
    name: bucket
    revision: 1.0.0
  providerRef:
    name: aws
  variables:
    bucket_name: test
  writeConnectionSecretToRef:
    name: aws-secret
status:
  configurationStatus: {}
`
				Expect(stdout.String()).ToNot(BeEmpty())
				Expect(stdout.String()).To(Equal(expected))
			})
		})
	})
})
