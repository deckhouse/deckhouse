/*
Copyright 2022 Flant JSC

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package dynamic_probe

import (
	"context"
	"fmt"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	"github.com/flant/shell-operator/pkg/kube_events_manager/types"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/yaml"

	"github.com/deckhouse/module-sdk/pkg"

	"github.com/deckhouse/deckhouse/go_lib/set"
)

// This hook populates internal values with object names that are used for
// dynamic probes. The names are for nginx ingress controllers and ephemeral
// nodes with available cloud zones.
var _ = sdk.RegisterFunc(
	&go_hook.HookConfig{
		Queue: "/modules/upmeter/dynamic_probes",
		Kubernetes: []go_hook.KubernetesConfig{
			{
				Name:       "upmeter_discovery_ingress_controllers",
				ApiVersion: "v1",
				Kind:       "ConfigMap",
				NameSelector: &types.NameSelector{
					MatchNames: []string{"upmeter-discovery-controllers"},
				},
				NamespaceSelector: &types.NamespaceSelector{
					NameSelector: &types.NameSelector{
						MatchNames: []string{"d8-ingress-nginx"},
					},
				},
				FilterFunc: filterNamesFromConfigmap,
			},
			{
				Name:       "upmeter_discovery_nodegroups",
				ApiVersion: "v1",
				Kind:       "ConfigMap",
				NameSelector: &types.NameSelector{
					MatchNames: []string{"upmeter-discovery-cloud-ephemeral-nodegroups"},
				},
				NamespaceSelector: &types.NamespaceSelector{
					NameSelector: &types.NameSelector{
						MatchNames: []string{"d8-cloud-instance-manager"},
					},
				},
				FilterFunc: filterNamesFromConfigmap,
			},
			{
				Name:       "cloud_provider_secret",
				ApiVersion: "v1",
				Kind:       "Secret",
				NameSelector: &types.NameSelector{
					MatchNames: []string{"d8-node-manager-cloud-provider"},
				},
				NamespaceSelector: &types.NamespaceSelector{
					NameSelector: &types.NameSelector{
						MatchNames: []string{"kube-system"},
					},
				},
				FilterFunc: filterCloudProviderAvailabilityZonesFromSecret,
			},
		},
	},

	collectDynamicNames,
)

// collectDynamicNames sets names of objects to internal values
func collectDynamicNames(_ context.Context, input *go_hook.HookInput) error {
	// Input, empty strings mean invalidated data
	var (
		ingressNames   = parseSingleStringSet(input.Snapshots.Get("upmeter_discovery_ingress_controllers")).Delete("").Slice()
		nodeGroupNames = parseSingleStringSet(input.Snapshots.Get("upmeter_discovery_nodegroups")).Delete("").Slice()
		loc            = parseCloudLocations(input.Snapshots.Get("cloud_provider_secret"))
	)

	// Populate values. `zonePrefix` is for cloud zones that are passed around without region
	// prefix, e.g. "west-1" will be just "1" in Azure.
	data := emptyNames().
		WithIngressControllers(ingressNames...).
		WithZonePrefix(loc.ZonePrefix)

	// We can track ephemeral node groups if only we have zones present in cloud provider secret.
	if len(loc.Zones) > 0 {
		data = data.
			WithZones(loc.Zones...).
			WithNodeGroups(nodeGroupNames...)
	}

	// Output
	input.Values.Set("upmeter.internal.dynamicProbes", data)
	return nil
}

func parseSingleStringSet(filtered []pkg.Snapshot) set.Set {
	if len(filtered) == 0 {
		return set.New()
	}
	var ss []string
	err := filtered[0].UnmarshalTo(&ss)
	if err != nil {
		// the secret MUST contain zones, so let it panic
		panic(fmt.Errorf("failed to unmarshal dynamic probe names: %w", err))
	}

	return set.New(ss...)
}

func filterNamesFromConfigmap(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	cm := new(v1.ConfigMap)
	err := sdk.FromUnstructured(obj, cm)
	if err != nil {
		return nil, err
	}

	namesRaw, ok := cm.Data["names"]
	if !ok {
		return []string{}, nil
	}

	var names []string
	if err := yaml.Unmarshal([]byte(namesRaw), &names); err != nil {
		return nil, err
	}
	return names, nil
}

// cloudLocations contains zones (with prefixes) and reqion prefix itself for NodeGroup fetcher in
// Upmeter Agent.
type cloudLocations struct {
	Zones      []string
	ZonePrefix string
}

func parseCloudLocations(filtered []pkg.Snapshot) cloudLocations {
	if len(filtered) != 1 {
		return cloudLocations{}
	}
	var loc cloudLocations
	err := filtered[0].UnmarshalTo(&loc)
	if err != nil {
		panic(fmt.Errorf("failed to unmarshal cloud locations: %w", err))
	}

	loc.Zones = set.New(loc.Zones...).Delete("").Slice() // unique and non-empty
	return loc
}

func filterCloudProviderAvailabilityZonesFromSecret(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	secret := new(v1.Secret)
	err := sdk.FromUnstructured(obj, secret)
	if err != nil {
		return nil, err
	}

	loc := cloudLocations{}

	zoneData, ok := secret.Data["zones"]
	if !ok {
		// zone absence is fine for static clusters
		return loc, nil
	}
	if err := yaml.Unmarshal(zoneData, &loc.Zones); err != nil {
		return nil, err
	}

	provider, ok := secret.Data["type"]
	if !ok {
		return loc, nil
	}
	if string(provider) == "azure" {
		region, ok := secret.Data["region"]
		if !ok {
			return loc, fmt.Errorf("azure cloud provider secret must contain region")
		}

		// Azure zones are in format "region-zone", and we have to track the knowledge of the zone
		// prefix since nodegroups don't carry the region information themselves.
		loc.ZonePrefix = string(region)
		for i, zone := range loc.Zones {
			loc.Zones[i] = fmt.Sprintf("%s-%s", region, zone)
		}
	}

	return loc, nil
}
