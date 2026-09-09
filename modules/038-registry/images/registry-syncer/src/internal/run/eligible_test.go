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
	"testing"

	"github.com/stretchr/testify/assert"

	registryv1alpha1 "github.com/deckhouse/deckhouse/go_lib/registry/apis/deckhouse.io/v1alpha1"
)

func replica(node string, full bool) registryv1alpha1.StorageReplicaStatus {
	return registryv1alpha1.StorageReplicaStatus{Node: node, Full: full}
}

func TestMayLead(t *testing.T) {
	cases := []struct {
		name     string
		self     string
		replicas []registryv1alpha1.StorageReplicaStatus
		want     bool
	}{{
		// Where every cluster starts. Somebody has to lead or nothing ever fills.
		name: "nobody has reported anything yet",
		self: "master-0",
		want: true,
	}, {
		name:     "nobody is full",
		self:     "master-0",
		replicas: []registryv1alpha1.StorageReplicaStatus{replica("master-0", false), replica("master-1", false)},
		want:     true,
	}, {
		name:     "this replica is the full one",
		self:     "master-0",
		replicas: []registryv1alpha1.StorageReplicaStatus{replica("master-0", true), replica("master-1", false)},
		want:     true,
	}, {
		// The case the whole rule exists for. In air-gap the empty leader could not fill
		// itself, the followers would replicate its emptiness, and the replica holding
		// the images would sit idle.
		name:     "another replica is full and this one is empty",
		self:     "master-0",
		replicas: []registryv1alpha1.StorageReplicaStatus{replica("master-0", false), replica("master-1", true)},
		want:     false,
	}, {
		name:     "every replica is full",
		self:     "master-0",
		replicas: []registryv1alpha1.StorageReplicaStatus{replica("master-0", true), replica("master-1", true)},
		want:     true,
	}, {
		// Has not reported yet, while a neighbour has the whole set. Standing aside is
		// the safe reading of "unknown".
		name:     "this replica has no entry of its own",
		self:     "master-2",
		replicas: []registryv1alpha1.StorageReplicaStatus{replica("master-0", true), replica("master-1", false)},
		want:     false,
	}, {
		// The counters may look complete while the last pass failed. Replicating from
		// that is not something to start on the strength of a stale count.
		name: "the only full replica is reporting an error",
		self: "master-0",
		replicas: []registryv1alpha1.StorageReplicaStatus{
			replica("master-0", false),
			{Node: "master-1", Full: true, Error: "3 of 459 references could not be copied"},
		},
		want: true,
	}, {
		name: "this replica is full but its last pass failed",
		self: "master-0",
		replicas: []registryv1alpha1.StorageReplicaStatus{
			{Node: "master-0", Full: true, Error: "the upstream refused the credentials"},
			replica("master-1", true),
		},
		want: false,
	}, {
		// Both failing: back to "nobody is usable", so leading is better than nobody
		// leading.
		name: "every replica is failing",
		self: "master-0",
		replicas: []registryv1alpha1.StorageReplicaStatus{
			{Node: "master-0", Full: true, Error: "disk full"},
			{Node: "master-1", Full: true, Error: "disk full"},
		},
		want: true,
	}}

	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			assert.Equal(t, test.want, MayLead(test.self, test.replicas))
		})
	}
}

// TestAirGapCannotGetStuck states the invariant the two rules give together, because
// neither of them says it alone and it is the reason both exist.
//
// In air-gap there is no upstream, so a leader cannot fill itself: content arrives through
// the publication endpoint, on whichever replica the ingress chose. The failure to avoid is
// an empty replica holding the lease while a full one follows it — the leader cannot fill,
// the follower has nothing to copy from, and nothing recovers on its own.
func TestAirGapCannotGetStuck(t *testing.T) {
	airGap := &registryv1alpha1.RegistryStorageSpec{}

	// `d8 mirror push` landed on master-1, which is not the current lease-holder.
	replicas := []registryv1alpha1.StorageReplicaStatus{
		replica("master-0", false),
		replica("master-1", true),
	}

	// The empty replica stands aside, so the lease can move.
	assert.False(t, MayLead("master-0", replicas))
	assert.True(t, MayLead("master-1", replicas))

	// And once it has moved, the replica that stood aside copies from the one that has the images.
	assert.Equal(t, ActionReplicate,
		Decide(airGap, false, &Leader{Node: "master-1", Address: "10.0.0.2:5001", Full: true}))

	// The stall this guards against is not "an incomplete leader" — a follower will copy from one of
	// those, taking what it has and reporting the rest as pending. It is an EMPTY leader in air-gap:
	// it has nothing to give and no upstream to get it from, so eligibility has to move the lease to
	// the replica the push landed on. Which is what the two assertions above check.
	assert.Equal(t, ActionReplicate,
		Decide(airGap, false, &Leader{Node: "master-0", Address: "10.0.0.1:5001", Full: false}),
		"copying what a partial leader has is not the stall; leading with nothing is")
}

// TestTheFullestReplicaLeadsWhenNobodyIsComplete is the case a live cluster showed: the lease moved to
// the replica holding 333 digests while another held 428, and every follower then waited on the one
// with least to catch up — because a follower only replicates from a leader that is complete.
//
// Nobody being full is the ordinary state of a cluster that is still filling, so "somebody has to
// lead" is right; which somebody is what this decides. The fullest replica is closest to becoming a
// source the others can use, and leading with the emptiest throws away what the others had copied.
func TestTheFullestReplicaLeadsWhenNobodyIsComplete(t *testing.T) {
	replicas := []registryv1alpha1.StorageReplicaStatus{
		{Node: "master-0", VerifiedDigests: 428},
		{Node: "master-1", VerifiedDigests: 333},
		{Node: "master-2", VerifiedDigests: 337},
	}

	assert.True(t, MayLead("master-0", replicas), "the replica holding the most leads")
	assert.False(t, MayLead("master-1", replicas))
	assert.False(t, MayLead("master-2", replicas))
}

// TestATieIsBrokenTheSameWayByEveryReplica: each replica reaches this answer on its own, from the same
// report, so they must not each prefer themselves — that is two leaders, or none.
func TestATieIsBrokenTheSameWayByEveryReplica(t *testing.T) {
	replicas := []registryv1alpha1.StorageReplicaStatus{
		{Node: "master-2", VerifiedDigests: 100},
		{Node: "master-0", VerifiedDigests: 100},
	}

	assert.True(t, MayLead("master-0", replicas))
	assert.False(t, MayLead("master-2", replicas))
}

// TestAFailingReplicaIsNotACandidate: what a replica holds after a failed pass is not something the
// others should be told to copy.
func TestAFailingReplicaIsNotACandidate(t *testing.T) {
	replicas := []registryv1alpha1.StorageReplicaStatus{
		{Node: "master-0", VerifiedDigests: 428, Error: "the fill did not finish"},
		{Node: "master-1", VerifiedDigests: 333},
	}

	assert.False(t, MayLead("master-0", replicas))
	assert.True(t, MayLead("master-1", replicas))
}

// TestSomebodyLeadsWhenThereIsNothingToCompare keeps a fresh cluster from deadlocking: with no
// reports at all, refusing to lead is refusing to start.
func TestSomebodyLeadsWhenThereIsNothingToCompare(t *testing.T) {
	assert.True(t, MayLead("master-0", nil))
	assert.True(t, MayLead("master-0", []registryv1alpha1.StorageReplicaStatus{
		{Node: "master-0", Error: "unreadable"},
		{Node: "master-1", Error: "unreadable"},
	}))
}

// TestNoElectionHappensAmongIncompleteReplicas is the rule the operator stated, and the reason for it
// is that leadership churn is worse than any particular choice of leader.
//
// A fill runs on the leader. Moving the lease to another incomplete replica abandons what the first
// had done and starts again elsewhere — and under a "fullest leads" rule that is self-perpetuating,
// because the fullest changes as they fill and the lease chases it. Measured on a cluster: the lease
// moving between replicas holding 428, 337 and 333 digests, none of them ever completing.
func TestNoElectionHappensAmongIncompleteReplicas(t *testing.T) {
	// Nobody is full, and master-1 currently leads — even though master-0 holds more.
	replicas := []registryv1alpha1.StorageReplicaStatus{
		{Node: "master-0", VerifiedDigests: 428, Role: registryv1alpha1.ReplicaRoleFollower},
		{Node: "master-1", VerifiedDigests: 333, Role: registryv1alpha1.ReplicaRoleLeader},
		{Node: "master-2", VerifiedDigests: 337, Role: registryv1alpha1.ReplicaRoleFollower},
	}

	assert.True(t, MayLead("master-1", replicas), "the replica that leads keeps leading")
	assert.False(t, MayLead("master-0", replicas),
		"holding more is not a reason to take the lease: the fill would start over")
	assert.False(t, MayLead("master-2", replicas))
}

// TestSomebodyStartsWhenNobodyLeads: the rule above must not deadlock a cluster that has just started,
// or one whose leader is gone. Then the fullest begins — it is closest to becoming a source the others
// can copy from.
func TestSomebodyStartsWhenNobodyLeads(t *testing.T) {
	replicas := []registryv1alpha1.StorageReplicaStatus{
		{Node: "master-0", VerifiedDigests: 428},
		{Node: "master-1", VerifiedDigests: 333},
		{Node: "master-2", VerifiedDigests: 337},
	}

	assert.True(t, MayLead("master-0", replicas))
	assert.False(t, MayLead("master-1", replicas))
	assert.False(t, MayLead("master-2", replicas))
}

// TestAFullReplicaStillTakesOver keeps the older rule intact: completeness outranks incumbency, because
// the leader is what every follower copies from and what the air-gap transition is gated on.
func TestAFullReplicaStillTakesOver(t *testing.T) {
	replicas := []registryv1alpha1.StorageReplicaStatus{
		{Node: "master-0", VerifiedDigests: 100, Role: registryv1alpha1.ReplicaRoleLeader},
		{Node: "master-1", VerifiedDigests: 459, Full: true},
	}

	assert.False(t, MayLead("master-0", replicas), "an incomplete incumbent yields to a full replica")
	assert.True(t, MayLead("master-1", replicas))
}
