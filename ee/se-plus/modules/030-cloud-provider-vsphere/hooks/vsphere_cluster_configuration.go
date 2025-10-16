/*
Copyright 2021 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package hooks

import (
	"encoding/json"
	"errors"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/utils/ptr"

	"github.com/deckhouse/deckhouse/dhctl/pkg/config"
	v1 "github.com/deckhouse/deckhouse/ee/se-plus/modules/030-cloud-provider-vsphere/hooks/internal/v1"
	cloudDataV1 "github.com/deckhouse/deckhouse/go_lib/cloud-data/apis/v1"
	"github.com/deckhouse/deckhouse/go_lib/hooks/cluster_configuration"
)

var _ = cluster_configuration.RegisterHook(func(input *go_hook.HookInput, metaCfg *config.MetaConfig, providerDiscoveryData *unstructured.Unstructured, _ bool) error {

	p := make(map[string]json.RawMessage)
	if metaCfg != nil {
		p = metaCfg.ProviderClusterConfig
	}

	var providerClusterConfiguration v1.VsphereProviderClusterConfiguration
	err := convertJSONRawMessageToStruct(p, &providerClusterConfiguration)
	if err != nil {
		return err
	}

	var moduleConfiguration v1.VsphereModuleConfiguration
	err = json.Unmarshal([]byte(input.Values.Get("cloudProviderVsphere").String()), &moduleConfiguration)
	if err != nil {
		return err
	}

	err = overrideValues(&providerClusterConfiguration, &moduleConfiguration)
	if err != nil {
		return err
	}
	input.Values.Set("cloudProviderVsphere.internal.providerClusterConfiguration", providerClusterConfiguration)

	var discoveryData cloudDataV1.VsphereCloudDiscoveryData
	if providerDiscoveryData != nil {
		err := sdk.FromUnstructured(providerDiscoveryData, &discoveryData)
		if err != nil {
			return err
		}
	}

	providerDiscoveryDataValuesJSON, ok := input.Values.GetOk("cloudProviderVsphere.internal.providerDiscoveryData")
	if ok && len(providerDiscoveryDataValuesJSON.String()) != 0 {
		var providerDiscoveryDataValues cloudDataV1.VsphereCloudDiscoveryData
		err = json.Unmarshal([]byte(providerDiscoveryDataValuesJSON.String()), &providerDiscoveryDataValues)
		if err != nil {
			return err
		}

		discoveryData = mergeDiscoveryData(discoveryData, providerDiscoveryDataValues)
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

func overrideValues(p *v1.VsphereProviderClusterConfiguration, m *v1.VsphereModuleConfiguration) error {
	if m.Host != nil {
		if p.Provider == nil {
			p.Provider = &v1.VsphereProvider{}
		}
		p.Provider.Server = m.Host
	}

	if m.Username != nil {
		if p.Provider == nil {
			p.Provider = &v1.VsphereProvider{}
		}
		p.Provider.Username = m.Username
	}

	if m.Password != nil {
		if p.Provider == nil {
			p.Provider = &v1.VsphereProvider{}
		}
		p.Provider.Password = m.Password
	}

	if m.Insecure != nil {
		if p.Provider == nil {
			p.Provider = &v1.VsphereProvider{}
		}
		p.Provider.Insecure = m.Insecure
	}

	if m.RegionTagCategory != nil {
		p.RegionTagCategory = m.RegionTagCategory
	}

	if p.RegionTagCategory == nil {
		p.RegionTagCategory = ptr.To("k8s-region")
	}

	if m.ZoneTagCategory != nil {
		p.ZoneTagCategory = m.ZoneTagCategory
	}

	if p.ZoneTagCategory == nil {
		p.ZoneTagCategory = ptr.To("k8s-zone")
	}

	if m.DisableTimesync != nil {
		p.DisableTimesync = m.DisableTimesync
	}

	if p.DisableTimesync == nil {
		p.DisableTimesync = ptr.To(true)
	}

	if m.ExternalNetworkNames != nil {
		p.ExternalNetworkNames = m.ExternalNetworkNames
	}

	if m.InternalNetworkNames != nil {
		p.InternalNetworkNames = m.InternalNetworkNames
	}

	if m.Region != nil {
		p.Region = m.Region
	}

	if m.Zones != nil {
		p.Zones = m.Zones
	}

	if p.Zones == nil {
		return errors.New("zones cannot be empty")
	}

	if m.VMFolderPath != nil {
		p.VMFolderPath = m.VMFolderPath
	}

	if m.SSHKeys != nil {
		p.SSHPublicKey = &(*m.SSHKeys)[0]
	}

	if m.Nsxt != nil {
		p.Nsxt = m.Nsxt
	}
	return nil
}

func mergeDiscoveryData(newValue cloudDataV1.VsphereCloudDiscoveryData, currentValue cloudDataV1.VsphereCloudDiscoveryData) cloudDataV1.VsphereCloudDiscoveryData {
	result := currentValue
	if newValue.APIVersion != "" && currentValue.APIVersion == "" {
		result.APIVersion = newValue.APIVersion
	}
	if newValue.Kind != "" && currentValue.Kind == "" {
		result.Kind = newValue.Kind
	}
	if newValue.VMFolderPath != "" && currentValue.VMFolderPath == "" {
		result.VMFolderPath = newValue.VMFolderPath
	}
	if newValue.ResourcePoolPath != "" && currentValue.ResourcePoolPath == "" {
		result.ResourcePoolPath = newValue.ResourcePoolPath
	}
	if len(newValue.Zones) > 0 && len(currentValue.Zones) == 0 {
		result.Zones = newValue.Zones
	}
	return result
}
