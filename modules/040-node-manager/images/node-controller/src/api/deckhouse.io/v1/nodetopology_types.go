/*
Copyright 2026 Flant JSC

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package v1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type NodeTopologyManagerState struct {
	// Enabled specifies whether topology manager is enabled.
	// +optional
	Enabled *bool `json:"enabled,omitempty"`

	// Policy specifies the topology manager policy.
	// +optional
	Policy string `json:"policy,omitempty"`

	// Scope specifies the topology manager scope.
	// +optional
	Scope string `json:"scope,omitempty"`
}

type NodeTopologyState struct {
	// TopologyManager contains topology manager settings.
	// +optional
	TopologyManager *NodeTopologyManagerState `json:"topologyManager,omitempty"`
}

type NodeTopologyStatus struct {
	// NodeName is the name of the Kubernetes Node.
	// +optional
	NodeName string `json:"nodeName,omitempty"`

	// NodeGroup is the name of the NodeGroup this node belongs to.
	// +optional
	NodeGroup string `json:"nodeGroup,omitempty"`

	// Desired contains settings expected from NodeGroup.
	// +optional
	Desired *NodeTopologyState `json:"desired,omitempty"`

	// Effective contains settings actually applied on the node.
	// +optional
	Effective *NodeTopologyState `json:"effective,omitempty"`

	// Conditions contains status conditions.
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Cluster
// +kubebuilder:printcolumn:name="NodeGroup",type=string,JSONPath=`.status.nodeGroup`
// +kubebuilder:printcolumn:name="DesiredPolicy",type=string,JSONPath=`.status.desired.topologyManager.policy`
// +kubebuilder:printcolumn:name="EffectivePolicy",type=string,JSONPath=`.status.effective.topologyManager.policy`
// +kubebuilder:printcolumn:name="InSync",type=string,JSONPath=`.status.conditions[?(@.type=="InSync")].status`
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`

type NodeTopology struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Status NodeTopologyStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

type NodeTopologyList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []NodeTopology `json:"items"`
}

func init() {
	SchemeBuilder.Register(&NodeTopology{}, &NodeTopologyList{})
}
