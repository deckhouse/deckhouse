// Copyright 2024 Flant JSC
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

type DVPCloudProviderDiscoveryData struct {
	APIVersion       string            `json:"apiVersion,omitempty"`
	Kind             string            `json:"kind,omitempty"`
	Layout           string            `json:"layout,omitempty"`
	Zones            []string          `json:"zones,omitempty"`
	StorageClassList []DVPStorageClass `json:"storageClasses,omitempty"`
}

type DVPStorageClass struct {
	Name                 string `json:"name,omitempty"`
	VolumeBindingMode    string `json:"volumeBindingMode,omitempty"`
	ReclaimPolicy        string `json:"reclaimPolicy,omitempty"`
	AllowVolumeExpansion bool   `json:"allowVolumeExpansion,omitempty"`
	IsEnabled            bool   `json:"isEnabled,omitempty"`
	IsDefault            bool   `json:"isDefault,omitempty"`
}
