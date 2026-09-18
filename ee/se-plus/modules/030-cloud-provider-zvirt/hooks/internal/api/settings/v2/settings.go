/*
Copyright 2026 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

// Package v2 mirrors the cloud-provider-zvirt ModuleConfig settings for the module hooks.
//
// The canonical, marker-annotated definition lives in pkg/api/settings/v2 and drives
// openapi/config-values.yaml. Hooks compile inside the repository-root go.mod, which does not
// require the module's own go.mod, so the shape is repeated here without the generator markers.
// Keep the two in sync: a field added there and missed here is silently dropped from the
// projection of the legacy ZvirtClusterConfiguration.
package v2

import (
	"reflect"

	cpapi "github.com/deckhouse/deckhouse/go_lib/cloud-provider/api"
)

var (
	_ cpapi.ModuleSettingsObject = (*ModuleConfigSettings)(nil)
)

// ModuleConfigSettings describes the configuration of the cloud-provider-zvirt module.
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
	Disabled bool `json:"disabled,omitempty"`
}

// ProviderParameters contains settings to connect to the zVirt API. The login ID and the
// password are deliberately absent: they live in the managed credential Secret.
type ProviderParameters struct {
	Server    string `json:"server"`
	ClusterID string `json:"clusterID"`
	CABundle  string `json:"caBundle,omitempty"`
	Insecure  bool   `json:"insecure,omitempty"`
}

type NodesParameters struct {
	SSHPublicKey         string                         `json:"sshPublicKey"`
	Layout               string                         `json:"layout"`
	CustomNetworkConfigs map[string]CustomNetworkConfig `json:"customNetworkConfigs,omitempty"`
}

// CustomNetworkConfig describes a static network configuration of the node group interfaces.
type CustomNetworkConfig struct {
	NetworkInterfaceName      string   `json:"networkInterfaceName"`
	NetworkInterfaceAddresses []string `json:"networkInterfaceAddresses"`
	NetworkInterfaceNetmask   string   `json:"networkInterfaceNetmask"`
	NetworkInterfaceGateway   string   `json:"networkInterfaceGateway"`
	DNSServers                []string `json:"dnsServers,omitempty"`
}

type StorageParameters struct {
	ExcludedStorageClasses []string `json:"excludedStorageClasses,omitempty"`
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
