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

// RegistryStorageSpec tells the storage how to operate.
//
// Written by the controller, read by the syncer sidecar, which reconfigures
// distribution on the fly. This is the path that makes a credential or
// certificate change work while the Deckhouse operator is down: it never goes
// through a helm re-render.
//
// It is a real spec rather than a status-only mirror because distribution needs
// to be told what to do, not merely observed.
type RegistryStorageSpec struct {
	// Upstream is the primary upstream projected by the controller. Set means a
	// pass-through cache. Absent means the cache is authoritative (air-gap).
	//
	// The controller removes this only once the leader is full, so nodes are
	// never left without a source of images.
	// +optional
	Upstream *Upstream `json:"upstream,omitempty"`

	// Source is the image set that must end up in the storage.
	// +optional
	Source *StorageSource `json:"source,omitempty"`

	// Store is where distribution keeps its blobs.
	// +optional
	Store StorageStore `json:"store,omitempty"`

	// Publish exposes an authenticated write endpoint (Ingress) for
	// `d8 mirror push`. Write credentials are separate from read credentials.
	// +optional
	Publish bool `json:"publish,omitempty"`

	// AirGapRequested says the configuration asked to run without an upstream, whether or not the
	// upstream has actually been dropped yet.
	//
	// Its own field because `Publish` used to carry this meaning as a side effect, and the two have
	// come apart: the write endpoint is now a separate instance that exists on every managed cluster,
	// while THIS is the transition window — air-gap asked for, the upstream still held so the cluster
	// keeps working, images arriving through `d8 mirror push` that the syncer never sees and therefore
	// cannot count.
	//
	// What depends on it is how completeness is measured, and that is the whole reason to keep it
	// separate: reading the catalogue is only honest while nothing can put images in the store by
	// itself. A pass-through cache fills its own store on every miss, so a catalogue read there would
	// count images the cluster fetched rather than images it holds on purpose — completeness would
	// arrive by accident. See StoreIsAuthority in the syncer.
	// +optional
	AirGapRequested bool `json:"airGapRequested,omitempty"`

	// Auth is the bearer token guarding the storage. Authentication is never
	// disabled, not even in air-gap where the cache only serves reads.
	// +optional
	Auth string `json:"auth,omitempty"`

	// NeedSync is how the controller hands the syncer a fill or replication
	// task.
	// +optional
	NeedSync bool `json:"needSync,omitempty"`

	// GarbageCollection is when and whether a replica reclaims the disk taken by releases
	// the cluster has moved past.
	//
	// Needed because nothing else ever removes anything: every release adds a slice of the
	// repository, and a cluster that lives for years fills its store and then stops being
	// able to pull.
	// +optional
	GarbageCollection *GarbageCollection `json:"garbageCollection,omitempty"`
}

// StorageStore is the on-disk location of the blobs.
type StorageStore struct {
	// Path on the node. A module constant, not user input.
	// +optional
	Path string `json:"path,omitempty"`

	// Size of the persistent volume, taken from ModuleConfig.
	// +optional
	Size string `json:"size,omitempty"`
}

// GarbageCollection is when a replica reclaims its disk.
type GarbageCollection struct {
	// Enabled is whether replicas collect at all. Off leaves the store to grow, which is
	// what a cluster with a very large disk and a very cautious operator may want.
	//
	// Serialized even when false, which is why it carries no `omitempty`: collection is on
	// unless it was explicitly turned off, so an absent key and `false` mean opposite things
	// to a reader while looking identical. An operator who has just turned it off would see a
	// `garbageCollection` block with only a schedule in it, and no way to tell their decision
	// from a default — which is the same reading that made two air-gap gates in the status look
	// unimplemented when they were merely false.
	Enabled bool `json:"enabled"`

	// Schedule is a five-field cron expression, in the replica's own time zone.
	//
	// A schedule and not an interval, because collecting puts a replica read-only for the
	// duration: it keeps serving every image it holds, but cannot store the result of a
	// cache miss and cannot accept a `d8 mirror push`. Neither is an outage, and both
	// belong at an hour the operator chose.
	// +optional
	Schedule string `json:"schedule,omitempty"`
}

// RegistryStorageStatus is the fact, aggregated by the controller from the
// syncer reports.
type RegistryStorageStatus struct {
	// ObservedGeneration is the spec generation the controller last acted on.
	// +optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`

	// Authoritative means the upstream has been dropped and the cache is now the
	// only source of images.
	// +optional
	Authoritative bool `json:"authoritative,omitempty"`

	// Phase is a coarse summary for humans.
	// +optional
	Phase StoragePhase `json:"phase,omitempty"`

	// Fill is the progress of the current fill task.
	// +optional
	Fill *FillProgress `json:"fill,omitempty"`

	// SafeToDropUpstream means the LEADER holds the expected set, which is the
	// gate for going air-gap. Deliberately the leader alone and not every
	// replica: followers are filled ahead of time and in parallel, so waiting
	// for all of them would delay the transition without adding safety.
	//
	// Serialized even when false, which is why it carries no `omitempty`: on a
	// bool that option erases the answer instead of shortening it, and "no, it is
	// not safe to drop the upstream" is the answer an operator most needs to see.
	// Absent, it reads as a field the implementation does not have — which is how
	// it was read on a live cluster — and the question "can this cluster go
	// air-gap yet" then has no visible answer at all, only a missing one.
	SafeToDropUpstream bool `json:"safeToDropUpstream"`

	// AllReplicasFull means every replica holds the expected set, so losing the
	// leader costs nothing.
	//
	// Serialized even when false, for the reason above: false here is what says a
	// replica loss is not yet survivable, and that has to be visible rather than
	// inferred from an absence.
	AllReplicasFull bool `json:"allReplicasFull"`

	// Leader is the node currently holding the storage leader lease.
	// +optional
	Leader string `json:"leader,omitempty"`

	// Replicas is the per-replica fill state. This is the ONLY place storage
	// completeness is reported; RegistryNode deliberately carries none of it,
	// since completeness is a property of the storage and not of a node.
	// +optional
	// +listType=map
	// +listMapKey=node
	Replicas []StorageReplicaStatus `json:"replicas,omitempty"`

	// Conditions carry the detailed state.
	// +optional
	// +listType=map
	// +listMapKey=type
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// FillProgress is how much of the expected set is in place.
type FillProgress struct {
	// Filled is the number of digests written so far.
	//
	// Serialized even at zero: this whole struct is a pointer, absent until a fill
	// task exists, so once it is here the numbers are the report. `filled: 0` is
	// what the beginning of a fill looks like, and hiding it turns "started,
	// nothing copied yet" into a progress report with no progress field —
	// indistinguishable, to a reader, from a field that was never implemented.
	Filled int32 `json:"filled"`

	// Total is the number of digests expected, serialized for the same reason.
	Total int32 `json:"total"`
}

// StorageReplicaStatus is one replica's fill state.
type StorageReplicaStatus struct {
	// Node the replica runs on.
	Node string `json:"node"`

	// Role in replication.
	// +optional
	Role ReplicaRole `json:"role,omitempty"`

	// Full means this replica holds the expected set.
	//
	// Derived from what the syncer actually WROTE, never from probing the serve
	// path: on a miss a pass-through cache would fetch the image from the
	// upstream and report success on an empty store. In air-gap self-filling is
	// impossible, so reading the catalogue and comparing against
	// Source.ExpectedDigests is honest there.
	// +optional
	Full bool `json:"full,omitempty"`

	// VerifiedDigests is how many digests OF THE SET were confirmed present.
	//
	// Of the set, and only of it: the set is what the cluster's releases and kept modules declare —
	// the digests deckhouse-controller knows about — so this number is directly comparable with
	// `Full` and is the one the air-gap gate is derived from. Anything else the store happens to hold
	// is counted in TotalDigests instead.
	// +optional
	VerifiedDigests int32 `json:"verifiedDigests,omitempty"`

	// TotalDigests is how many distinct digests this replica holds ALTOGETHER.
	//
	// A different population from the one above, kept as its own field rather than folded into it,
	// because conflating the two is what made the status unreadable: `333/556` beside `full: true`
	// invited the reading that a transition had been authorised while a third of the images were
	// missing. Nothing is decided from this number — it is there so an operator can see the store's
	// actual size next to the part of it the cluster needs, and so that a bundle pushed with more than
	// this cluster runs looks like what it is rather than like a discrepancy.
	// +optional
	TotalDigests int32 `json:"totalDigests,omitempty"`

	// DeclaredDigests is how big the set IS — the number VerifiedDigests is out of.
	//
	// The third population, and the one nothing else could supply: the set is what the cluster's
	// releases and kept modules declare, and only the replica computing it knows its size. The
	// controller owns this object's status but has no way to count that set, so before this field
	// existed the only denominator available to it was `spec.source.expectedDigests` — which an
	// operator states when declaring air-gap and nobody states otherwise. A cluster with an upstream
	// therefore reported `filled: 0, total: 0` through the whole of its longest phase, with gigabytes
	// moving: measured on the static stand, 11.7 GiB copied against `0/0`.
	//
	// Reported, never decided from. Completeness stays `Full`, derived by the replica from the same
	// reading, and the air-gap gate stays on `expectedDigests` — an operator's declaration is not
	// replaced by a number the cluster computes about itself.
	// +optional
	DeclaredDigests int32 `json:"declaredDigests,omitempty"`

	// Address is where this replica answers, as "host:port".
	//
	// Reported by the replica itself so that a follower can replicate from the
	// leader without having to resolve a node name into an address, and without the
	// syncers needing to read Node objects at all.
	// +optional
	Address string `json:"address,omitempty"`

	// Source is the replica this one is filled from. Empty for the leader.
	// +optional
	Source string `json:"source,omitempty"`

	// Error is why the last fill or verification task failed, as reported by this
	// replica's syncer. Empty when the last task succeeded.
	//
	// Safety does not depend on this field: a failed replica is simply not Full,
	// and the upstream is only ever dropped once the leader IS full. It exists so
	// that an operator can tell "still filling" from "broken", which the fill
	// counters alone cannot express.
	// +optional
	Error string `json:"error,omitempty"`

	// CollectedAt is when this replica last reclaimed its disk.
	//
	// Absent until it has. Reported per replica because each collects its own store, one at
	// a time: a cluster where one replica has not collected for a month while the others
	// have is a different problem from one where none has.
	// +optional
	CollectedAt *metav1.Time `json:"collectedAt,omitempty"`

	// CollectionError is why the last attempt did not finish, empty when it did.
	//
	// Separate from Error, which is about filling. A store that cannot be reclaimed still
	// serves every image it holds, so the two say different things about how worried to be.
	// +optional
	CollectionError string `json:"collectionError,omitempty"`
}

// LeaderReplica returns the leader's status, or nil when no leader is reported.
func (s *RegistryStorageStatus) LeaderReplica() *StorageReplicaStatus {
	for i := range s.Replicas {
		if s.Replicas[i].Role == ReplicaRoleLeader {
			return &s.Replicas[i]
		}
	}
	return nil
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Cluster,path=registrystorages,singular=registrystorage,shortName=rstor

// RegistryStorage is how the in-cluster registry storage operates and what
// state it is actually in.
type RegistryStorage struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   RegistryStorageSpec   `json:"spec,omitempty"`
	Status RegistryStorageStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// RegistryStorageList is a list of RegistryStorage.
type RegistryStorageList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []RegistryStorage `json:"items"`
}
