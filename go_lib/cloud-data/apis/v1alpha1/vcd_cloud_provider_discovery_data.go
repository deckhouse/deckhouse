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

type VCDCloudProviderDiscoveryData struct {
	APIVersion string `json:"apiVersion,omitempty"`
	Kind       string `json:"kind,omitempty"`

	SizingPolicies   []string            `json:"sizingPolicies,omitempty"`
	InternalNetworks []string            `json:"internalNetworks,omitempty"`
	StorageProfiles  []VCDStorageProfile `json:"storageProfiles,omitempty"`
	Version          VCDVersion          `json:"version,omitempty"`
}

type VCDStorageProfile struct {
	Name                    string `json:"name"`
	IsEnabled               bool   `json:"isEnabled,omitempty"`
	IsDefaultStorageProfile bool   `json:"isDefaultStorageProfile,omitempty"`
}

type VCDVersion struct {
	VCDVersion string `json:"vcdVersion,omitempty"`
	APIVersion string `json:"apiVersion,omitempty"`
}
