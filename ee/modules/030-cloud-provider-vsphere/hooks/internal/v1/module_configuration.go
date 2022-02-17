/*
Copyright 2021 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package v1

type VsphereModuleConfiguration struct {
	// the domain of the vCenter server
	Host *string `json:"host,omitempty" yaml:"host,omitempty"`
	// the login ID
	Username *string `json:"username,omitempty" yaml:"username,omitempty"`
	// the password
	Password *string `json:"password,omitempty" yaml:"password,omitempty"`
	// can be set to `true` if vCenter has a self-signed certificate
	Insecure *bool `json:"insecure,omitempty" yaml:"insecure,omitempty"`
	// the path to the VirtualMachine Folder where the cloned VMs will be created
	VMFolderPath *string `json:"vmFolderPath,omitempty" yaml:"vmFolderPath,omitempty"`
	// the name of the tag **category** used to identify the region (vSphere Datacenter)
	RegionTagCategory *string `json:"regionTagCategory,omitempty" yaml:"regionTagCategory,omitempty"`
	// the name of the tag **category** used to identify the region (vSphere Cluster)
	ZoneTagCategory *string `json:"zoneTagCategory,omitempty" yaml:"zoneTagCategory,omitempty"`
	// disable time synchronization on the vSphere side
	DisableTimesync *bool `json:"disableTimesync,omitempty" yaml:"disableTimesync,omitempty"`
	// is a tag added to the vSphere Datacenter where all actions will occur: provisioning VirtualMachines, storing virtual disks on datastores, connecting to the network
	Region *string `json:"region,omitempty" yaml:"region,omitempty"`
	// the globally restricted set of zones that this Cloud Provider works with
	Zones *[]string `json:"zones,omitempty" yaml:"zones,omitempty"`
	// a list of public SSH keys in plain-text format
	SSHKeys *[]string `json:"sshKeys,omitempty" yaml:"sshKeys,omitempty"`
	// a list of names of networks (just the name and not the full path) connected to VirtualMachines and used by vsphere-cloud-controller-manager to insert ExternalIP into the `.status.addresses` field in the Node API object
	ExternalNetworkNames *[]string `json:"externalNetworkNames,omitempty" yaml:"externalNetworkNames,omitempty"`
	// a list of names of networks (just the name and not the full path) connected to VirtualMachines and used by vsphere-cloud-controller-manager to insert InternalIP into the `.status.addresses` field in the Node API object
	InternalNetworkNames *[]string `json:"internalNetworkNames,omitempty" yaml:"internalNetworkNames,omitempty"`
	// storageclass settings
	StorageClass *VsphereModuleStorageClass `json:"storageClass,omitempty" yaml:"storageClass,omitempty"`
	// a flag allowing the use of the old CSI version
	CompatibilityFlag *string `json:"compatibilityFlag,omitempty" yaml:"compatibilityFlag,omitempty"`
	// nsx-t settings
	Nsxt *VsphereNsxt `json:"nsxt,omitempty" yaml:"nsxt,omitempty"`
}

type VsphereModuleStorageClass struct {
	// a list of StorageClass names (or regex expressions for names) to exclude from the creation in the cluster
	Exclude *[]string `json:"exclude,omitempty" yaml:"exclude,omitempty"`
	// the name of StorageClass that will be used by default in the cluster
	Default *string `json:"default,omitempty" yaml:"default,omitempty"`
}
