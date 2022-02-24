/*
Copyright 2021 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package v1

// Parameters of a group of vSphere VirtualMachines used by `machine-controller-manager`
type OpenstackInstanceClass struct {
	MainNetwork              string            `json:"mainNetwork,omitempty" yaml:"mainNetwork,omitempty"`
	AdditionalNetworks       []string          `json:"additionalNetworks,omitempty" yaml:"additionalNetworks,omitempty"`
	AdditionalSecurityGroups []string          `json:"additionalSecurityGroups,omitempty" yaml:"additionalSecurityGroups,omitempty"`
	AdditionalTags           map[string]string `json:"additionalTags,omitempty" yaml:"additionalTags,omitempty"`
	FlavorName               string            `json:"flavorName,omitempty" yaml:"flavorName,omitempty"`
	ImageName                string            `json:"imageName,omitempty" yaml:"imageName,omitempty"`
	RootDiskSize             int32             `json:"rootDiskSize,omitempty" yaml:"rootDiskSize,omitempty"`
}

// type VsphereInstanceClassRuntimeOptions struct {
// 	// Whether to enable or disable nested [hardware virtualization](https://docs.vmware.com/en/VMware-vSphere/6.5/com.vmware.vsphere.vm_admin.doc/GUID-2A98801C-68E8-47AF-99ED-00C63E4857F6.html).
// 	NestedHardwareVirtualization bool `json:"nestedHardwareVirtualization,omitempty" yaml:"nestedHardwareVirtualization,omitempty"`
// 	// The relative amount of CPU Shares for VMs to be created.
// 	CPUShares *int32 `json:"cpuShares,omitempty" yaml:"cpuShares,omitempty"`
// 	// CPU limit in MHz.
// 	CPULimit *int32 `json:"cpuLimit,omitempty" yaml:"cpuLimit,omitempty"`
// 	// CPU reservation in MHz.
// 	CPUReservation *int32 `json:"cpuReservation,omitempty" yaml:"cpuReservation,omitempty"`
// 	// The relative amount of Memory Shares for VMs to be created.
// 	MemoryShares *int32 `json:"memoryShares,omitempty" yaml:"memoryShares,omitempty"`
// 	// Memory limit in MB.
// 	MemoryLimit *int32 `json:"memoryLimit,omitempty" yaml:"memoryLimit,omitempty"`
// 	// VM memory reservation in percent (relative to `.spec.memory`).
// 	MemoryReservation *int32 `json:"memoryReservation,omitempty" yaml:"memoryReservation,omitempty"`
// }
