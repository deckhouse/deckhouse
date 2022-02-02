/*
Copyright 2021 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package v1

type VsphereCloudDiscoveryData struct {
	APIVersion       *string `json:"apiVersion" yaml:"apiVersion"`
	Kind             *string `json:"kind" yaml:"kind"`
	VMFolderPath     *string `json:"vmFolderPath" yaml:"vmFolderPath"`
	ResourcePoolPath *string `json:"resourcePoolPath,omitempty" yaml:"resourcePoolPath,omitempty"`
}
