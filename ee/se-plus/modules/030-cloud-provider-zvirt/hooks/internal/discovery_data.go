/*
Copyright 2026 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package internal

import (
	"encoding/json"
	"fmt"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"

	sdkobjectpatch "github.com/deckhouse/module-sdk/pkg/object-patch"

	clouddatav1 "github.com/deckhouse/deckhouse/go_lib/cloud-data/apis/v1"
)

// ResolveDiscoveryData picks the discovery data the module renders from:
//
//  1. the candi Secret — what dhctl recorded from the infrastructure run of a cluster configured
//     through the ModuleConfig;
//  2. the legacy PCC Secret, while the cluster still has one;
//  3. nothing — a hybrid cluster has neither, because no infrastructure run happened.
//
// The result always carries the defaults, so templates/registration.yaml, which requires the
// discovery data, renders even in case 3. That also breaks the cycle a hybrid cluster would
// otherwise be stuck in: the storage domains come from cloud-data-discoverer, and the discoverer
// is deployed by the very release that needs the discovery data to render.
//
// The storage domains the discoverer has already put into values are kept when the chosen source
// has none: neither dhctl nor the PCC carries them (base-infrastructure outputs an empty list),
// and discover.go runs at the same order as the caller, so it may have run first.
func ResolveDiscoveryData(
	input *go_hook.HookInput,
	pcc *PCCSecretFilterResult,
) (clouddatav1.ZvirtCloudProviderDiscoveryData, error) {
	candiResults, err := sdkobjectpatch.UnmarshalToStruct[CandiDiscoveryDataFilterResult](input.Snapshots, "candi_discovery_data")
	if err != nil {
		return clouddatav1.ZvirtCloudProviderDiscoveryData{}, fmt.Errorf("unmarshal candi_discovery_data snapshots: %w", err)
	}

	var discoveryData clouddatav1.ZvirtCloudProviderDiscoveryData
	switch {
	case len(candiResults) > 0 && candiResults[0].ProviderDiscoveryData != nil:
		discoveryData = *candiResults[0].ProviderDiscoveryData
	case pcc != nil && pcc.ProviderDiscoveryData != nil:
		discoveryData = *pcc.ProviderDiscoveryData
	}

	if len(discoveryData.StorageDomains) == 0 {
		if raw, ok := input.Values.GetOk("cloudProviderZvirt.internal.providerDiscoveryData"); ok {
			var current clouddatav1.ZvirtCloudProviderDiscoveryData
			if err := json.Unmarshal([]byte(raw.Raw), &current); err != nil {
				return clouddatav1.ZvirtCloudProviderDiscoveryData{}, fmt.Errorf("decode cloudProviderZvirt.internal.providerDiscoveryData: %w", err)
			}
			discoveryData.StorageDomains = current.StorageDomains
		}
	}

	discoveryData.SetDefaults()

	return discoveryData, nil
}
