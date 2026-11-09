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

package run

import (
	"fmt"

	registryv1alpha1 "github.com/deckhouse/deckhouse/go_lib/registry/apis/deckhouse.io/v1alpha1"
)

// MayLead reports whether this replica should stand in the election — see WhyNotLead.
func MayLead(
	self string, replicas []registryv1alpha1.StorageReplicaStatus, currentSet string, lease Lease,
) bool {
	return WhyNotLead(self, replicas, currentSet, lease) == ""
}

// WhyNotLead says why this replica should not stand in the election, given what every replica has
// reported about itself, and is empty when it should.
//
// A reason rather than a yes or no, because it is what the replica logs when it steps aside. A log
// that always said "another replica holds the whole image set" sent whoever read a failover with
// nobody full looking for a full replica that did not exist.
//
// Plain leader election is not enough, because leadership is not a symmetric role: the
// leader is the replication source every follower copies from, and the one whose
// completeness gates going air-gap. Handing it to an empty replica while a full one exists
// is how a cluster gets stuck, most sharply in air-gap — the lease-holder has no upstream to
// fill from, the followers replicate its emptiness, and the replica that has the images sits
// idle. A `d8 mirror push` through the Ingress lands on the lease holder, because the
// controller's registry-push EndpointSlice follows the leader, so short of pushing the whole
// set again nothing recovers on its own.
//
// So a full replica leads and anything else follows — unless nobody is full, which is where
// every cluster starts, and then someone has to lead in order to begin filling at all.
//
// A pure function of the reported state, because leadership decided wrongly is expensive to
// notice: the cluster keeps serving images throughout, and the damage shows up only as a
// fill that never completes.
//
// "Full" means full for the set the cluster needs now, currentSet — see isFull. A replica's report
// is only as good as the set it was measured against, and when the set changes the leader withdraws
// its own before verifying again; a follower still showing a Full from the previous set is not
// complete for this one. Counted as full, it took the lease from a leader that was only
// re-verifying, and the new leader, finding the set changed too, gave it straight back — four
// handovers in three minutes on a live cluster, for a set that had merely grown by a module.
//
// lease is what the storage lease says about who leads; see Lease.
//
// The rule stays a pure function of the reports. What a report cannot say is that its replica has
// stopped acting on it — a syncer cut from the API server leaves its last entry standing on a node that
// stays Ready — and only that is local state, kept by the caller: see Waiting.
func WhyNotLead(
	self string, replicas []registryv1alpha1.StorageReplicaStatus, currentSet string, lease Lease,
) string {
	why, _ := whyNotLead(self, replicas, currentSet, lease, nil)
	return why
}

// whyNotLead is WhyNotLead with the replicas the caller has passed over while nobody is full and nobody
// leads — see Waiting. Its second result is the replica the rule chose to begin then: this one, when it
// should stand, and empty when the decision was not that one, or when this replica cannot serve and so
// waits on any replica that can, passing nobody over.
func whyNotLead(
	self string, replicas []registryv1alpha1.StorageReplicaStatus, currentSet string, lease Lease,
	passedOver map[string]bool,
) (string, string) {
	var full *registryv1alpha1.StorageReplicaStatus
	selfIsFull := false

	for i := range replicas {
		replica := &replicas[i]

		if isFull(replica, currentSet) {
			if full == nil {
				full = replica
			}
			if replica.Node == self {
				selfIsFull = true
			}
		}
	}
	incumbent := lease.incumbent(replicas)

	// A leader that can complete by itself keeps leading while it does, full follower or not.
	//
	// A follower is filled FROM the leader, so a follower that is full while the leader is not is
	// one that finished counting first: the leader reports full only at the end of a pass, and the
	// copy of a pass it is still verifying can land before that. Handing the lease over then gains
	// nothing and costs a move of the publication endpoint and of every agent's route — which is
	// what a module installed on a healthy three-master cluster used to cause, once, every time.
	//
	// Only with an upstream, only while the leader holds a live lease, only while it reports no error,
	// and only while its store accepts writes. Without an upstream the leader has nothing to complete
	// from, and a full follower is the only way the set gets back to where the others copy it from;
	// with an expired lease the leader may be gone; with an error it is not completing at all; and a
	// store at its budget or at the node's reserve cannot take the rest of the set, so it would hold
	// the lease — and the air-gap transition with it — until somebody freed its disk.
	if full != nil && lease.Known && lease.CanFill && lease.Live && incumbent != nil &&
		incumbent.Error == "" && acceptsWrites(incumbent) {
		if incumbent.Node == self {
			return "", ""
		}
		return fmt.Sprintf("%s leads and completes the set from the upstream by itself", incumbent.Node), ""
	}

	// Otherwise a full replica leads. That is the whole rule while one exists.
	if full != nil {
		if selfIsFull {
			return "", ""
		}
		return fmt.Sprintf("%s holds the whole image set and this replica does not", full.Node), ""
	}

	// Nobody is full, and here an election must not happen at all.
	//
	// Leadership among incomplete replicas is worse than any particular choice of leader, because
	// every change of it restarts the work: a fill runs on the leader, and moving the lease to
	// another incomplete replica abandons what the first had done and begins again elsewhere. With
	// the "fullest leads" rule that is self-perpetuating: the fullest changes as they fill, so the
	// lease chases it between replicas and nobody ever arrives.
	//
	// So while nobody is full, whoever leads keeps leading, and everyone else stands aside.
	if incumbent != nil {
		if incumbent.Node == self {
			return "", ""
		}
		return fmt.Sprintf("nobody holds the whole image set and %s leads: the lease stays where the fill is",
			incumbent.Node), ""
	}

	// And nobody leads either — a cluster that has just started, or one whose leader is gone. Someone
	// has to begin, and it should be the replica holding the most: it is closest to becoming a source
	// the others can copy from, and starting elsewhere throws away what it already has. Ties by node
	// name, so that every replica reaches the same answer from the same report rather than each
	// preferring itself.
	//
	// A replica that cannot serve waits on any that can, passed over or not. A pass-over must not make a
	// candidate of it: it would lead without being able to fill or serve, and the incumbent rule above
	// would keep it there after the healthy replica it passed over came back. Only with every report
	// unable to serve is there nothing better, and then it stands as any of them may.
	own := ownReport(self, replicas)
	if own != nil && own.HealthError != "" {
		if leadsWhenNobodyIsFull(replicas, nil) != nil {
			return "this replica cannot serve what it holds: " + own.HealthError, ""
		}
		return "", self
	}
	best := leadsWhenNobodyIsFull(replicas, passedOver)
	if best == nil || best.Node == self {
		// Nothing to compare — no reports at all, or every one of them unable to serve or passed over.
		// Then the election decides, because refusing to lead here is refusing to start.
		return "", self
	}
	if own != nil && !acceptsWrites(own) && acceptsWrites(best) {
		why := fmt.Sprintf("this replica's store refuses writes (%s), so it cannot take the rest of the set, "+
			"and %s holds the most among those that can (%d digests)", own.Store.Reason, best.Node, best.VerifiedDigests)
		return why, best.Node
	}
	return fmt.Sprintf("nobody leads or holds the whole image set, and %s holds the most of it (%d digests)",
		best.Node, best.VerifiedDigests), best.Node
}

// ownReport is this replica's entry among the reports, if it has one.
func ownReport(self string, replicas []registryv1alpha1.StorageReplicaStatus) *registryv1alpha1.StorageReplicaStatus {
	for i := range replicas {
		if replicas[i].Node == self {
			return &replicas[i]
		}
	}
	return nil
}

// Lease is what the storage lease says about who leads, read by the caller.
//
// The lease rather than the role each replica reports, because a role is what a replica last said
// about itself, and one whose pod is gone never takes it back. Deciding from roles, a leader removed
// while nobody was full kept every other replica out of the election for as long as its report
// stood: twelve minutes on a live cluster, against two when the decision was the full replica's.
type Lease struct {
	// Known is false when the lease could not be read; the reported roles then decide, as before.
	Known bool

	// Holder is the replica holding the lease, or held it until a moment ago — empty when it was
	// released, or expired long enough ago that its holder is not coming back. A short grace past
	// expiry, so that an API server unreachable for a minute does not hand an incomplete fill to
	// another incomplete replica and start it over.
	Holder string

	// Live says the holder renewed within the lease's duration.
	Live bool

	// CanFill says the storage has an upstream, so a leader can complete the set by itself.
	CanFill bool
}

// incumbent is the replica that leads now, if any.
func (l Lease) incumbent(replicas []registryv1alpha1.StorageReplicaStatus) *registryv1alpha1.StorageReplicaStatus {
	for i := range replicas {
		replica := &replicas[i]
		if l.Known && replica.Node == l.Holder {
			return replica
		}
		if !l.Known && replica.Role == registryv1alpha1.ReplicaRoleLeader {
			return replica
		}
	}
	return nil
}

// acceptsWrites reports whether a replica's store takes writes, as it reported itself: at neither limit,
// and with a directory the registry can write to. A replica that reports no measurement is taken as
// writable, as every replica was before stores were measured.
func acceptsWrites(replica *registryv1alpha1.StorageReplicaStatus) bool {
	return replica.Store == nil || replica.Store.Writable
}

// leadsWhenNobodyIsFull picks the fullest replica that can serve, by name where the counts are equal,
// and nil when there is none.
//
// Consulted only when there is no leader at all: once one exists it keeps leading until it is full or
// gone, because moving the lease between incomplete replicas restarts the fill each time.
//
// A replica whose registry does not answer, or whose disk refuses writes, is not a candidate while any
// other is: it cannot be what the others copy from, nor take the rest of the set. Its health watch says
// so in healthError.
//
// Nor is one whose store is at its budget or at the node's reserve, while one with room is. The watch
// leaves that to the store's own measurement — store.writable — because a full store is not a failing
// one, so it has no healthError; and its pass's error, which used to pass it over, no longer does. It
// cannot take the rest of the set, from the upstream or from a push, which lands on the lease holder,
// and once chosen it would keep the lease while nobody is full — and the air-gap transition with it —
// until somebody freed its disk: the rule for a full incumbent above guards the same. With every store
// out of room the fullest is still the best source of what there is, and is chosen.
//
// Any other error is no reason to pass a replica over. It says that a pass failed, not that what the
// replica holds is wrong: the count comes from the store — through reportHeld, or the catalogue —
// whenever the store could be read. And with nobody full and nobody leading, the error is almost always
// about a copy from a leader that is gone, or about the upstream, which every candidate shares. On a
// DVP stand the full leader was powered off while one follower held 322 of 446 digests and the other
// held none; both carried "could not be read from" the dead leader, the fuller one for about 20 s
// longer, and for those 20 s the empty one was the only candidate and stood. Only client-go's minute of
// watching an unchanged lease before it takes one kept it from leading and filling from scratch, and a
// lease released by a leader that stops gracefully gives no such minute.
//
// Air-gap included: the fullest leads, Decide gives it the catalogue to count, and the others copy from
// it through the lease. Passing it over for an error-free empty replica chose a leader that could never
// fill.
//
// The price: a replica whose error is its own but escapes the watch — a survey of the store failing on
// EIO while writes still go through — can be chosen when it is the fullest, and then keeps the lease
// while nobody is full, as any incumbent does.
//
// Nor, finally, is a replica in passedOver: one the caller chose and waited on for too long without its
// taking the lease — see Waiting. The rules above then apply among the replicas left, and with every one
// that can serve passed over there is nothing to compare, and the election decides — unless the caller
// itself cannot serve: see whyNotLead.
func leadsWhenNobodyIsFull(
	replicas []registryv1alpha1.StorageReplicaStatus, passedOver map[string]bool,
) *registryv1alpha1.StorageReplicaStatus {
	var best, bestWithRoom *registryv1alpha1.StorageReplicaStatus

	for i := range replicas {
		replica := &replicas[i]
		if replica.HealthError != "" || passedOver[replica.Node] {
			continue
		}
		if holdsMore(replica, best) {
			best = replica
		}
		if acceptsWrites(replica) && holdsMore(replica, bestWithRoom) {
			bestWithRoom = replica
		}
	}
	if bestWithRoom != nil {
		return bestWithRoom
	}
	return best
}

// holdsMore reports whether a replica is to be preferred over another — nil, for none yet — by what it
// holds, and by name where the counts are equal.
func holdsMore(replica, other *registryv1alpha1.StorageReplicaStatus) bool {
	return other == nil || replica.VerifiedDigests > other.VerifiedDigests ||
		(replica.VerifiedDigests == other.VerifiedDigests && replica.Node < other.Node)
}

// isFull reads a replica's own report of itself.
//
// An error disqualifies it even when the counters look complete: `full` says what it
// holds, and the error says whether its last pass finished. A replica that reports both
// is one whose completeness nobody should be replicating from yet.
func isFull(replica *registryv1alpha1.StorageReplicaStatus, currentSet string) bool {
	if !replica.Full || replica.Error != "" {
		return false
	}
	// A report that names no set — written before sets were named, or on a pass that could not
	// tell — is taken at its word, as it always was; so is every report when the current set
	// itself cannot be told. Otherwise it has to be about this set.
	return currentSet == "" || replica.VerifiedSet == "" || replica.VerifiedSet == currentSet
}
