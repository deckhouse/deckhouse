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
	"unicode/utf8"

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
	// explicitly on success so a fixed problem stops being reported. Also what the
	// health watch found wrong with the registry or the store, while the last task
	// did not fail — see PublishHealth.
	Error string

	// HealthError is what the health watch found wrong with the registry or the store, and
	// nothing else. Written by PublishHealth alone; every other report keeps the one standing.
	//
	// Apart from Error because the other replicas read the two differently: a pass's error may
	// be about a copy from a leader that is gone, and says nothing about whether this replica
	// can serve — see run.MayLead.
	HealthError string

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

	// AheadSet is the fingerprint of what is ahead of the set this replica holds as well — see the
	// field of the same name on the CRD.
	AheadSet string

	// SyncerStartedAt is when this process started, and nil until it has copied into, counted or
	// rechecked its store — see run.Loop.storeRead. Nil is "not yet", never "unknown": the controller
	// reads an entry without it as one the running process has not vouched for, and holds the upstream
	// on it — see the field of the same name on the CRD.
	SyncerStartedAt *time.Time
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

// Demote takes back everything this replica's entry says it holds, because its store was found not to
// hold the set another replica holds whole: it hands the lease over and copies the set from the next
// leader — see run.Loop.StepDown.
//
// Full, VerifiedSet, AheadSet and the count go together, and the count to zero: a store that lost the
// release's tag or most of a sample of the set does not hold the number the entry carried, and the
// next pass that reads the store says what it does hold. Source goes, since what this replica holds is
// no longer a copy of anything; Role goes to Follower, so that a follower reading the roles while the
// lease cannot be read does not copy from it. And the two permissions the controller derived from the
// leader's report, in the same write, as Withdraw does: this is the leader's report being taken back.
//
// Error and HealthError, the store's measurement, the collection and the size of the set stay: none of
// them is a claim about what the store holds of the set.
func (p *Publisher) Demote(ctx context.Context, state State) error {
	if state.Node == "" {
		return fmt.Errorf("a demotion needs the node it came from")
	}

	return p.publish(ctx, func(storage *registryv1alpha1.RegistryStorage) bool {
		changed := MergeDemote(&storage.Status.Replicas, state)
		return withdrawPermissions(storage) || changed
	})
}

// TakeBack is Demote for a leader that keeps the lease: everything its entry says it holds goes, as
// Demote takes it, and the two permissions with it — and the role stays, because no other replica
// holds the set and this one is still the one that fills it — see run.Loop.TakeBackFull.
//
// The role matters as much as Full does here. A follower that reads the roles while the lease cannot be
// read copies from the replica that says Leader, and the election, in that case, keeps the replica that
// says Leader leading while nobody is full; a leader that called itself a follower would leave the
// cluster with nobody to fill the set, the one thing it needs from the leader now.
func (p *Publisher) TakeBack(ctx context.Context, state State) error {
	if state.Node == "" {
		return fmt.Errorf("taking a report back needs the node it came from")
	}

	return p.publish(ctx, func(storage *registryv1alpha1.RegistryStorage) bool {
		changed := MergeTakeBack(&storage.Status.Replicas, state)
		return withdrawPermissions(storage) || changed
	})
}

// withdrawPermissions takes back the two permissions the controller derived from the leader's report,
// and reports whether either stood.
func withdrawPermissions(storage *registryv1alpha1.RegistryStorage) bool {
	if !storage.Status.SafeToDropUpstream && !storage.Status.AllReplicasFull {
		return false
	}
	storage.Status.SafeToDropUpstream = false
	storage.Status.AllReplicasFull = false
	return true
}

// MergeDemote clears what this replica's entry says it holds — see Demote — and leaves every other
// entry alone: unlike Merge it demotes nobody else, since it claims nothing.
func MergeDemote(replicas *[]registryv1alpha1.StorageReplicaStatus, state State) bool {
	return takeBackHeld(replicas, state.Node, registryv1alpha1.ReplicaRoleFollower)
}

// MergeTakeBack clears what this replica's entry says it holds, as MergeDemote does, and leaves its role
// as it is — see TakeBack.
func MergeTakeBack(replicas *[]registryv1alpha1.StorageReplicaStatus, state State) bool {
	return takeBackHeld(replicas, state.Node, "")
}

// takeBackHeld clears Full, the count, VerifiedSet, AheadSet and Source in the entry of node, and sets
// its role to role unless that is empty. Every other field, and every other entry, stays.
func takeBackHeld(
	replicas *[]registryv1alpha1.StorageReplicaStatus, node string, role registryv1alpha1.ReplicaRole,
) bool {
	for i := range *replicas {
		entry := &(*replicas)[i]
		if entry.Node != node {
			continue
		}
		if role == "" {
			role = entry.Role
		}
		if !entry.Full && entry.VerifiedDigests == 0 && entry.VerifiedSet == "" && entry.AheadSet == "" &&
			entry.Source == "" && entry.Role == role {
			return false
		}
		entry.Full = false
		entry.VerifiedDigests = 0
		entry.VerifiedSet = ""
		entry.AheadSet = ""
		entry.Source = ""
		entry.Role = role
		return true
	}
	// No entry: nothing was claimed, so there is nothing to take back.
	return false
}

// PublishProgress says that a pass is under way and how far it has got, and nothing else.
//
// The pass reports its outcome when it ends, and a fill over a slow link ends an hour later. Until
// then the entry carried the previous pass's words — an error included, which the controller reads
// before anything else — so a fill working normally showed `Failed` with no progress for as long as
// it ran. A pass that is copying has, by that alone, got past whatever stopped the previous one:
// the error is replaced with state.Error, and the count shows how far it is.
//
// state.Error and not nothing, because a copy that goes through is not a store with nothing wrong
// with it. A store the registry cannot write to still counts every reference it already holds as
// done, and the error that says so is what keeps it from being elected or copied from — a progress
// report that cleared it left a store `full` from an earlier pass looking healthy, with no error,
// for as long as the pass ran. The pass's caller puts in what it knows to be wrong with the store
// while copying; empty clears.
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

// PublishHealth records what the syncer's health watch found about this replica's own registry and
// store, and nothing else: only the Error and HealthError fields of its entry. See
// run.Loop.watchHealth.
//
// Its own reporter, written apart from the pass, because the failures it reports are the ones that
// stop a pass from reporting anything. A pass walks the store and measures it before it publishes, and
// on a disk stuck in the kernel it never gets there: the status kept the last clean report — full, no
// error — and the replica kept the lease, renewed by an elector that does not wait for passes, for as
// long as the disk hung.
//
// previous is the error this reporter published last, and the only one it takes back from Error: an
// error the pass reported — a fill that failed — is the pass's to clear, and a health error never
// replaces one, since either already keeps the replica from counting as full. HealthError is this
// reporter's alone and always says what it found.
//
// A leader reporting an error takes safeToDropUpstream back in the same write, and the controller keeps
// it back: the permission needs a leader without an error — see LeaderFull there. allReplicasFull is
// left alone. It is about what the replicas hold, which the health watch does not touch, and the
// controller derives it from Full alone: taken back here, it would be put back by the controller and
// taken back again by the next round, every half a minute for as long as the failure lasts.
func (p *Publisher) PublishHealth(ctx context.Context, state State, previous string) error {
	if state.Node == "" {
		return fmt.Errorf("a health report needs the node it came from")
	}

	return p.publish(ctx, func(storage *registryv1alpha1.RegistryStorage) bool {
		changed := MergeHealth(&storage.Status.Replicas, state, previous)
		if state.Error != "" && state.Role == registryv1alpha1.ReplicaRoleLeader && storage.Status.SafeToDropUpstream {
			storage.Status.SafeToDropUpstream = false
			changed = true
		}
		return changed
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
		AheadSet:        state.AheadSet,
	}
	// To the second, as the API server keeps it: a stamp with a fraction would never equal the one read
	// back, and every pass would write the status again for nothing.
	if state.SyncerStartedAt != nil {
		stamp := metav1.NewTime(state.SyncerStartedAt.Truncate(time.Second))
		entry.SyncerStartedAt = &stamp
	}

	// At most one replica may carry the leader's role, and the one publishing it now is the one
	// holding the lease.
	//
	// A replica writes only its own entry, so an entry outlives whatever it last said: a replica that
	// led, lost the lease and then stopped publishing — because its pass fails, or its process is
	// restarting — leaves `role: Leader` behind it forever. Two entries then claim Leader at once,
	// one the actual lease-holder and one a memory of an earlier one. Which is worse than untidy,
	// because the controller reads this status to find the leader, and so do the followers deciding
	// what to replicate from whenever the lease cannot be read — see run.LeaderOf.
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
		// The same for the health watch, which owns HealthError: a pass knows nothing about it,
		// and the watch writes it again only while something is wrong.
		entry.HealthError = (*replicas)[i].HealthError
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

// MessageLimit is the most text a single message in a status may carry, in bytes.
//
// A status is read by every replica and by the controller, and stored in etcd beside everything else
// in the object. One failed sweep once put the whole output of `registry garbage-collect` into two
// replicas' entries — 136 KB each, a 280 KB RegistryStorage — and what a reader needs from it is the
// last line.
const MessageLimit = 1024

// Clip cuts text to MessageLimit bytes on a character boundary, saying that it did.
func Clip(text string) string {
	if len(text) <= MessageLimit {
		return text
	}
	const marker = " … (truncated)"
	cut := MessageLimit - len(marker)
	for cut > 0 && !utf8.RuneStart(text[cut]) {
		cut--
	}
	return text[:cut] + marker
}

// MergeCollection records the outcome of a garbage collection, and nothing else.
//
// The mirror of the rule above: this reporter owns two fields and must leave every other one
// alone. A replica that had just finished filling would otherwise be reported as empty by the
// next collection.
func MergeCollection(replicas *[]registryv1alpha1.StorageReplicaStatus, state State) bool {
	state.CollectionError = Clip(state.CollectionError)
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

// MergeHealth sets this replica's Error and HealthError to what the health watch found, or takes back
// what it found before, and leaves every other field alone — Full and VerifiedSet above all: a registry
// that answers again, or a disk that takes writes again, gives the replica back its completeness
// without a copy.
//
// An Error already standing that the health watch did not write is left as it is, whichever way: it
// belongs to the pass that reported it. HealthError is set beside it all the same, because that is what
// the other replicas read to tell a replica that cannot serve from one whose copy failed.
func MergeHealth(replicas *[]registryv1alpha1.StorageReplicaStatus, state State, previous string) bool {
	for i := range *replicas {
		entry := &(*replicas)[i]
		if entry.Node != state.Node {
			continue
		}
		changed := false
		if entry.HealthError != state.HealthError {
			entry.HealthError = state.HealthError
			changed = true
		}
		if (entry.Error == "" || entry.Error == previous) && entry.Error != state.Error {
			entry.Error = state.Error
			changed = true
		}
		return changed
	}

	if state.Error == "" && state.HealthError == "" {
		return false
	}
	// No entry yet: a replica whose registry or store failed before its first pass reported. The entry
	// is created with what is known, which is the node and the failure.
	*replicas = append(*replicas, registryv1alpha1.StorageReplicaStatus{
		Node:        state.Node,
		Role:        state.Role,
		Address:     state.Address,
		Error:       state.Error,
		HealthError: state.HealthError,
	})
	return true
}

// MergeProgress records a pass under way in this replica's entry: its error replaced with the one the
// progress carries — what the pass finds wrong with the store while copying, see PublishProgress, and
// what the health watch has standing — its count raised to what the pass has reached, its denominator
// filled in if it had none. Every other field is left alone — above all
// Full, which a pass in progress has not yet earned or lost.
func MergeProgress(replicas *[]registryv1alpha1.StorageReplicaStatus, state State) bool {
	for i := range *replicas {
		entry := &(*replicas)[i]
		if entry.Node != state.Node {
			continue
		}

		changed := false
		if entry.Error != state.Error {
			entry.Error = state.Error
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
		Error:           state.Error,
		VerifiedDigests: state.VerifiedDigests,
		DeclaredDigests: state.DeclaredDigests,
	})
	return true
}
