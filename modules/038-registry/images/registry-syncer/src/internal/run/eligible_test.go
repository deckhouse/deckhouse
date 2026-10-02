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
			assert.Equal(t, test.want, MayLead(test.self, test.replicas, "", Lease{}))
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
	assert.False(t, MayLead("master-0", replicas, "", Lease{}))
	assert.True(t, MayLead("master-1", replicas, "", Lease{}))

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

// TestTheFullestReplicaLeadsWhenNobodyIsComplete is the case this rule exists for: the lease landing
// on the emptiest replica while a fuller one stands by, after which every follower waits on the one
// holding least to catch up — because a follower only replicates from a leader that is complete.
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

	assert.True(t, MayLead("master-0", replicas, "", Lease{}), "the replica holding the most leads")
	assert.False(t, MayLead("master-1", replicas, "", Lease{}))
	assert.False(t, MayLead("master-2", replicas, "", Lease{}))
}

// TestATieIsBrokenTheSameWayByEveryReplica: each replica reaches this answer on its own, from the same
// report, so they must not each prefer themselves — that is two leaders, or none.
func TestATieIsBrokenTheSameWayByEveryReplica(t *testing.T) {
	replicas := []registryv1alpha1.StorageReplicaStatus{
		{Node: "master-2", VerifiedDigests: 100},
		{Node: "master-0", VerifiedDigests: 100},
	}

	assert.True(t, MayLead("master-0", replicas, "", Lease{}))
	assert.False(t, MayLead("master-2", replicas, "", Lease{}))
}

// TestAFailingReplicaIsNotACandidate: what a replica holds after a failed pass is not something the
// others should be told to copy.
func TestAFailingReplicaIsNotACandidate(t *testing.T) {
	replicas := []registryv1alpha1.StorageReplicaStatus{
		{Node: "master-0", VerifiedDigests: 428, Error: "the fill did not finish"},
		{Node: "master-1", VerifiedDigests: 333},
	}

	assert.False(t, MayLead("master-0", replicas, "", Lease{}))
	assert.True(t, MayLead("master-1", replicas, "", Lease{}))
}

// TestSomebodyLeadsWhenThereIsNothingToCompare keeps a fresh cluster from deadlocking: with no
// reports at all, refusing to lead is refusing to start.
func TestSomebodyLeadsWhenThereIsNothingToCompare(t *testing.T) {
	assert.True(t, MayLead("master-0", nil, "", Lease{}))
	assert.True(t, MayLead("master-0", []registryv1alpha1.StorageReplicaStatus{
		{Node: "master-0", Error: "unreadable"},
		{Node: "master-1", Error: "unreadable"},
	}, "", Lease{}))
}

// TestNoElectionHappensAmongIncompleteReplicas is the rule the operator stated, and the reason for it
// is that leadership churn is worse than any particular choice of leader.
//
// A fill runs on the leader. Moving the lease to another incomplete replica abandons what the first
// had done and starts again elsewhere — and under a "fullest leads" rule that is self-perpetuating,
// because the fullest changes as they fill and the lease chases it between replicas, none of them
// ever completing.
func TestNoElectionHappensAmongIncompleteReplicas(t *testing.T) {
	// Nobody is full, and master-1 currently leads — even though master-0 holds more.
	replicas := []registryv1alpha1.StorageReplicaStatus{
		{Node: "master-0", VerifiedDigests: 428, Role: registryv1alpha1.ReplicaRoleFollower},
		{Node: "master-1", VerifiedDigests: 333, Role: registryv1alpha1.ReplicaRoleLeader},
		{Node: "master-2", VerifiedDigests: 337, Role: registryv1alpha1.ReplicaRoleFollower},
	}

	assert.True(t, MayLead("master-1", replicas, "", Lease{}), "the replica that leads keeps leading")
	assert.False(t, MayLead("master-0", replicas, "", Lease{}),
		"holding more is not a reason to take the lease: the fill would start over")
	assert.False(t, MayLead("master-2", replicas, "", Lease{}))
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

	assert.True(t, MayLead("master-0", replicas, "", Lease{}))
	assert.False(t, MayLead("master-1", replicas, "", Lease{}))
	assert.False(t, MayLead("master-2", replicas, "", Lease{}))
}

// TestAFullReplicaStillTakesOver keeps the older rule intact where nobody can complete by itself:
// completeness outranks incumbency, because the leader is what every follower copies from and what the
// air-gap transition is gated on.
func TestAFullReplicaStillTakesOver(t *testing.T) {
	replicas := []registryv1alpha1.StorageReplicaStatus{
		{Node: "master-0", VerifiedDigests: 100, Role: registryv1alpha1.ReplicaRoleLeader},
		{Node: "master-1", VerifiedDigests: 459, Full: true},
	}

	assert.False(t, MayLead("master-0", replicas, "", Lease{}), "an incomplete incumbent yields to a full replica")
	assert.True(t, MayLead("master-1", replicas, "", Lease{}))
}

// TestAFullFromAnotherSetDoesNotTakeTheLease is the handover loop a live cluster ran into: the set grew
// by a module, the leader withdrew its full to verify the new set, and a follower still reporting full
// for the old one took the lease — whose new holder, finding the set changed as well, gave it back.
func TestAFullFromAnotherSetDoesNotTakeTheLease(t *testing.T) {
	leader := func(full bool, set string) registryv1alpha1.StorageReplicaStatus {
		return registryv1alpha1.StorageReplicaStatus{
			Node: "master-0", Role: registryv1alpha1.ReplicaRoleLeader, Full: full, VerifiedSet: set,
		}
	}
	follower := func(set string) registryv1alpha1.StorageReplicaStatus {
		return registryv1alpha1.StorageReplicaStatus{Node: "master-2", Full: true, VerifiedSet: set}
	}

	// The leader is re-verifying; the follower's full is about the previous set. Nobody is complete
	// for this one, so the incumbent keeps leading and nothing moves.
	stale := []registryv1alpha1.StorageReplicaStatus{leader(false, ""), follower("old")}
	assert.True(t, MayLead("master-0", stale, "new", Lease{}), "the re-verifying leader keeps the lease")
	assert.False(t, MayLead("master-2", stale, "new", Lease{}), "a full from the previous set is no claim on it")

	// A follower complete for the set the cluster needs now is a claim, as before.
	current := []registryv1alpha1.StorageReplicaStatus{leader(false, ""), follower("new")}
	assert.False(t, MayLead("master-0", current, "new", Lease{}))
	assert.True(t, MayLead("master-2", current, "new", Lease{}))

	// Both complete for this set: both may stand, and the lease stays with its holder — eligibility
	// decides who may hold it, not who takes it from whom.
	both := []registryv1alpha1.StorageReplicaStatus{leader(true, "new"), follower("new")}
	assert.True(t, MayLead("master-0", both, "new", Lease{}))
	assert.True(t, MayLead("master-2", both, "new", Lease{}))
}

// TestAReportWithoutASetIsTakenAtItsWord: a replica on a version that names no set, or a current set
// that cannot be told, leaves eligibility where it always was — including the air-gapped case of a
// full follower taking over from an empty leader.
func TestAReportWithoutASetIsTakenAtItsWord(t *testing.T) {
	unnamed := []registryv1alpha1.StorageReplicaStatus{
		{Node: "master-0", Role: registryv1alpha1.ReplicaRoleLeader},
		{Node: "master-1", Full: true},
	}
	assert.True(t, MayLead("master-1", unnamed, "new", Lease{}), "a report written before sets were named")

	named := []registryv1alpha1.StorageReplicaStatus{
		{Node: "master-0", Role: registryv1alpha1.ReplicaRoleLeader},
		{Node: "master-1", Full: true, VerifiedSet: "old"},
	}
	assert.True(t, MayLead("master-1", named, "", Lease{}), "the current set could not be told")
}

// TestALeaderThatCanCompleteKeepsTheLease is the handover a healthy three-master cluster made every
// time a module was installed. The leader withdrew its full to verify the grown set; a follower copying
// from it finished counting first, because the leader says full only at the end of its pass; and the
// lease moved, for nothing — the new leader held exactly what it had copied from the old one.
func TestALeaderThatCanCompleteKeepsTheLease(t *testing.T) {
	replicas := []registryv1alpha1.StorageReplicaStatus{
		{Node: "master-0", Role: registryv1alpha1.ReplicaRoleLeader, VerifiedDigests: 397},
		{Node: "master-1", Full: true, VerifiedSet: "new", Source: "master-0"},
		{Node: "master-2", VerifiedDigests: 412, Source: "master-0"},
	}

	assert.True(t, MayLead("master-0", replicas, "new", liveLease("master-0")), "the leader completes by itself")
	assert.False(t, MayLead("master-1", replicas, "new", liveLease("master-0")), "a full copy of it is no reason to move")
	assert.False(t, MayLead("master-2", replicas, "new", liveLease("master-0")))

	// Nobody can complete by itself — air-gapped, or the lease is not live — and the full replica leads,
	// as before: it is the only way the set gets back to where the others copy it from.
	assert.False(t, MayLead("master-0", replicas, "new", Lease{}))
	assert.True(t, MayLead("master-1", replicas, "new", Lease{}))

	// A leader whose store refuses writes cannot take the rest of the set: the full copy leads.
	refusing := append([]registryv1alpha1.StorageReplicaStatus(nil), replicas...)
	refusing[0].Store = &registryv1alpha1.StoreUsage{Writable: false, Reason: registryv1alpha1.StoreReserveExhausted}
	assert.False(t, MayLead("master-0", refusing, "new", liveLease("master-0")))
	assert.True(t, MayLead("master-1", refusing, "new", liveLease("master-0")))

	// A leader reporting an error is not completing anything.
	failing := append([]registryv1alpha1.StorageReplicaStatus(nil), replicas...)
	failing[0].Error = "MANIFEST_UNKNOWN"
	assert.False(t, MayLead("master-0", failing, "new", liveLease("master-0")))
	assert.True(t, MayLead("master-1", failing, "new", liveLease("master-0")))
}

// liveLease is a lease held by a replica that renews it, on a storage with an upstream.
func liveLease(holder string) Lease {
	return Lease{Known: true, Holder: holder, Live: true, CanFill: true}
}

// TestALeaderThatIsGoneDoesNotHoldTheElection is what a live cluster did when its leader's replica
// was removed while nobody was full yet: the others read the role Leader the removed replica had left
// in its report and stood aside for it for twelve minutes. The lease says who leads instead —
// released when the replica stepped down, it names nobody — and the caller drops the reports of
// replicas whose pod is gone, so the fullest of those still there begins.
func TestALeaderThatIsGoneDoesNotHoldTheElection(t *testing.T) {
	present := []registryv1alpha1.StorageReplicaStatus{
		{Node: "master-0", VerifiedDigests: 206},
		{Node: "master-2", VerifiedDigests: 18},
	}
	released := Lease{Known: true, CanFill: true}

	assert.True(t, MayLead("master-0", present, "", released), "nobody leads, and the fullest still here begins")
	assert.False(t, MayLead("master-2", present, "", released))

	// While a lease is held, or expired only a moment ago, its holder keeps leading even with nothing
	// complete: incomplete replicas do not take a fill over from one another.
	withLeader := append([]registryv1alpha1.StorageReplicaStatus{
		{Node: "master-1", VerifiedDigests: 114, Role: registryv1alpha1.ReplicaRoleLeader},
	}, present...)
	recent := Lease{Known: true, Holder: "master-1", CanFill: true}
	assert.True(t, MayLead("master-1", withLeader, "", recent))
	assert.False(t, MayLead("master-0", withLeader, "", recent))

	// The role alone no longer holds anyone back once the lease names nobody.
	assert.True(t, MayLead("master-0", withLeader, "", released))
	assert.False(t, MayLead("master-1", withLeader, "", released))

	// The lease unreadable: the reported roles decide, as they did before.
	assert.True(t, MayLead("master-1", withLeader, "", Lease{}))
	assert.False(t, MayLead("master-0", withLeader, "", Lease{}))
}
