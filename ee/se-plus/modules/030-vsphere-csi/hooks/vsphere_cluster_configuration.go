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
	apisv1 "github.com/deckhouse/deckhouse/go_lib/cloud-data/apis/v1"
	v1 "github.com/deckhouse/deckhouse/go_lib/cloud-data/apis/vsphere/v1"
	"github.com/deckhouse/deckhouse/go_lib/hooks/cluster_configuration"
)

var _ = cluster_configuration.RegisterHook(func(input *go_hook.HookInput, metaCfg *config.MetaConfig, providerDiscoveryData *unstructured.Unstructured, _ bool) error {
	var (
		providerClusterConfiguration v1.VsphereProviderClusterConfiguration
		moduleConfiguration          v1.VsphereModuleConfiguration
	)

	err := json.Unmarshal([]byte(input.Values.Get("vsphereCsi").String()), &moduleConfiguration)
	if err != nil {
		return err
	}

	err = overrideValues(&providerClusterConfiguration, &moduleConfiguration)
	if err != nil {
		return err
	}
	input.Values.Set("vsphereCsi.internal.providerClusterConfiguration", providerClusterConfiguration)

	var discoveryData apisv1.VsphereCloudDiscoveryData
	if providerDiscoveryData != nil {
		err := sdk.FromUnstructured(providerDiscoveryData, &discoveryData)
		if err != nil {
			return err
		}
	}
	input.Values.Set("vsphereCsi.internal.providerDiscoveryData", discoveryData)

	return nil
})

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

	if m.Region != nil {
		p.Region = m.Region
	}

	if m.Zones != nil {
		p.Zones = m.Zones
	}

	if p.Zones == nil {
		return errors.New("zones cannot be empty")
	}

	if m.SSHKeys != nil {
		p.SSHPublicKey = &(*m.SSHKeys)[0]
	}

	return nil
}
