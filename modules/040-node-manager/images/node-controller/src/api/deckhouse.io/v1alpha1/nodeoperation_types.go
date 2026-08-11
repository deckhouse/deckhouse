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

// NodeOperationNodeLabel names the node a NodeOperation is for, so one node's
// operations can be listed without reading everyone else's. Shared between the
// creating and reconciling controllers so the lookup contract cannot drift.
const NodeOperationNodeLabel = "node-manager.deckhouse.io/node"

// NodeOperation is one interruption of a node's work: a reboot, an eviction of
// its workload, or the permission a node needs before applying a disruptive
// configuration. A record of intent: who asked, for what, and how it ended.
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
	// NodeOperationTypeReboot reboots the node.
	NodeOperationTypeReboot NodeOperationType = "Reboot"
	// NodeOperationTypeDrain evicts the workload and leaves the node unschedulable.
	NodeOperationTypeDrain NodeOperationType = "Drain"
	// NodeOperationTypeApproveDisruption allows the node to apply a configuration
	// whose application interrupts it.
	NodeOperationTypeApproveDisruption NodeOperationType = "ApproveDisruption"
)

// NodeOperationSpec is immutable: a different intent is a different operation.
// No CRD is generated from this file — crds/nodeoperation.yaml is written by
// hand and is the authority on validation; kubebuilder markers here would drift.
type NodeOperationSpec struct {
	// Type is the operation to perform.
	Type NodeOperationType `json:"type"`

	// NodeName is the node the operation applies to.
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
	// NodeOperationPhasePending is queued but not started.
	NodeOperationPhasePending NodeOperationPhase = "Pending"
	// NodeOperationPhaseInProgress means the node was prepared and is carrying the
	// operation out.
	NodeOperationPhaseInProgress NodeOperationPhase = "InProgress"
	// NodeOperationPhaseCompleted means it finished successfully.
	NodeOperationPhaseCompleted NodeOperationPhase = "Completed"
	// NodeOperationPhaseFailed means it did not.
	NodeOperationPhaseFailed NodeOperationPhase = "Failed"
)

// NodeOperationStatus is written by whoever is moving the operation forward:
// node-manager prepares the node, the on-node agent carries the work out.
type NodeOperationStatus struct {
	// Phase is the current phase of the operation.
	// +optional
	Phase NodeOperationPhase `json:"phase,omitempty"`

	// ObservedGeneration is the generation of the spec this status reflects.
	// +optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`

	// StartedAt is when the node was handed the operation; the wait for the
	// node is measured from here rather than from a condition timestamp, which
	// would still be the moment the operation was queued.
	// +optional
	StartedAt *metav1.Time `json:"startedAt,omitempty"`

	// FinishedAt is when the operation reached a terminal phase. A finished
	// operation is kept as the record of what was done to the node and is
	// collected once it is old enough, which is measured from here.
	// +optional
	FinishedAt *metav1.Time `json:"finishedAt,omitempty"`

	// NodeWasUnschedulable is whether the node was already out of the scheduler
	// when the operation reached it; release restores this rather than forcing
	// schedulable, so an operator's cordon is not quietly undone.
	// +optional
	NodeWasUnschedulable *bool `json:"nodeWasUnschedulable,omitempty"`

	// DrainDeadline is when the requested eviction runs out of time. Pinned at
	// request time from the group's drain timeout as it stood then — the bound
	// the draining controller pinned into its own context — never re-derived.
	// +optional
	DrainDeadline *metav1.Time `json:"drainDeadline,omitempty"`

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
