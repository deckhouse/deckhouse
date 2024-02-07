/*
Copyright 2021 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package v1

type OpenstackModuleConfiguration struct {
	Connection                     *provider         `json:"connection,omitempty" yaml:"connection,omitempty"`
	InternalNetworkNames           []string          `json:"internalNetworkNames,omitempty" yaml:"internalNetworkNames,omitempty"`
	ExternalNetworkNames           []string          `json:"externalNetworkNames,omitempty" yaml:"externalNetworkNames,omitempty"`
	AdditionalExternalNetworkNames []string          `json:"additionalExternalNetworkNames,omitempty" yaml:"additionalExternalNetworkNames,omitempty"`
	PodNetworkMode                 string            `json:"podNetworkMode,omitempty" yaml:"podNetworkMode,omitempty"`
	Instances                      *instances        `json:"instances,omitempty" yaml:"instances,omitempty"`
	LoadBalancer                   *loadBalancer     `json:"loadBalancer,omitempty" yaml:"loadBalancer,omitempty"`
	Zones                          []string          `json:"zones,omitempty" yaml:"zones,omitempty"`
	Tags                           map[string]string `json:"tags,omitempty" yaml:"tags,omitempty"`
	StorageClass                   struct {
		Exclude         []string `json:"exclude,omitempty" yaml:"exclude,omitempty"`
		Default         string   `json:"default,omitempty" yaml:"default,omitempty"`
		TopologyEnabled bool     `json:"topologyEnabled,omitempty" yaml:"topologyEnabled,omitempty"`
	} `json:"storageClass,omitempty" yaml:"storageClass,omitempty"`
}

type instances struct {
	ImageName          string   `json:"imageName,omitempty" yaml:"imageName,omitempty"`
	MainNetwork        string   `json:"mainNetwork,omitempty" yaml:"mainNetwork,omitempty"`
	SSHKeyPairName     string   `json:"sshKeyPairName,omitempty" yaml:"sshKeyPairName,omitempty"`
	SecurityGroups     []string `json:"securityGroups,omitempty" yaml:"securityGroups,omitempty"`
	AdditionalNetworks []string `json:"additionalNetworks,omitempty" yaml:"additionalNetworks,omitempty"`
}

func (i instances) IsEmpty() bool {
	if i.ImageName != "" {
		return false
	}

	if i.MainNetwork != "" {
		return false
	}

	if i.SSHKeyPairName != "" {
		return false
	}

	if len(i.SecurityGroups) > 0 {
		return false
	}
	if len(i.AdditionalNetworks) > 0 {
		return false
	}
	return true
}

type loadBalancer struct {
	SubnetID          string `json:"subnetID,omitempty" yaml:"subnetID,omitempty"`
	FloatingNetworkID string `json:"floatingNetworkID,omitempty" yaml:"floatingNetworkID,omitempty"`
}

func (lb loadBalancer) IsEmpty() bool {
	if lb.SubnetID != "" {
		return false
	}

	if lb.FloatingNetworkID != "" {
		return false
	}

	return true
}
