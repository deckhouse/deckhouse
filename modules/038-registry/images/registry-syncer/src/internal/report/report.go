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
// Each replica owns exactly one entry in RegistryStorage.status.replicas and touches no other,
// which is what lets several syncers write one status without a coordinator; the controller
// derives the cluster-wide summary from those entries, because no replica can know about the
// others.
//
// "Touches no other" took a lock to make true of the WRITE as well: the entries are one list, a
// merge patch replaces the whole list, so each replica sends every other entry as it last read
// them. Two replicas reporting at once then do not merge — the later write puts back the earlier
// reader's picture, and the leader's `full` can go with it, which is what the air-gap transition
// is gated on. So every write here is one read-modify-write under an optimistic lock, redone
// against the new state when the API server refuses it. See publish.
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

	// Store is this replica's store against its budget and its reserve. Nil when the pass did
	// not measure it, and then the last measurement stays.
	Store *registryv1alpha1.StoreUsage

	// VerifiedSet is the fingerprint of the set Full was found against — see the field of the same
	// name on the CRD. Empty unless Full is.
	VerifiedSet string
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
	// processes into one object, so the conclusion can outlive the fact: `safeToDropUpstream: true`
	// beside a leader entry, in the same object, saying it does not hold the set. Between those two
	// the cluster could be cut off from its upstream on evidence that no longer exists.
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
// Without it the first fill of a store is invisible: a replica publishes at the END of a pass and the
// fill runs inside that pass, so with no replica report the controller calls the storage `Idle` for
// the whole of it and then jumps to `Ready`. The documented `Filling` phase never appears, and an
// operator cannot tell "nothing is happening" from "the cache is filling right now".
//
// Two rules make it safe to write from here. It NEVER overwrites an existing report: a replica that
// has published holds real numbers and an announcement carries none, so replacing one with the other
// would report a full store as empty and, on a leader, withdraw the permission the transition is
// gated on. And it is never completeness — `Full` is forced false whatever the caller passed, because
// the one field the cluster's safety depends on must be earned by reading the store.
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

// Withdraw takes this replica's Full back, because the set it was measured against is no longer the
// set the cluster needs.
//
// Before the pass that verifies the new set, not after it: a verification over a slow link takes
// minutes, and for all of them a full leader would keep authorizing the move to air-gap on a set the
// cluster has stopped needing. So the leader withdraws first — its Full, and the two permissions the
// controller derived from it, in the same write, as Announce and Publish do — and the pass that
// follows earns Full back, or does not.
func (p *Publisher) Withdraw(ctx context.Context, state State) error {
	if state.Node == "" {
		return fmt.Errorf("a withdrawal needs the node it came from")
	}

	return p.publish(ctx, func(storage *registryv1alpha1.RegistryStorage) bool {
		changed := MergeWithdraw(&storage.Status.Replicas, state)
		if state.Role == registryv1alpha1.ReplicaRoleLeader &&
			(storage.Status.SafeToDropUpstream || storage.Status.AllReplicasFull) {
			storage.Status.SafeToDropUpstream = false
			storage.Status.AllReplicasFull = false
			changed = true
		}
		return changed
	})
}

// MergeWithdraw clears Full and VerifiedSet in this replica's entry and leaves every other field
// alone: the counts still say what the store holds, which a withdrawal does not change.
func MergeWithdraw(replicas *[]registryv1alpha1.StorageReplicaStatus, state State) bool {
	for i := range *replicas {
		entry := &(*replicas)[i]
		if entry.Node != state.Node {
			continue
		}
		if !entry.Full && entry.VerifiedSet == "" {
			return false
		}
		entry.Full = false
		entry.VerifiedSet = ""
		return true
	}
	return false
}

// PublishProgress says that a pass is under way and how far it has got, and nothing else.
//
// The pass reports its outcome when it ends, and a fill over a slow link ends an hour later. Until
// then the entry carried the previous pass's words — an error included, which the controller reads
// before anything else — so a fill working normally showed `Failed` with no progress for as long as
// it ran. A pass that is copying has, by that alone, got past whatever stopped the previous one:
// the error is cleared, and the count shows how far it is.
//
// The count only ever goes up here. A pass counts from zero, while the entry may hold what the
// store was measured to hold last time; showing the lower of the two would read as a store that
// emptied. The pass's own report at its end is the one that may say otherwise.
func (p *Publisher) PublishProgress(ctx context.Context, state State) error {
	if state.Node == "" {
		return fmt.Errorf("a progress report needs the node it came from")
	}

	return p.publish(ctx, func(storage *registryv1alpha1.RegistryStorage) bool {
		return MergeProgress(&storage.Status.Replicas, state)
	})
}

// PublishStore records a measurement of the store, and nothing else.
//
// Its own reporter because a pass can take an hour: a fill over a slow link publishes when it
// ends, and a status that only moved then would show a store at zero bytes for the whole of the
// fill that is filling it — while the budget alerts and the StoreWritable condition read it.
func (p *Publisher) PublishStore(ctx context.Context, state State) error {
	if state.Node == "" {
		return fmt.Errorf("a store report needs the node it came from")
	}
	if state.Store == nil {
		return nil
	}

	return p.publish(ctx, func(storage *registryv1alpha1.RegistryStorage) bool {
		return MergeStore(&storage.Status.Replicas, state)
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
		Store:           state.Store,
		VerifiedSet:     state.VerifiedSet,
	}

	// At most one replica may carry the leader's role, and the one publishing it now is the one
	// holding the lease.
	//
	// A replica writes only its own entry, so an entry outlives whatever it last said: a replica that
	// led, lost the lease and then stopped publishing — because its pass fails, or its process is
	// restarting — leaves `role: Leader` behind it forever. Two entries then claim Leader at once,
	// one the actual lease-holder and one a memory of an earlier one. Which is worse than untidy,
	// because everything downstream reads this status to
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
		// A report that did not measure the store — the one announcing a pass, say — keeps the
		// last measurement rather than erasing it, or the status would flap between the two.
		if entry.Store == nil {
			entry.Store = (*replicas)[i].Store
		}

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

// MergeStore records a measurement of the store in this replica's entry, and leaves every other
// field alone — the same rule MergeCollection follows, for the same reason.
func MergeStore(replicas *[]registryv1alpha1.StorageReplicaStatus, state State) bool {
	for i := range *replicas {
		if (*replicas)[i].Node != state.Node {
			continue
		}
		if equality.Semantic.DeepEqual((*replicas)[i].Store, state.Store) {
			return false
		}
		(*replicas)[i].Store = state.Store.DeepCopy()
		return true
	}

	// No entry yet: the first pass has not reported. The entry is created with what is known.
	*replicas = append(*replicas, registryv1alpha1.StorageReplicaStatus{
		Node:    state.Node,
		Role:    state.Role,
		Address: state.Address,
		Store:   state.Store.DeepCopy(),
	})
	return true
}

// MergeProgress records a pass under way in this replica's entry: its error cleared, its count
// raised to what the pass has reached, its denominator filled in if it had none. Every other field is
// left alone — above all Full, which a pass in progress has not yet earned or lost.
func MergeProgress(replicas *[]registryv1alpha1.StorageReplicaStatus, state State) bool {
	for i := range *replicas {
		entry := &(*replicas)[i]
		if entry.Node != state.Node {
			continue
		}

		changed := false
		if entry.Error != "" {
			entry.Error = ""
			changed = true
		}
		if state.VerifiedDigests > entry.VerifiedDigests {
			entry.VerifiedDigests = state.VerifiedDigests
			changed = true
		}
		if entry.DeclaredDigests == 0 && state.DeclaredDigests > 0 {
			entry.DeclaredDigests = state.DeclaredDigests
			changed = true
		}
		if state.Source != "" && entry.Source != state.Source {
			entry.Source = state.Source
			changed = true
		}
		return changed
	}

	// No entry yet: the first pass of this replica, which Announce normally covers.
	*replicas = append(*replicas, registryv1alpha1.StorageReplicaStatus{
		Node:            state.Node,
		Role:            state.Role,
		Address:         state.Address,
		Source:          state.Source,
		VerifiedDigests: state.VerifiedDigests,
		DeclaredDigests: state.DeclaredDigests,
	})
	return true
}
