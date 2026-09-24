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

type DynamixCloudProviderDiscoveryData struct {
	APIVersion      string                 `json:"apiVersion,omitempty"`
	Kind            string                 `json:"kind,omitempty"`
	StoragePolicies []DynamixStoragePolicy `json:"storagePolicies,omitempty"`
}

// DynamixStoragePolicy is a storage policy available to the account the cluster
// runs under. Deckhouse creates one StorageClass per policy; the platform picks
// the actual storage endpoint and pool inside the policy on its own.
//
// The cloud-data-discoverer publishes only policies in the ENABLED state, so
// there is no isEnabled flag here. There is no isDefault flag either: the
// default StorageClass is the user's choice (storageClass.default), not the
// cloud's.
type DynamixStoragePolicy struct {
	Name      string `json:"name"`
	LimitIOPS uint64 `json:"limitIOPS"`
}
