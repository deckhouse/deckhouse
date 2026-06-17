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

// NodeType identifies the NodeGroup provisioning model.
type NodeType string

const (
	// NodeTypeCloudPermanent marks a permanently provisioned cloud NodeGroup.
	NodeTypeCloudPermanent NodeType = "CloudPermanent"
)

// NodeGroup is a typed view of the deckhouse.io NodeGroup resource.
type NodeGroup struct {
	TypeMeta   `json:",inline"`
	ObjectMeta `json:"metadata,omitempty"`

	Spec NodeGroupSpec `json:"spec,omitempty"`
}

// NodeGroupSpec holds NodeGroup parameters relevant to cloud-provider validation.
type NodeGroupSpec struct {
	NodeType       NodeType        `json:"nodeType,omitempty"`
	CloudInstances *CloudInstances `json:"cloudInstances,omitempty"`
}

// CloudInstances describes cloud instance provisioning settings for a NodeGroup.
type CloudInstances struct {
	ClassReference *ClassReference `json:"classReference,omitempty"`
	MaxPerZone     int             `json:"maxPerZone,omitempty"`
}

// ClassReference points a NodeGroup to an InstanceClass resource.
type ClassReference struct {
	Kind string `json:"kind,omitempty"`
	Name string `json:"name,omitempty"`
}
