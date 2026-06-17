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

package api

// InstanceClass is a provider-specific instance class resource.
type InstanceClass struct {
	TypeMeta   `json:",inline"`
	ObjectMeta `json:"metadata,omitempty"`

	Spec   InstanceClassSpec   `json:"spec,omitempty"`
	Status InstanceClassStatus `json:"status,omitempty"`
}

// InstanceClassSpec holds provider-specific instance class parameters.
type InstanceClassSpec struct {
	EtcdDisk map[string]any `json:"etcdDisk,omitempty"`
}

// InstanceClassStatus holds runtime status fields populated by the provider module.
type InstanceClassStatus struct {
	NodeGroupConsumers []any `json:"nodeGroupConsumers,omitempty"`
}
