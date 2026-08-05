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

	for i := range replicas {
		replica := &replicas[i]

		if replica.Node == self {
			if isFull(replica) {
				// Already holds the whole set, so it is the right leader by definition.
				return true
			}
			continue
		}

		if isFull(replica) {
			someoneIsFull = true
		}
	}

	// Nobody has the whole set, so being empty is not a reason to stand aside — there is
	// nothing better available, and a cluster with no leader never starts filling.
	return !someoneIsFull
}

// isFull reads a replica's own report of itself.
//
// An error disqualifies it even when the counters look complete: `full` says what it
// holds, and the error says whether its last pass finished. A replica that reports both
// is one whose completeness nobody should be replicating from yet.
func isFull(replica *registryv1alpha1.StorageReplicaStatus) bool {
	return replica.Full && replica.Error == ""
}
