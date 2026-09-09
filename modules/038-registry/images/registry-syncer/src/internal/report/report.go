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

// Package report publishes what one storage replica actually holds.
//
// Each replica owns exactly one entry in RegistryStorage.status.replicas and
// touches no other, which is what lets several syncers write the same status
// without a coordinator. The controller derives the cluster-wide summary from
// these entries; no replica claims anything about the others, because no replica
// can know.
//
// "Touches no other" is a property of the merge below, and it took a lock to make it a property of
// the WRITE as well. The entries are one list, a merge patch of a list replaces the whole list, so
// each replica sends every other replica's entry as it last read them — and a read taken a moment
// before somebody else's write is indistinguishable from a fresh one. Two replicas reporting at the
// same time then did not merge: the later write put back the earlier reader's picture, and what the
// others published in between was gone. Measured in a test: a leader's `full: true` with 400 verified
// digests reverted to `full: false` with none by a follower's ordinary progress report — which is
// the fact the air-gap transition is gated on, in the direction that leaves it waiting forever, and
// in the other direction cuts a cluster off from an upstream it still needs.
//
// So every write here is one read-modify-write under an optimistic lock: the patch carries the
// resourceVersion it was built from, the API server refuses it if anything changed, and the whole
// sequence is redone against the new state. See publish.
package report

import (
	"context"
	"fmt"
	"time"

	"k8s.io/apimachinery/pkg/api/equality"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/util/retry"
	"sigs.k8s.io/controller-runtime/pkg/client"

	registryv1alpha1 "github.com/deckhouse/deckhouse/go_lib/registry/apis/deckhouse.io/v1alpha1"
)

// State is what a replica has to say about itself.
type State struct {
	// Node this replica runs on, and the key of its entry.
	Node string

	// Role in replication.
	Role registryv1alpha1.ReplicaRole

	// Full reports that this replica holds the whole expected set.
	//
	// The single most consequential field in the whole status: the controller drops
	// the upstream once the leader reports it. It must therefore be derived from
	// what was written or read, never from what the registry claims it can serve.
	Full bool

	// VerifiedDigests is how many digests OF THE SET were confirmed present — the set being what the
	// cluster's releases and kept modules declare. See the field of the same name on the CRD.
	VerifiedDigests int32

	// TotalDigests is how many distinct digests this replica holds altogether, the set included.
	// Reported, never decided from.
	TotalDigests int32

	// DeclaredDigests is the size of the set — what VerifiedDigests is out of. Carried because the
	// controller writes the status and cannot count the set itself. Reported, never decided from.
	DeclaredDigests int32

	// Address is where OTHER replicas reach this one, so a follower can replicate
	// from the leader without resolving a node name.
	Address string

	// Source is the replica this one is filled from. Empty for the leader.
	Source string

	// Error is why the last task failed, and empty when it succeeded. Cleared
	// explicitly on success so a fixed problem stops being reported.
	Error string

	// CollectedAt is when this replica last reclaimed its disk, if it has.
	CollectedAt *time.Time

	// CollectionError is why the last attempt to reclaim did not finish.
	//
	// Separate from Error, which is about filling: a store that cannot be reclaimed still
	// serves every image it holds, so the two say different things about how worried to be.
	CollectionError string
}

// Publisher writes a replica's own entry.
type Publisher struct {
	Client client.Client

	// Name of the RegistryStorage object.
	Name string
}

// publish is one read-modify-write of the status, retried while the object keeps changing under it.
//
// The mutation reports whether it changed anything, and a mutation that changed nothing writes
// nothing: every replica writes this object and the controller watches it, so a needless write
// multiplies into a reconciliation of the whole layout.
//
// A missing storage object is not an error, on any of these paths: the controller may not have
// created it yet, or the cache may have just been turned off, and a syncer crash-looping over that
// would be noise rather than information.
func (p *Publisher) publish(
	ctx context.Context, mutate func(*registryv1alpha1.RegistryStorage) bool,
) error {
	name := p.Name
	if name == "" {
		name = registryv1alpha1.SingletonName
	}

	return retry.RetryOnConflict(retry.DefaultRetry, func() error {
		storage := &registryv1alpha1.RegistryStorage{}
		if err := p.Client.Get(ctx, types.NamespacedName{Name: name}, storage); err != nil {
			return client.IgnoreNotFound(err)
		}

		// The lock is the point: this patch carries the list of every replica as this read saw
		// it, so it must be refused if that is no longer what the object holds. Refused, the
		// sequence starts again from a fresh read — which is why the mutation is a function
		// rather than something done once outside the loop.
		patch := client.MergeFromWithOptions(storage.DeepCopy(), client.MergeFromWithOptimisticLock{})

		if !mutate(storage) {
			return nil
		}

		return p.Client.Status().Patch(ctx, storage, patch)
	})
}

// Publish merges the replica's state into the status, leaving every other entry
// alone.
//
// A missing storage object is not an error: the controller may not have created it
// yet, or the cache may have just been turned off, and a syncer crash-looping over
// that would be noise rather than information.
func (p *Publisher) Publish(ctx context.Context, state State) error {
	if state.Node == "" {
		return fmt.Errorf("a replica report needs the node it came from")
	}

	return p.publish(ctx, func(storage *registryv1alpha1.RegistryStorage) bool {
		return p.mergeReport(storage, state)
	})
}

// mergeReport is what one report changes about the status, as a function of the state it is
// merged into: the read it is applied to may be the second or third attempt.
func (p *Publisher) mergeReport(storage *registryv1alpha1.RegistryStorage, state State) bool {
	// A safety interlock, not co-ownership of the field.
	//
	// `safeToDropUpstream` is the controller's: it derives it from the leader's report and the
	// transition is gated on it. But the derivation and the report are written by different
	// processes into one object, so the conclusion outlives the fact — measured on a cluster as
	// `safeToDropUpstream: true` while the leader's own entry, in the same object, said it did not
	// hold the set. Between those two the cluster could be cut off from its upstream on evidence
	// that no longer existed.
	//
	// So a leader that is not full withdraws the permission as it reports. Only ever withdraws:
	// granting it stays the controller's, which is what keeps one decision in one place.
	withdrawn := false
	if state.Role == registryv1alpha1.ReplicaRoleLeader && !state.Full &&
		(storage.Status.SafeToDropUpstream || storage.Status.AllReplicasFull) {
		storage.Status.SafeToDropUpstream = false
		storage.Status.AllReplicasFull = false
		withdrawn = true
	}

	return Merge(&storage.Status.Replicas, state) || withdrawn
}

// Announce records that this replica has STARTED filling, before any of it is done.
//
// Without it the first fill of a store is invisible. A replica publishes at the END of a pass and the
// fill runs inside that same pass, so while gigabytes are moving there is no replica report at all —
// and the controller, seeing none, reports the storage as `Idle`. Measured on a store emptied and
// refilled: nine minutes of `phase: Idle` beside a condition that said `FillInProgress`, more than
// 1500 blobs written, and then a jump straight to `Ready`. The documented `Filling` phase never
// appeared, so an operator watching the phase after enabling the cache could not tell "nothing is
// happening" from "the cache is filling right now".
//
// Two rules make this safe to write from here.
//
// It NEVER overwrites an existing report. A replica that has already published holds real numbers,
// and an announcement carries none — replacing one with the other would report a full store as empty
// and, on a leader, withdraw the permission the transition is gated on. So this is create-only: it
// speaks exactly once per replica, on the pass where the alternative is silence.
//
// And it is never completeness: `Full` is forced false whatever the caller passed. An announcement is
// a statement that work has begun, and the one field the cluster's safety depends on must be earned
// by reading the store, not asserted before the work.
func (p *Publisher) Announce(ctx context.Context, state State) error {
	if state.Node == "" {
		return fmt.Errorf("a replica announcement needs the node it came from")
	}

	state.Full = false

	return p.publish(ctx, func(storage *registryv1alpha1.RegistryStorage) bool {
		for i := range storage.Status.Replicas {
			if storage.Status.Replicas[i].Node == state.Node {
				// Already speaking for itself. Nothing to announce and nothing to overwrite.
				// Asked on every attempt, not once before them: a retry means somebody else
				// wrote, and that somebody may have been this replica's own report.
				return false
			}
		}

		// The same interlock Publish carries, for the same reason: a leader saying it is not
		// full must take back a permission derived from an earlier report, never leave it
		// standing on a fact that has stopped being one.
		withdrawn := false
		if state.Role == registryv1alpha1.ReplicaRoleLeader &&
			(storage.Status.SafeToDropUpstream || storage.Status.AllReplicasFull) {
			storage.Status.SafeToDropUpstream = false
			storage.Status.AllReplicasFull = false
			withdrawn = true
		}

		return Merge(&storage.Status.Replicas, state) || withdrawn
	})
}

// PublishCollection records the outcome of a garbage collection.
func (p *Publisher) PublishCollection(ctx context.Context, state State) error {
	if state.Node == "" {
		return fmt.Errorf("a collection report needs the node it came from")
	}

	return p.publish(ctx, func(storage *registryv1alpha1.RegistryStorage) bool {
		return MergeCollection(&storage.Status.Replicas, state)
	})
}

// Merge replaces this replica's entry in place, or appends it, and reports whether
// anything changed.
//
// Split out from the client work so the merge semantics — the part that must not
// disturb other replicas — are testable on their own.
func Merge(replicas *[]registryv1alpha1.StorageReplicaStatus, state State) bool {
	changedOther := false

	entry := registryv1alpha1.StorageReplicaStatus{
		Node:            state.Node,
		Role:            state.Role,
		Full:            state.Full,
		VerifiedDigests: state.VerifiedDigests,
		DeclaredDigests: state.DeclaredDigests,
		TotalDigests:    state.TotalDigests,
		Address:         state.Address,
		Source:          state.Source,
		Error:           state.Error,
	}

	// At most one replica may carry the leader's role, and the one publishing it now is the one
	// holding the lease.
	//
	// A replica writes only its own entry, so an entry outlives whatever it last said: a replica that
	// led, lost the lease and then stopped publishing — because its pass fails, or its process is
	// restarting — leaves `role: Leader` behind it forever. Measured on a cluster: two entries
	// claiming Leader at once, one of them the actual lease-holder and one of them a memory of an
	// earlier one. Which is worse than untidy, because everything downstream reads this status to
	// find the leader, including the followers deciding what to replicate from.
	//
	// Demoted rather than deleted: the stale entry still says truthfully how much that replica held,
	// and it will correct the rest of itself the moment it publishes again.
	if state.Role == registryv1alpha1.ReplicaRoleLeader {
		for i := range *replicas {
			if (*replicas)[i].Node != state.Node &&
				(*replicas)[i].Role == registryv1alpha1.ReplicaRoleLeader {
				(*replicas)[i].Role = registryv1alpha1.ReplicaRoleFollower
				changedOther = true
			}
		}
	}

	for i := range *replicas {
		if (*replicas)[i].Node != state.Node {
			continue
		}

		// The garbage collection reports on its own schedule and owns two fields of this
		// entry. Rebuilding the entry from a fill report would erase them, and the two
		// reporters would take turns wiping each other — which looks like a status that
		// flaps for no reason.
		entry.CollectedAt = (*replicas)[i].CollectedAt
		entry.CollectionError = (*replicas)[i].CollectionError

		if equality.Semantic.DeepEqual((*replicas)[i], entry) {
			return changedOther
		}
		(*replicas)[i] = entry
		return true
	}

	*replicas = append(*replicas, entry)
	return true
}

// MergeCollection records the outcome of a garbage collection, and nothing else.
//
// The mirror of the rule above: this reporter owns two fields and must leave every other one
// alone. A replica that had just finished filling would otherwise be reported as empty by the
// next collection.
func MergeCollection(replicas *[]registryv1alpha1.StorageReplicaStatus, state State) bool {
	collectedAt := (*metav1.Time)(nil)
	if state.CollectedAt != nil {
		stamp := metav1.NewTime(*state.CollectedAt)
		collectedAt = &stamp
	}

	for i := range *replicas {
		if (*replicas)[i].Node != state.Node {
			continue
		}

		if equality.Semantic.DeepEqual((*replicas)[i].CollectedAt, collectedAt) &&
			(*replicas)[i].CollectionError == state.CollectionError {
			return false
		}
		(*replicas)[i].CollectedAt = collectedAt
		(*replicas)[i].CollectionError = state.CollectionError
		return true
	}

	// No entry yet: a replica that collected before it ever reported a fill. The entry is
	// created with what is known, which is the node and the collection.
	*replicas = append(*replicas, registryv1alpha1.StorageReplicaStatus{
		Node:            state.Node,
		Role:            state.Role,
		Address:         state.Address,
		CollectedAt:     collectedAt,
		CollectionError: state.CollectionError,
	})
	return true
}
