/*
Copyright 2021 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package v1

import (
	"sort"
)

type OpenstackCloudDiscoveryData struct {
	APIVersion string `json:"apiVersion,omitempty" yaml:"apiVersion,omitempty"`
	Kind       string `json:"kind,omitempty" yaml:"kind,omitempty"`

	Layout               string       `json:"layout,omitempty" yaml:"layout,omitempty"`
	InternalNetworkNames []string     `json:"internalNetworkNames,omitempty" yaml:"internalNetworkNames,omitempty"`
	ExternalNetworkNames []string     `json:"externalNetworkNames,omitempty" yaml:"externalNetworkNames,omitempty"`
	PodNetworkMode       string       `json:"podNetworkMode,omitempty" yaml:"podNetworkMode,omitempty"`
	Zones                []string     `json:"zones,omitempty" yaml:"zones,omitempty"`
	Instances            instances    `json:"instances,omitempty" yaml:"instances,omitempty"`
	LoadBalancer         loadBalancer `json:"loadBalancer,omitempty" yaml:"loadBalancer,omitempty"`
}

func (dd *OpenstackCloudDiscoveryData) PathWithDiscoveryData(module OpenstackModuleConfiguration) {
	if len(module.InternalNetworkNames) > 0 {
		dd.InternalNetworkNames = module.InternalNetworkNames
	}
	dd.InternalNetworkNames = unique(dd.InternalNetworkNames)

	if len(module.ExternalNetworkNames) > 0 {
		dd.ExternalNetworkNames = module.ExternalNetworkNames
	}

	if len(module.AdditionalExternalNetworkNames) > 0 {
		dd.ExternalNetworkNames = append(dd.ExternalNetworkNames, module.AdditionalExternalNetworkNames...)
	}

	dd.ExternalNetworkNames = unique(dd.ExternalNetworkNames)

	if len(module.Zones) > 0 {
		dd.Zones = module.Zones
	}
	dd.Zones = unique(dd.Zones)

	if module.Instances != nil && !module.Instances.IsEmpty() {
		dd.Instances = *module.Instances
	}

	if module.PodNetworkMode != "" {
		dd.PodNetworkMode = module.PodNetworkMode
	}

	if module.LoadBalancer != nil && !module.LoadBalancer.IsEmpty() {
		dd.LoadBalancer = *module.LoadBalancer
	}
}

func unique(array []string) []string {
	tmp := make(map[string]struct{})

	for _, v := range array {
		tmp[v] = struct{}{}
	}

	result := make([]string, 0, len(tmp))
	for k := range tmp {
		result = append(result, k)
	}

	sort.Strings(result)
	return result
}
