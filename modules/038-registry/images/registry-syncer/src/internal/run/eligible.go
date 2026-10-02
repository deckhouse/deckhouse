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

import registryv1alpha1 "github.com/deckhouse/deckhouse/go_lib/registry/apis/deckhouse.io/v1alpha1"

// MayLead reports whether this replica should stand in the election, given what every
// replica has reported about itself.
//
// Plain leader election is not enough, because leadership is not a symmetric role: the
// leader is the replication source every follower copies from, and the one whose
// completeness gates going air-gap. Handing it to an empty replica while a full one exists
// is how a cluster gets stuck, most sharply in air-gap — `d8 mirror push` lands on whatever
// replica the endpoint routes it to, and if the lease-holder is not that one it has no
// upstream to fill from, the followers replicate its emptiness, and the replica that has
// the images sits idle. Nothing recovers on its own.
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
func MayLead(
	self string, replicas []registryv1alpha1.StorageReplicaStatus, currentSet string, lease Lease,
) bool {
	someoneIsFull := false
	selfIsFull := false

	for i := range replicas {
		replica := &replicas[i]

		if isFull(replica, currentSet) {
			someoneIsFull = true
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
	if someoneIsFull && lease.Known && lease.CanFill && lease.Live && incumbent != nil &&
		incumbent.Error == "" && acceptsWrites(incumbent) {
		return incumbent.Node == self
	}

	// Otherwise a full replica leads. That is the whole rule while one exists.
	if someoneIsFull {
		return selfIsFull
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
		return incumbent.Node == self
	}

	// And nobody leads either — a cluster that has just started, or one whose leader is gone. Someone
	// has to begin, and it should be the replica holding the most: it is closest to becoming a source
	// the others can copy from, and starting elsewhere throws away what it already has. Ties by node
	// name, so that every replica reaches the same answer from the same report rather than each
	// preferring itself.
	return leadsWhenNobodyIsFull(self, replicas)
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

// acceptsWrites reports whether a replica's store takes writes, as it measured itself. A replica that
// reports no measurement is taken as writable, as every replica was before stores were measured.
func acceptsWrites(replica *registryv1alpha1.StorageReplicaStatus) bool {
	return replica.Store == nil || replica.Store.Writable
}

// leadsWhenNobodyIsFull picks the fullest replica, by name where the counts are equal.
//
// Consulted only when there is no leader at all: once one exists it keeps leading until it is full or
// gone, because moving the lease between incomplete replicas restarts the fill each time.
func leadsWhenNobodyIsFull(self string, replicas []registryv1alpha1.StorageReplicaStatus) bool {
	best := ""
	var most int32 = -1

	for i := range replicas {
		replica := &replicas[i]
		// A replica whose last pass failed is not a candidate while any other is: what it holds is
		// not something the others should be told to copy.
		if replica.Error != "" {
			continue
		}
		if replica.VerifiedDigests > most ||
			(replica.VerifiedDigests == most && replica.Node < best) {
			best, most = replica.Node, replica.VerifiedDigests
		}
	}

	if best == "" {
		// Nothing to compare — no reports at all, or every one of them failing. Then the election
		// decides, because refusing to lead here is refusing to start.
		return true
	}
	return best == self
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
