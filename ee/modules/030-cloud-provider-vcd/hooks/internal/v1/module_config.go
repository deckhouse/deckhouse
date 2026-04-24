/*
Copyright 2026 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package v1

type VCDModuleConfig struct {
	Provider               *VCDProvider      `json:"provider,omitempty" yaml:"provider,omitempty"`
	Organization           string            `json:"organization,omitempty" yaml:"organization,omitempty"`
	VirtualDataCenter      string            `json:"virtualDataCenter,omitempty" yaml:"virtualDataCenter,omitempty"`
	VirtualApplicationName string            `json:"virtualApplicationName,omitempty" yaml:"virtualApplicationName,omitempty"`
	MainNetwork            string            `json:"mainNetwork,omitempty" yaml:"mainNetwork,omitempty"`
	SSHPublicKey           string            `json:"sshPublicKey,omitempty" yaml:"sshPublicKey,omitempty"`
	Metadata               map[string]string `json:"metadata,omitempty" yaml:"metadata,omitempty"`
}
