/*
Copyright 2021 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package v1

import (
	v1 "k8s.io/api/core/v1"
)

type VsphereProviderClusterConfiguration struct {
	APIVersion           *string                 `json:"apiVersion,omitempty" yaml:"apiVersion,omitempty"`
	Kind                 *string                 `json:"kind,omitempty" yaml:"kind,omitempty"`
	MasterNodeGroup      *VsphereMasterNodeGroup `json:"masterNodeGroup,omitempty" yaml:"masterNodeGroup,omitempty"`
	NodeGroups           *[]VsphereNodeGroup     `json:"nodeGroups,omitempty" yaml:"nodeGroups,omitempty"`
	SSHPublicKey         *string                 `json:"sshPublicKey" yaml:"sshPublicKey"`
	RegionTagCategory    *string                 `json:"regionTagCategory" yaml:"regionTagCategory"`
	ZoneTagCategory      *string                 `json:"zoneTagCategory" yaml:"zoneTagCategory"`
	DisableTimesync      *bool                   `json:"disableTimesync,omitempty" yaml:"disableTimesync,omitempty"`
	ExternalNetworkNames *[]string               `json:"externalNetworkNames,omitempty" yaml:"externalNetworkNames,omitempty"`
	InternalNetworkNames *[]string               `json:"internalNetworkNames,omitempty" yaml:"internalNetworkNames,omitempty"`
	InternalNetworkCIDR  *string                 `json:"internalNetworkCIDR,omitempty" yaml:"internalNetworkCIDR,omitempty"`
	VMFolderPath         *string                 `json:"vmFolderPath" yaml:"vmFolderPath"`
	Region               *string                 `json:"region" yaml:"region"`
	Zones                *[]string               `json:"zones" yaml:"zones"`
	BaseResourcePool     *string                 `json:"baseResourcePool,omitempty" yaml:"baseResourcePool,omitempty"`
	Layout               *string                 `json:"layout,omitempty" yaml:"layout,omitempty"`
	Provider             *VsphereProvider        `json:"provider" yaml:"provider"`
	Nsxt                 *VsphereNsxt            `json:"nsxt,omitempty" yaml:"nsxt,omitempty"`
}

type VsphereProvider struct {
	Server   *string `json:"server" yaml:"server"`
	Username *string `json:"username" yaml:"username"`
	Password *string `json:"password" yaml:"password"`
	Insecure *bool   `json:"insecure" yaml:"insecure"`
}

type VsphereMasterNodeGroup struct {
	Replicas      *int32                         `json:"replicas" yaml:"replicas"`
	Zones         *[]string                      `json:"zones,omitempty" yaml:"zones,omitempty"`
	InstanceClass *VsphereNodeGroupInstanceClass `json:"instanceClass" yaml:"instanceClass"`
}

type VsphereNodeGroupInstanceClass struct {
	VsphereInstanceClass
	MainNetworkIPAddresses *[]VsphereMainNetworkIPAddresses `json:"mainNetworkIPAddresses,omitempty" yaml:"mainNetworkIPAddresses,omitempty"`
}

type VsphereMainNetworkIPAddresses struct {
	Address     *string                 `json:"address" yaml:"address"`
	Gateway     *string                 `json:"gateway" yaml:"gateway"`
	Nameservers *IPAddressesNameservers `json:"nameservers" yaml:"nameservers"`
}

type IPAddressesNameservers struct {
	Addresses *[]string `json:"addresses" yaml:"addresses"`
	Search    *[]string `json:"search" yaml:"search"`
}

type VsphereNodeGroup struct {
	Name          *string                        `json:"name" yaml:"name"`
	Replicas      *int32                         `json:"replicas" yaml:"replicas"`
	Zones         *[]string                      `json:"zones" yaml:"zones"`
	NodeTemplate  *NodeTemplate                  `json:"nodeTemplate,omitempty" yaml:"nodeTemplate"`
	InstanceClass *VsphereNodeGroupInstanceClass `json:"instanceClass" yaml:"instanceClass"`
}

// NodeTemplate copied from 040-node-manager/hooks/internal/v1/nodegroup.go
type NodeTemplate struct {
	// Annotations is an unstructured key value map that is used as default
	// annotations for Nodes in NodeGroup.
	// More info: http://kubernetes.io/docs/user-guide/annotations
	// +optional
	Annotations map[string]string `json:"annotations,omitempty"`

	// Map of string keys and values that is used as default
	// labels for Nodes in NodeGroup.
	// More info: http://kubernetes.io/docs/user-guide/labels
	// +optional
	Labels map[string]string `json:"labels"`

	// Default taints for Nodes in NodeGroup.
	Taints *[]v1.Taint `json:"taints,omitempty"`
}
