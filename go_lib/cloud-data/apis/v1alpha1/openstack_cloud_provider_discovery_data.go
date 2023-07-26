// Copyright 2023 Flant JSC
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

package v1alpha1

type OpenStackCloudProviderDiscoveryData struct {
	APIVersion string `json:"apiVersion,omitempty"`
	Kind       string `json:"kind,omitempty"`

	Flavors                  []string                                        `json:"flavors"`
	AdditionalNetworks       []string                                        `json:"additionalNetworks"`
	AdditionalSecurityGroups []string                                        `json:"additionalSecurityGroups"`
	DefaultImageName         string                                          `json:"defaultImageName"`
	Images                   []string                                        `json:"images"`
	MainNetwork              string                                          `json:"mainNetwork"`
	Zones                    []string                                        `json:"zones"`
	VolumeTypes              []OpenStackCloudProviderDiscoveryDataVolumeType `json:"volumeTypes"`
}

type OpenStackCloudProviderDiscoveryDataVolumeType struct {
	ID          string            `json:"id,omitempty"`
	Name        string            `json:"name,omitempty"`
	Description string            `json:"description,omitempty"`
	ExtraSpecs  map[string]string `json:"extraSpecs,omitempty"`
	IsPublic    bool              `json:"isPublic,omitempty"`
	QosSpecID   string            `json:"qosSpecID,omitempty"`
}
