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

// openstackInstanceTypes generated from https://docs.openstack.org/nova/rocky/admin/flavors.html Default Flavors
var openstackInstanceTypes = map[string]*InstanceType{
	"m1.tiny": {
		InstanceType: "m1.tiny",
		VCPU:         1,
		MemoryMb:     512,
		GPU:          0,
		Architecture: "amd64",
	},
	"m1.small": {
		InstanceType: "m1.small",
		VCPU:         1,
		MemoryMb:     2048,
		GPU:          0,
		Architecture: "amd64",
	},
	"m1.medium": {
		InstanceType: "m1.medium",
		VCPU:         2,
		MemoryMb:     4096,
		GPU:          0,
		Architecture: "amd64",
	},
	"m1.large": {
		InstanceType: "m1.large",
		VCPU:         4,
		MemoryMb:     8192,
		GPU:          0,
		Architecture: "amd64",
	},
	"m1.xlarge": {
		InstanceType: "m1.xlarge",
		VCPU:         8,
		MemoryMb:     16384,
		GPU:          0,
		Architecture: "amd64",
	},
}
