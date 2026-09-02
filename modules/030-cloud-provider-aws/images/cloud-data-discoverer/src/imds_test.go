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
	"errors"
	"testing"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/request"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/aws/aws-sdk-go/service/ec2/ec2iface"
)

type fakeEC2 struct {
	ec2iface.EC2API

	pages []*ec2.DescribeInstancesOutput
	err   error

	gotInput *ec2.DescribeInstancesInput
}

func (f *fakeEC2) DescribeInstancesPagesWithContext(_ aws.Context, input *ec2.DescribeInstancesInput, fn func(*ec2.DescribeInstancesOutput, bool) bool, _ ...request.Option) error {
	f.gotInput = input

	if f.err != nil {
		return f.err
	}

	for i, page := range f.pages {
		if !fn(page, i == len(f.pages)-1) {
			break
		}
	}

	return nil
}

const nodeProfile = "kube-node"

func instance(profileName, httpTokens, httpEndpoint string) *ec2.Instance {
	i := &ec2.Instance{
		MetadataOptions: &ec2.InstanceMetadataOptionsResponse{
			HttpTokens:   aws.String(httpTokens),
			HttpEndpoint: aws.String(httpEndpoint),
		},
	}

	if profileName != "" {
		i.IamInstanceProfile = &ec2.IamInstanceProfile{
			Arn: aws.String("arn:aws:iam::123456789012:instance-profile/" + profileName),
		}
	}

	return i
}

func page(instances ...*ec2.Instance) *ec2.DescribeInstancesOutput {
	return &ec2.DescribeInstancesOutput{
		Reservations: []*ec2.Reservation{{Instances: instances}},
	}
}

func TestCountInstancesAllowingIMDSv1(t *testing.T) {
	tests := []struct {
		name  string
		pages []*ec2.DescribeInstancesOutput
		want  int
	}{
		{
			name:  "no instances at all",
			pages: nil,
			want:  0,
		},
		{
			name: "every instance requires a session token",
			pages: []*ec2.DescribeInstancesOutput{page(
				instance(nodeProfile, ec2.HttpTokensStateRequired, ec2.InstanceMetadataEndpointStateEnabled),
				instance(nodeProfile, ec2.HttpTokensStateRequired, ec2.InstanceMetadataEndpointStateEnabled),
			)},
			want: 0,
		},
		{
			name: "instances allowing IMDSv1 are counted across pages",
			pages: []*ec2.DescribeInstancesOutput{
				page(
					instance(nodeProfile, ec2.HttpTokensStateOptional, ec2.InstanceMetadataEndpointStateEnabled),
					instance(nodeProfile, ec2.HttpTokensStateRequired, ec2.InstanceMetadataEndpointStateEnabled),
				),
				page(instance(nodeProfile, ec2.HttpTokensStateOptional, ec2.InstanceMetadataEndpointStateEnabled)),
			},
			want: 2,
		},
		{
			name: "a disabled metadata service serves no IMDSv1",
			pages: []*ec2.DescribeInstancesOutput{page(
				instance(nodeProfile, ec2.HttpTokensStateOptional, ec2.InstanceMetadataEndpointStateDisabled),
			)},
			want: 0,
		},
		{
			name:  "an instance without reported metadata options is not counted",
			pages: []*ec2.DescribeInstancesOutput{page(&ec2.Instance{})},
			want:  0,
		},
		{
			name: "instances of another cluster are not counted",
			pages: []*ec2.DescribeInstancesOutput{page(
				instance("other-node", ec2.HttpTokensStateOptional, ec2.InstanceMetadataEndpointStateEnabled),
			)},
			want: 0,
		},
		{
			name: "a cluster whose profile name starts the same way is not counted",
			pages: []*ec2.DescribeInstancesOutput{page(
				instance(nodeProfile+"-2", ec2.HttpTokensStateOptional, ec2.InstanceMetadataEndpointStateEnabled),
			)},
			want: 0,
		},
		{
			name: "an instance with no instance profile is not counted",
			pages: []*ec2.DescribeInstancesOutput{page(
				instance("", ec2.HttpTokensStateOptional, ec2.InstanceMetadataEndpointStateEnabled),
			)},
			want: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := countInstancesAllowingIMDSv1(context.Background(), &fakeEC2{pages: tt.pages}, nodeProfile)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tt.want {
				t.Fatalf("got %d instances allowing IMDSv1, want %d", got, tt.want)
			}
		})
	}
}

func TestCountInstancesAllowingIMDSv1SelectsClusterInstances(t *testing.T) {
	client := &fakeEC2{}

	if _, err := countInstancesAllowingIMDSv1(context.Background(), client, nodeProfile); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	filters := map[string][]string{}
	for _, filter := range client.gotInput.Filters {
		filters[aws.StringValue(filter.Name)] = aws.StringValueSlice(filter.Values)
	}

	// Narrow the request down to the instances the alert is about. No wildcards: only the filters
	// whose matching semantics the EC2 API documents are used, and the cluster of an instance is
	// decided locally, on its instance profile ARN.
	if got := filters["metadata-options.http-tokens"]; len(got) != 1 || got[0] != ec2.HttpTokensStateOptional {
		t.Fatalf("got http tokens filter %v, want [%s]", got, ec2.HttpTokensStateOptional)
	}
	if _, ok := filters["iam-instance-profile.arn"]; ok {
		t.Fatal("the instance profile must not be filtered server side, its ARN pattern would need wildcards")
	}

	// Terminated instances keep being returned by the API for a while, and metadata options of
	// a stopped instance say nothing about the state its node will boot with.
	states := filters["instance-state-name"]
	if len(states) != 2 || states[0] != ec2.InstanceStateNamePending || states[1] != ec2.InstanceStateNameRunning {
		t.Fatalf("got instance state filter %v, want [pending running]", states)
	}
}

func TestCountInstancesAllowingIMDSv1Error(t *testing.T) {
	wantErr := errors.New("access denied")

	_, err := countInstancesAllowingIMDSv1(context.Background(), &fakeEC2{err: wantErr}, nodeProfile)
	if err == nil {
		t.Fatal("expected an error when the API call fails")
	}
	if !errors.Is(err, wantErr) {
		t.Fatalf("got error %v, want it to wrap %v", err, wantErr)
	}
}
