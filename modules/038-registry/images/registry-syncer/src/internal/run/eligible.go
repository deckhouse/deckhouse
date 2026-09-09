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
// Plain leader election is not enough here, because leadership is not a symmetric role:
// the leader is the replication source every follower copies from, and the one whose
// completeness gates going air-gap. Handing it to an empty replica while a full one
// exists is how a cluster gets stuck — most sharply in air-gap, where the leader has no
// upstream to fill itself from:
//
//	`d8 mirror push` lands on some replica through the publication endpoint, and it need
//	not be the one holding the lease. If the lease-holder is empty, it cannot fill (there
//	is no upstream), the followers dutifully replicate its emptiness, and the replica that
//	actually has the images sits idle. Nothing recovers on its own.
//
// So the rule is: a full replica leads. Anything else follows — unless nobody is full,
// which is where every cluster starts, and then someone has to lead in order to begin
// filling at all.
//
// A pure function of the reported state, because leadership decided wrongly is expensive
// to notice: the cluster keeps serving images throughout and the damage shows up only as
// a fill that never completes.
func MayLead(self string, replicas []registryv1alpha1.StorageReplicaStatus) bool {
	someoneIsFull := false
	selfIsFull := false
	incumbent := ""

	for i := range replicas {
		replica := &replicas[i]

		if isFull(replica) {
			someoneIsFull = true
			if replica.Node == self {
				selfIsFull = true
			}
		}
		if replica.Role == registryv1alpha1.ReplicaRoleLeader {
			incumbent = replica.Node
		}
	}

	// A full replica leads. That is the whole rule while one exists.
	if someoneIsFull {
		return selfIsFull
	}

	// Nobody is full, and here an election must not happen at all.
	//
	// Leadership among incomplete replicas is worse than any particular choice of leader, because
	// every change of it restarts the work: a fill runs on the leader, and moving the lease to
	// another incomplete replica abandons what the first had done and begins again elsewhere. With
	// the "fullest leads" rule that was self-perpetuating — the fullest changes as they fill, so the
	// lease chased it, and nobody ever arrived. Measured on a cluster: the lease moving between three
	// replicas holding 428, 337 and 333 digests, none of them ever completing.
	//
	// So while nobody is full, whoever leads keeps leading, and everyone else stands aside.
	if incumbent != "" {
		return incumbent == self
	}

	// And nobody leads either — a cluster that has just started, or one whose leader is gone. Someone
	// has to begin, and it should be the replica holding the most: it is closest to becoming a source
	// the others can copy from, and starting elsewhere throws away what it already has. Ties by node
	// name, so that every replica reaches the same answer from the same report rather than each
	// preferring itself.
	return leadsWhenNobodyIsFull(self, replicas)
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
func isFull(replica *registryv1alpha1.StorageReplicaStatus) bool {
	return replica.Full && replica.Error == ""
}
