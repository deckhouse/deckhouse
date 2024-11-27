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

	v1 "github.com/deckhouse/deckhouse/ee/se_plus/modules/030-cloud-provider-vsphere/hooks/internal/v1"
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

func filter(arr []vsphere.ZonedDataStore, cond func(vsphere.ZonedDataStore) bool) []vsphere.ZonedDataStore {
	var result []vsphere.ZonedDataStore
	for _, x := range arr {
		if cond(x) {
			result = append(result, x)
		}
	}
	return result
}

func doDiscover(input *go_hook.HookInput, dc dependency.Container) error {
	configJSON, ok := input.Values.GetOk("cloudProviderVsphere.internal.providerClusterConfiguration")
	if !ok {
		return fmt.Errorf("no providerClusterConfiguration present, discovery is not possible")
	}

	var c v1.VsphereProviderClusterConfiguration
	err := json.Unmarshal([]byte(configJSON.String()), &c)
	if err != nil {
		return fmt.Errorf("error Unmarshalling ProviderClusterConfiguration: %v", err)
	}

	config := &vsphere.ProviderClusterConfiguration{
		Region:            *c.Region,
		RegionTagCategory: *c.RegionTagCategory,
		ZoneTagCategory:   *c.ZoneTagCategory,
		Provider: vsphere.Provider{
			Server:   *c.Provider.Server,
			Username: *c.Provider.Username,
			Password: *c.Provider.Password,
			Insecure: *c.Provider.Insecure,
		},
	}

	vc, err := dc.GetVsphereClient(config)
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

		if len(excludes) > 0 {
			r := regexp.MustCompile(`^(` + strings.Join(excludes, "|") + `)$`)

			storageClasses = filter(storageClasses, func(val vsphere.ZonedDataStore) bool {
				matched := r.MatchString(val.Name)
				return !matched
			})
		}
	}

	input.Values.Set("cloudProviderVsphere.internal.storageClasses", storageClasses)

	return nil
}
