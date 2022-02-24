/*
Copyright 2021 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package v1

type OpenstackCloudDiscoveryData struct {
	APIVersion string `json:"apiVersion,omitempty" yaml:"apiVersion,omitempty"`
	Kind       string `json:"kind,omitempty" yaml:"kind,omitempty"`

	Layout               string   `json:"layout,omitempty" yaml:"layout,omitempty"`
	InternalNetworkNames []string `json:"internalNetworkNames,omitempty" yaml:"internalNetworkNames,omitempty"`
	ExternalNetworkNames []string `json:"externalNetworkNames,omitempty" yaml:"externalNetworkNames,omitempty"`
	PodNetworkMode       string   `json:"podNetworkMode,omitempty" yaml:"podNetworkMode,omitempty"`
	Zones                []string `json:"zones,omitempty" yaml:"zones,omitempty"`
	Instances            struct {
		ImageName          string   `json:"imageName,omitempty" yaml:"imageName,omitempty"`
		MainNetwork        string   `json:"mainNetwork,omitempty" yaml:"mainNetwork,omitempty"`
		SSHKeyPairName     string   `json:"sshKeyPairName,omitempty" yaml:"sshKeyPairName,omitempty"`
		SecurityGroups     []string `json:"securityGroups,omitempty" yaml:"securityGroups,omitempty"`
		AdditionalNetworks []string `json:"additionalNetworks,omitempty" yaml:"additionalNetworks,omitempty"`
	} `json:"instances,omitempty" yaml:"instances,omitempty"`
	LoadBalancer struct {
		SubnetID          string `json:"subnetID,omitempty" yaml:"subnetID,omitempty"`
		FloatingNetworkID string `json:"floatingNetworkID,omitempty" yaml:"floatingNetworkID,omitempty"`
	} `json:"loadBalancer,omitempty" yaml:"loadBalancer,omitempty"`
}
