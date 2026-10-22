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
	"cmp"
	"slices"
	"strings"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

// NodeOperationNodeNameField selects the operations of one node. The CRD
// declares it in selectableFields, and node-controller indexes its cache by it.
const NodeOperationNodeNameField = "spec.nodeName"

const (
	// NodeOperationMachineUIDLabel marks the Drain node-manager creates when a
	// Cluster API Machine is deleted. The value is the UID of that Machine.
	// Only node-controller may set it, the admission policy enforces that.
	NodeOperationMachineUIDLabel = "node-manager.deckhouse.io/machine-uid"
	// NodeOperationMachineAnnotation names that Machine for the messages.
	NodeOperationMachineAnnotation = "node-manager.deckhouse.io/machine"
	// NodeOperationUnlimitedDrainAnnotation marks the Drain of a deleted machine
	// of a Static or CloudStatic group without nodeDrainTimeoutSecond. Cluster API
	// drained such a machine without a time limit, and so does this Drain. It is
	// read only on an operation with NodeOperationMachineUIDLabel.
	NodeOperationUnlimitedDrainAnnotation = "node-manager.deckhouse.io/drain-without-time-limit"

	// NodeOperationConditionEvictionBlocked is set by the draining controller while
	// the eviction it runs for the operation is refused. The message has the
	// refusals as the eviction reports them; the condition goes once they stop.
	NodeOperationConditionEvictionBlocked = "EvictionBlocked"
)

// NodeOperation is one interruption of a node's work: a reboot, an eviction of
// its workload, or the disruption a node needs to apply a NodeConfig revision.
// A record of intent: who asked, for what, and how it ended. The operations of
// a deleted Cluster API Machine carry NodeOperationMachineUIDLabel.
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
	// node-manager also creates one when a Cluster API Machine is deleted,
	// before the machine is removed.
	NodeOperationTypeDrain NodeOperationType = "Drain"
	// NodeOperationTypeDisruption interrupts the node so that it can apply a
	// NodeConfig revision that needs it. node-manager creates it, and it waits
	// for approval.
	NodeOperationTypeDisruption NodeOperationType = "Disruption"
)

// NodeOperationSpec is immutable except Approved: a different intent is a
// different operation. No CRD is generated from this file — crds/nodeoperation.yaml is written by
// hand and is the authority on validation; kubebuilder markers here would drift.
type NodeOperationSpec struct {
	// Type is the operation to perform.
	Type NodeOperationType `json:"type"`

	// NodeName is the node the operation applies to.
	NodeName string `json:"nodeName"`

	// ConfigGeneration is the NodeConfig revision a Disruption covers.
	// The permission is deliberately narrow: a configuration published
	// afterwards asks for its own.
	// +optional
	ConfigGeneration *int64 `json:"configGeneration,omitempty"`

	// NodeConfigUID is the NodeConfig object a Disruption covers: a
	// recreated NodeConfig counts its generation from 1 again.
	// +optional
	NodeConfigUID types.UID `json:"nodeConfigUID,omitempty"`

	// Approved lets a Disruption go ahead. node-manager sets it on creation in any
	// approval mode but Manual. In Manual an operator does. It only goes false -> true.
	// +optional
	Approved *bool `json:"approved,omitempty"`

	// Drain controls the eviction that precedes the interruption.
	// +optional
	Drain *NodeOperationDrainSpec `json:"drain,omitempty"`

	// Cause is why the node is interrupted. node-manager fills it for a
	// Disruption from the node's request, and for the Drain of a deleted
	// machine with the machine's name. An operator may set it on the others.
	// +optional
	Cause string `json:"cause,omitempty"`
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
	// NodeOperationPhasePending is queued but not started: waiting for approval
	// or just created.
	NodeOperationPhasePending NodeOperationPhase = "Pending"
	// NodeOperationPhaseApproved means the operation may go ahead and the node
	// is being prepared for it. A Drain never has this phase.
	NodeOperationPhaseApproved NodeOperationPhase = "Approved"
	// NodeOperationPhaseInProgress means the eviction was requested (Drain) or
	// the node was prepared and may carry the operation out (the other types).
	NodeOperationPhaseInProgress NodeOperationPhase = "InProgress"
	// NodeOperationPhaseCompleted means it finished successfully. A Disruption
	// completes when the node releases it, with its revision applied or not.
	NodeOperationPhaseCompleted NodeOperationPhase = "Completed"
	// NodeOperationPhaseFailed means it did not.
	NodeOperationPhaseFailed NodeOperationPhase = "Failed"
)

// NodeOperationStatus is written by node-manager. The on-node agent writes only
// its NodeReported condition, and node-manager derives the phase from it.
type NodeOperationStatus struct {
	// Phase is the current phase of the operation.
	// +optional
	Phase NodeOperationPhase `json:"phase,omitempty"`

	// Reason is the CamelCase cause of the current phase.
	// +optional
	Reason string `json:"reason,omitempty"`

	// Message explains Reason to a human.
	// +optional
	Message string `json:"message,omitempty"`

	// Deadline is when the current stretch of the operation times out; unset
	// while the operation waits for approval and once it has finished.
	// +optional
	Deadline *metav1.Time `json:"deadline,omitempty"`

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

	// BootID is the node's bootID when the reboot started. The reboot is over
	// once the node runs with another one and is Ready.
	// +optional
	BootID string `json:"bootID,omitempty"`

	// DrainDeadline is when the requested eviction runs out of time. Pinned at
	// request time from the group's drain timeout as it stood then — the bound
	// the draining controller pinned into its own context — never re-derived.
	// +optional
	DrainDeadline *metav1.Time `json:"drainDeadline,omitempty"`

	// Conditions are Drained, EvictionBlocked and Retry (node-manager) and
	// NodeReported (the node).
	// +optional
	// +listType=map
	// +listMapKey=type
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// +kubebuilder:object:root=true
type NodeOperationList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []NodeOperation `json:"items"`
}

func init() {
	SchemeBuilder.Register(&NodeOperation{}, &NodeOperationList{})
}

// FailedDisruptions returns the failed attempts of one NodeConfig revision oldest first, the
// order retention collects them in: by NodeOperationFinishedAt, then by name. Both controllers
// count and name the attempts by it, so they agree on which one is the newest.
func FailedDisruptions(ops []NodeOperation, nodeConfigUID types.UID, generation int64) []*NodeOperation {
	var failed []*NodeOperation
	for i := range ops {
		op := &ops[i]
		if op.Status.Phase == NodeOperationPhaseFailed && DisruptionOf(op, nodeConfigUID, generation) {
			failed = append(failed, op)
		}
	}
	slices.SortFunc(failed, func(a, b *NodeOperation) int {
		return cmp.Or(NodeOperationFinishedAt(a).Compare(NodeOperationFinishedAt(b)), strings.Compare(a.Name, b.Name))
	})
	return failed
}

// DisruptionOf reports whether op is a Disruption of the NodeConfig revision. Generation alone
// is not identity: a recreated NodeConfig counts from 1 again.
func DisruptionOf(op *NodeOperation, nodeConfigUID types.UID, generation int64) bool {
	if op.Spec.Type != NodeOperationTypeDisruption || op.Spec.NodeConfigUID != nodeConfigUID {
		return false
	}
	return op.Spec.ConfigGeneration != nil && *op.Spec.ConfigGeneration == generation
}

// NodeOperationFinishedAt is when an operation ended. The stamp is written a moment after the
// phase, so an end not stamped yet counts from the creation rather than from nothing.
func NodeOperationFinishedAt(op *NodeOperation) time.Time {
	if op.Status.FinishedAt != nil {
		return op.Status.FinishedAt.Time
	}
	return op.CreationTimestamp.Time
}
