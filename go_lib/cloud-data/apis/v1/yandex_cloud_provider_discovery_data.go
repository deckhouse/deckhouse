// Copyright 2026 Flant JSC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package v1

const (
	YandexCloudDiscoveryDataDefaultRegion = "ru-central1"
)

type YandexCloudDiscoveryData struct {
	APIVersion                    string            `json:"apiVersion" yaml:"apiVersion"`
	Kind                          string            `json:"kind" yaml:"kind"`
	Region                        string            `json:"region" yaml:"region"`
	RouteTableID                  string            `json:"routeTableID,omitempty" yaml:"routeTableID,omitempty"`
	DefaultLbTargetGroupNetworkID string            `json:"defaultLbTargetGroupNetworkId,omitempty" yaml:"defaultLbTargetGroupNetworkId,omitempty"`
	InternalNetworkIDs            []string          `json:"internalNetworkIDs,omitempty" yaml:"internalNetworkIDs,omitempty"`
	Zones                         []string          `json:"zones,omitempty" yaml:"zones,omitempty"`
	ZoneToSubnetIDMap             map[string]string `json:"zoneToSubnetIdMap,omitempty" yaml:"zoneToSubnetIdMap,omitempty"`
	ShouldAssignPublicIPAddress   *bool             `json:"shouldAssignPublicIPAddress,omitempty" yaml:"shouldAssignPublicIPAddress,omitempty"`
	NATInstanceName               string            `json:"natInstanceName,omitempty" yaml:"natInstanceName,omitempty"`
	NATInstanceZone               string            `json:"natInstanceZone,omitempty" yaml:"natInstanceZone,omitempty"`
	MonitoringAPIKey              string            `json:"monitoringAPIKey,omitempty" yaml:"monitoringAPIKey,omitempty"`
}

// SetDefaults stamps the type markers, which identify the payload and never vary, and fills in
// the region when the payload does not carry one - every layout's cloud_discovery_data output
// hardcodes the same value, so an absent region means "the usual one".
//
// The region is defaulted rather than overwritten: a payload that does carry a region states a
// fact about the infrastructure that was actually created, and silently rewriting it would hide
// a real mismatch instead of surfacing it.
//
// The receiver has to be a pointer - with a value receiver the assignments land on a copy and the
// call silently does nothing, which is how it behaved until 2026-08-27. Its sibling
// VCDCloudProviderDiscoveryData.SetDefaults takes a pointer for the same reason.
func (d *YandexCloudDiscoveryData) SetDefaults() {
	d.APIVersion = APIVersion
	d.Kind = YandexCloudDiscoveryDataKind

	if d.Region == "" {
		d.Region = YandexCloudDiscoveryDataDefaultRegion
	}
}
