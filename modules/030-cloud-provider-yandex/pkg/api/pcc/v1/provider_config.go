// Copyright 2026 Flant JSC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package v1

import (
	"reflect"

	cpapi "github.com/deckhouse/deckhouse/go_lib/cloud-provider/api"
)

var (
	_ cpapi.ProviderClusterConfigObject = (*YandexProviderClusterConfiguration)(nil)
)

// YandexProviderClusterConfiguration describes the configuration of a cloud cluster in Yandex Cloud.
type YandexProviderClusterConfiguration struct {
	APIVersion   string `json:"apiVersion" yaml:"apiVersion"`
	Kind         string `json:"kind" yaml:"kind"`
	Layout       string `json:"layout" yaml:"layout"`
	SSHPublicKey string `json:"sshPublicKey" yaml:"sshPublicKey"`

	// This subnet will be split into three equal parts.
	// They will serve as a basis for subnets in three Yandex Cloud zones.
	NodeNetworkCIDR string `json:"nodeNetworkCIDR" yaml:"nodeNetworkCIDR"`

	MasterNodeGroup           YandexMasterNodeGroup   `json:"masterNodeGroup" yaml:"masterNodeGroup"`
	NodeGroups                []YandexStaticNodeGroup `json:"nodeGroups,omitempty" yaml:"nodeGroups,omitempty"`
	Provider                  YandexProvider          `json:"provider" yaml:"provider"`
	WithNATInstance           *YandexWithNATInstance  `json:"withNATInstance,omitempty" yaml:"withNATInstance,omitempty"`
	ExistingNetworkID         *string                 `json:"existingNetworkID,omitempty" yaml:"existingNetworkID,omitempty"`
	ExistingZoneToSubnetIDMap map[string]string       `json:"existingZoneToSubnetIDMap,omitempty" yaml:"existingZoneToSubnetIDMap,omitempty"`
	Labels                    map[string]string       `json:"labels,omitempty" yaml:"labels,omitempty"`
	DHCPOptions               *YandexDHCPOptions      `json:"dhcpOptions,omitempty" yaml:"dhcpOptions,omitempty"`
	Zones                     []string                `json:"zones,omitempty" yaml:"zones,omitempty"`
}

// YandexMasterNodeGroup defines the master's NodeGroup.
type YandexMasterNodeGroup struct {
	// The number of master nodes to create. Must be an odd number for quorum.
	Replicas      int                       `json:"replicas" yaml:"replicas"`
	Zones         []string                  `json:"zones,omitempty" yaml:"zones,omitempty"`
	InstanceClass YandexMasterInstanceClass `json:"instanceClass" yaml:"instanceClass"`
}

// YandexStaticNodeGroup defines a NodeGroup for creating static nodes.
type YandexStaticNodeGroup struct {
	Name          string                    `json:"name" yaml:"name"`
	Replicas      int                       `json:"replicas" yaml:"replicas"`
	Zones         []string                  `json:"zones,omitempty" yaml:"zones,omitempty"`
	NodeTemplate  map[string]any            `json:"nodeTemplate,omitempty" yaml:"nodeTemplate,omitempty"`
	InstanceClass YandexStaticInstanceClass `json:"instanceClass" yaml:"instanceClass"`
}

// YandexInstanceClass contains the common fields for Yandex Compute Instance provisioning.
type YandexInstanceClass struct {
	Platform            *string           `json:"platform,omitempty" yaml:"platform,omitempty"`
	Cores               int               `json:"cores" yaml:"cores"`
	CoreFraction        *int              `json:"coreFraction,omitempty" yaml:"coreFraction,omitempty"`
	Memory              int               `json:"memory" yaml:"memory"`
	ImageID             string            `json:"imageID" yaml:"imageID"`
	DiskSizeGB          *int              `json:"diskSizeGB,omitempty" yaml:"diskSizeGB,omitempty"`
	DiskType            *string           `json:"diskType,omitempty" yaml:"diskType,omitempty"`
	ExternalIPAddresses []string          `json:"externalIPAddresses,omitempty" yaml:"externalIPAddresses,omitempty"`
	ExternalSubnetID    *string           `json:"externalSubnetID,omitempty" yaml:"externalSubnetID,omitempty"`
	ExternalSubnetIDs   []string          `json:"externalSubnetIDs,omitempty" yaml:"externalSubnetIDs,omitempty"`
	AdditionalLabels    map[string]string `json:"additionalLabels,omitempty" yaml:"additionalLabels,omitempty"`
	NetworkType         *string           `json:"networkType,omitempty" yaml:"networkType,omitempty"`
}

// YandexMasterInstanceClass extends the base YandexInstanceClass with master-specific fields.
type YandexMasterInstanceClass struct {
	YandexInstanceClass
	EtcdDiskSizeGB *int `json:"etcdDiskSizeGb,omitempty" yaml:"etcdDiskSizeGb,omitempty"`
}

// YandexStaticInstanceClass extends the base YandexInstanceClass with node group-specific fields.
type YandexStaticInstanceClass struct {
	YandexInstanceClass
}

// YandexProvider contains settings to connect to the Yandex Cloud API.
type YandexProvider struct {
	CloudID            string `json:"cloudID" yaml:"cloudID"`
	FolderID           string `json:"folderID" yaml:"folderID"`
	ServiceAccountJSON string `json:"serviceAccountJSON" yaml:"serviceAccountJSON"`
}

// YandexWithNATInstance contains settings for the WithNATInstance layout.
type YandexWithNATInstance struct {
	ExporterAPIKey             *string                     `json:"exporterAPIKey,omitempty" yaml:"exporterAPIKey,omitempty"`
	NATInstanceExternalAddress *string                     `json:"natInstanceExternalAddress,omitempty" yaml:"natInstanceExternalAddress,omitempty"`
	NATInstanceInternalAddress *string                     `json:"natInstanceInternalAddress,omitempty" yaml:"natInstanceInternalAddress,omitempty"`
	InternalSubnetID           *string                     `json:"internalSubnetID,omitempty" yaml:"internalSubnetID,omitempty"`
	InternalSubnetCIDR         *string                     `json:"internalSubnetCIDR,omitempty" yaml:"internalSubnetCIDR,omitempty"`
	ExternalSubnetID           *string                     `json:"externalSubnetID,omitempty" yaml:"externalSubnetID,omitempty"`
	NATInstanceResources       *YandexNATInstanceResources `json:"natInstanceResources,omitempty" yaml:"natInstanceResources,omitempty"`
}

// YandexNATInstanceResources defines computing resources allocated to the NAT instance.
type YandexNATInstanceResources struct {
	Cores    *int    `json:"cores,omitempty" yaml:"cores,omitempty"`
	Memory   *int    `json:"memory,omitempty" yaml:"memory,omitempty"`
	Platform *string `json:"platform,omitempty" yaml:"platform,omitempty"`
}

// YandexDHCPOptions defines DHCP parameters for all subnets.
type YandexDHCPOptions struct {
	DomainName        *string  `json:"domainName,omitempty" yaml:"domainName,omitempty"`
	DomainNameServers []string `json:"domainNameServers,omitempty" yaml:"domainNameServers,omitempty"`
}

// HasMasterNodeGroup reports whether the masterNodeGroup section is set.
func (c *YandexProviderClusterConfiguration) HasMasterNodeGroup() bool {
	return c != nil && !reflect.DeepEqual(c.MasterNodeGroup, YandexMasterNodeGroup{})
}

// NodeGroupNames returns names of the additional node groups.
func (c *YandexProviderClusterConfiguration) NodeGroupNames() []string {
	if c == nil || len(c.NodeGroups) == 0 {
		return nil
	}

	names := make([]string, 0, len(c.NodeGroups))
	for _, nodeGroup := range c.NodeGroups {
		names = append(names, nodeGroup.Name)
	}

	return names
}
