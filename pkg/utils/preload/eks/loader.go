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
	"fmt"
	"strings"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/aws/aws-sdk-go/service/ec2/ec2iface"
	"github.com/aws/aws-sdk-go/service/eks"
	"github.com/aws/aws-sdk-go/service/eks/eksiface"
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
		session:     config.Session,
	}, nil
}

// Load implements the preload.Interface and used to retrieve details on an EKS cluster
func (p *eksPreloader) Load(ctx context.Context) (preload.Data, error) {
	data := make(preload.Data)

	// @step: first we check the cluster is exists and extract the cluster details
	resp, err := p.ekscc.DescribeClusterWithContext(ctx, &eks.DescribeClusterInput{
		Name: aws.String(p.clusterName),
	})
	if err != nil {
		if IsResourceNotFoundException(err) {
			return nil, fmt.Errorf("failed to find eks cluster: %s", p.clusterName)
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

	// @step: we extract the cluster details and fill in the preload data
	data.Add("eks", preload.Entry{
		Description: "AWS ARN for the Kubernetes cluster",
		Value:       aws.StringValue(resp.Cluster.Arn),
	})
	data.Add("eks_cluster_security_group_id", preload.Entry{
		Description: "The security group ID attached to the EKS cluster",
		Value:       aws.StringValue(resp.Cluster.ResourcesVpcConfig.ClusterSecurityGroupId),
	})
	data.Add("eks_endpoint", preload.Entry{
		Description: "The endpoint for the EKS cluster",
		Value:       aws.StringValue(resp.Cluster.Endpoint),
	})
	data.Add("eks_name", preload.Entry{
		Description: "The name of the EKS cluster",
		Value:       aws.StringValue(resp.Cluster.Name),
	})
	data.Add("eks_platform_version", preload.Entry{
		Description: "The platform version of the EKS cluster",
		Value:       aws.StringValue(resp.Cluster.PlatformVersion),
	})
	data.Add("eks_private_access", preload.Entry{
		Description: "Indicates whether or not the EKS cluster has private access enabled",
		Value:       aws.BoolValue(resp.Cluster.ResourcesVpcConfig.EndpointPrivateAccess),
	})
	data.Add("eks_public_access", preload.Entry{
		Description: "Indicates whether or not the EKS cluster has public access enabled",
		Value:       aws.BoolValue(resp.Cluster.ResourcesVpcConfig.EndpointPublicAccess),
	})
	data.Add("eks_public_access_cidrs", preload.Entry{
		Description: "The CIDR blocks that are allowed access to the EKS cluster",
		Value:       aws.StringValueSlice(resp.Cluster.ResourcesVpcConfig.PublicAccessCidrs),
	})
	data.Add("eks_role_arn", preload.Entry{
		Description: "The ARN of the IAM role that provides permissions for the EKS cluster",
		Value:       aws.StringValue(resp.Cluster.RoleArn),
	})
	data.Add("eks_security_group_ids", preload.Entry{
		Description: "The security group IDs attached to the EKS cluster",
		Value:       aws.StringValueSlice(resp.Cluster.ResourcesVpcConfig.SecurityGroupIds),
	})
	data.Add("eks_service_cidr_ipv4", preload.Entry{
		Description: "The CIDR block used by the EKS cluster for Kubernetes service IPv4 addresses",
		Value:       aws.StringValue(resp.Cluster.KubernetesNetworkConfig.ServiceIpv4Cidr),
	})
	data.Add("eks_service_cidr_ipv6", preload.Entry{
		Description: "The CIDR block used by the EKS cluster for Kubernetes service IPv6 addresses",
		Value:       aws.StringValue(resp.Cluster.KubernetesNetworkConfig.ServiceIpv6Cidr),
	})
	data.Add("eks_subnet_ids", preload.Entry{
		Description: "The subnets used by the EKS cluster",
		Value:       aws.StringValueSlice(resp.Cluster.ResourcesVpcConfig.SubnetIds),
	})
	data.Add("eks_tags", preload.Entry{
		Description: "The tags attached to the EKS cluster",
		Value:       ToMapTags(resp.Cluster.Tags),
	})
	data.Add("eks_version", preload.Entry{
		Description: "The Kubernetes version of the EKS cluster",
		Value:       aws.StringValue(resp.Cluster.Version),
	})
	data.Add("eks_vpc_id", preload.Entry{
		Description: "The ID of the VPC used by the EKS cluster",
		Value:       aws.StringValue(resp.Cluster.ResourcesVpcConfig.VpcId),
	})
	data.Add("vpc_id", preload.Entry{
		Description: "The ID of the VPC used by the EKS cluster",
		Value:       aws.StringValue(resp.Cluster.ResourcesVpcConfig.VpcId),
	})

	// @step: retrieve a list of all the subnets and try and workout public and private
	subnets, err := p.ec2cc.DescribeSubnetsWithContext(ctx, &ec2.DescribeSubnetsInput{
		Filters: []*ec2.Filter{
			{
				Name:   aws.String("vpc-id"),
				Values: aws.StringSlice([]string{aws.StringValue(resp.Cluster.ResourcesVpcConfig.VpcId)}),
			},
		},
	})
	if err != nil {
		return nil, fmt.Errorf("unable to retrieve subnets for cluster: %s, error: %w", p.clusterName, err)
	}

	// @step: next we grab the route tables associated to the subnets
	routes, err := p.ec2cc.DescribeRouteTablesWithContext(ctx, &ec2.DescribeRouteTablesInput{
		Filters: []*ec2.Filter{
			{
				Name:   aws.String("vpc-id"),
				Values: aws.StringSlice([]string{aws.StringValue(resp.Cluster.ResourcesVpcConfig.VpcId)}),
			},
		},
	})
	if err != nil {
		return nil, fmt.Errorf("unable to retrieve route tables for cluster: %s, error: %w", p.clusterName, err)
	}

	data.Add("private_subnet_ids", preload.Entry{
		Description: "A list of all subnets associated to the EKS cluster network which are labeled private",
		Value:       []string{},
	})
	data.Add("public_subnet_ids", preload.Entry{
		Description: "A list of all subnets associated to the EKS cluster network which are labeled public",
		Value:       []string{},
	})
	data.Add("subnet_ids", preload.Entry{
		Description: "A list of all subnets associated to the EKS cluster network",
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

	// @step: ensure they are all unqiue and sorted
	data.Get("private_subnet_ids").Value = utils.Unique(utils.Sorted(data.Get("private_subnet_ids").Value.([]string)))
	data.Get("public_subnet_ids").Value = utils.Unique(utils.Sorted(data.Get("public_subnet_ids").Value.([]string)))
	data.Get("subnet_ids").Value = utils.Unique(utils.Sorted(data.Get("subnet_ids").Value.([]string)))

	// @step: build all the route tables ids
	data.Add("eks_route_tables_ids", preload.Entry{
		Description: "A list of all route tables id associate to the subnets which are part of the EKS cluster",
		Value:       []string{},
	})
	data.Add("route_tables_ids", preload.Entry{
		Description: "A list of all route tables id associated to the network the EKS cluster is part of",
		Value:       []string{},
	})
	data.Add("private_route_tables_ids", preload.Entry{
		Description: "A list of all route tables id which are associated to subnets labeled private in the network the EKS cluster is part of",
		Value:       []string{},
	})
	data.Add("public_route_tables_ids", preload.Entry{
		Description: "A list of all route tables id which are associated to subnets labeled public in the network the EKS cluster is part of",
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

	// @step: here we create a map of subnets and their route tables based on a known tag 'Network'
	discoveredSubnets := make(map[string][]string)
	discoveredRouteTables := make(map[string][]string)

	// we should end up with a map Network -> []SubnetId
	for _, subnet := range subnets.Subnets {
		value, found := GetTagValue(subnet.Tags, "Network")
		if found {
			if _, found := discoveredSubnets[value]; !found {
				discoveredSubnets[value] = []string{}
			}
			discoveredSubnets[value] = append(discoveredSubnets[value], aws.StringValue(subnet.SubnetId))
		}
	}
	// we need to ensure the subnets are unique and sorted
	for k, value := range discoveredSubnets {
		name := strings.ReplaceAll(strings.ToLower(k), " ", "_")
		data.Add(fmt.Sprintf("network_%s_subnet_ids", name), preload.Entry{
			Description: fmt.Sprintf("A list of all subnets id associate to the network %s", k),
			Value:       utils.Unique(utils.Sorted(value)),
		})
	}

	// @step: now we want to find the routing tables associated to the subnets dicoevered
	for key, subnets := range discoveredSubnets {
		for _, route := range routes.RouteTables {
			for _, associate := range route.Associations {
				if utils.Contains(aws.StringValue(associate.SubnetId), subnets) {
					if _, found := discoveredRouteTables[key]; !found {
						discoveredRouteTables[key] = []string{}
					}
					discoveredRouteTables[key] = append(discoveredRouteTables[key], aws.StringValue(associate.RouteTableId))
				}
			}
		}
	}
	for k, value := range discoveredRouteTables {
		name := strings.ReplaceAll(strings.ToLower(k), " ", "_")
		data.Add(fmt.Sprintf("network_%s_route_tables_ids", name), preload.Entry{
			Description: fmt.Sprintf("A list of all route tables id associate to the network %s", k),
			Value:       utils.Unique(utils.Sorted(value)),
		})
	}

	// @step: might be useful to have a routing table and routing_id. Note we will ignore
	// any routing tables without a name tag
	for _, route := range routes.RouteTables {
		value, found := GetTagValue(route.Tags, "Name")
		if !found {
			continue
		}
		name := strings.ReplaceAll(strings.ToLower(value), "-", "_")
		data.Add(fmt.Sprintf("route_table_%s_id", name), preload.Entry{
			Description: fmt.Sprintf("The route table id of the route table %s", value),
			Value:       aws.StringValue(route.RouteTableId),
		})
	}

	return data, nil
}
