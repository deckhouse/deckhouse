/*
Copyright 2025 Flant JSC

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package v1

type DvpProviderClusterConfiguration struct {
	APIVersion      *string      `json:"apiVersion,omitempty" yaml:"apiVersion,omitempty"`
	Kind            *string      `json:"kind,omitempty" yaml:"kind,omitempty"`
	Provider        *DvpProvider `json:"provider,omitempty" yaml:"provider,omitempty"`
	Layout          *string      `json:"layout,omitempty" yaml:"layout,omitempty"`
	MasterNodeGroup any          `json:"masterNodeGroup,omitempty" yaml:"masterNodeGroup,omitempty"`
	NodeGroups      []any        `json:"nodeGroups,omitempty" yaml:"nodeGroups,omitempty"`
	SSHPublicKey    *string      `json:"sshPublicKey,omitempty" yaml:"sshPublicKey,omitempty"`
	Region          *string      `json:"region,omitempty" yaml:"region,omitempty"`
	Zones           *[]string    `json:"zones,omitempty" yaml:"zones,omitempty"`
}

type DvpProvider struct {
	KubeconfigDataBase64 *string `json:"kubeconfigDataBase64,omitempty" yaml:"kubeconfigDataBase64,omitempty"`
	Namespace            *string `json:"namespace,omitempty" yaml:"namespace,omitempty"`
}

type DvpModuleConfiguration struct {
	Provider *DvpProvider `json:"provider,omitempty" yaml:"provider,omitempty"`
	Zones    *[]string    `json:"zones,omitempty" yaml:"zones,omitempty"`

	// v2 fields: provider.parameters, nodes.parameters, storage.parameters
	ProviderV2 *DvpProviderV2 `json:"providerV2,omitempty" yaml:"providerV2,omitempty"`
	Nodes      *DvpNodesV2    `json:"nodes,omitempty" yaml:"nodes,omitempty"`
	Storage    *DvpStorageV2  `json:"storage,omitempty" yaml:"storage,omitempty"`
}

// DvpProviderV2 represents the v2 schema provider section (from ModuleConfig).
type DvpProviderV2 struct {
	Parameters *DvpProviderParameters `json:"parameters,omitempty" yaml:"parameters,omitempty"`
}

type DvpProviderParameters struct {
	Namespace     *string `json:"namespace,omitempty" yaml:"namespace,omitempty"`
	NetworkPolicy *string `json:"networkPolicy,omitempty" yaml:"networkPolicy,omitempty"`
}

// DvpNodesV2 represents the v2 schema nodes section (from ModuleConfig).
type DvpNodesV2 struct {
	Enabled    *bool               `json:"enabled,omitempty" yaml:"enabled,omitempty"`
	Parameters *DvpNodesParameters `json:"parameters,omitempty" yaml:"parameters,omitempty"`
}

type DvpNodesParameters struct {
	Layout       *string            `json:"layout,omitempty" yaml:"layout,omitempty"`
	SSHPublicKey *string            `json:"sshPublicKey,omitempty" yaml:"sshPublicKey,omitempty"`
	Region       *string            `json:"region,omitempty" yaml:"region,omitempty"`
	Zones        *[]string          `json:"zones,omitempty" yaml:"zones,omitempty"`
	IPAddresses  map[string][]string `json:"ipAddresses,omitempty"`
}

// DvpStorageV2 represents the v2 schema storage section (from ModuleConfig).
type DvpStorageV2 struct {
	Enabled    *bool                 `json:"enabled,omitempty" yaml:"enabled,omitempty"`
	Parameters *DvpStorageParameters `json:"parameters,omitempty" yaml:"parameters,omitempty"`
}

type DvpStorageParameters struct {
	ExcludedStorageClasses []string `json:"excludedStorageClasses,omitempty" yaml:"excludedStorageClasses,omitempty"`
}
