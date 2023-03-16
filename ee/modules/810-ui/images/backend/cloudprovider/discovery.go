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

// Copied from modules/040-node-manager/hooks/internal/autoscaler/instance_types/instance_type.go

package cloudprovider

import (
	"context"

	"k8s.io/client-go/kubernetes"
)

func Discover(ctx context.Context, providerName string, clientset *kubernetes.Clientset) (map[string]interface{}, error) {
	config, err := GetClusterConfig(ctx, clientset)
	if err != nil {
		return nil, err
	}

	var knownTypes map[string]*InstanceType

	switch providerName {
	case "aws":
		knownTypes = awsInstanceTypes
	case "azure":
		knownTypes = azureInstanceTypes
	case "gcp":
		knownTypes = gcpInstanceTypes
	case "openstack":
		knownTypes = openstackInstanceTypes
	}

	res := map[string]interface{}{
		"configuration": map[string]interface{}{
			"zones": config.Zones(),
		},
	}

	if knownTypes != nil {
		res["knownInstanceTypes"] = knownTypes
	}

	return res, nil
}

// InstanceType is the spec of an instance
type InstanceType struct {
	InstanceType string
	VCPU         int
	MemoryMb     int
	GPU          int
	Architecture string
}
