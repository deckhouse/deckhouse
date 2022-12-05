/*
Copyright 2022 Flant JSC

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

package instance_types

import "errors"

var (
	ErrNotFound = errors.New("Instance Type not found")
)

// InstanceType is spec of EC2 instance
type InstanceType struct {
	InstanceType string
	VCPU         int
	MemoryMb     int
	GPU          int
	Architecture string
}

// 	GetInstanceType search for instance type by specified instance class (provider)
func GetInstanceType(instanceClass, instanceTypeName string) (*InstanceType, error) {
	var (
		instanceType *InstanceType
		found        bool
	)

	switch instanceClass {
	case "AWSInstanceClass":
		instanceType, found = awsInstanceTypes[instanceTypeName]

	case "AzureInstanceClass":
		instanceType, found = azureInstanceTypes[instanceTypeName]

	case "GCPInstanceClass":
		instanceType, found = gcpInstanceTypes[instanceTypeName]

	case "OpenStackInstanceClass":
		instanceType, found = openstackInstanceTypes[instanceTypeName]

	default:
		return nil, ErrNotFound
	}

	if !found {
		return nil, ErrNotFound
	}

	return instanceType, nil
}
