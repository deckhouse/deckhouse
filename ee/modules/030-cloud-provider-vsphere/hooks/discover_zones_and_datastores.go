/*
Copyright 2021 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package hooks

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"

	"github.com/deckhouse/deckhouse/go_lib/dependency"
	"github.com/deckhouse/deckhouse/go_lib/dependency/vsphere"
)

type vsphereDiscoveryData struct {
	Datacenter string   `json:"datacenter"`
	Zones      []string `json:"zones"`
}

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	Queue: "/modules/cloud-provider-vsphere",
	Schedule: []go_hook.ScheduleConfig{
		{
			Name:    "vsphere_discover_zones_and_datastores",
			Crontab: "53 * * * *",
		},
	},
	OnBeforeHelm: &go_hook.OrderedConfig{Order: 20},
}, dependency.WithExternalDependencies(doDiscover))

func filter(arr []vsphere.ZonedDataStore, cond func(vsphere.ZonedDataStore) bool) (result []vsphere.ZonedDataStore) {
	for i := range arr {
		if cond(arr[i]) {
			result = append(result, arr[i])
		}
	}
	return
}

func doDiscover(input *go_hook.HookInput, dc dependency.Container) error {
	configJSON, ok := input.Values.GetOk("cloudProviderVsphere.internal.providerClusterConfiguration")
	if !ok {
		return fmt.Errorf("no providerClusterConfiguration present, skipping discovery")
	}

	var config vsphere.ProviderClusterConfiguration
	err := json.Unmarshal([]byte(configJSON.String()), &config)
	if err != nil {
		return fmt.Errorf("error Unmarshalling ProviderClusterConfiguration: %v", err)
	}

	vc, err := dc.GetVsphereClient(&config)
	if err != nil {
		return err
	}
	output, err := vc.GetZonesDatastores()
	if err != nil {
		return fmt.Errorf("error on GetZonesDatastores: %v", err)
	}

	input.Values.Set("cloudProviderVsphere.internal.vsphereDiscoveryData", vsphereDiscoveryData{
		Datacenter: output.Datacenter,
		Zones:      output.Zones,
	})

	storageClasses := output.ZonedDataStores

	if exclude, ok := input.Values.GetOk("cloudProviderVsphere.storageClass.exclude"); ok {
		var excludes []string
		for _, e := range exclude.Array() {
			excludes = append(excludes, e.String())
		}
		r := regexp.MustCompile(`^(` + strings.Join(excludes, "|") + `)$`)

		storageClasses = filter(storageClasses, func(val vsphere.ZonedDataStore) bool {
			matched := r.MatchString(val.Name)
			return !matched
		})
	}
	input.Values.Set("cloudProviderVsphere.internal.storageClasses", storageClasses)

	if v, ok := input.Values.GetOk("cloudProviderVsphere.storageClass.default"); ok {
		input.Values.Set("cloudProviderVsphere.internal.defaultStorageClass", v.String())
	} else {
		input.Values.Remove("cloudProviderVsphere.internal.defaultStorageClass")
	}

	return nil
}
