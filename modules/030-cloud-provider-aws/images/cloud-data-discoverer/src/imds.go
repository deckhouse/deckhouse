/*
Copyright 2026 Flant JSC

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package main

import (
	"context"
	"fmt"
	"strings"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/aws/aws-sdk-go/service/ec2/ec2iface"
	"github.com/prometheus/client_golang/prometheus"
)

// imdsv1AllowedInstances reports the observed IMDS state of the cluster instances, which is what
// tells apart "IMDSv2 is requested in AWSClusterConfiguration" from "the instances have actually
// been converged to it": master, static and bastion instances are only updated by a converge run,
// and ephemeral ones only when their nodes are rolled.
var imdsv1AllowedInstances = prometheus.NewGauge(prometheus.GaugeOpts{
	Namespace: "cloud_data",
	Subsystem: "discovery",
	Name:      "aws_instances_imdsv1_allowed",
	Help:      "Number of the cluster EC2 instances whose instance metadata service still allows IMDSv1.",
})

// registerIMDSMetrics publishes the gauge. It is called only when the instances can actually be
// observed: a registered gauge is exported as 0 until it is first set, and a 0 nobody measured
// would read as "the cluster has converged".
func registerIMDSMetrics() {
	prometheus.MustRegister(imdsv1AllowedInstances)
}

// countInstancesAllowingIMDSv1 counts the running instances of the cluster that do not require a
// session token for metadata requests. The API is asked for the instances that allow IMDSv1, and
// the ones of this cluster are picked out of them by their IAM instance profile: every instance of
// the cluster runs with the "<prefix>-node" profile, bastion and ephemeral nodes included. The
// profile is matched together with its leading slash, so a cluster whose profile name merely
// starts with the same string ("kube-node-2" next to "kube-node") is not counted in.
func countInstancesAllowingIMDSv1(ctx context.Context, ec2Client ec2iface.EC2API, instanceProfileName string) (int, error) {
	input := &ec2.DescribeInstancesInput{
		Filters: []*ec2.Filter{
			{
				Name:   aws.String("metadata-options.http-tokens"),
				Values: aws.StringSlice([]string{ec2.HttpTokensStateOptional}),
			},
			{
				Name: aws.String("instance-state-name"),
				Values: aws.StringSlice([]string{
					ec2.InstanceStateNamePending,
					ec2.InstanceStateNameRunning,
				}),
			},
		},
	}

	profileSuffix := "/" + instanceProfileName
	count := 0

	err := ec2Client.DescribeInstancesPagesWithContext(ctx, input, func(page *ec2.DescribeInstancesOutput, _ bool) bool {
		for _, reservation := range page.Reservations {
			for _, instance := range reservation.Instances {
				if instance.IamInstanceProfile == nil ||
					!strings.HasSuffix(aws.StringValue(instance.IamInstanceProfile.Arn), profileSuffix) {
					// Not an instance of this cluster.
					continue
				}
				if instance.MetadataOptions == nil {
					// The API has not reported the state of the metadata service yet.
					continue
				}
				if aws.StringValue(instance.MetadataOptions.HttpEndpoint) == ec2.InstanceMetadataEndpointStateDisabled {
					// The metadata service is turned off entirely, so it serves no IMDSv1 either.
					continue
				}
				if aws.StringValue(instance.MetadataOptions.HttpTokens) != ec2.HttpTokensStateRequired {
					count++
				}
			}
		}

		return true
	})
	if err != nil {
		return 0, fmt.Errorf("failed to describe instances: %w", err)
	}

	return count, nil
}
