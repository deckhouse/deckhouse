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

func Discover(providerName string) (map[string]interface{}, error) {
	switch providerName {
	case "aws":
		return map[string]interface{}{
			"knownInstanceTypes": awsInstanceTypes,
		}, nil
	case "azure":
		return map[string]interface{}{
			"knownInstanceTypes": azureInstanceTypes,
		}, nil
	case "gcp":
		return map[string]interface{}{
			"knownInstanceTypes": gcpInstanceTypes,
		}, nil
	case "openstack":
		return map[string]interface{}{
			"knownInstanceTypes": openstackInstanceTypes,
		}, nil
	default:
		return map[string]interface{}{}, nil
	}
}

// InstanceType is the spec of an instance
type InstanceType struct {
	InstanceType string
	VCPU         int
	MemoryMb     int
	GPU          int
	Architecture string
}
