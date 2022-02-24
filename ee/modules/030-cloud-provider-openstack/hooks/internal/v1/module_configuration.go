/*
Copyright 2021 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package v1

type OpenstackModuleConfiguration struct {
	Connection struct {
		AuthURL    string `json:"authURL,omitempty" yaml:"authURL,omitempty"`
		CACert     string `json:"caCert,omitempty" yaml:"caCert,omitempty"`
		DomainName string `json:"domainName,omitempty" yaml:"domainName,omitempty"`
		TenantName string `json:"tenantName,omitempty" yaml:"tenantName,omitempty"`
		TenantID   string `json:"tenantID,omitempty" yaml:"tenantID,omitempty"`
		Username   string `json:"username,omitempty" yaml:"username,omitempty"`
		Password   string `json:"password,omitempty" yaml:"password,omitempty"`
		Region     string `json:"region,omitempty" yaml:"region,omitempty"`
	} `json:"connection,omitempty" yaml:"connection,omitempty"`
	InternalNetworkNames           []string `json:"internalNetworkNames,omitempty" yaml:"internalNetworkNames,omitempty"`
	ExternalNetworkNames           []string `json:"externalNetworkNames,omitempty" yaml:"externalNetworkNames,omitempty"`
	AdditionalExternalNetworkNames []string `json:"additionalExternalNetworkNames,omitempty" yaml:"additionalExternalNetworkNames,omitempty"`
	PodNetworkMode                 string   `json:"podNetworkMode,omitempty" yaml:"podNetworkMode,omitempty"`
	Instances                      struct {
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
	Zones        []string          `json:"zones,omitempty" yaml:"zones,omitempty"`
	Tags         map[string]string `json:"tags,omitempty" yaml:"tags,omitempty"`
	StorageClass struct {
		Exclude         []string `json:"exclude,omitempty" yaml:"exclude,omitempty"`
		Default         string   `json:"default,omitempty" yaml:"default,omitempty"`
		TopologyEnabled bool     `json:"topologyEnabled,omitempty" yaml:"topologyEnabled,omitempty"`
	} `json:"storageClass,omitempty" yaml:"storageClass,omitempty"`
}
