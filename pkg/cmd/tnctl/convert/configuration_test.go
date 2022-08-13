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

package convert

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

	"github.com/appvia/terranetes-controller/pkg/cmd"
)

var testFixture = `
apiVersion: terranetes.appvia.io/v1alpha1
kind: Configuration
metadata:
  name: test
spec:
  module: https://github.com/terraform-aws-modules/terraform-aws-s3-bucket.git?ref=v3.1.0
  variables:
    bucket: terranetes-controller-ci-bucket
    acl: private
    versioning:
      enabled: true
    block_public_acls: true
    block_public_policy: true
    ignore_public_acls: true
    restrict_public_buckets: true
    server_side_encryption_configuration:
      rule:
        apply_server_side_encryption_by_default:
          sse_algorithm: "aws:kms"
        bucket_key_enabled: true
`

func TestSuite(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Running Test Suite")
}

var _ = Describe("Convert Configuration Command", func() {
	logrus.SetOutput(ioutil.Discard)

	var factory cmd.Factory
	var streams genericclioptions.IOStreams
	var stdout *bytes.Buffer
	var command *cobra.Command
	var err error

	BeforeEach(func() {
		streams, _, stdout, _ = genericclioptions.NewTestIOStreams()
		factory, _ = cmd.NewFactory(cmd.WithStreams(streams))
		command = NewConfigurationCommand(factory)
		command.SetErr(ioutil.Discard)
		command.SetOut(ioutil.Discard)
	})

	When("no path provided", func() {
		BeforeEach(func() {
			err = command.ExecuteContext(context.Background())
		})

		It("should fail due to missing path", func() {
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("accepts 1 arg(s), received 0"))
		})
	})

	When("provided path does not exist", func() {
		BeforeEach(func() {
			os.Args = []string{"configuration", "missing.file"}
			err = command.ExecuteContext(context.Background())
		})

		It("should fail due to file not existing", func() {
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("missing.file: no such file or directory"))
		})
	})

	When("provided path exists", func() {
		var tmpfile *os.File

		BeforeEach(func() {
			tmpfile, err = os.CreateTemp(os.TempDir(), "convert-config.XXXXX")
			Expect(err).NotTo(HaveOccurred())
			Expect(tmpfile).ToNot(BeNil())

			_, err = tmpfile.WriteString(testFixture)
			Expect(err).NotTo(HaveOccurred())
			Expect(tmpfile.Close()).To(Succeed())
		})

		AfterEach(func() {
			Expect(os.Remove(tmpfile.Name())).To(Succeed())
		})

		When("converting a valid configuration", func() {
			BeforeEach(func() {
				os.Args = []string{"configuration", tmpfile.Name()}
				err = command.ExecuteContext(context.Background())
			})

			It("should not fail", func() {
				Expect(err).NotTo(HaveOccurred())
			})

			It("should output the converted configuration", func() {
				expected := `module "main" {
  source = "https://github.com/terraform-aws-modules/terraform-aws-s3-bucket.git?ref=v3.1.0"

  acl = "private"
  block_public_acls = true
  block_public_policy = true
  bucket = "terranetes-controller-ci-bucket"
  ignore_public_acls = true
  restrict_public_buckets = true
  server_side_encryption_configuration {
    rule {
      apply_server_side_encryption_by_default {
        sse_algorithm = "aws:kms"
      }
      bucket_key_enabled = true
    }
  }
  versioning {
    enabled = true
  }
}
`
				Expect(stdout.String()).To(Equal(expected))
			})
		})
	})
})
