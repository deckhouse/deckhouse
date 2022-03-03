/*
Copyright 2021 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package v1

// Parameters of a group of vSphere VirtualMachines used by `machine-controller-manager`
type OpenstackInstanceClass struct {
	MainNetwork              string            `json:"mainNetwork,omitempty" yaml:"mainNetwork,omitempty"`
	AdditionalNetworks       []string          `json:"additionalNetworks,omitempty" yaml:"additionalNetworks,omitempty"`
	AdditionalSecurityGroups []string          `json:"additionalSecurityGroups,omitempty" yaml:"additionalSecurityGroups,omitempty"`
	AdditionalTags           map[string]string `json:"additionalTags,omitempty" yaml:"additionalTags,omitempty"`
	FlavorName               string            `json:"flavorName,omitempty" yaml:"flavorName,omitempty"`
	ImageName                string            `json:"imageName,omitempty" yaml:"imageName,omitempty"`
	RootDiskSize             int32             `json:"rootDiskSize,omitempty" yaml:"rootDiskSize,omitempty"`
}
