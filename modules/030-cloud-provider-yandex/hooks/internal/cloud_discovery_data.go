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
	"k8s.io/utils/ptr"

	clouddatav1 "github.com/deckhouse/deckhouse/go_lib/cloud-data/apis/v1"
)

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
