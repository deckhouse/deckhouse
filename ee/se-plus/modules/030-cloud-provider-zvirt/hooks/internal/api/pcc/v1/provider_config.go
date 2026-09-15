/*
Copyright 2026 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

// Package v1 contains the typed legacy ZvirtClusterConfiguration model used by the module hooks.
package v1

import (
	"reflect"

	cpapi "github.com/deckhouse/deckhouse/go_lib/cloud-provider/api"
)

var (
	_ cpapi.ProviderClusterConfigObject = (*ZvirtProviderClusterConfiguration)(nil)
)

// ZvirtProviderClusterConfiguration describes the configuration of a cloud cluster in zVirt.
type ZvirtProviderClusterConfiguration struct {
	APIVersion string `json:"apiVersion,omitempty" yaml:"apiVersion,omitempty"`
	Kind       string `json:"kind,omitempty" yaml:"kind,omitempty"`
	Layout     string `json:"layout,omitempty" yaml:"layout,omitempty"`

	SSHPublicKey string `json:"sshPublicKey,omitempty" yaml:"sshPublicKey,omitempty"`

	// ClusterID is the zVirt cluster with shared storage domains and CPUs of the same type
	// used to create virtual machines.
	ClusterID string `json:"clusterID,omitempty" yaml:"clusterID,omitempty"`

	MasterNodeGroup ZvirtMasterNodeGroup   `json:"masterNodeGroup,omitempty" yaml:"masterNodeGroup,omitempty"`
	NodeGroups      []ZvirtStaticNodeGroup `json:"nodeGroups,omitempty" yaml:"nodeGroups,omitempty"`
	Provider        ZvirtProvider          `json:"provider,omitempty" yaml:"provider,omitempty"`
}

// ZvirtProvider contains settings to connect to the zVirt API.
type ZvirtProvider struct {
	Server   string `json:"server,omitempty" yaml:"server,omitempty"`
	Username string `json:"username,omitempty" yaml:"username,omitempty"`
	Password string `json:"password,omitempty" yaml:"password,omitempty"`
	CABundle string `json:"caBundle,omitempty" yaml:"caBundle,omitempty"`
	Insecure bool   `json:"insecure,omitempty" yaml:"insecure,omitempty"`
}

// ZvirtMasterNodeGroup defines the master's NodeGroup.
type ZvirtMasterNodeGroup struct {
	Replicas      int                      `json:"replicas,omitempty" yaml:"replicas,omitempty"`
	InstanceClass ZvirtMasterInstanceClass `json:"instanceClass,omitempty" yaml:"instanceClass,omitempty"`
}

// ZvirtStaticNodeGroup defines a NodeGroup for creating static nodes.
type ZvirtStaticNodeGroup struct {
	Name          string                   `json:"name,omitempty" yaml:"name,omitempty"`
	Replicas      int                      `json:"replicas,omitempty" yaml:"replicas,omitempty"`
	NodeTemplate  *cpapi.NodeTemplate      `json:"nodeTemplate,omitempty" yaml:"nodeTemplate,omitempty"`
	InstanceClass ZvirtStaticInstanceClass `json:"instanceClass,omitempty" yaml:"instanceClass,omitempty"`
}

// ZvirtInstanceClass contains the common fields for zVirt VirtualMachine provisioning.
type ZvirtInstanceClass struct {
	NumCPUs             int                 `json:"numCPUs,omitempty" yaml:"numCPUs,omitempty"`
	Memory              int                 `json:"memory,omitempty" yaml:"memory,omitempty"`
	RootDiskSizeGb      *int                `json:"rootDiskSizeGb,omitempty" yaml:"rootDiskSizeGb,omitempty"`
	Template            string              `json:"template,omitempty" yaml:"template,omitempty"`
	VNICProfileID       string              `json:"vnicProfileID,omitempty" yaml:"vnicProfileID,omitempty"`
	StorageDomainID     string              `json:"storageDomainID,omitempty" yaml:"storageDomainID,omitempty"`
	CustomNetworkConfig *ZvirtNetworkConfig `json:"customNetworkConfig,omitempty" yaml:"customNetworkConfig,omitempty"`
}

// ZvirtMasterInstanceClass extends the base ZvirtInstanceClass with master-specific fields.
type ZvirtMasterInstanceClass struct {
	ZvirtInstanceClass `json:",inline" yaml:",inline"`

	EtcdDiskSizeGb *int `json:"etcdDiskSizeGb,omitempty" yaml:"etcdDiskSizeGb,omitempty"`
}

// ZvirtStaticInstanceClass extends the base ZvirtInstanceClass with node group-specific fields.
type ZvirtStaticInstanceClass struct {
	ZvirtInstanceClass `json:",inline" yaml:",inline"`
}

// ZvirtNetworkConfig describes a static network configuration of the node group interfaces.
type ZvirtNetworkConfig struct {
	NetworkInterfaceName    string   `json:"networkInterfaceName,omitempty" yaml:"networkInterfaceName,omitempty"`
	NetworkInterfaceAddress []string `json:"networkInterfaceAddress,omitempty" yaml:"networkInterfaceAddress,omitempty"`
	NetworkInterfaceNetmask string   `json:"networkInterfaceNetmask,omitempty" yaml:"networkInterfaceNetmask,omitempty"`
	NetworkInterfaceGateway string   `json:"networkInterfaceGateway,omitempty" yaml:"networkInterfaceGateway,omitempty"`
	DNSServers              string   `json:"dnsServers,omitempty" yaml:"dnsServers,omitempty"`
}

// HasMasterNodeGroup reports whether the masterNodeGroup section is set.
func (c *ZvirtProviderClusterConfiguration) HasMasterNodeGroup() bool {
	return c != nil && !reflect.DeepEqual(c.MasterNodeGroup, ZvirtMasterNodeGroup{})
}

// NodeGroupNames returns names of the additional node groups.
func (c *ZvirtProviderClusterConfiguration) NodeGroupNames() []string {
	if c == nil || len(c.NodeGroups) == 0 {
		return nil
	}

	names := make([]string, 0, len(c.NodeGroups))
	for _, nodeGroup := range c.NodeGroups {
		names = append(names, nodeGroup.Name)
	}

	return names
}
