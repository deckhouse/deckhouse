/*
Copyright 2021 Flant JSC

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

package v1

// +genclient
// +genclient:nonNamespaced
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// Parameters of a group of vSphere VirtualMachines used by `machine-controller-manager`
type VsphereInstanceClass struct {
	// Count of vCPUs to allocate to vSphere VirtualMachines.
	NumCPUs int `json:"numCPUs" yaml:"numCPUs"`
	// Memory in MiB to allocate to vSphere VirtualMachines.
	Memory int `json:"memory" yaml:"memory"`
	// Root disk size in GiB to use in vSphere VirtualMachines.
	RootDiskSize int `json:"rootDiskSize" yaml:"rootDiskSize"`
	// Path to the template to be cloned. Relative to the datacenter.
	Template string `json:"template" yaml:"template"`
	// Path to the network that VirtualMachines' primary NICs will connect to (default gateway). Relative to the datacenter.
	MainNetwork string `json:"mainNetwork" yaml:"mainNetwork"`
	// Paths to networks that VirtualMachines' secondary NICs will connect to. Relative to the datacenter.
	AdditionalNetworks []string `json:"additionalNetworks" yaml:"additionalNetworks"`
	// Path to a datastore in which VirtualMachines will be cloned. Relative to the datacenter.
	Datastore string `json:"datastore" yaml:"datastore"`
	// Disable time synchronization in Guest VM.
	DisableTimesync bool `json:"disableTimesync" yaml:"disableTimesync"`
	// Path to a Resource Pool in which VirtualMachines will be cloned. Relative to the zone (vSphere Cluster).
	ResourcePool string `json:"resourcePool" yaml:"resourcePool"`
	// Additional VM's parameters.
	RuntimeOptions VsphereInstanceClassRuntimeOptions `json:"runtimeOptions" yaml:"runtimeOptions"`
}

type VsphereInstanceClassRuntimeOptions struct {
	// Whether to enable or disable nested [hardware virtualization](https://docs.vmware.com/en/VMware-vSphere/6.5/com.vmware.vsphere.vm_admin.doc/GUID-2A98801C-68E8-47AF-99ED-00C63E4857F6.html).
	NestedHardwareVirtualization bool `json:"nestedHardwareVirtualization" yaml:"nestedHardwareVirtualization"`
	// The relative amount of CPU Shares for VMs to be created.
	CpuShares int `json:"cpuShares" yaml:"cpuShares"`
	// CPU limit in MHz.
	CpuLimit int `json:"cpuLimit" yaml:"cpuLimit"`
	// CPU reservation in MHz.
	CpuReservation int `json:"cpuReservation" yaml:"cpuReservation"`
	// The relative amount of Memory Shares for VMs to be created.
	MemoryShares int `json:"memoryShares" yaml:"memoryShares"`
	// Memory limit in MB.
	MemoryLimit int `json:"memoryLimit" yaml:"memoryLimit"`
	// VM memory reservation in percent (relative to `.spec.memory`).
	MemoryReservation int `json:"memoryReservation" yaml:"memoryReservation"`
}
