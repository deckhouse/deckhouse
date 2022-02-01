/*
Copyright 2021 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package v1

type VsphereCloudDiscoveryData struct {
	ApiVersion       string `json:"apiVersion" yaml:"apiVersion"`
	Kind             string `json:"kind" yaml:"kind"`
	VmFolderPath     string `json:"vmFolderPath" yaml:"vmFolderPath"`
	ResourcePoolPath string `json:"resourcePoolPath,omitempty" yaml:"resourcePoolPath,omitempty"`
}
