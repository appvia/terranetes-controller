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
	"encoding/base64"
	"errors"
	"fmt"
	"strings"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/aws/aws-sdk-go/service/ec2/ec2iface"
	"github.com/aws/aws-sdk-go/service/eks"
	"github.com/aws/aws-sdk-go/service/eks/eksiface"
	"github.com/aws/aws-sdk-go/service/sts"
	"github.com/aws/aws-sdk-go/service/sts/stsiface"
	log "github.com/sirupsen/logrus"

	"github.com/appvia/terranetes-controller/pkg/utils"
	"github.com/appvia/terranetes-controller/pkg/utils/preload"
)

// eksPreloader is a preloader for EKS clusters
type eksPreloader struct {
	// clusterName is the name of the EKS cluster to preload
	clusterName string
	// ec2cc is a client to the EC2 API
	ec2cc ec2iface.EC2API
	// ekscc is a client to  the EKS API
	ekscc eksiface.EKSAPI
	// stscc is a client to the STS API
	stscc stsiface.STSAPI
	// session is the aws session used to query details on EKS
	session *session.Session
}

// New creates and returns a preloader for EKS clusters
func New(config Config) (preload.Interface, error) {
	switch {
	case config.Session == nil:
		return nil, errors.New("session is required")
	case config.ClusterName == "":
		return nil, errors.New("cluster name is required")
	}

	return &eksPreloader{
		clusterName: config.ClusterName,
		ekscc:       eks.New(config.Session),
		ec2cc:       ec2.New(config.Session),
		stscc:       sts.New(config.Session),
		session:     config.Session,
	}, nil
}

// Load implements the preload.Interface and used to retrieve details on an EKS cluster
func (e *eksPreloader) Load(ctx context.Context) (preload.Data, error) {
	data := make(preload.Data)

	// @step: first we check the cluster is exists and extract the cluster details
	resp, err := e.ekscc.DescribeClusterWithContext(ctx, &eks.DescribeClusterInput{
		Name: aws.String(e.clusterName),
	})
	if err != nil {
		if IsResourceNotFoundException(err) {
			return nil, fmt.Errorf("failed to find eks cluster: %s", e.clusterName)
		}

		return nil, fmt.Errorf("failed to retrieve the eks cluster details, error: %w", err)
	}
	logger := log.WithFields(log.Fields{
		"cluster": aws.StringValue(resp.Cluster.Name),
		"status":  strings.ToLower(aws.StringValue(resp.Cluster.Status)),
	})
	logger.Debug("retrieved details on the eks cluster")

	// @step: ensure the cluster is condition we can query it
	switch aws.StringValue(resp.Cluster.Status) {
	case eks.ClusterStatusCreating, eks.ClusterStatusDeleting, eks.ClusterStatusFailed, eks.ClusterStatusPending:
		return nil, preload.ErrNotReady
	case eks.ClusterStatusUpdating, eks.ClusterStatusActive:
		break
	default:
		return nil, fmt.Errorf("unknown cluster status: %s", aws.StringValue(resp.Cluster.Status))
	}

	data.Add("region", preload.Entry{
		Description: "AWS region the cluster is running in",
		Value:       aws.StringValue(e.session.Config.Region),
	})

	// @step: find details on the account
	if err := e.findAccount(ctx, &data); err != nil {
		return nil, err
	}

	// @step: extract the cluster details
	e.findCluster(ctx, resp.Cluster, &data)

	// @step: retrieve a list of all the subnets and try and workout public and private
	subnets, err := e.ec2cc.DescribeSubnetsWithContext(ctx, &ec2.DescribeSubnetsInput{
		Filters: []*ec2.Filter{
			{
				Name:   aws.String("vpc-id"),
				Values: aws.StringSlice([]string{aws.StringValue(resp.Cluster.ResourcesVpcConfig.VpcId)}),
			},
		},
	})
	if err != nil {
		return nil, fmt.Errorf("unable to retrieve subnets for cluster: %s, error: %w", e.clusterName, err)
	}

	// @step: next we grab the route tables associated to the subnets
	routes, err := e.ec2cc.DescribeRouteTablesWithContext(ctx, &ec2.DescribeRouteTablesInput{
		Filters: []*ec2.Filter{
			{
				Name:   aws.String("vpc-id"),
				Values: aws.StringSlice([]string{aws.StringValue(resp.Cluster.ResourcesVpcConfig.VpcId)}),
			},
		},
	})
	if err != nil {
		return nil, fmt.Errorf("unable to retrieve route tables for cluster: %s, error: %w", e.clusterName, err)
	}

	// @step: lets discover the subnets
	e.findSubnets(subnets.Subnets, routes.RouteTables, &data)
	// @step: lets discover the route tables
	e.findRouteTables(routes.RouteTables, &data)

	data.Add("private_subnet_ids", preload.Entry{
		Description: "A list of all subnets associated to the EKS cluster network which are tagged private",
		Value:       []string{},
	})
	data.Add("public_subnet_ids", preload.Entry{
		Description: "A list of all subnets associated to the EKS cluster network which are tagged public",
		Value:       []string{},
	})
	data.Add("subnet_ids", preload.Entry{
		Description: "A list of all subnets associated to the network the EKS cluster is deployed in",
		Value:       []string{},
	})

	// @step: we iterate the subnets and use tagging to determine if public or private
	for _, subnet := range subnets.Subnets {
		switch {
		case
			HasTag(subnet.Tags, "kubernetes.io/role/internal-elb", "1"),
			HasTag(subnet.Tags, "[pP]rivate", "[Tt]rue"),
			HasTag(subnet.Tags, "[pP]rivate", "1"):
			data.Get("private_subnet_ids").Value = append(data.Get("private_subnet_ids").Value.([]string), aws.StringValue(subnet.SubnetId))

		case
			HasTag(subnet.Tags, "kubernetes.io/role/elb", "1"),
			HasTag(subnet.Tags, "[pP]ublic", "[Tt]rue"),
			HasTag(subnet.Tags, "[pP]ublic", "1"):
			data.Get("public_subnet_ids").Value = append(data.Get("public_subnet_ids").Value.([]string), aws.StringValue(subnet.SubnetId))
		}
		data.Get("subnet_ids").Value = append(data.Get("subnet_ids").Value.([]string), aws.StringValue(subnet.SubnetId))
	}

	// @step: ensure they are all unique and sorted
	data.Get("private_subnet_ids").Value = utils.Unique(utils.Sorted(data.Get("private_subnet_ids").Value.([]string)))
	data.Get("public_subnet_ids").Value = utils.Unique(utils.Sorted(data.Get("public_subnet_ids").Value.([]string)))
	data.Get("subnet_ids").Value = utils.Unique(utils.Sorted(data.Get("subnet_ids").Value.([]string)))

	// @step: build all the route tables ids
	data.Add("eks_route_tables_ids", preload.Entry{
		Description: "A list of all route tables ids associated to the subnets in the EKS definition",
		Value:       []string{},
	})
	data.Add("route_tables_ids", preload.Entry{
		Description: "A list of all route tables ids associated to the network the EKS cluster is part of",
		Value:       []string{},
	})
	data.Add("private_route_tables_ids", preload.Entry{
		Description: "A list of all route tables ids which are associated to subnets labeled private in the network the EKS cluster is part of",
		Value:       []string{},
	})
	data.Add("public_route_tables_ids", preload.Entry{
		Description: "A list of all route tables ids which are associated to subnets labeled public in the network the EKS cluster is part of",
		Value:       []string{},
	})

	// @step: we extract the route table details
	for _, route := range routes.RouteTables {
		// add the route table to the all list
		data.Get("route_tables_ids").Value = append(data.Get("route_tables_ids").Value.([]string), aws.StringValue(route.RouteTableId))

		for _, association := range route.Associations {
			subnet := aws.StringValue(association.SubnetId)
			if utils.Contains(subnet, data.Get("private_subnet_ids").Value.([]string)) {
				data.Get("private_route_tables_ids").Value = append(data.Get("private_route_tables_ids").Value.([]string), aws.StringValue(route.RouteTableId))
			}
			if utils.Contains(subnet, data.Get("public_subnet_ids").Value.([]string)) {
				data.Get("public_route_tables_ids").Value = append(data.Get("public_route_tables_ids").Value.([]string), aws.StringValue(route.RouteTableId))
			}
			if utils.Contains(subnet, data.Get("eks_subnet_ids").Value.([]string)) {
				data.Get("eks_route_tables_ids").Value = append(data.Get("eks_route_tables_ids").Value.([]string), aws.StringValue(route.RouteTableId))
			}
		}
	}

	data.Get("eks_route_tables_ids").Value = utils.Unique(utils.Sorted(data.Get("eks_route_tables_ids").Value.([]string)))
	data.Get("route_tables_ids").Value = utils.Unique(utils.Sorted(data.Get("route_tables_ids").Value.([]string)))
	data.Get("private_route_tables_ids").Value = utils.Unique(utils.Sorted(data.Get("private_route_tables_ids").Value.([]string)))
	data.Get("public_route_tables_ids").Value = utils.Unique(utils.Sorted(data.Get("public_route_tables_ids").Value.([]string)))

	// @step: lets discover the security groups
	if err := e.findSecurityGroups(ctx, &data); err != nil {
		return nil, err
	}
	// @step: find any kms keys in the account and region
	if err := e.findKMSKeys(ctx, &data); err != nil {
		return nil, err
	}

	return data, nil
}

// findCluster is responsible for finding the cluster details and extracting into the
// data structure
func (e *eksPreloader) findCluster(_ context.Context, cluster *eks.Cluster, data *preload.Data) {
	// @step: we extract the cluster details and fill in the preload data
	data.Add("eks", preload.Entry{
		Description: "AWS ARN for the Kubernetes cluster",
		Value:       aws.StringValue(cluster.Arn),
	})
	data.Add("eks_id", preload.Entry{
		Description: "The ID of the EKS cluster",
		Value:       aws.StringValue(cluster.Id),
	})
	data.Add("eks_cluster_security_group_id", preload.Entry{
		Description: "The security group ID attached to the EKS cluster",
		Value:       aws.StringValue(cluster.ResourcesVpcConfig.ClusterSecurityGroupId),
	})
	data.Add("eks_cluster_security_group_ids", preload.Entry{
		Description: "The cluster security group id attached to the EKS cluster as a list",
		Value:       []string{aws.StringValue(cluster.ResourcesVpcConfig.ClusterSecurityGroupId)},
	})
	data.Add("eks_endpoint", preload.Entry{
		Description: "The endpoint for the EKS cluster",
		Value:       aws.StringValue(cluster.Endpoint),
	})
	data.Add("eks_name", preload.Entry{
		Description: "The name of the EKS cluster",
		Value:       aws.StringValue(cluster.Name),
	})
	data.Add("eks_platform_version", preload.Entry{
		Description: "The platform version of the EKS cluster",
		Value:       aws.StringValue(cluster.PlatformVersion),
	})
	data.Add("eks_private_access", preload.Entry{
		Description: "Indicates whether or not the EKS cluster has private access enabled",
		Value:       aws.BoolValue(cluster.ResourcesVpcConfig.EndpointPrivateAccess),
	})
	data.Add("eks_public_access", preload.Entry{
		Description: "Indicates whether or not the EKS cluster has public access enabled",
		Value:       aws.BoolValue(cluster.ResourcesVpcConfig.EndpointPublicAccess),
	})
	data.Add("eks_public_access_cidrs", preload.Entry{
		Description: "The CIDR blocks that are allowed access to the EKS cluster when public access is enabled",
		Value:       aws.StringValueSlice(cluster.ResourcesVpcConfig.PublicAccessCidrs),
	})
	data.Add("eks_role_arn", preload.Entry{
		Description: "The ARN of the IAM role that provides permissions for the EKS cluster",
		Value:       aws.StringValue(cluster.RoleArn),
	})
	data.Add("eks_security_group_ids", preload.Entry{
		Description: "The additional security group IDs attached to the EKS cluster",
		Value:       aws.StringValueSlice(cluster.ResourcesVpcConfig.SecurityGroupIds),
	})
	data.Add("eks_service_cidr_ipv4", preload.Entry{
		Description: "The CIDR block used by the EKS cluster for Kubernetes service IPv4 addresses",
		Value:       aws.StringValue(cluster.KubernetesNetworkConfig.ServiceIpv4Cidr),
	})
	data.Add("eks_service_cidr_ipv6", preload.Entry{
		Description: "The CIDR block used by the EKS cluster for Kubernetes service IPv6 addresses",
		Value:       aws.StringValue(cluster.KubernetesNetworkConfig.ServiceIpv6Cidr),
	})
	data.Add("eks_subnet_ids", preload.Entry{
		Description: "The subnets associated to the EKS cluster definition, where nodegroups live",
		Value:       aws.StringValueSlice(cluster.ResourcesVpcConfig.SubnetIds),
	})
	data.Add("eks_tags", preload.Entry{
		Description: "The resource tags associated to the EKS cluster",
		Value:       ToMapTags(cluster.Tags),
	})
	data.Add("eks_version", preload.Entry{
		Description: "The current Kubernetes version of the EKS cluster",
		Value:       aws.StringValue(cluster.Version),
	})
	data.Add("eks_vpc_id", preload.Entry{
		Description: "The ID of the VPC where the EKS cluster is deployed",
		Value:       aws.StringValue(cluster.ResourcesVpcConfig.VpcId),
	})
	data.Add("vpc_id", preload.Entry{
		Description: "The ID of the VPC where the EKS cluster is deployed",
		Value:       aws.StringValue(cluster.ResourcesVpcConfig.VpcId),
	})

	if cluster.CertificateAuthority != nil {
		data.Add("eks_certificate_authority", preload.Entry{
			Description: "The certificate authority data for the EKS cluster",
			Value:       base64.StdEncoding.EncodeToString([]byte(aws.StringValue(cluster.CertificateAuthority.Data))),
		})
	}

	if cluster.Identity != nil && cluster.Identity.Oidc != nil {
		data.Add("eks_oidc_issuer", preload.Entry{
			Description: "The OIDC issuer URL of the EKS cluster",
			Value:       aws.StringValue(cluster.Identity.Oidc.Issuer),
		})

		// @step: the arn is not available in the response, we just need to construct it
		data.Add("eks_oidc_issuer_arn", preload.Entry{
			Description: "The ARN of the OIDC issuer URL of the EKS cluster",
			Value: fmt.Sprintf("arn:aws:iam::%s:oidc-provider/%s",
				data.Get("account_id").Value.(string),
				strings.TrimPrefix(aws.StringValue(cluster.Identity.Oidc.Issuer), "https://"),
			),
		})
	}
}

// findAccount is responsible for extracting anything related to the account we are living in
func (e *eksPreloader) findAccount(ctx context.Context, data *preload.Data) error {
	resp, err := e.stscc.GetCallerIdentityWithContext(ctx, &sts.GetCallerIdentityInput{})
	if err != nil {
		return err
	}

	data.Add("account_id", preload.Entry{
		Description: "The AWS account the cluster resides in",
		Value:       aws.StringValue(resp.Account),
	})
	data.Add("account_ids", preload.Entry{
		Description: "The AWS account the cluster resides in a list",
		Value:       []string{aws.StringValue(resp.Account)},
	})

	return nil
}

// findKMSKeys is responsible for finding any kms keys in the account and region
func (e *eksPreloader) findKMSKeys(ctx context.Context, data *preload.Data) error {

	return nil
}

// findSecurityGroups is responsible for finding the security groups and extracting into the data
func (e *eksPreloader) findSecurityGroups(ctx context.Context, data *preload.Data) error {
	resp, err := e.ec2cc.DescribeSecurityGroupsWithContext(ctx, &ec2.DescribeSecurityGroupsInput{
		Filters: []*ec2.Filter{
			{
				Name:   aws.String("vpc-id"),
				Values: []*string{aws.String(data.Get("vpc_id").Value.(string))},
			},
		},
	})
	if err != nil {
		return err
	}

	// @step: iterate the security groups and if they have name fields we can use them
	for _, sg := range resp.SecurityGroups {
		if name, found := GetTagValue(sg.Tags, "Name"); found {

			data.Add(fmt.Sprintf("security_group_%s_id", SanitizeName(name)),
				preload.Entry{
					Description: fmt.Sprintf("The security group ID associated to the security group tagged with Name: %s", name),
					Value:       aws.StringValue(sg.GroupId),
				},
			)
			data.Add(fmt.Sprintf("security_group_%s_ids", SanitizeName(name)),
				preload.Entry{
					Description: fmt.Sprintf("The security group ID associated to the security group tagged with Name: %s as a list", name),
					Value:       []string{aws.StringValue(sg.GroupId)},
				},
			)
		}
	}

	return nil
}

// findNetworks is responsible for listing all the subnets connected to the cluster vpc, and using
// tagging information to build a collection list of Network types and route tables associated. This
// can be useful for things like, give me all subnets with 'Network=Private' and all route tables
// associated to those subnets.
func (e *eksPreloader) findSubnets(subnets []*ec2.Subnet, routes []*ec2.RouteTable, data *preload.Data) {
	ds := make(map[string][]string)
	dr := make(map[string][]string)

	// @step: we should end up with a map of Network -> []SubnetId
	for _, subnet := range subnets {
		// does the subnet have a network tag?
		value, found := GetTagValue(subnet.Tags, "Network")
		if !found {
			continue
		}
		// have we seen this network before?
		if _, found := ds[value]; !found {
			ds[value] = []string{}
		}
		ds[value] = append(ds[value], aws.StringValue(subnet.SubnetId))
	}
	// @step: iterate the discovered subnets add the map[Network][]SubnetId to the data
	for name, value := range ds {
		data.Add(fmt.Sprintf("network_%s_subnet_ids", SanitizeName(name)), preload.Entry{
			Description: fmt.Sprintf("A list of all subnets ids associated with the subnet tagged with %s", name),
			Value:       utils.Unique(utils.Sorted(value)),
		})
	}

	// @tip: now we want to find all the route table ids associated to the subnets discovered
	for name, subnets := range ds {
		// find the route table this subnet is associated to
		for _, route := range routes {
			for _, associate := range route.Associations {
				if utils.Contains(aws.StringValue(associate.SubnetId), subnets) {
					if _, found := dr[name]; !found {
						dr[name] = []string{}
					}
					dr[name] = append(dr[name], aws.StringValue(associate.RouteTableId))
				}
			}
		}
	}
	// @step: now we need to add the route tables to the data
	for name, value := range dr {
		data.Add(fmt.Sprintf("network_%s_route_table_ids", SanitizeName(name)), preload.Entry{
			Description: fmt.Sprintf("A list of all route table ids associated to the subnet tagged with %s", name),
			Value:       utils.Unique(utils.Sorted(value)),
		})
	}
}

// findRouteTables is responsible for listing all the route tables associated to the cluster vpc, and using
// tagging information to build a collection list of route tables types. This can be useful for things like,
// give me all route table with 'Network=Private' or Network=Main'
func (e *eksPreloader) findRouteTables(tables []*ec2.RouteTable, data *preload.Data) {
	// @step: might be useful to have a routing table and routing_id. Note we will ignore
	// any routing tables without a name tag
	for _, route := range tables {
		// does the route table have a name tag?
		value, found := GetTagValue(route.Tags, "Name")
		if !found {
			continue
		}
		// add the route table id to the data
		data.Add(fmt.Sprintf("network_%s_route_table_id", SanitizeName(value)), preload.Entry{
			Description: fmt.Sprintf("The route table id of the route table named %s", value),
			Value:       aws.StringValue(route.RouteTableId),
		})
	}
}
