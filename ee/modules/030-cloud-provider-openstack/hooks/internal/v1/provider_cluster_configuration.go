/*
Copyright 2021 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package v1

import v1 "k8s.io/api/core/v1"

type OpenstackProviderClusterConfiguration struct {
	APIVersion string `json:"apiVersion,omitempty" yaml:"apiVersion,omitempty"`
	Kind       string `json:"kind,omitempty" yaml:"kind,omitempty"`

	SSHPublicKey string            `json:"sshPublicKey,omitempty" yaml:"sshPublicKey,omitempty"`
	Tags         map[string]string `json:"tags,omitempty" yaml:"tags,omitempty"`
	Zones        []string          `json:"zones,omitempty" yaml:"zones,omitempty"`

	MasterNodeGroup struct {
		Replicas      int32                  `json:"replicas,omitempty" yaml:"replicas,omitempty"`
		VolumeTypeMap map[string]string      `json:"volumeTypeMap,omitempty" yaml:"volumeTypeMap,omitempty"`
		InstanceClass OpenstackInstanceClass `json:"instanceClass,omitempty" yaml:"instanceClass,omitempty"`
	} `json:"masterNodeGroup,omitempty" yaml:"masterNodeGroup,omitempty"`
	Provider provider `json:"provider,omitempty" yaml:"provider,omitempty"`

	Layout   string `json:"layout,omitempty" yaml:"layout,omitempty"`
	Standard struct {
		Bastion struct {
			Zone          string                 `json:"zone,omitempty" yaml:"zone,omitempty"`
			VolumeType    string                 `json:"volumeType,omitempty" yaml:"volumeType,omitempty"`
			InstanceClass OpenstackInstanceClass `json:"instanceClass,omitempty" yaml:"instanceClass,omitempty"`
		} `json:"bastion,omitempty" yaml:"bastion,omitempty"`
		ExternalNetworkName       string   `json:"externalNetworkName,omitempty" yaml:"externalNetworkName,omitempty"`
		InternalNetworkCIDR       string   `json:"internalNetworkCIDR,omitempty" yaml:"internalNetworkCIDR,omitempty"`
		InternalNetworkDNSServers []string `json:"internalNetworkDNSServers,omitempty" yaml:"internalNetworkDNSServers,omitempty"`
		InternalNetworkSecurity   bool     `json:"internalNetworkSecurity,omitempty" yaml:"internalNetworkSecurity,omitempty"`
	} `json:"standard,omitempty" yaml:"standard,omitempty"`
	StandardWithNoRouter struct {
		InternalNetworkCIDR     string `json:"internalNetworkCIDR,omitempty" yaml:"internalNetworkCIDR,omitempty"`
		InternalNetworkSecurity bool   `json:"internalNetworkSecurity,omitempty" yaml:"internalNetworkSecurity,omitempty"`
		ExternalNetworkName     string `json:"externalNetworkName,omitempty" yaml:"externalNetworkName,omitempty"`
		ExternalNetworkDHCP     *bool  `json:"externalNetworkDHCP,omitempty" yaml:"externalNetworkDHCP,omitempty"`
	} `json:"standardWithNoRouter,omitempty" yaml:"standardWithNoRouter,omitempty"`
	Simple struct {
		ExternalNetworkName string `json:"externalNetworkName,omitempty" yaml:"externalNetworkName,omitempty"`
		ExternalNetworkDHCP *bool  `json:"externalNetworkDHCP,omitempty" yaml:"externalNetworkDHCP,omitempty"`
		PodNetworkMode      string `json:"podNetworkMode,omitempty" yaml:"podNetworkMode,omitempty"`
	} `json:"simple,omitempty" yaml:"simple,omitempty"`
	SimpleWithInternalNetwork struct {
		InternalSubnetName           string `json:"internalSubnetName,omitempty" yaml:"internalSubnetName,omitempty"`
		PodNetworkMode               string `json:"podNetworkMode,omitempty" yaml:"podNetworkMode,omitempty"`
		ExternalNetworkName          string `json:"externalNetworkName,omitempty" yaml:"externalNetworkName,omitempty"`
		ExternalNetworkDHCP          *bool  `json:"externalNetworkDHCP,omitempty" yaml:"externalNetworkDHCP,omitempty"`
		MasterWithExternalFloatingIP bool   `json:"masterWithExternalFloatingIP,omitempty" yaml:"masterWithExternalFloatingIP,omitempty"`
	} `json:"simpleWithInternalNetwork,omitempty" yaml:"simpleWithInternalNetwork,omitempty"`

	NodeGroups []struct {
		Name         string `json:"name,omitempty" yaml:"name,omitempty"`
		Replicas     int16  `json:"replicas,omitempty" yaml:"replicas,omitempty"`
		NodeTemplate struct {
			Labels      map[string]string `json:"labels,omitempty" yaml:"labels,omitempty"`
			Annotations map[string]string `json:"annotations,omitempty" yaml:"annotations,omitempty"`
			Taints      []v1.Taint        `json:"taints,omitempty" yaml:"taints,omitempty"`
		} `json:"nodeTemplate,omitempty" yaml:"nodeTemplate,omitempty"`
		InstanceClass struct {
			OpenstackInstanceClass
			ConfigDrive                  bool     `json:"configDrive,omitempty" yaml:"configDrive,omitempty"`
			NetworksWithSecurityDisabled []string `json:"networksWithSecurityDisabled,omitempty" yaml:"networksWithSecurityDisabled,omitempty"`
			FloatingIPPools              []string `json:"floatingIPPools,omitempty" yaml:"floatingIPPools,omitempty"`
		} `json:"instanceClass,omitempty" yaml:"instanceClass,omitempty"`
	} `json:"nodeGroups,omitempty" yaml:"nodeGroups,omitempty"`
}

func (cc *OpenstackProviderClusterConfiguration) PatchWithModuleConfig(module OpenstackModuleConfiguration) {
	if module.Connection != nil && !module.Connection.IsEmpty() {
		cc.Provider = *module.Connection
	}

	if len(module.Tags) > 0 {
		cc.Tags = module.Tags
	}

	if len(cc.Tags) == 0 {
		cc.Tags = make(map[string]string)
	}
}

type provider struct {
	AuthURL    string `json:"authURL,omitempty" yaml:"authURL,omitempty"`
	CACert     string `json:"caCert,omitempty" yaml:"caCert,omitempty"`
	DomainName string `json:"domainName,omitempty" yaml:"domainName,omitempty"`
	TenantName string `json:"tenantName,omitempty" yaml:"tenantName,omitempty"`
	TenantID   string `json:"tenantID,omitempty" yaml:"tenantID,omitempty"`
	Username   string `json:"username,omitempty" yaml:"username,omitempty"`
	Password   string `json:"password,omitempty" yaml:"password,omitempty"`
	Region     string `json:"region,omitempty" yaml:"region,omitempty"`
}

func (p provider) IsEmpty() bool {
	if p.AuthURL != "" {
		return false
	}
	if p.CACert != "" {
		return false
	}
	if p.DomainName != "" {
		return false
	}
	if p.TenantName != "" {
		return false
	}
	if p.TenantID != "" {
		return false
	}
	if p.Username != "" {
		return false
	}
	if p.Password != "" {
		return false
	}
	if p.Region != "" {
		return false
	}

	return true
}
