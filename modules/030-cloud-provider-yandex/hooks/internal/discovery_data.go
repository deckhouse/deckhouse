/*
Copyright 2026 Flant JSC

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

package internal

import (
	"fmt"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"

	sdkobjectpatch "github.com/deckhouse/module-sdk/pkg/object-patch"

	clouddatav1 "github.com/deckhouse/deckhouse/go_lib/cloud-data/apis/v1"
)

// ResolveDiscoveryData picks the discovery data both migration hooks render from, in one place so
// the two cannot disagree.
//
// Source priority, high to low:
//  1. the candi Secret - the infrastructure run's recorded output;
//  2. the legacy PCC discovery payload - also the infrastructure run's recorded output.
//
// Both describe an infrastructure run, so a cluster whose infrastructure DKP does not create has
// neither, and the result carries only the type markers and the region default. That is why
// openapi/values.yaml requires almost nothing here: the workloads read the network facts through
// templates/_helpers.tpl, which falls back to the operator's own nodes.parameters.existing*
// whenever this payload does not carry them.
//
// The candi Secret exists long before the infrastructure run fills it in, so an empty payload
// counts as no source at all and must not shadow the PCC payload - hence the nil check rather
// than a mere "is the snapshot there" test.
//
// Every payload goes through SetDefaults, which fills in the apiVersion/kind markers and the
// region. Skipping that leaves internal.providerDiscoveryData without them.
func ResolveDiscoveryData(
	input *go_hook.HookInput,
	pcc *PCCSecretFilterResult,
) (clouddatav1.YandexCloudDiscoveryData, error) {
	candiResults, err := sdkobjectpatch.UnmarshalToStruct[CandiDiscoveryDataFilterResult](input.Snapshots, "candi_discovery_data")
	if err != nil {
		return clouddatav1.YandexCloudDiscoveryData{}, fmt.Errorf("unmarshal candi_discovery_data snapshots: %w", err)
	}

	var discoveryData clouddatav1.YandexCloudDiscoveryData
	switch {
	case len(candiResults) > 0 && candiResults[0].ProviderDiscoveryData != nil:
		discoveryData = *candiResults[0].ProviderDiscoveryData
	case pcc != nil && pcc.ProviderDiscoveryData != nil:
		discoveryData = *pcc.ProviderDiscoveryData
	}

	discoveryData.SetDefaults()

	return discoveryData, nil
}
