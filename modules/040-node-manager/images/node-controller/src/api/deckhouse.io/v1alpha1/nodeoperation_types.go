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

package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// NodeOperation is one interruption of a node's work: a reboot, an eviction of
// its workload, or the permission a node needs before applying a configuration
// it cannot apply without a pause.
//
// It is a record of intent rather than a switch — created once, carried through
// its phases, and kept afterwards as the history of what was done to the node.
// That is what an annotation could not be: it says who asked, for what, and how
// it ended.
//
// +kubebuilder:object:root=true
// +kubebuilder:resource:scope=Cluster,shortName=nop
// +kubebuilder:subresource:status
type NodeOperation struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   NodeOperationSpec   `json:"spec"`
	Status NodeOperationStatus `json:"status,omitempty"`
}

// NodeOperationType is what the operation does to the node.
type NodeOperationType string

const (
	// NodeOperationReboot reboots the node.
	NodeOperationReboot NodeOperationType = "Reboot"
	// NodeOperationDrain evicts the workload and leaves the node unschedulable.
	NodeOperationDrain NodeOperationType = "Drain"
	// NodeOperationApproveDisruption allows the node to apply a configuration
	// whose application interrupts it.
	NodeOperationApproveDisruption NodeOperationType = "ApproveDisruption"
)

// NodeOperationSpec is immutable: an operation describes one intent, and a
// different intent is a different operation.
type NodeOperationSpec struct {
	// Type is the operation to perform.
	// +kubebuilder:validation:Enum=Reboot;Drain;ApproveDisruption
	Type NodeOperationType `json:"type"`

	// NodeName is the node the operation applies to.
	// +kubebuilder:validation:MinLength=1
	NodeName string `json:"nodeName"`

	// ConfigGeneration is the NodeConfig revision an ApproveDisruption covers.
	// The permission is deliberately narrow: a configuration published
	// afterwards asks for its own.
	// +optional
	ConfigGeneration *int64 `json:"configGeneration,omitempty"`

	// Drain controls the eviction that precedes the interruption.
	// +optional
	Drain *NodeOperationDrainSpec `json:"drain,omitempty"`
}

// NodeOperationDrainSpec controls whether the workload leaves before the node
// is interrupted.
type NodeOperationDrainSpec struct {
	// Skip interrupts the node without evicting its workload, which is then cut
	// off rather than moved.
	// +optional
	Skip bool `json:"skip,omitempty"`
}

// NodeOperationPhase is where the operation is in its life.
type NodeOperationPhase string

const (
	// NodeOperationPending is queued but not started.
	NodeOperationPending NodeOperationPhase = "Pending"
	// NodeOperationInProgress means the node was prepared and is carrying the
	// operation out.
	NodeOperationInProgress NodeOperationPhase = "InProgress"
	// NodeOperationCompleted means it finished successfully.
	NodeOperationCompleted NodeOperationPhase = "Completed"
	// NodeOperationFailed means it did not.
	NodeOperationFailed NodeOperationPhase = "Failed"
)

// NodeOperationStatus is written by whoever is moving the operation forward:
// node-manager prepares the node, the on-node agent carries the work out.
type NodeOperationStatus struct {
	// Phase is the current phase of the operation.
	// +optional
	// +kubebuilder:validation:Enum=Pending;InProgress;Completed;Failed
	Phase NodeOperationPhase `json:"phase,omitempty"`

	// ObservedGeneration is the generation of the spec this status reflects.
	// +optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`

	// StartedAt is when the node was handed the operation. The wait for the
	// node is measured from here rather than from a condition timestamp, which
	// only moves when the condition's status changes and would still be the
	// moment the operation was queued.
	// +optional
	StartedAt *metav1.Time `json:"startedAt,omitempty"`

	// Conditions carry the details of the operation's progress.
	// +optional
	// +listType=map
	// +listMapKey=type
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// NodeOperationList is a list of NodeOperation objects.
//
// +kubebuilder:object:root=true
type NodeOperationList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []NodeOperation `json:"items"`
}

func init() {
	SchemeBuilder.Register(&NodeOperation{}, &NodeOperationList{})
}
