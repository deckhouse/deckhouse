/*
Copyright 2021 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package v1

type VsphereCloudDiscoveryData struct {
	APIVersion       *string  `json:"apiVersion,omitempty" yaml:"apiVersion,omitempty"`
	Kind             *string  `json:"kind,omitempty" yaml:"kind,omitempty"`
	VMFolderPath     *string  `json:"vmFolderPath,omitempty" yaml:"vmFolderPath,omitempty"`
	ResourcePoolPath *string  `json:"resourcePoolPath,omitempty" yaml:"resourcePoolPath,omitempty"`
	Zones            []string `json:"zones,omitempty" yaml:"zones,omitempty"`
}
