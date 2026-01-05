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

package eks

import (
	"context"
	"errors"
	"io"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/eks"
	ekstypes "github.com/aws/aws-sdk-go-v2/service/eks/types"
	"github.com/aws/smithy-go"
	"github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/sirupsen/logrus"

	"github.com/appvia/terranetes-controller/pkg/utils/preload"
	"github.com/appvia/terranetes-controller/pkg/utils/preload/eks/mocks"
)

//go:generate go run ../../../../vendor/github.com/golang/mock/mockgen -package mocks -destination=mocks/ec2_zz.go github.com/appvia/terranetes-controller/pkg/utils/preload/eks EC2API
//go:generate go run ../../../../vendor/github.com/golang/mock/mockgen -package mocks -destination=mocks/eks_zz.go github.com/appvia/terranetes-controller/pkg/utils/preload/eks EKSAPI

func TestReconcile(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Running Test Suite")
}

var _ = Describe("EKS Preload", func() {
	logrus.SetOutput(io.Discard)

	var err error
	var mc *gomock.Controller
	var ekscc *mocks.MockEKSAPI
	var ec2cc *mocks.MockEC2API
	var loader *eksPreloader
	var data preload.Data

	expectedCluster := &ekstypes.Cluster{
		Arn:             aws.String("arn:aws:eks:eu-west-1:123456789012:cluster/test"),
		Name:            aws.String("test"),
		Endpoint:        aws.String("https://test"),
		PlatformVersion: aws.String("eks.1"),
		Version:         aws.String("1.14"),
		KubernetesNetworkConfig: &ekstypes.KubernetesNetworkConfigResponse{
			ServiceIpv4Cidr: aws.String("10.0.0.0/12"),
		},
		ResourcesVpcConfig: &ekstypes.VpcConfigResponse{
			ClusterSecurityGroupId: aws.String("sg-1234567890"),
			EndpointPrivateAccess:  true,
			EndpointPublicAccess:   true,
			PublicAccessCidrs:      []string{"0.0.0.0/0"},
			SecurityGroupIds:       []string{"sg-1234567890"},
			SubnetIds: []string{
				"subnet-12345678",
				"subnet-12345679",
				"subnet-12345670",
			},
			VpcId: aws.String("vpc-12345678"),
		},
	}
	_ = expectedCluster

	BeforeEach(func() {
		mc = gomock.NewController(GinkgoT())
		ekscc = mocks.NewMockEKSAPI(mc)
		ec2cc = mocks.NewMockEC2API(mc)

		loader = &eksPreloader{
			clusterName: "test",
			ec2cc:       ec2cc,
			ekscc:       ekscc,
			config:      aws.Config{Region: "eu-west-1"},
		}
	})

	AfterEach(func() {
		mc.Finish()
	})

	When("loading the preload data for the cluster", func() {
		Context("when describeing the cluster errors", func() {
			BeforeEach(func() {
				ekscc.EXPECT().DescribeCluster(gomock.Any(), gomock.Any()).Return(&eks.DescribeClusterOutput{}, errors.New("bad"))

				data, err = loader.Load(context.Background())
			})

			It("should return an error", func() {
				Expect(err).To(HaveOccurred())
				Expect(err).To(MatchError("failed to retrieve the eks cluster details, error: bad"))
			})

			It("should not return any data", func() {
				Expect(data).To(BeNil())
			})
		})

		Context("when the cluster is not found", func() {
			BeforeEach(func() {
				apiErr := &smithy.GenericAPIError{
					Code:    "ResourceNotFoundException",
					Message: "not found",
				}
				ekscc.EXPECT().DescribeCluster(gomock.Any(), gomock.Any()).Return(
					&eks.DescribeClusterOutput{},
					apiErr,
				)

				data, err = loader.Load(context.Background())
			})

			It("should return an error", func() {
				Expect(err).To(HaveOccurred())
				Expect(err).To(MatchError("failed to find eks cluster: test"))
			})
		})

		Context("when the cluster is found", func() {
			It("should return the cluster details", func() {

			})
		})
	})
})
