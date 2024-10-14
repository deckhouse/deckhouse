/*
Copyright 2021 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package v1

import (
	nm "github.com/deckhouse/deckhouse/modules/040-node-manager/hooks/pkg/schema"
)

type VsphereProviderClusterConfiguration struct {
	APIVersion           *string                 `json:"apiVersion,omitempty" yaml:"apiVersion,omitempty"`
	Kind                 *string                 `json:"kind,omitempty" yaml:"kind,omitempty"`
	MasterNodeGroup      *VsphereMasterNodeGroup `json:"masterNodeGroup,omitempty" yaml:"masterNodeGroup,omitempty"`
	NodeGroups           *[]VsphereNodeGroup     `json:"nodeGroups,omitempty" yaml:"nodeGroups,omitempty"`
	SSHPublicKey         *string                 `json:"sshPublicKey,omitempty" yaml:"sshPublicKey,omitempty"`
	RegionTagCategory    *string                 `json:"regionTagCategory,omitempty" yaml:"regionTagCategory,omitempty"`
	ZoneTagCategory      *string                 `json:"zoneTagCategory,omitempty" yaml:"zoneTagCategory,omitempty"`
	DisableTimesync      *bool                   `json:"disableTimesync,omitempty" yaml:"disableTimesync,omitempty"`
	ExternalNetworkNames *[]string               `json:"externalNetworkNames,omitempty" yaml:"externalNetworkNames,omitempty"`
	InternalNetworkNames *[]string               `json:"internalNetworkNames,omitempty" yaml:"internalNetworkNames,omitempty"`
	InternalNetworkCIDR  *string                 `json:"internalNetworkCIDR,omitempty" yaml:"internalNetworkCIDR,omitempty"`
	VMFolderPath         *string                 `json:"vmFolderPath,omitempty" yaml:"vmFolderPath,omitempty"`
	VMFolderExists       bool                    `json:"vmFolderExists" yaml:"vmFolderExists"`
	Region               *string                 `json:"region,omitempty" yaml:"region,omitempty"`
	Zones                *[]string               `json:"zones,omitempty" yaml:"zones,omitempty"`
	BaseResourcePool     *string                 `json:"baseResourcePool,omitempty" yaml:"baseResourcePool,omitempty"`
	Layout               *string                 `json:"layout,omitempty" yaml:"layout,omitempty"`
	Provider             *VsphereProvider        `json:"provider,omitempty" yaml:"provider,omitempty"`
	Nsxt                 *VsphereNsxt            `json:"nsxt,omitempty" yaml:"nsxt,omitempty"`
}

type VsphereProvider struct {
	Server   *string `json:"server,omitempty" yaml:"server,omitempty"`
	Username *string `json:"username,omitempty" yaml:"username,omitempty"`
	Password *string `json:"password,omitempty" yaml:"password,omitempty"`
	Insecure *bool   `json:"insecure,omitempty" yaml:"insecure,omitempty"`
}

type VsphereMasterNodeGroup struct {
	Replicas      *int32                         `json:"replicas,omitempty" yaml:"replicas,omitempty"`
	Zones         *[]string                      `json:"zones,omitempty" yaml:"zones,omitempty"`
	InstanceClass *VsphereNodeGroupInstanceClass `json:"instanceClass,omitempty" yaml:"instanceClass,omitempty"`
}

type VsphereNodeGroupInstanceClass struct {
	VsphereInstanceClass
	MainNetworkIPAddresses *[]VsphereMainNetworkIPAddresses `json:"mainNetworkIPAddresses,omitempty" yaml:"mainNetworkIPAddresses,omitempty"`
}

type VsphereMainNetworkIPAddresses struct {
	Address     *string                 `json:"address,omitempty" yaml:"address,omitempty"`
	Gateway     *string                 `json:"gateway,omitempty" yaml:"gateway,omitempty"`
	Nameservers *IPAddressesNameservers `json:"nameservers,omitempty" yaml:"nameservers,omitempty"`
}

type IPAddressesNameservers struct {
	Addresses *[]string `json:"addresses,omitempty" yaml:"addresses,omitempty"`
	Search    *[]string `json:"search,omitempty" yaml:"search,omitempty"`
}

type VsphereNodeGroup struct {
	Name          *string                        `json:"name,omitempty" yaml:"name,omitempty"`
	Replicas      *int32                         `json:"replicas,omitempty" yaml:"replicas,omitempty"`
	Zones         *[]string                      `json:"zones,omitempty" yaml:"zones,omitempty"`
	NodeTemplate  *nm.NodeTemplate               `json:"nodeTemplate,omitempty" yaml:"nodeTemplate,omitempty"`
	InstanceClass *VsphereNodeGroupInstanceClass `json:"instanceClass,omitempty" yaml:"instanceClass,omitempty"`
}
