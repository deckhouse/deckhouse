/*
Copyright 2021 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package v1

type OvirtProviderClusterConfiguration struct {
	APIVersion string `json:"apiVersion,omitempty" yaml:"apiVersion,omitempty"`
	Kind       string `json:"kind,omitempty" yaml:"kind,omitempty"`

	SSHPublicKey string `json:"sshPublicKey,omitempty" yaml:"sshPublicKey,omitempty"`

	Provider provider `json:"provider,omitempty" yaml:"provider,omitempty"`
}

func (cc *OvirtProviderClusterConfiguration) PatchWithModuleConfig(module OvirtModuleConfiguration) {
	if module.Connection != nil && !module.Connection.IsEmpty() {
		cc.Provider = *module.Connection
	}
}

type provider struct {
	AuthURL     string `json:"authURL,omitempty" yaml:"authURL,omitempty"`
	CABundle    string `json:"caBundle,omitempty" yaml:"caBundle,omitempty"`
	Username    string `json:"username,omitempty" yaml:"username,omitempty"`
	Password    string `json:"password,omitempty" yaml:"password,omitempty"`
	TLSInsecure bool   `json:"tlsInsecure,omitempty" yaml:"tlsInsecure,omitempty"`
}

func (p provider) IsEmpty() bool {
	if p.AuthURL != "" {
		return false
	}
	if p.CABundle != "" {
		return false
	}
	if p.Username != "" {
		return false
	}
	if p.Password != "" {
		return false
	}
	return true
}
