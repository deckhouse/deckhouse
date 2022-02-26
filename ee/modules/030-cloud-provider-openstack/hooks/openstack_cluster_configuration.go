/*
Copyright 2021 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package hooks

import (
	"encoding/json"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/deckhouse/deckhouse/dhctl/pkg/config"
	v1 "github.com/deckhouse/deckhouse/ee/modules/030-cloud-provider-openstack/hooks/internal/v1"
	"github.com/deckhouse/deckhouse/go_lib/hooks/cluster_configuration"
)

var _ = cluster_configuration.RegisterHook(func(input *go_hook.HookInput, metaCfg *config.MetaConfig, providerDiscoveryData *unstructured.Unstructured, secretFound bool) error {
	p := make(map[string]json.RawMessage)
	if metaCfg != nil {
		p = metaCfg.ProviderClusterConfig
	}

	var providerClusterConfiguration v1.OpenstackProviderClusterConfiguration
	err := convertJSONRawMessageToStruct(p, &providerClusterConfiguration)
	if err != nil {
		return err
	}

	var moduleConfiguration v1.OpenstackModuleConfiguration
	err = json.Unmarshal([]byte(input.Values.Get("cloudProviderOpenstack").String()), &moduleConfiguration)
	if err != nil {
		return err
	}

	var discoveryData v1.OpenstackCloudDiscoveryData
	if providerDiscoveryData != nil {
		err := sdk.FromUnstructured(providerDiscoveryData, &discoveryData)
		if err != nil {
			return err
		}
	}
	providerClusterConfiguration.PatchWithModuleConfig(moduleConfiguration)
	discoveryData.PathWithDiscoveryData(moduleConfiguration)

	//   connection=$(echo "$cloudProviderOpenstack" | jq -r --argjson provider "$provider" '.connection //= $provider | .connection')
	//  values::set cloudProviderOpenstack.internal.connection "$connection"
	input.Values.Set("cloudProviderOpenstack.internal.connection", providerClusterConfiguration.Provider)

	//  i=$(echo "$cloudProviderOpenstack" | jq -r --argjson data "$provider_discovery_data" '.internalNetworkNames //= $data.internalNetworkNames | .internalNetworkNames | . //= [] | unique')
	//  values::set cloudProviderOpenstack.internal.internalNetworkNames "$i"
	input.Values.Set("cloudProviderOpenstack.internal.internalNetworkNames", discoveryData.InternalNetworkNames)

	//  i=$(echo "$cloudProviderOpenstack" | jq -r --argjson data "$provider_discovery_data" '.externalNetworkNames //= $data.externalNetworkNames | .externalNetworkNames + .additionalExternalNetworkNames | . //= [] | unique')
	//  values::set cloudProviderOpenstack.internal.externalNetworkNames "$i"
	input.Values.Set("cloudProviderOpenstack.internal.externalNetworkNames", discoveryData.ExternalNetworkNames)

	//
	//  i=$(echo "$cloudProviderOpenstack" | jq -r --argjson data "$provider_discovery_data" '.zones //= $data.zones | .zones | . //= [] | unique')
	//  values::set cloudProviderOpenstack.internal.zones "$i"
	input.Values.Set("cloudProviderOpenstack.internal.zones", discoveryData.Zones)

	//  i=$(echo "$cloudProviderOpenstack" | jq -r --argjson data "$provider_discovery_data" 'if (.instances == null or .instances == {}) then $data.instances else .instances end | . //= {}')
	//  values::set cloudProviderOpenstack.internal.instances "$i"
	input.Values.Set("cloudProviderOpenstack.internal.instances", discoveryData.Instances)
	//  i=$(echo "$cloudProviderOpenstack" | jq -r --argjson data "$provider_discovery_data" 'if .podNetworkMode == null then $data.podNetworkMode else .podNetworkMode end')
	//  values::set cloudProviderOpenstack.internal.podNetworkMode "$i"
	input.Values.Set("cloudProviderOpenstack.internal.podNetworkMode", discoveryData.PodNetworkMode)
	//  i=$(echo "$cloudProviderOpenstack" | jq -r --argjson data "$provider_discovery_data" 'if (.loadBalancer == null or .loadBalancer == {}) then $data.loadBalancer else .loadBalancer end | . //= {}')
	//  values::set cloudProviderOpenstack.internal.loadBalancer "$i"
	input.Values.Set("cloudProviderOpenstack.internal.loadBalancer", discoveryData.LoadBalancer)
	//  i=$(echo "$cloudProviderOpenstack" | jq -r --argjson tags "$tags" '.tags //= $tags | .tags')
	//  values::set cloudProviderOpenstack.internal.tags "$i"
	input.Values.Set("cloudProviderOpenstack.internal.tags", providerClusterConfiguration.Tags)

	return nil
})

func convertJSONRawMessageToStruct(in map[string]json.RawMessage, out interface{}) error {
	b, err := json.Marshal(in)
	if err != nil {
		return err
	}
	err = json.Unmarshal(b, out)
	if err != nil {
		return err
	}
	return nil
}
