// Copyright 2023 Flant JSC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//	http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package v1

// Parameters of a group of vSphere VirtualMachines used by `machine-controller-manager`
type VsphereInstanceClass struct {
	// Count of vCPUs to allocate to vSphere VirtualMachines.
	NumCPUs *int32 `json:"numCPUs,omitempty" yaml:"numCPUs,omitempty"`
	// Memory in MiB to allocate to vSphere VirtualMachines.
	Memory *int32 `json:"memory,omitempty" yaml:"memory,omitempty"`
	// Root disk size in GiB to use in vSphere VirtualMachines.
	RootDiskSize *int32 `json:"rootDiskSize,omitempty" yaml:"rootDiskSize,omitempty"`
	// Path to the template to be cloned. Relative to the datacenter.
	Template *string `json:"template,omitempty" yaml:"template,omitempty"`
	// Path to the network that VirtualMachines' primary NICs will connect to (default gateway). Relative to the datacenter.
	MainNetwork *string `json:"mainNetwork,omitempty" yaml:"mainNetwork,omitempty"`
	// Paths to networks that VirtualMachines' secondary NICs will connect to. Relative to the datacenter.
	AdditionalNetworks *[]string `json:"additionalNetworks,omitempty" yaml:"additionalNetworks,omitempty"`
	// Path to a datastore in which VirtualMachines will be cloned. Relative to the datacenter.
	Datastore *string `json:"datastore,omitempty" yaml:"datastore,omitempty"`
	// Disable time synchronization in Guest VM.
	DisableTimesync *bool `json:"disableTimesync,omitempty" yaml:"disableTimesync,omitempty"`
	// Path to a Resource Pool in which VirtualMachines will be cloned. Relative to the zone (vSphere Cluster).
	ResourcePool *string `json:"resourcePool,omitempty" yaml:"resourcePool,omitempty"`
	// Additional VM's parameters.
	RuntimeOptions *VsphereInstanceClassRuntimeOptions `json:"runtimeOptions,omitempty" yaml:"runtimeOptions,omitempty"`
}

type VsphereInstanceClassRuntimeOptions struct {
	// Whether to enable or disable nested [hardware virtualization](https://docs.vmware.com/en/VMware-vSphere/6.5/com.vmware.vsphere.vm_admin.doc/GUID-2A98801C-68E8-47AF-99ED-00C63E4857F6.html).
	NestedHardwareVirtualization bool `json:"nestedHardwareVirtualization,omitempty" yaml:"nestedHardwareVirtualization,omitempty"`
	// The relative amount of CPU Shares for VMs to be created.
	CPUShares *int32 `json:"cpuShares,omitempty" yaml:"cpuShares,omitempty"`
	// CPU limit in MHz.
	CPULimit *int32 `json:"cpuLimit,omitempty" yaml:"cpuLimit,omitempty"`
	// CPU reservation in MHz.
	CPUReservation *int32 `json:"cpuReservation,omitempty" yaml:"cpuReservation,omitempty"`
	// The relative amount of Memory Shares for VMs to be created.
	MemoryShares *int32 `json:"memoryShares,omitempty" yaml:"memoryShares,omitempty"`
	// Memory limit in MB.
	MemoryLimit *int32 `json:"memoryLimit,omitempty" yaml:"memoryLimit,omitempty"`
	// VM memory reservation in percent (relative to `.spec.memory`).
	MemoryReservation *int32 `json:"memoryReservation,omitempty" yaml:"memoryReservation,omitempty"`
}
