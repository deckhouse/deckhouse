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

	overrideValues(&providerClusterConfiguration, &moduleConfiguration)
	input.Values.Set("cloudProviderOpenstack.internal.providerClusterConfiguration", providerClusterConfiguration)

	var discoveryData v1.OpenstackCloudDiscoveryData
	if providerDiscoveryData != nil {
		err := sdk.FromUnstructured(providerDiscoveryData, &discoveryData)
		if err != nil {
			return err
		}
	}
	input.Values.Set("cloudProviderVsphere.internal.providerDiscoveryData", discoveryData)

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

func overrideValues(p *v1.OpenstackProviderClusterConfiguration, m *v1.OpenstackModuleConfiguration) {
	if len(m.Zones) > 0 {
		p.Zones = m.Zones
	}

	if len(m.AdditionalExternalNetworkNames) > 0 {
		p
	}

	m.Connection

	m.
}
