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

var _ = cluster_configuration.RegisterHook(func(input *go_hook.HookInput, metaCfg *config.MetaConfig, providerDiscoveryData *unstructured.Unstructured, _ bool) error {
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

	input.Values.Set("cloudProviderOpenstack.internal.connection", providerClusterConfiguration.Provider)
	input.Values.Set("cloudProviderOpenstack.internal.internalNetworkNames", discoveryData.InternalNetworkNames)
	input.Values.Set("cloudProviderOpenstack.internal.externalNetworkNames", discoveryData.ExternalNetworkNames)
	input.Values.Set("cloudProviderOpenstack.internal.zones", discoveryData.Zones)
	input.Values.Set("cloudProviderOpenstack.internal.instances", discoveryData.Instances)
	input.Values.Set("cloudProviderOpenstack.internal.podNetworkMode", discoveryData.PodNetworkMode)
	input.Values.Set("cloudProviderOpenstack.internal.loadBalancer", discoveryData.LoadBalancer)
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
