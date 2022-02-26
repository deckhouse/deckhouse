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
	//  i=$(echo "$cloudProviderOpenstack" | jq -r --argjson data "$provider_discovery_data" '.internalNetworkNames //= $data.internalNetworkNames | .internalNetworkNames | . //= [] | unique')
	//  values::set cloudProviderOpenstack.internal.internalNetworkNames "$i"
	if len(module.InternalNetworkNames) > 0 {
		dd.InternalNetworkNames = module.InternalNetworkNames
	}
	dd.InternalNetworkNames = unique(dd.InternalNetworkNames)

	//  i=$(echo "$cloudProviderOpenstack" | jq -r --argjson data "$provider_discovery_data" '.externalNetworkNames //= $data.externalNetworkNames | .externalNetworkNames + .additionalExternalNetworkNames | . //= [] | unique')
	//  values::set cloudProviderOpenstack.internal.externalNetworkNames "$i"
	if len(module.ExternalNetworkNames) > 0 {
		dd.ExternalNetworkNames = module.ExternalNetworkNames
	}

	if len(module.AdditionalExternalNetworkNames) > 0 {
		dd.ExternalNetworkNames = append(dd.ExternalNetworkNames, module.AdditionalExternalNetworkNames...)
	}

	dd.ExternalNetworkNames = unique(dd.ExternalNetworkNames)

	//  i=$(echo "$cloudProviderOpenstack" | jq -r --argjson data "$provider_discovery_data" '.zones //= $data.zones | .zones | . //= [] | unique')
	//  values::set cloudProviderOpenstack.internal.zones "$i"
	if len(module.Zones) > 0 {
		dd.Zones = module.Zones
	}
	dd.Zones = unique(dd.Zones)

	//  i=$(echo "$cloudProviderOpenstack" | jq -r --argjson data "$provider_discovery_data" 'if (.instances == null or .instances == {}) then $data.instances else .instances end | . //= {}')
	//  values::set cloudProviderOpenstack.internal.instances "$i"
	if module.Instances != nil && !module.Instances.IsEmpty() {
		dd.Instances = *module.Instances
	}

	//  i=$(echo "$cloudProviderOpenstack" | jq -r --argjson data "$provider_discovery_data" 'if .podNetworkMode == null then $data.podNetworkMode else .podNetworkMode end')
	//  values::set cloudProviderOpenstack.internal.podNetworkMode "$i"

	if module.PodNetworkMode != "" {
		dd.PodNetworkMode = module.PodNetworkMode
	}

	//  i=$(echo "$cloudProviderOpenstack" | jq -r --argjson data "$provider_discovery_data" 'if (.loadBalancer == null or .loadBalancer == {}) then $data.loadBalancer else .loadBalancer end | . //= {}')
	//  values::set cloudProviderOpenstack.internal.loadBalancer "$i"

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
