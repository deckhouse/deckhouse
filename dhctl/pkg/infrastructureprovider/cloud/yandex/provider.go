// Copyright 2025 Flant JSC
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

package yandex

const ProviderName = "yandex"

type instanceClass struct {
	ExternalIPAddresses []string `json:"externalIPAddresses"`
}

type masterNodeGroupSpec struct {
	Replicas      int           `json:"replicas"`
	InstanceClass instanceClass `json:"instanceClass"`
}

type nodeGroupSpec struct {
	Name          string        `json:"name"`
	Replicas      int           `json:"replicas"`
	InstanceClass instanceClass `json:"instanceClass"`
}

type withNatInstanceSpec struct {
	InternalSubnetCIDR string `json:"internalSubnetCIDR,omitempty"`
	InternalSubnetID   string `json:"internalSubnetID,omitempty"`
}
