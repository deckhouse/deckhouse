/*
Copyright 2021 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package hooks

import (
	"encoding/json"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/tidwall/gjson"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/deckhouse/deckhouse/dhctl/pkg/config"
	v1 "github.com/deckhouse/deckhouse/ee/modules/030-cloud-provider-vsphere/hooks/internal/v1"
	"github.com/deckhouse/deckhouse/go_lib/hooks/cluster_configuration"
)

type OverrideRule struct {
	ConfigKey string
	ValueKey  string
	ValuePath string
	Default   interface{}
}

func (rule OverrideRule) ValueFullPath() string {
	path := rule.ValueKey
	if len(rule.ValuePath) > 0 {
		path = path + "." + rule.ValuePath
	}
	return path
}

const configPrefix = "cloudProviderVsphere."
const internalPrefix = configPrefix + "internal."
const clusterConfigurationPrefix = internalPrefix + "providerClusterConfiguration."

var _ = cluster_configuration.RegisterHook(func(input *go_hook.HookInput, metaCfg *config.MetaConfig, providerDiscoveryData *unstructured.Unstructured, secretFound bool) error {
	overrideMap := []OverrideRule{
		{
			ConfigKey: "host",
			ValueKey:  "provider",
			ValuePath: "server",
		},
		{
			ConfigKey: "username",
			ValueKey:  "provider",
			ValuePath: "username",
		},
		{
			ConfigKey: "password",
			ValueKey:  "provider",
			ValuePath: "password",
		},
		{
			ConfigKey: "insecure",
			ValueKey:  "provider",
			ValuePath: "insecure",
			Default:   false,
		},
		{
			ConfigKey: "regionTagCategory",
			ValueKey:  "regionTagCategory",
		},
		{
			ConfigKey: "zoneTagCategory",
			ValueKey:  "zoneTagCategory",
		},
		{
			ConfigKey: "disableTimesync",
			ValueKey:  "disableTimesync",
			Default:   true,
		},
		{
			ConfigKey: "externalNetworkNames",
			ValueKey:  "externalNetworkNames",
		},
		{
			ConfigKey: "internalNetworkNames",
			ValueKey:  "internalNetworkNames",
		},
		{
			ConfigKey: "region",
			ValueKey:  "region",
		},
		{
			ConfigKey: "zones",
			ValueKey:  "zones",
		},
		{
			ConfigKey: "vmFolderPath",
			ValueKey:  "vmFolderPath",
		},
		{
			ConfigKey: "sshKeys.0",
			ValueKey:  "sshPublicKey",
		},
	}

	p := map[string]json.RawMessage{}
	if metaCfg != nil {
		p = metaCfg.ProviderClusterConfig
	}

	var providerClusterConfiguration v1.VsphereProviderClusterConfiguration
	err := convertJsonRawMessageToStruct(p, &providerClusterConfiguration)
	if err != nil {
		return err
	}

	input.Values.Set("cloudProviderVsphere.internal.providerClusterConfiguration", providerClusterConfiguration)
	if _, ok := p["provider"]; !ok {
		input.Values.Set(clusterConfigurationPrefix+"provider", map[string]interface{}{})
	}

	for _, rule := range overrideMap {
		configResult, configOk := input.Values.GetOk(configPrefix + rule.ConfigKey)
		providerResultRaw, providerOk := p[rule.ValueKey]
		if len(rule.ValuePath) > 0 && providerOk {
			providerResult := gjson.Get(string(providerResultRaw), rule.ValuePath)
			providerOk = providerResult.Exists()
		}
		if configOk {
			input.Values.Set(clusterConfigurationPrefix+rule.ValueFullPath(), configResult.Value())
		} else if rule.Default != nil && !providerOk {
			input.Values.Set(clusterConfigurationPrefix+rule.ValueFullPath(), rule.Default)
		}
	}

	providerDiscoveryDataObject := map[string]interface{}{}
	if providerDiscoveryData != nil {
		providerDiscoveryDataObject = providerDiscoveryData.Object
	}
	input.Values.Set(internalPrefix+"providerDiscoveryData", providerDiscoveryDataObject)

	return nil
})

func convertJsonRawMessageToStruct(in map[string]json.RawMessage, out interface{}) error {
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
