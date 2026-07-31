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

type YandexCloudDiscoveryData struct {
	APIVersion                    string            `json:"apiVersion,omitempty" yaml:"apiVersion,omitempty"`
	Kind                          string            `json:"kind,omitempty" yaml:"kind,omitempty"`
	Region                        string            `json:"region,omitempty" yaml:"region,omitempty"`
	RouteTableID                  string            `json:"routeTableID,omitempty" yaml:"routeTableID,omitempty"`
	DefaultLbTargetGroupNetworkID string            `json:"defaultLbTargetGroupNetworkId,omitempty" yaml:"defaultLbTargetGroupNetworkId,omitempty"`
	InternalNetworkIDs            []string          `json:"internalNetworkIDs,omitempty" yaml:"internalNetworkIDs,omitempty"`
	Zones                         []string          `json:"zones,omitempty" yaml:"zones,omitempty"`
	ZoneToSubnetIDMap             map[string]string `json:"zoneToSubnetIdMap,omitempty" yaml:"zoneToSubnetIdMap,omitempty"`
	ShouldAssignPublicIPAddress   bool              `json:"shouldAssignPublicIPAddress,omitempty" yaml:"shouldAssignPublicIPAddress,omitempty"`
	NATInstanceName               string            `json:"natInstanceName,omitempty" yaml:"natInstanceName,omitempty"`
	NATInstanceZone               string            `json:"natInstanceZone,omitempty" yaml:"natInstanceZone,omitempty"`
	MonitoringAPIKey              string            `json:"monitoringAPIKey,omitempty" yaml:"monitoringAPIKey,omitempty"`
}
