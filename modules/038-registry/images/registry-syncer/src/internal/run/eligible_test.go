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
		// The counters may look complete while the last pass failed, and a replica that
		// reports an error is not full. So nobody is full, nobody leads, and the counts
		// decide, the error being no reason to pass it over: with nothing counted on
		// either side, the tie goes by name.
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
		// Both failing: back to "nobody is full", and a pass's error is no reason to
		// pass a replica over then. The tie on what they counted goes by name.
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
// `d8 mirror push`, which through the Ingress lands it on the lease holder. The failure to avoid
// is an empty replica holding the lease while a full one follows it, as after the lease moved
// off the replica the push filled — the leader cannot fill, the follower has nothing to copy
// from, and nothing recovers on its own.
func TestAirGapCannotGetStuck(t *testing.T) {
	// NeedSync, because a replica — master-0 — is short of what the leader holds: the controller
	// keeps it raised until every replica holds the set (FollowersBehind).
	airGap := &registryv1alpha1.RegistryStorageSpec{NeedSync: true}

	// `d8 mirror push` landed on master-1 while it held the lease, which has since moved to master-0.
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
	// the replica the push filled. Which is what the two assertions above check.
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

// TestACopyFromAGoneLeaderDoesNotDisqualify is stand H: the full leader master-2 was powered off while
// master-0 held 322 of 446 digests and master-1 none, and both carried the error of a copy from it. The
// empty replica's error went first, and for about 20 s it was the only candidate and stood; client-go's
// minute of watching the lease is all that kept it from leading and filling from scratch. The error says
// the copy failed, not what master-0 holds, and the fuller replica begins.
func TestACopyFromAGoneLeaderDoesNotDisqualify(t *testing.T) {
	const gone = "discovering what to copy: reading the image set of pr1110: " +
		"the image set could not be read from 10.12.4.204:5001"

	for _, lease := range []Lease{
		{Known: true, CanFill: true},                     // released, or expired past its grace
		{Known: true, CanFill: true, Holder: "master-2"}, // its holder's report dropped with its node
		{Known: true}, // air-gap: the fullest is all there is
		{},            // the lease unreadable
	} {
		emptyIsClean := []registryv1alpha1.StorageReplicaStatus{
			{Node: "master-0", Role: registryv1alpha1.ReplicaRoleFollower, Source: "master-2",
				VerifiedDigests: 322, DeclaredDigests: 446, Error: gone},
			{Node: "master-1", Role: registryv1alpha1.ReplicaRoleFollower, DeclaredDigests: 446},
		}
		assert.True(t, MayLead("master-0", emptyIsClean, "", lease), "lease %+v", lease)
		assert.False(t, MayLead("master-1", emptyIsClean, "", lease), "lease %+v", lease)

		both := append([]registryv1alpha1.StorageReplicaStatus(nil), emptyIsClean...)
		both[1].Source, both[1].Error = "master-2", gone
		assert.True(t, MayLead("master-0", both, "", lease), "both failing: the fuller still begins, lease %+v", lease)
		assert.False(t, MayLead("master-1", both, "", lease), "and only it, lease %+v", lease)
	}
}

// TestAReplicaThatCannotServeIsNotACandidate: a replica whose registry does not answer, or whose store
// refuses writes, cannot be what the others copy from, however much it holds. Its health watch says so
// in healthError, and that alone passes it over.
func TestAReplicaThatCannotServeIsNotACandidate(t *testing.T) {
	const failing = "this replica's store does not accept writes: read-only file system"
	replicas := []registryv1alpha1.StorageReplicaStatus{
		{Node: "master-0", VerifiedDigests: 428, Error: failing, HealthError: failing},
		{Node: "master-1", VerifiedDigests: 333},
	}

	assert.False(t, MayLead("master-0", replicas, "", Lease{}))
	assert.True(t, MayLead("master-1", replicas, "", Lease{}))

	// The same error from a pass, with the watch finding nothing: no reason to pass it over.
	replicas[0].HealthError = ""
	assert.True(t, MayLead("master-0", replicas, "", Lease{}))
	assert.False(t, MayLead("master-1", replicas, "", Lease{}))
}

// TestSomebodyLeadsWhenThereIsNothingToCompare keeps a fresh cluster from deadlocking: with no
// reports at all, refusing to lead is refusing to start. Nor with every replica unable to serve.
func TestSomebodyLeadsWhenThereIsNothingToCompare(t *testing.T) {
	assert.True(t, MayLead("master-0", nil, "", Lease{}))

	const failing = "this replica's registry is not answering: 10.0.0.1:5001: connection refused"
	unable := []registryv1alpha1.StorageReplicaStatus{
		{Node: "master-0", Error: failing, HealthError: failing},
		{Node: "master-1", Error: failing, HealthError: failing},
	}
	assert.True(t, MayLead("master-0", unable, "", Lease{}))
	assert.True(t, MayLead("master-1", unable, "", Lease{}))
}

// TestAStoreWithoutRoomIsNotACandidate: a store at its budget or at the node's reserve is not a failing
// one, so the health watch leaves it to store.writable, and the error its pass reports no longer passes
// it over. It cannot take the rest of the set though, from the upstream or from a push, and once it
// began it would keep the lease while nobody is full. The fullest replica with room begins instead.
func TestAStoreWithoutRoomIsNotACandidate(t *testing.T) {
	replicas := []registryv1alpha1.StorageReplicaStatus{
		{Node: "master-0", VerifiedDigests: 400,
			Error: "the destination store is full, stopped at registry.example/d8/install@sha256:0123: 507",
			Store: &registryv1alpha1.StoreUsage{Writable: false, Reason: registryv1alpha1.StoreReserveExhausted}},
		{Node: "master-1", VerifiedDigests: 300},
		{Node: "master-2", VerifiedDigests: 12},
	}

	for _, lease := range []Lease{
		{Known: true, CanFill: true}, // completes from the upstream
		{Known: true},                // air-gap: the push lands on the lease holder
		{},                           // the lease unreadable
	} {
		assert.False(t, MayLead("master-0", replicas, "", lease), "lease %+v", lease)
		assert.True(t, MayLead("master-1", replicas, "", lease), "lease %+v", lease)
		assert.False(t, MayLead("master-2", replicas, "", lease), "lease %+v", lease)
	}

	// With every store out of room, the fullest is still the best source of what there is.
	noRoom := append([]registryv1alpha1.StorageReplicaStatus(nil), replicas...)
	for i := range noRoom {
		noRoom[i].Store = &registryv1alpha1.StoreUsage{Writable: false, Reason: registryv1alpha1.StoreBudgetExhausted}
	}
	assert.True(t, MayLead("master-0", noRoom, "", Lease{Known: true, CanFill: true}))
	assert.False(t, MayLead("master-1", noRoom, "", Lease{Known: true, CanFill: true}))
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

// TestALeaderThatCannotServeStepsAside: a leader whose registry does not answer, or whose disk refuses
// writes, reports an error — through the health watch, while its pass cannot — and a full follower takes
// the lease, even with an upstream and a live lease. Without a full follower it keeps leading, as with
// any other error: moving the lease to an incomplete replica would only start the fill over.
func TestALeaderThatCannotServeStepsAside(t *testing.T) {
	const failing = "this replica's store does not accept writes: read-only file system"
	lease := liveLease("master-0")

	published := []registryv1alpha1.StorageReplicaStatus{
		{Node: "master-0", Role: registryv1alpha1.ReplicaRoleLeader, Full: true, Error: failing},
		{Node: "master-1", Full: true},
	}
	assert.False(t, MayLead("master-0", published, "", lease))
	assert.True(t, MayLead("master-1", published, "", lease), "the full follower takes over")

	alone := []registryv1alpha1.StorageReplicaStatus{
		{Node: "master-0", Role: registryv1alpha1.ReplicaRoleLeader, Full: true, Error: failing},
		{Node: "master-1", VerifiedDigests: 12},
	}
	assert.True(t, MayLead("master-0", alone, "", lease), "nobody full beside it: it keeps leading")
	assert.False(t, MayLead("master-1", alone, "", lease))

	single := alone[:1]
	assert.True(t, MayLead("master-0", single, "", lease), "nor does a single replica walk away from itself")
	assert.True(t, MayLead("master-0", single, "", Lease{Known: true}), "and it takes a lease nobody holds")
}

// TestALeaderJudgesItselfByItsOwnHealth: the status may not have caught up — the API server is what
// stands in the way, or the watch has not published yet — and the replica deciding about itself does
// not wait for it. Its report as it stands, clean and full, with what its health watch found.
func TestALeaderJudgesItselfByItsOwnHealth(t *testing.T) {
	const failing = "this replica's registry is not answering: 10.0.0.1:5001: connection refused"
	lease := liveLease("master-0")
	reported := []registryv1alpha1.StorageReplicaStatus{
		{Node: "master-0", Role: registryv1alpha1.ReplicaRoleLeader, Full: true},
		{Node: "master-1", Full: true},
	}

	assert.True(t, MayLead("master-0", WithOwnHealth("master-0", reported, ""), "", lease), "healthy: it leads")
	assert.False(t, MayLead("master-0", WithOwnHealth("master-0", reported, failing), "", lease),
		"in trouble: it steps aside for the full follower")
	assert.Empty(t, reported[0].Error, "the reports themselves are left alone")

	worse := []registryv1alpha1.StorageReplicaStatus{
		{Node: "master-0", Error: "3 of 459 references could not be copied"}, {Node: "master-1"},
	}
	judged := WithOwnHealth("master-0", worse, failing)
	assert.Equal(t, "3 of 459 references could not be copied", judged[0].Error, "an error the pass reported stays")
	assert.Equal(t, failing, judged[0].HealthError, "and what the watch found is beside it")
	assert.Empty(t, judged[1].Error, "and only its own entry is judged")
	assert.Empty(t, judged[1].HealthError)

	// Nobody full and nobody leading: the fullest begins, unless it is this replica and cannot serve —
	// which it knows before the status does.
	alone := []registryv1alpha1.StorageReplicaStatus{
		{Node: "master-0", VerifiedDigests: 428, Error: "3 of 459 references could not be copied"},
		{Node: "master-1", VerifiedDigests: 333},
	}
	released := Lease{Known: true, CanFill: true}
	assert.True(t, MayLead("master-0", alone, "", released))
	assert.False(t, MayLead("master-0", WithOwnHealth("master-0", alone, failing), "", released))
}

// TestWhyNotLeadNamesTheReplica: the reason is what a replica logs when it steps aside, and it has to
// be the one that decided — above all not a full replica that does not exist, which is what a replica
// on a stand logged while nobody was full.
func TestWhyNotLeadNamesTheReplica(t *testing.T) {
	const failing = "this replica's store does not accept writes: read-only file system"
	cases := []struct {
		name     string
		replicas []registryv1alpha1.StorageReplicaStatus
		lease    Lease
		says     []string
		full     bool
	}{{
		name: "a leader that completes by itself",
		replicas: []registryv1alpha1.StorageReplicaStatus{
			{Node: "master-0", Full: true},
			{Node: "master-2", Role: registryv1alpha1.ReplicaRoleLeader, VerifiedDigests: 397},
			{Node: "master-1", Full: true},
		},
		lease: liveLease("master-2"),
		says:  []string{"master-2 leads and completes the set from the upstream by itself"},
		full:  true,
	}, {
		name: "a full replica",
		replicas: []registryv1alpha1.StorageReplicaStatus{
			{Node: "master-0", VerifiedDigests: 12},
			{Node: "master-1", Full: true, VerifiedDigests: 446},
		},
		says: []string{"master-1 holds the whole image set and this replica does not"},
		full: true,
	}, {
		name: "an incumbent while nobody is full",
		replicas: []registryv1alpha1.StorageReplicaStatus{
			{Node: "master-0", VerifiedDigests: 428},
			{Node: "master-1", VerifiedDigests: 333},
		},
		lease: Lease{Known: true, Holder: "master-1", CanFill: true},
		says:  []string{"master-1 leads", "nobody holds the whole image set"},
	}, {
		name: "the fullest while nobody leads",
		replicas: []registryv1alpha1.StorageReplicaStatus{
			{Node: "master-0", VerifiedDigests: 12},
			{Node: "master-1", VerifiedDigests: 322, Error: "the image set could not be read from 10.12.4.204:5001"},
		},
		lease: Lease{Known: true, CanFill: true},
		says:  []string{"master-1 holds the most of it (322 digests)"},
	}, {
		name: "this replica cannot serve",
		replicas: []registryv1alpha1.StorageReplicaStatus{
			{Node: "master-0", VerifiedDigests: 428, Error: failing, HealthError: failing},
			{Node: "master-1", VerifiedDigests: 333},
		},
		lease: Lease{Known: true, CanFill: true},
		says:  []string{"this replica cannot serve what it holds: " + failing},
	}, {
		name: "this replica's store has no room",
		replicas: []registryv1alpha1.StorageReplicaStatus{
			{Node: "master-0", VerifiedDigests: 428,
				Store: &registryv1alpha1.StoreUsage{Writable: false, Reason: registryv1alpha1.StoreBudgetExhausted}},
			{Node: "master-1", VerifiedDigests: 333},
		},
		lease: Lease{Known: true, CanFill: true},
		says: []string{"this replica's store refuses writes (BudgetExhausted)",
			"master-1 holds the most among those that can (333 digests)"},
	}}

	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			why := WhyNotLead("master-0", test.replicas, "", test.lease)
			for _, said := range test.says {
				assert.Contains(t, why, said)
			}
			if !test.full {
				assert.NotRegexp(t, `master-\d holds the whole image set`, why, "nobody does")
			}
			assert.False(t, MayLead("master-0", test.replicas, "", test.lease))
		})
	}
}
