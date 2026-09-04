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
	"k8s.io/utils/ptr"

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
// Every payload goes through MergeDiscoveryData, which stamps the apiVersion/kind markers and the
// region default onto it. Skipping that leaves internal.providerDiscoveryData without them.
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

	// The resolved payload is the value to keep, so it goes in as currentValue: MergeDiscoveryData
	// stamps the type markers and the region default onto it and grafts nothing on top. Passing it
	// as newValue instead would lose its region - the merge defaults currentValue before grafting,
	// and region is not one of the grafted fields.
	return MergeDiscoveryData(clouddatav1.YandexCloudDiscoveryData{}, discoveryData), nil
}

// MergeDiscoveryData grafts new discovery-data fields onto the existing set
// without overwriting already-populated values.
//
// The only writer of these values is yandex_cluster_configuration.go, so "existing" means what a
// previous run of that same hook resolved: the merge keeps a cluster rendering when the candi
// Secret or the legacy PCC is momentarily unreadable, rather than blanking the values it had.
func MergeDiscoveryData(newValue, currentValue clouddatav1.YandexCloudDiscoveryData) clouddatav1.YandexCloudDiscoveryData {
	result := currentValue

	result.SetDefaults()

	if newValue.RouteTableID != "" && result.RouteTableID == "" {
		result.RouteTableID = newValue.RouteTableID
	}
	if newValue.DefaultLbTargetGroupNetworkID != "" && result.DefaultLbTargetGroupNetworkID == "" {
		result.DefaultLbTargetGroupNetworkID = newValue.DefaultLbTargetGroupNetworkID
	}
	if len(newValue.InternalNetworkIDs) > 0 && len(result.InternalNetworkIDs) == 0 {
		result.InternalNetworkIDs = newValue.InternalNetworkIDs
	}
	if len(newValue.Zones) > 0 && len(result.Zones) == 0 {
		result.Zones = newValue.Zones
	}
	if len(newValue.ZoneToSubnetIDMap) > 0 && len(result.ZoneToSubnetIDMap) == 0 {
		result.ZoneToSubnetIDMap = newValue.ZoneToSubnetIDMap
	}
	if newValue.ShouldAssignPublicIPAddress != nil && result.ShouldAssignPublicIPAddress == nil {
		result.ShouldAssignPublicIPAddress = ptr.To(*newValue.ShouldAssignPublicIPAddress)
	}
	if newValue.NATInstanceName != "" && result.NATInstanceName == "" {
		result.NATInstanceName = newValue.NATInstanceName
	}
	if newValue.NATInstanceZone != "" && result.NATInstanceZone == "" {
		result.NATInstanceZone = newValue.NATInstanceZone
	}
	if newValue.MonitoringAPIKey != "" && result.MonitoringAPIKey == "" {
		result.MonitoringAPIKey = newValue.MonitoringAPIKey
	}

	return result
}
