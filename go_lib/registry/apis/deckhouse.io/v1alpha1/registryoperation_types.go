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
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// RegistryOperationType is what an operation does.
//
// +kubebuilder:validation:Enum=GarbageCollection
type RegistryOperationType string

const (
	// OperationGarbageCollection collects the cache: every storage replica removes what the releases
	// the cluster has moved past left in its store.
	OperationGarbageCollection RegistryOperationType = "GarbageCollection"
)

// OperationCancelLabel cancels a RegistryOperation when set to "true".
//
// A label rather than a spec field, because the spec is immutable: an operation says what was asked
// for, and cancelling it is not a different operation. Deleting the object cancels it too, but only
// before a replica has started; the label also reaches a replica that is already running.
const OperationCancelLabel = "registry.deckhouse.io/cancel"

// OperationStartDeadline is how long after acceptance a replica may still be held back by a store that
// is being written to. The first fill of a cache takes twenty to thirty minutes, so an hour covers it;
// a store written to for longer than that is not one to collect now.
//
// Waiting for the other replicas to finish is not charged against it: they collect one at a time, and
// the turn of the last one depends on how long the others take, not on anything wrong with its store.
const OperationStartDeadline = time.Hour

// OperationTimeout is how long after acceptance an operation may stay unfinished at all. It catches a
// replica that stopped answering and that nothing else shows to be gone: one is given up on sooner when
// it stops holding the collection lease while it collects, or when its node is not Ready while it waits.
//
// A day, because a replica that answers can take long without being stuck: the wait for its turn and
// its own collection are bounded apart by the syncer's window of four hours each, and the third of
// three masters may use most of both behind the others. Twelve hours was what three such windows
// already added up to.
const OperationTimeout = 24 * time.Hour

// OperationRetention is how long a finished operation is kept as the record of what was done. It is
// deleted after that, as NodeOperation is.
const OperationRetention = 24 * time.Hour

// Annotations the controller keeps on RegistryStorage for the operations it creates itself. On the
// storage rather than on the operations, because the operations are deleted a day after they finish
// and what these remember must outlive them.
const (
	// ScheduledCollectionAnnotation is the last firing of the garbage collection schedule the
	// controller acted on, in RFC 3339.
	ScheduledCollectionAnnotation = "registry.deckhouse.io/gc-scheduled-at"

	// ScheduleAnnotation is the schedule that firing belongs to. A firing of another schedule is no
	// point to count the next one from: counted from it, a schedule changed at ten to "0 9 * * *"
	// fired at once, for a nine o'clock that had already passed.
	ScheduleAnnotation = "registry.deckhouse.io/gc-schedule"

	// CollectedSetAnnotation is the image set — RegistryStorage.status.replicas[].verifiedSet — the
	// controller last created a garbage collection for.
	CollectedSetAnnotation = "registry.deckhouse.io/gc-collected-set"

	// CollectedReleaseAnnotation is the Deckhouse release that was deployed when that collection was
	// created, which tells an update of the platform from any other change of the set.
	CollectedReleaseAnnotation = "registry.deckhouse.io/gc-collected-release"

	// RefusalCollectedAnnotation is the Deckhouse release a collection was created for because the
	// store refused writes. One such collection per release: a store that a collection could not free
	// is not collected again and again, and an update — or the store accepting writes again — allows
	// the next one.
	RefusalCollectedAnnotation = "registry.deckhouse.io/gc-refusal-release"
)

// RegistryOperationPhase is where a RegistryOperation is.
//
// +kubebuilder:validation:Enum=Pending;Running;Succeeded;Failed;Canceled
type RegistryOperationPhase string

const (
	// OperationPending means no replica has started yet.
	OperationPending RegistryOperationPhase = "Pending"

	// OperationRunning means at least one replica has started and not every one has finished.
	OperationRunning RegistryOperationPhase = "Running"

	// OperationSucceeded means every replica finished the operation.
	OperationSucceeded RegistryOperationPhase = "Succeeded"

	// OperationFailed means the operation was refused, or a replica did not finish it.
	OperationFailed RegistryOperationPhase = "Failed"

	// OperationCanceled means the operation was canceled before every replica finished.
	OperationCanceled RegistryOperationPhase = "Canceled"
)

// Finished reports whether the phase is terminal.
func (p RegistryOperationPhase) Finished() bool {
	switch p {
	case OperationSucceeded, OperationFailed, OperationCanceled:
		return true
	}
	return false
}

// ReplicaOperationPhase is where one replica is with an operation.
//
// +kubebuilder:validation:Enum=Pending;Waiting;Running;Succeeded;Failed;Skipped;Canceled
type ReplicaOperationPhase string

const (
	// ReplicaOperationPending means the replica has not picked the operation up yet.
	ReplicaOperationPending ReplicaOperationPhase = "Pending"

	// ReplicaOperationWaiting means the replica has picked the operation up and is held back; the
	// reason says by what.
	ReplicaOperationWaiting ReplicaOperationPhase = "Waiting"

	// ReplicaOperationRunning means the replica is running the operation.
	ReplicaOperationRunning ReplicaOperationPhase = "Running"

	// ReplicaOperationSucceeded means the replica finished the operation.
	ReplicaOperationSucceeded ReplicaOperationPhase = "Succeeded"

	// ReplicaOperationFailed means the replica did not finish the operation.
	ReplicaOperationFailed ReplicaOperationPhase = "Failed"

	// ReplicaOperationSkipped means the replica left the storage before it started.
	ReplicaOperationSkipped ReplicaOperationPhase = "Skipped"

	// ReplicaOperationCanceled means the operation was canceled before the replica finished.
	ReplicaOperationCanceled ReplicaOperationPhase = "Canceled"
)

// Finished reports whether the phase is terminal.
func (p ReplicaOperationPhase) Finished() bool {
	switch p {
	case ReplicaOperationSucceeded, ReplicaOperationFailed, ReplicaOperationSkipped, ReplicaOperationCanceled:
		return true
	}
	return false
}

// Reasons of a RegistryOperation and of its replicas.
const (
	// ReasonOperationAccepted: the operation is accepted and handed to the replicas.
	ReasonOperationAccepted = "Accepted"

	// ReasonOperationNoCache: the cache is turned off, so there is no store to work on.
	ReasonOperationNoCache = "NoCache"

	// ReasonOperationNoReplicas: no replica of the storage has reported yet.
	ReasonOperationNoReplicas = "NoReplicas"

	// ReasonOperationAnotherActive: an older operation is not finished yet.
	ReasonOperationAnotherActive = "AnotherOperationActive"

	// ReasonOperationInProgress: at least one replica is running the operation.
	ReasonOperationInProgress = "InProgress"

	// ReasonOperationCompleted: every replica finished the operation.
	ReasonOperationCompleted = "Completed"

	// ReasonOperationReplicaFailed: a replica did not finish the operation.
	ReasonOperationReplicaFailed = "ReplicaFailed"

	// ReasonOperationCanceled: the cancel label was set.
	ReasonOperationCanceled = "Canceled"

	// ReasonOperationFilling: the store is being filled, and the replica waits for the fill to finish.
	ReasonOperationFilling = "Filling"

	// ReasonOperationPushing: an image set is being pushed into the store from outside.
	ReasonOperationPushing = "Pushing"

	// ReasonOperationTurn: another replica is running the operation, and this one waits for its turn. Also
	// a replica whose window closed before its turn came, while it waits for the store to go quiet before
	// waiting for the turn again: it was past the store once, and the deadline no longer applies to it.
	ReasonOperationTurn = "Turn"

	// ReasonOperationFailed: the replica ran the operation and it returned an error.
	ReasonOperationFailed = "OperationFailed"

	// ReasonOperationDeadlineExceeded: the store was still being written to at the deadline.
	ReasonOperationDeadlineExceeded = "DeadlineExceeded"

	// ReasonOperationTimedOut: the replica did not report a result in time.
	ReasonOperationTimedOut = "TimedOut"

	// ReasonOperationReplicaLost: the replica is gone while it collected or waited — it stopped holding
	// the collection lease, or its node was not Ready for a while — and did not report a result.
	ReasonOperationReplicaLost = "ReplicaLost"

	// ReasonOperationReplicaGone: the replica left the storage.
	ReasonOperationReplicaGone = "ReplicaGone"
)

// RegistryOperationSpec is what an operation does. Immutable, so that what was asked for stays what
// the status describes.
type RegistryOperationSpec struct {
	// Type is what the operation does.
	Type RegistryOperationType `json:"type"`

	// Cause is why the operation was created, for a person reading it.
	// +optional
	Cause string `json:"cause,omitempty"`
}

// RegistryOperationStatus is the progress of an operation.
//
// Two writers: the controller accepts the operation, names the replicas and derives everything at the
// top level; each replica's syncer writes its own entry in Replicas and nothing else. Both write under
// an optimistic lock, because a merge patch replaces the whole list.
type RegistryOperationStatus struct {
	// Phase is where the operation is.
	// +optional
	Phase RegistryOperationPhase `json:"phase,omitempty"`

	// Reason is a machine-readable reason for the phase.
	// +optional
	Reason string `json:"reason,omitempty"`

	// Message is a human-readable explanation of the reason.
	// +optional
	Message string `json:"message,omitempty"`

	// AcceptedAt is when the controller accepted the operation.
	// +optional
	AcceptedAt *metav1.Time `json:"acceptedAt,omitempty"`

	// Deadline is when a replica held back by a store that is being written to gives up.
	// +optional
	Deadline *metav1.Time `json:"deadline,omitempty"`

	// StartedAt is when the first replica started.
	// +optional
	StartedAt *metav1.Time `json:"startedAt,omitempty"`

	// FinishedAt is when the operation reached a terminal phase. It is deleted a day later.
	// +optional
	FinishedAt *metav1.Time `json:"finishedAt,omitempty"`

	// GarbageCollection sums what the replicas collected. Only for a GarbageCollection.
	// +optional
	GarbageCollection *GarbageCollectionResult `json:"garbageCollection,omitempty"`

	// Replicas is one entry per replica the operation was handed to.
	// +optional
	// +listType=map
	// +listMapKey=node
	Replicas []ReplicaOperationStatus `json:"replicas,omitempty"`
}

// GarbageCollectionResult is what a garbage collection removed.
type GarbageCollectionResult struct {
	// RemovedManifests is how many manifests were deleted by digest — the images the fill writes with
	// no tag at all, which is where nearly all of a store's weight is. Summed over the replicas at the
	// top of the status.
	// +optional
	RemovedManifests int32 `json:"removedManifests,omitempty"`

	// RemovedBlobs is how many blobs the sweep removed, as distribution counts them. Absent when the
	// sweep did not run or did not say. A sweep also removes what an earlier one failed to, so its
	// blobs need not belong to this run's manifests.
	// +optional
	RemovedBlobs *int32 `json:"removedBlobs,omitempty"`

	// RemovedVersions are the Deckhouse releases removed, sorted.
	// +optional
	RemovedVersions []string `json:"removedVersions,omitempty"`

	// ReclaimedBytes is how much the store shrank: its blobs and uploads measured before the
	// collection and after. Absent when it could not be measured.
	// +optional
	ReclaimedBytes *int64 `json:"reclaimedBytes,omitempty"`
}

// ReplicaOperationStatus is one replica's part of an operation.
type ReplicaOperationStatus struct {
	// Node the replica runs on.
	Node string `json:"node"`

	// Phase is where this replica is.
	Phase ReplicaOperationPhase `json:"phase"`

	// Reason is a machine-readable reason for the phase.
	// +optional
	Reason string `json:"reason,omitempty"`

	// Message is a human-readable explanation of the reason.
	// +optional
	Message string `json:"message,omitempty"`

	// StartedAt is when this replica started.
	// +optional
	StartedAt *metav1.Time `json:"startedAt,omitempty"`

	// FinishedAt is when this replica finished.
	// +optional
	FinishedAt *metav1.Time `json:"finishedAt,omitempty"`

	// GarbageCollection is what this replica collected. Only for a GarbageCollection that ran.
	// +optional
	GarbageCollection *GarbageCollectionResult `json:"garbageCollection,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Cluster,path=registryoperations,singular=registryoperation,shortName=rop

// RegistryOperation is an operation every storage replica performs on its store, one replica at a
// time: for now, a garbage collection.
type RegistryOperation struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   RegistryOperationSpec   `json:"spec"`
	Status RegistryOperationStatus `json:"status,omitempty"`
}

// Canceled reports whether the cancel label is set.
func (o *RegistryOperation) Canceled() bool {
	return o.Labels[OperationCancelLabel] == "true"
}

// Replica returns the entry of the replica on node, or nil.
func (s *RegistryOperationStatus) Replica(node string) *ReplicaOperationStatus {
	for i := range s.Replicas {
		if s.Replicas[i].Node == node {
			return &s.Replicas[i]
		}
	}
	return nil
}

// +kubebuilder:object:root=true

// RegistryOperationList is a list of RegistryOperation.
type RegistryOperationList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []RegistryOperation `json:"items"`
}
