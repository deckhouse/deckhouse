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

package v1

type VsphereCloudDiscoveryData struct {
	APIVersion string `json:"apiVersion,omitempty"`
	Kind       string `json:"kind,omitempty"`

	VMFolderPath     string                 `json:"vmFolderPath,omitempty"`
	ResourcePoolPath string                 `json:"resourcePoolPath,omitempty"`
	Datacenter       string                 `json:"datacenter,omitempty"`
	Zones            []string               `json:"zones,omitempty"`
	Datastores       []VsphereDatastore     `json:"datastores,omitempty"`
	StoragePolicies  []VsphereStoragePolicy `json:"storagePolicies,omitempty"`
}

type VsphereDatastore struct {
	Zones         []string `json:"zones,omitempty"`
	InventoryPath string   `json:"path,omitempty"`
	Name          string   `json:"name,omitempty"`
	DatastoreType string   `json:"datastoreType,omitempty"`
	DatastoreURL  string   `json:"datastoreURL,omitempty"`
}

type VsphereStoragePolicy struct {
	Name string `json:"name"`
	ID   string `json:"id"`
}
