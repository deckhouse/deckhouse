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

// Package settings contains the ModuleConfig root type for the cloud-provider-yandex module.
package v2

import (
	"reflect"

	cpapi "github.com/deckhouse/deckhouse/go_lib/cloud-provider/api"
)

var (
	_ cpapi.ModuleSettingsObject = (*ModuleConfigSettings)(nil)
)

// Describes the configuration of the cloud-provider-yandex module.
//
// Run the following command to change the configuration in a running cluster:
//
// ```shell
// d8 k edit moduleconfig cloud-provider-yandex
// ```
type ModuleConfigSettings struct {
	Provider Provider `json:"provider"`
	Nodes    Nodes    `json:"nodes"`
	Storage  Storage  `json:"storage,omitempty"`
	CCM      CCM      `json:"ccm,omitempty"`
}

type Provider struct {
	Parameters ProviderParameters `json:"parameters"`
}

type Nodes struct {
	Disabled   bool            `json:"disabled,omitempty"`
	Parameters NodesParameters `json:"parameters"`
}

type Storage struct {
	Disabled   bool              `json:"disabled,omitempty"`
	Parameters StorageParameters `json:"parameters"`
}

type CCM struct {
	Disabled   bool          `json:"disabled,omitempty"`
	Parameters CCMParameters `json:"parameters"`
}

// Contains settings to connect to the Yandex Cloud API.
type ProviderParameters struct {
	// The cloud ID.
	CloudID string `json:"cloudID"`
	// ID of the directory.
	FolderID string `json:"folderID"`
}

type NodesParameters struct {
	// A public key for accessing nodes.
	SSHPublicKey string `json:"sshPublicKey"`
	// The way resources are located in the cloud.
	//
	// [Read more](https://deckhouse.io/modules/cloud-provider-yandex/layouts.html) about possible provider layouts.
	Layout string `json:"layout"`
	// This subnet will be split into **three** equal parts.
	//
	// They will serve as a basis for subnets in three Yandex Cloud zones.
	NodeNetworkCIDR string `json:"nodeNetworkCIDR"`
	// Settings for the [`WithNATInstance`](https://deckhouse.io/modules/cloud-provider-yandex/layouts.html#withnatinstance) layout.
	WithNATInstance NATInstanceParameters `json:"withNATInstance,omitempty"`
	// The ID of the existing VPC Network.
	ExistingNetworkID string `json:"existingNetworkID,omitempty"`
	// One or more pre-existing subnets mapped to respective zone.
	//
	// **Warning.** When using `cni-simple-bridge`, DKP creates a route table that must be manually associated with the specified subnets. Only one route table can be associated with a subnet, so multiple clusters using `cni-simple-bridge` cannot be deployed in the same subnets. Starting with DKP 1.76, new clusters in Yandex Cloud use `cni-cilium` in `VXLAN` mode by default. In this mode, pod traffic routing does not depend on Yandex Cloud route tables.
	ExistingZoneToSubnetIDMap map[string]string `json:"existingZoneToSubnetIDMap,omitempty"`
	// A list of DHCP parameters to use for all subnets.
	//
	// Note that setting dhcpOptions may lead to [problems](https://deckhouse.io/modules/cloud-provider-yandex/faq.html#dhcpoptions-related-problems-and-ways-to-address-them).
	DHCPOptions DHCPOptions `json:"dhcpOptions,omitempty"`
	// External IP addresses per node group name.
	//
	// The key is the node group name (e.g., `system`, `master`, `worker`), and the value is the list of external IP addresses for nodes in that group, listed in the order of the zones where nodes will be created.
	//
	// The following values can be specified in the list:
	// - IP address from an additional external network for the corresponding zone (parameter `externalSubnetIDs`);
	// - [reserved public IP address](faq.html#how-to-reserve-a-public-ip-address), if the list of additional external networks is not defined (parameter `externalSubnetIDs`);
	// - `Auto`, to order a public IP address in the corresponding zone.
	//
	ExternalIPAddresses map[string][]string `json:"externalIPAddresses,omitempty"`
	// IDs of additional external networks per node group name.
	//
	// The key is the node group name (e.g., `system`, `master`, `worker`), and the value is the list of external subnet IDs for nodes in that group, listed in the order of the zones where nodes will be created.
	ExternalSubnetIDs map[string][]string `json:"externalSubnetIDs,omitempty"`
	// Labels to attach to resources created in the Yandex Cloud.
	//
	// Note that you have to re-create all the machines to add new labels if labels were modified in the running cluster.
	Labels map[string]string `json:"labels,omitempty"`
	// The globally restricted set of zones that this cloud provider works with.
	Zones []string `json:"zones,omitempty"`
}

// DHCPOptions defines DHCP parameters to use for all subnets.
type DHCPOptions struct {
	// The name of the search domain.
	DomainName string `json:"domainName,omitempty"`
	// A list of recursive DNS addresses.
	DomainNameServers []string `json:"domainNameServers,omitempty"`
}

type NATInstanceParameters struct {
	// If specified, an additional network interface will be added to the node (the latter will use it as a default route).
	ExternalSubnetID string `json:"externalSubnetID,omitempty"`
	// ID of a subnet for the internal interface.
	InternalSubnetID string `json:"internalSubnetID,omitempty"`
	// CIDR of an automatically created subnet for the internal interface. Overrides `internalSubnetID` parameter.
	InternalSubnetCIDR string `json:"internalSubnetCIDR,omitempty"`
	// A [reserved external IP address](https://deckhouse.io/modules/cloud-provider-yandex/faq.html#how-to-reserve-a-public-ip-address) (or `externalSubnetID` address if specified).
	NATInstanceExternalAddress string `json:"natInstanceExternalAddress,omitempty"`
	// Consider using automatically generated address instead.
	NATInstanceInternalAddress string `json:"natInstanceInternalAddress,omitempty"`
	// Computing resources that are allocated to the NAT instance. If not specified, the default values will be used.
	//
	// > **Warning.** If these parameters are changed, `terraform-auto-converger` will automatically restart NAT-instance if [autoConvergerEnabled](../terraform-manager/configuration.html#parameters-autoconvergerenabled) is set to `true`. This may result in a temporary interruption of network traffic in the cluster.
	NATInstanceResources NATInstanceResources `json:"natInstanceResources,omitempty"`
}

type NATInstanceResources struct {
	// Amount of CPU cores to provision on the NAT instance.
	Cores int `json:"cores,omitempty"`
	// Amount of primary memory in MB provision on the NAT instance.
	Memory int `json:"memory,omitempty"`
	// Processor platform type on the NAT instance.
	Platform string `json:"platform,omitempty"`
}

type StorageParameters struct {
	// List of storage classes to exclude from use in the cluster.
	ExcludedStorageClasses []string `json:"excludedStorageClasses,omitempty"`
}

type CCMParameters struct {
	// Additional external network IDs to be recognized as external networks by the CCM.
	AdditionalExternalNetworkIDs []string `json:"additionalExternalNetworkIDs,omitempty"`
}

// HasProviderSection reports whether the provider settings section is set.
func (s *ModuleConfigSettings) HasProviderSection() bool {
	return s != nil && !reflect.DeepEqual(s.Provider, Provider{})
}

// HasNodesSection reports whether the nodes settings section is set.
func (s *ModuleConfigSettings) HasNodesSection() bool {
	return s != nil && !reflect.DeepEqual(s.Nodes, Nodes{})
}

// HasStorageSection reports whether the storage settings section is set.
func (s *ModuleConfigSettings) HasStorageSection() bool {
	return s != nil && !reflect.DeepEqual(s.Storage, Storage{})
}

// HasCCMSection reports whether the ccm settings section is set.
func (s *ModuleConfigSettings) HasCCMSection() bool {
	return s != nil && !reflect.DeepEqual(s.CCM, CCM{})
}
