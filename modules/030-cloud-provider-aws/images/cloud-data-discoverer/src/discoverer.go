/*
Copyright 2023 Flant JSC

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
	"errors"
	"fmt"
	"os"
	"strconv"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/aws/aws-sdk-go/service/ec2/ec2iface"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/utils/ptr"

	"github.com/deckhouse/deckhouse/go_lib/cloud-data/apis/v1alpha1"
	"github.com/deckhouse/deckhouse/pkg/log"
)

type Discoverer struct {
	logger *log.Logger
	region string
	// nodeInstanceProfileName selects the instances of this cluster when their IMDS state is
	// checked. Empty when the module has not passed it, in which case the check is skipped.
	nodeInstanceProfileName string
}

func NewDiscoverer(logger *log.Logger) *Discoverer {
	region := os.Getenv("AWS_REGION")
	if region == "" {
		logger.Fatal("AWS_REGION not found")
	}

	nodeInstanceProfileName := os.Getenv("AWS_NODE_IAM_INSTANCE_PROFILE")
	if nodeInstanceProfileName == "" {
		logger.Warn("AWS_NODE_IAM_INSTANCE_PROFILE not found, the IMDS state of the instances will not be checked")
	} else {
		registerIMDSMetrics()
	}

	return &Discoverer{
		logger:                  logger,
		region:                  region,
		nodeInstanceProfileName: nodeInstanceProfileName,
	}
}

func (d *Discoverer) CheckCloudConditions(ctx context.Context) ([]v1alpha1.CloudCondition, error) {
	sess, err := session.NewSessionWithOptions(session.Options{
		Config: aws.Config{
			Region: aws.String(d.region),
		},
	})
	if err != nil {
		return nil, fmt.Errorf("failed to initialize new session: %v", err)
	}

	ec2Client := ec2.New(sess)

	var res []v1alpha1.CloudCondition

	// check permission for DescribeAddressesAttribute
	log.Debug("checking permission for DescribeAddressesAttribute")
	_, err = ec2Client.DescribeAddressesAttribute(&ec2.DescribeAddressesAttributeInput{
		DryRun: aws.Bool(true),
	})
	if err != nil {
		var awsErr awserr.Error
		if !errors.As(err, &awsErr) {
			return nil, fmt.Errorf("DescribeAddressesAttribute AWS IAM permission check error: %v", err)
		}

		if awsErr.Code() != "DryRunOperation" {
			res = append(res, v1alpha1.CloudCondition{
				Name:    "INSUFFICIENT_AWS_SA_PERMISSIONS",
				Message: "DescribeAddressesAttribute is not allowed",
				Ok:      false,
			})
		}
	}

	// check permission for DescribeInstanceTopology
	log.Debug("checking permission for DescribeInstanceTopology")
	_, err = ec2Client.DescribeInstanceTopology(&ec2.DescribeInstanceTopologyInput{
		DryRun: aws.Bool(true),
	})
	if err != nil {
		var awsErr awserr.Error
		if !errors.As(err, &awsErr) {
			return nil, fmt.Errorf("DescribeInstanceTopology AWS IAM permission check error: %v", err)
		}

		switch awsErr.Code() {
		case "DryRunOperation":
		case "UnsupportedOperation":
			d.logger.Warn("DescribeInstanceTopology is not supported in this region, skipping permission check",
				"region", d.region,
				"aws_error_code", awsErr.Code(),
				"aws_error_message", awsErr.Message(),
			)
		default:
			res = append(res, v1alpha1.CloudCondition{
				Name:    "INSUFFICIENT_AWS_SA_PERMISSIONS",
				Message: "DescribeInstanceTopology is not allowed",
				Ok:      false,
			})
		}
	}

	// The IMDS state of the instances is observed here because this is the one method the
	// reconciler calls on every discovery period. It is exported as a metric rather than
	// returned as a condition: allowing IMDSv1 is the supported default, it only becomes a
	// problem once the cluster asked for IMDSv2, and that is known to the module, not here.
	d.updateIMDSMetrics(ctx, ec2Client)

	return res, nil
}

func (d *Discoverer) updateIMDSMetrics(ctx context.Context, ec2Client ec2iface.EC2API) {
	if d.nodeInstanceProfileName == "" {
		return
	}

	count, err := countInstancesAllowingIMDSv1(ctx, ec2Client, d.nodeInstanceProfileName)
	if err != nil {
		// A failed check must not hide the permission conditions gathered above, and keeping
		// the previous value is better than reporting a made up one.
		d.logger.Error("Error occurred while checking the IMDS state of the instances", "error", err)
		return
	}

	imdsv1AllowedInstances.Set(float64(count))
}

func (d *Discoverer) InstanceTypes(_ context.Context) ([]v1alpha1.InstanceType, error) {
	sess, err := session.NewSessionWithOptions(session.Options{
		Config: aws.Config{
			Region: aws.String(d.region),
		},
	})

	if err != nil {
		return nil, fmt.Errorf("failed to initialize new session: %v", err)
	}

	ec2Client := ec2.New(sess)
	res := make([]v1alpha1.InstanceType, 0)

	var token *string

	for {
		out, err := ec2Client.DescribeInstanceTypes(&ec2.DescribeInstanceTypesInput{
			MaxResults: ptr.To(int64(100)),
			NextToken:  token,
		})
		if err != nil {
			return nil, err
		}

		for _, ins := range out.InstanceTypes {
			if ins.InstanceType == nil {
				return nil, fmt.Errorf("instance type is nil")
			}

			name := *ins.InstanceType

			if ins.VCpuInfo == nil {
				return nil, fmt.Errorf("VCpuInfo is nil for %s", name)
			}

			if ins.VCpuInfo.DefaultVCpus == nil {
				return nil, fmt.Errorf("VCpuInfo.DefaultVCpus is nil for %s", name)
			}

			if ins.MemoryInfo == nil {
				return nil, fmt.Errorf("MemoryInfo is nil for %s", name)
			}

			if ins.MemoryInfo.SizeInMiB == nil {
				return nil, fmt.Errorf("MemoryInfo.SizeInMiB is nil for %s", name)
			}

			res = append(res, v1alpha1.InstanceType{
				Name:     name,
				CPU:      resource.MustParse(strconv.FormatInt(*ins.VCpuInfo.DefaultVCpus, 10)),
				Memory:   resource.MustParse(strconv.FormatInt(*ins.MemoryInfo.SizeInMiB, 10) + "Mi"),
				RootDisk: resource.MustParse("0"),
			})
		}

		if out.NextToken == nil || *out.NextToken == "" {
			break
		} else {
			token = out.NextToken
		}
	}

	return res, nil
}

func (d *Discoverer) DiscoveryData(ctx context.Context, cloudProviderDiscoveryData []byte) ([]byte, error) {
	return nil, nil
}

// NotImplemented
func (d *Discoverer) DisksMeta(ctx context.Context) ([]v1alpha1.DiskMeta, error) {
	return []v1alpha1.DiskMeta{}, nil
}
