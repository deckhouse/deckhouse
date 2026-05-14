/*
Copyright 2026 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package v1

type VCDProviderClusterConfiguration struct {
	APIVersion                          *string           `json:"apiVersion,omitempty" yaml:"apiVersion,omitempty"`
	Kind                                *string           `json:"kind,omitempty" yaml:"kind,omitempty"`
	Layout                              *string           `json:"layout,omitempty" yaml:"layout,omitempty"`
	Provider                            *VCDProvider      `json:"provider,omitempty" yaml:"provider,omitempty"`
	Organization                        *string           `json:"organization,omitempty" yaml:"organization,omitempty"`
	VirtualDataCenter                   *string           `json:"virtualDataCenter,omitempty" yaml:"virtualDataCenter,omitempty"`
	VirtualApplicationName              *string           `json:"virtualApplicationName,omitempty" yaml:"virtualApplicationName,omitempty"`
	Bastion                             any               `json:"bastion,omitempty" yaml:"bastion,omitempty"`
	MasterNodeGroup                     any               `json:"masterNodeGroup,omitempty" yaml:"masterNodeGroup,omitempty"`
	NodeGroups                          []any             `json:"nodeGroups,omitempty" yaml:"nodeGroups,omitempty"`
	EdgeGateway                         any               `json:"edgeGateway,omitempty" yaml:"edgeGateway,omitempty"`
	CreateDefaultFirewallRules          *bool             `json:"createDefaultFirewallRules,omitempty" yaml:"createDefaultFirewallRules,omitempty"`
	MainNetwork                         *string           `json:"mainNetwork,omitempty" yaml:"mainNetwork,omitempty"`
	InternalNetworkCIDR                 *string           `json:"internalNetworkCIDR,omitempty" yaml:"internalNetworkCIDR,omitempty"`
	InternalNetworkDNSServers           []string          `json:"internalNetworkDNSServers,omitempty" yaml:"internalNetworkDNSServers,omitempty"`
	InternalNetworkDHCPPoolStartAddress *int              `json:"internalNetworkDHCPPoolStartAddress,omitempty" yaml:"internalNetworkDHCPPoolStartAddress,omitempty"`
	SSHPublicKey                        *string           `json:"sshPublicKey,omitempty" yaml:"sshPublicKey,omitempty"`
	LegacyMode                          *bool             `json:"legacyMode,omitempty" yaml:"legacyMode,omitempty"`
	Metadata                            map[string]string `json:"metadata,omitempty" yaml:"metadata,omitempty"`
}

type VCDProvider struct {
	Server   *string `json:"server,omitempty" yaml:"server,omitempty"`
	Username *string `json:"username,omitempty" yaml:"username,omitempty"`
	Password *string `json:"password,omitempty" yaml:"password,omitempty"`
	APIToken *string `json:"apiToken,omitempty" yaml:"apiToken,omitempty"`
	Insecure *bool   `json:"insecure,omitempty" yaml:"insecure,omitempty"`
}

func (p VCDProvider) IsEmpty() bool {
	if p.Server != nil {
		return false
	}
	if p.Username != nil {
		return false
	}
	if p.Password != nil {
		return false
	}
	if p.APIToken != nil {
		return false
	}
	if p.Insecure != nil {
		return false
	}

	return true
}
