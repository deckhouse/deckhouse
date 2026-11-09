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

package layout

import (
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	registryv1alpha1 "github.com/deckhouse/deckhouse/go_lib/registry/apis/deckhouse.io/v1alpha1"
)

func passThroughSpec() *registryv1alpha1.RegistryStorageSpec {
	return &registryv1alpha1.RegistryStorageSpec{
		Upstream: testUpstream("registry.deckhouse.io"),
		Source:   testSource(),
	}
}

func airGapSpec() *registryv1alpha1.RegistryStorageSpec {
	return &registryv1alpha1.RegistryStorageSpec{Source: testSource()}
}

// syncerStart is when the syncer on the lease holder's node started, in these tests, and what leader()
// stamps its report with: a report the running syncer wrote after reading its store.
var syncerStart = metav1.NewTime(time.Date(2026, 10, 30, 9, 0, 0, 0, time.UTC))

// running is syncerStart, as the controller reads it off the pod.
func running() *metav1.Time {
	started := syncerStart
	return &started
}

func leader(node string, full bool, digests int32) registryv1alpha1.StorageReplicaStatus {
	return registryv1alpha1.StorageReplicaStatus{
		Node: node, Role: registryv1alpha1.ReplicaRoleLeader, Full: full, VerifiedDigests: digests,
		SyncerStartedAt: running(),
	}
}

func follower(node string, full bool, digests int32, from string) registryv1alpha1.StorageReplicaStatus {
	return registryv1alpha1.StorageReplicaStatus{
		Node: node, Role: registryv1alpha1.ReplicaRoleFollower, Full: full, VerifiedDigests: digests, Source: from,
	}
}

func TestAggregate(t *testing.T) {
	tests := []struct {
		name     string
		spec     *registryv1alpha1.RegistryStorageSpec
		replicas []registryv1alpha1.StorageReplicaStatus

		// holder is the identity in the election lease, which is what decides who leads.
		// Empty means nobody holds it — not "whoever the reports say", which is the whole
		// point: a replica that has gone away leaves its claim behind.
		holder string

		wantPhase         registryv1alpha1.StoragePhase
		wantLeader        string
		wantSafeToDrop    bool
		wantAllFull       bool
		wantAuthoritative bool
	}{
		{
			name: "nothing has reported yet",
			spec: passThroughSpec(),
			// Idle rather than Ready: Ready authorizes dropping the upstream, so an
			// absent report must never read as a complete cache.
			wantPhase: registryv1alpha1.StoragePhaseIdle,
		},
		{
			// The case the syncer's announcement exists for: a leader that has said "I have
			// started, and I hold none of it yet". Without the announcement there is no report
			// here at all, and the phase reads `Idle` for the whole of a first fill, with the
			// entire image set being written underneath.
			name: "the leader has announced a fill and holds nothing yet",
			spec: passThroughSpec(),
			replicas: []registryv1alpha1.StorageReplicaStatus{
				{
					Node:            "master-0",
					Role:            registryv1alpha1.ReplicaRoleLeader,
					DeclaredDigests: 443,
				},
			},

			holder:     "master-0",
			wantPhase:  registryv1alpha1.StoragePhaseFilling,
			wantLeader: "master-0",
		},
		{
			name:     "the leader is filling",
			spec:     passThroughSpec(),
			replicas: []registryv1alpha1.StorageReplicaStatus{leader("master-0", false, 312)},

			holder:     "master-0",
			wantPhase:  registryv1alpha1.StoragePhaseFilling,
			wantLeader: "master-0",
		},
		{
			name: "the leader is full while the followers catch up",
			spec: passThroughSpec(),
			replicas: []registryv1alpha1.StorageReplicaStatus{
				leader("master-0", true, 459),
				follower("master-1", false, 210, "master-0"),
			},

			// Ready on the leader alone: that is the gate, and the followers are
			// filled ahead of time in parallel.
			holder:         "master-0",
			wantPhase:      registryv1alpha1.StoragePhaseReady,
			wantLeader:     "master-0",
			wantSafeToDrop: true,
			wantAllFull:    false,
		},
		{
			name: "every replica is full",
			spec: passThroughSpec(),
			replicas: []registryv1alpha1.StorageReplicaStatus{
				leader("master-0", true, 459),
				follower("master-1", true, 459, "master-0"),
				follower("master-2", true, 459, "master-0"),
			},

			holder:         "master-0",
			wantPhase:      registryv1alpha1.StoragePhaseReady,
			wantLeader:     "master-0",
			wantSafeToDrop: true,
			wantAllFull:    true,
		},
		{
			name: "the leader failed",
			spec: passThroughSpec(),
			replicas: []registryv1alpha1.StorageReplicaStatus{
				{Node: "master-0", Role: registryv1alpha1.ReplicaRoleLeader, VerifiedDigests: 100, Error: "401 from upstream"},
			},

			holder:     "master-0",
			wantPhase:  registryv1alpha1.StoragePhaseFailed,
			wantLeader: "master-0",
			// The whole point: a failing leader must not authorize going air-gap.
			wantSafeToDrop: false,
		},
		{
			name: "the leader claims full but reports an error",
			spec: passThroughSpec(),
			replicas: []registryv1alpha1.StorageReplicaStatus{
				{
					Node: "master-0", Role: registryv1alpha1.ReplicaRoleLeader,
					Full: true, VerifiedDigests: 459, Error: "verification failed: 12 digests missing",
				},
			},

			holder:     "master-0",
			wantPhase:  registryv1alpha1.StoragePhaseFailed,
			wantLeader: "master-0",
			// The reported field has to agree with the gate the transition reads,
			// otherwise the status claims "safe" while the controller refuses.
			wantSafeToDrop: false,
			wantAllFull:    true,
		},
		{
			name: "a follower failed while the leader is still filling",
			spec: passThroughSpec(),
			replicas: []registryv1alpha1.StorageReplicaStatus{
				leader("master-0", false, 300),
				{Node: "master-1", Role: registryv1alpha1.ReplicaRoleFollower, Error: "no space left on device"},
			},

			holder:     "master-0",
			wantPhase:  registryv1alpha1.StoragePhaseFailed,
			wantLeader: "master-0",
		},
		{
			name: "a follower failed after the leader became full",
			spec: passThroughSpec(),
			replicas: []registryv1alpha1.StorageReplicaStatus{
				leader("master-0", true, 459),
				{Node: "master-1", Role: registryv1alpha1.ReplicaRoleFollower, Error: "no space left on device"},
			},

			// The leader is the source of truth for readiness; a broken follower
			// costs redundancy, not availability.
			holder:         "master-0",
			wantPhase:      registryv1alpha1.StoragePhaseReady,
			wantLeader:     "master-0",
			wantSafeToDrop: true,
		},
		{
			name: "replicas exist but none holds the lease",
			spec: passThroughSpec(),
			replicas: []registryv1alpha1.StorageReplicaStatus{
				follower("master-1", true, 459, "master-0"),
				follower("master-2", true, 459, "master-0"),
			},

			// During a leader change there is nothing whose completeness could
			// authorize the transition, even though every replica reports full.
			wantPhase:      registryv1alpha1.StoragePhaseIdle,
			wantLeader:     "",
			wantSafeToDrop: false,
			wantAllFull:    true,
		},
		{
			// The case that matters: the lease has moved while the departed replica's entry
			// goes on saying Leader. Two entries claim it, and only the lease can say which
			// claim is current.
			name: "a departed replica still claims the leadership it lost",
			spec: passThroughSpec(),
			replicas: []registryv1alpha1.StorageReplicaStatus{
				// Gone, and full as of when it left. Nothing can retract this.
				leader("master-0", true, 459),
				// The replica that actually holds the lease now, still filling.
				leader("master-1", false, 120),
			},
			holder: "master-1",

			// Read by array order, the stale entry came first and answered "full" — which
			// would have taken the upstream away while the real leader held 120 of 459.
			wantPhase:      registryv1alpha1.StoragePhaseFilling,
			wantLeader:     "master-1",
			wantSafeToDrop: false,
		},
		{
			name: "the replica holding the lease has not reported yet",
			spec: passThroughSpec(),
			replicas: []registryv1alpha1.StorageReplicaStatus{
				leader("master-0", true, 459),
				follower("master-1", true, 459, "master-0"),
			},
			// A replica that has taken the lease but not yet written its entry.
			holder: "master-2",

			// Nothing to say about the leader, so nothing that could authorize anything.
			wantPhase:      registryv1alpha1.StoragePhaseIdle,
			wantLeader:     "",
			wantSafeToDrop: false,
			wantAllFull:    true,
		},
		{
			name:              "the cache is authoritative once no upstream is left",
			spec:              airGapSpec(),
			replicas:          []registryv1alpha1.StorageReplicaStatus{leader("master-0", true, 459)},
			holder:            "master-0",
			wantPhase:         registryv1alpha1.StoragePhaseReady,
			wantLeader:        "master-0",
			wantSafeToDrop:    true,
			wantAllFull:       true,
			wantAuthoritative: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Nothing names the upstream here, so the table is about the leader alone; the
			// other half of the gate is TestSafeToDropUpstreamWaitsForTheReferences.
			nothing := int32(0)
			got := Aggregate(tt.spec, tt.replicas, tt.holder, &nothing, running())

			assert.Equal(t, tt.wantPhase, got.Phase)
			assert.Equal(t, tt.wantLeader, got.Leader)
			assert.Equal(t, tt.wantSafeToDrop, got.SafeToDropUpstream)
			assert.Equal(t, tt.wantAllFull, got.AllReplicasFull)
			assert.Equal(t, tt.wantAuthoritative, got.Authoritative)
		})
	}
}

func TestAggregateFillProgress(t *testing.T) {
	got := Aggregate(passThroughSpec(), []registryv1alpha1.StorageReplicaStatus{leader("master-0", false, 312)}, "master-0", nil, running())

	require.NotNil(t, got.Fill)
	assert.EqualValues(t, 312, got.Fill.Filled)
	assert.EqualValues(t, 459, got.Fill.Total)

	// With an upstream and no air-gap declaration, the leader's own count of the set is the
	// denominator. This is the ordinary cluster, and it used to report nothing at all — the whole set
	// moving against `0/0` — because the only denominator the controller had was a number an operator
	// states when declaring air-gap.
	fromLeader := Aggregate(
		&registryv1alpha1.RegistryStorageSpec{Upstream: testUpstream("registry.deckhouse.io")},
		[]registryv1alpha1.StorageReplicaStatus{withDeclared(leader("master-0", false, 312), 400)},
		"master-0", nil, running(),
	)
	require.NotNil(t, fromLeader.Fill)
	assert.EqualValues(t, 312, fromLeader.Fill.Filled)
	assert.EqualValues(t, 400, fromLeader.Fill.Total)

	// The declaration wins while there is one: a transition gated on a stated number has to show
	// progress against that number, not against one the cluster computed about itself.
	declared := Aggregate(passThroughSpec(), []registryv1alpha1.StorageReplicaStatus{withDeclared(leader("master-0", false, 312), 400)}, "master-0", nil, running())
	require.NotNil(t, declared.Fill)
	assert.EqualValues(t, 459, declared.Fill.Total)

	// And a leader that has not counted the set yet reports no progress rather than "312 of 0",
	// which would read as a store that shrank.
	noDenominator := Aggregate(
		&registryv1alpha1.RegistryStorageSpec{Upstream: testUpstream("registry.deckhouse.io")},
		[]registryv1alpha1.StorageReplicaStatus{leader("master-0", false, 312)},
		"master-0", nil, running(),
	)
	assert.Nil(t, noDenominator.Fill)
}

// A full leader is half of the gate. The other half is that nothing on the platform still names
// the upstream: such a reference has no route once the upstream is gone, whatever the cache holds.
// Measured on the `scenarios` unit on 2026-10-05, where the upstream went 70 seconds after the
// platform began moving off it and a pod that restarted in between never started again.
func TestSafeToDropUpstreamWaitsForTheReferences(t *testing.T) {
	full := []registryv1alpha1.StorageReplicaStatus{leader("master-0", true, 459)}
	count := func(n int32) *int32 { return &n }

	tests := []struct {
		name       string
		spec       *registryv1alpha1.RegistryStorageSpec
		replicas   []registryv1alpha1.StorageReplicaStatus
		references *int32
		want       bool
	}{
		{
			name: "a full leader and nothing naming the upstream", spec: passThroughSpec(),
			replicas: full, references: count(0), want: true,
		},
		{
			name: "a full leader while the platform still names the upstream", spec: passThroughSpec(),
			replicas: full, references: count(3), want: false,
		},
		{
			// The count failed, or was never taken. No count is no permission.
			name: "a full leader and no count at all", spec: passThroughSpec(),
			replicas: full, references: nil, want: false,
		},
		{
			name: "nothing naming the upstream does not make up for an incomplete leader", spec: passThroughSpec(),
			replicas:   []registryv1alpha1.StorageReplicaStatus{leader("master-0", false, 312)},
			references: count(0), want: false,
		},
		{
			// Already air-gapped: there is no upstream left for anything to name.
			name: "no upstream left to count against", spec: airGapSpec(),
			replicas: full, references: nil, want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, SafeToDropUpstream(tt.spec, tt.replicas, "master-0", tt.references, running()))

			got := Aggregate(tt.spec, tt.replicas, "master-0", tt.references, running())
			assert.Equal(t, tt.want, got.SafeToDropUpstream, "the status reports the same gate")
			if tt.spec.Upstream == nil {
				assert.Nil(t, got.UpstreamReferences, "with no upstream there is no count to report")
			} else {
				assert.Equal(t, tt.references, got.UpstreamReferences)
			}
		})
	}
}

func withDeclared(replica registryv1alpha1.StorageReplicaStatus, declared int32) registryv1alpha1.StorageReplicaStatus {
	replica.DeclaredDigests = declared
	return replica
}

// TestLeaderFull covers the gate the air-gap transition reads. It is deliberately
// derived from the reports rather than from the aggregate, so a stale or
// hand-edited aggregate cannot authorize the transition — and from the lease rather than from
// the Role in a report, so a replica that has gone away cannot authorize it either.
func TestLeaderFull(t *testing.T) {
	tests := []struct {
		name     string
		replicas []registryv1alpha1.StorageReplicaStatus
		holder   string
		want     bool
	}{
		{name: "no replicas", holder: "master-0", want: false},
		{
			name:     "the leader is full",
			replicas: []registryv1alpha1.StorageReplicaStatus{leader("master-0", true, 459)},
			holder:   "master-0",
			want:     true,
		},
		{
			name:     "the leader is filling",
			replicas: []registryv1alpha1.StorageReplicaStatus{leader("master-0", false, 312)},
			holder:   "master-0",
			want:     false,
		},
		{
			name: "the leader claims full but reports an error",
			replicas: []registryv1alpha1.StorageReplicaStatus{
				{Node: "master-0", Role: registryv1alpha1.ReplicaRoleLeader, Full: true, Error: "verification failed"},
			},
			holder: "master-0",
			// A replica that both claims completeness and reports a failure is not
			// trustworthy enough to cut the cluster off its upstream.
			want: false,
		},
		{
			name: "only full followers",
			replicas: []registryv1alpha1.StorageReplicaStatus{
				follower("master-1", true, 459, "master-0"),
			},
			holder: "master-0",
			want:   false,
		},
		{
			// The defect this predicate had: a full entry left behind by a replica that is
			// gone would take the upstream away while the replica that actually leads is
			// still filling. It is the only decision in this module that cannot be undone
			// — in an air-gapped cluster the upstream it dropped answers nothing.
			name: "a full entry left behind by a replica that no longer leads",
			replicas: []registryv1alpha1.StorageReplicaStatus{
				leader("master-0", true, 459),
				leader("master-1", false, 120),
			},
			holder: "master-1",
			want:   false,
		},
		{
			name: "the same, with the current leader full",
			replicas: []registryv1alpha1.StorageReplicaStatus{
				leader("master-0", false, 10),
				leader("master-1", true, 459),
			},
			holder: "master-1",
			want:   true,
		},
		{
			name:     "nobody holds the lease",
			replicas: []registryv1alpha1.StorageReplicaStatus{leader("master-0", true, 459)},
			holder:   "",
			want:     false,
		},
		{
			name:     "the holder has not reported",
			replicas: []registryv1alpha1.StorageReplicaStatus{leader("master-0", true, 459)},
			holder:   "master-2",
			want:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, LeaderFull(tt.replicas, tt.holder))
		})
	}
}

// TestLeaderReportCurrent is defect 52 at the gate: a leader recreated on the same node takes its own
// lease back at once, and until its new syncer reads the store its entry is the previous process's —
// full over a store that may have been wiped with the pod. Only an entry stamped no earlier than the
// syncer container running on the holder's node counts.
func TestLeaderReportCurrent(t *testing.T) {
	stamped := func(at *metav1.Time) []registryv1alpha1.StorageReplicaStatus {
		entry := leader("master-0", true, 459)
		entry.SyncerStartedAt = at
		return []registryv1alpha1.StorageReplicaStatus{entry}
	}
	at := func(offset time.Duration) *metav1.Time {
		stamp := metav1.NewTime(syncerStart.Add(offset))
		return &stamp
	}

	tests := []struct {
		name     string
		replicas []registryv1alpha1.StorageReplicaStatus
		holder   string
		running  *metav1.Time
		want     bool
	}{
		{name: "nobody holds the lease", replicas: stamped(running()), running: running(), want: false},
		{
			// No Ready pod on the holder's node, or one whose syncer is not running: no evidence.
			name: "no syncer running on the holder's node", replicas: stamped(running()), holder: "master-0",
			running: nil, want: false,
		},
		{
			// Either a syncer that has not read its store yet, or one from before the field existed.
			name: "no stamp", replicas: stamped(nil), holder: "master-0", running: running(), want: false,
		},
		{
			name: "stamped by the previous process", replicas: stamped(at(-time.Minute)), holder: "master-0",
			running: running(), want: false,
		},
		{
			// The stamp is the process's start, which comes after its container's, but both are whole
			// seconds and may fall into the same one.
			name: "stamped in the second its container started", replicas: stamped(running()), holder: "master-0",
			running: running(), want: true,
		},
		{
			name: "stamped by the running process", replicas: stamped(at(3 * time.Second)), holder: "master-0",
			running: running(), want: true,
		},
		{
			name: "the holder has not reported", replicas: stamped(running()), holder: "master-1",
			running: running(), want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, LeaderReportCurrent(tt.replicas, tt.holder, tt.running))
		})
	}
}

// TestSafeToDropUpstreamWantsTheRunningSyncersReport: the status reports the same gate the transition
// is held on, so a full report from the previous process does not read as safe either.
func TestSafeToDropUpstreamWantsTheRunningSyncersReport(t *testing.T) {
	nothing := int32(0)
	stale := leader("master-0", true, 459)
	before := metav1.NewTime(syncerStart.Add(-time.Minute))
	stale.SyncerStartedAt = &before
	replicas := []registryv1alpha1.StorageReplicaStatus{stale}

	assert.False(t, SafeToDropUpstream(passThroughSpec(), replicas, "master-0", &nothing, running()))
	got := Aggregate(passThroughSpec(), replicas, "master-0", &nothing, running())
	assert.False(t, got.SafeToDropUpstream)
	assert.Equal(t, registryv1alpha1.StoragePhaseReady, got.Phase, "the phase is the leader's report, as before")

	replicas[0].SyncerStartedAt = running()
	assert.True(t, SafeToDropUpstream(passThroughSpec(), replicas, "master-0", &nothing, running()))
	assert.True(t, Aggregate(passThroughSpec(), replicas, "master-0", &nothing, running()).SafeToDropUpstream)
}

func TestStoreWritable(t *testing.T) {
	measured := func(node string, writable bool, reason registryv1alpha1.StoreRefusal) registryv1alpha1.StorageReplicaStatus {
		return registryv1alpha1.StorageReplicaStatus{
			Node:  node,
			Store: &registryv1alpha1.StoreUsage{Writable: writable, Reason: reason},
		}
	}

	t.Run("unknown until measured", func(t *testing.T) {
		condition := StoreWritable([]registryv1alpha1.StorageReplicaStatus{{Node: "master-0"}}, 3)
		assert.Equal(t, metav1.ConditionUnknown, condition.Status)
		assert.Equal(t, int64(3), condition.ObservedGeneration)
	})

	t.Run("true when every replica writes", func(t *testing.T) {
		condition := StoreWritable([]registryv1alpha1.StorageReplicaStatus{
			measured("master-0", true, ""), measured("master-1", true, ""), {Node: "master-2"},
		}, 1)
		assert.Equal(t, metav1.ConditionTrue, condition.Status)
		assert.Equal(t, registryv1alpha1.ReasonStoreWritable, condition.Reason)
	})

	t.Run("false names every refusing node and its way out", func(t *testing.T) {
		condition := StoreWritable([]registryv1alpha1.StorageReplicaStatus{
			measured("master-2", false, registryv1alpha1.StoreReserveExhausted),
			measured("master-0", true, ""),
			measured("master-1", false, registryv1alpha1.StoreReserveExhausted),
		}, 1)
		assert.Equal(t, metav1.ConditionFalse, condition.Status)
		assert.Equal(t, "ReserveExhausted", condition.Reason)
		assert.Contains(t, condition.Message, "master-1, master-2")
		assert.Contains(t, condition.Message, "dedicated disk")
		assert.NotContains(t, condition.Message, "master-0")
	})

	t.Run("both limits at once", func(t *testing.T) {
		condition := StoreWritable([]registryv1alpha1.StorageReplicaStatus{
			measured("master-0", false, registryv1alpha1.StoreBudgetExhausted),
			measured("master-1", false, registryv1alpha1.StoreBudgetExhausted),
			measured("master-2", false, registryv1alpha1.StoreReserveExhausted),
		}, 1)
		assert.Equal(t, "BudgetExhausted", condition.Reason, "the limit most replicas hit")
		assert.Contains(t, condition.Message, "storage.size")
		assert.Contains(t, condition.Message, "dedicated disk")
	})

	// On a disk of the store's own the reserve is the margin alone: the disk is full, and moving the
	// store to a dedicated disk is what the operator has already done.
	t.Run("a store on its own disk is told to grow it", func(t *testing.T) {
		ownDisk := measured("master-1", false, registryv1alpha1.StoreReserveExhausted)
		ownDisk.Store.DedicatedFilesystem = true
		condition := StoreWritable([]registryv1alpha1.StorageReplicaStatus{
			measured("master-0", false, registryv1alpha1.StoreReserveExhausted), ownDisk,
		}, 1)
		assert.Equal(t, "ReserveExhausted", condition.Reason)
		assert.Contains(t, condition.Message, "on master-0 is down to the space kept free for the node")
		assert.Contains(t, condition.Message, "own disk on master-1 is full: grow the disk")
		assert.NotContains(t, condition.Message, "on master-0, master-1")

		alone := StoreWritable([]registryv1alpha1.StorageReplicaStatus{ownDisk}, 1)
		assert.NotContains(t, alone.Message, "dedicated disk")
	})

	// No room helps a store the registry cannot write to, so it is the reason however many replicas
	// are at a limit, and the message sends the operator to the replica's error for the command.
	t.Run("a store that is not writable wins over the limits", func(t *testing.T) {
		condition := StoreWritable([]registryv1alpha1.StorageReplicaStatus{
			measured("master-0", false, registryv1alpha1.StoreBudgetExhausted),
			measured("master-1", false, registryv1alpha1.StoreBudgetExhausted),
			measured("master-2", false, registryv1alpha1.StoreNotWritable),
		}, 1)
		assert.Equal(t, metav1.ConditionFalse, condition.Status)
		assert.Equal(t, "NotWritable", condition.Reason)
		assert.Contains(t, condition.Message, "cannot create files in its tree in the store on master-2,")
		assert.True(t, strings.HasPrefix(condition.Message, "Writes are refused; "),
			"not \"reads still served\": a pull the store would keep fails")
		assert.Contains(t, condition.Message, "status.replicas[].error")
		assert.Contains(t, condition.Message, "on master-0, master-1 holds as much as storage.size allows",
			"the limits are still named")

		alone := StoreWritable([]registryv1alpha1.StorageReplicaStatus{
			measured("master-1", false, registryv1alpha1.StoreNotWritable),
			measured("master-0", false, registryv1alpha1.StoreNotWritable),
		}, 1)
		assert.Contains(t, alone.Message, "on master-0, master-1,")
		assert.NotContains(t, alone.Message, "storage.size")
		assert.NotContains(t, alone.Message, "garbage collection", "no collection helps it")

		limits := StoreWritable([]registryv1alpha1.StorageReplicaStatus{
			measured("master-0", false, registryv1alpha1.StoreBudgetExhausted),
		}, 1)
		assert.True(t, strings.HasPrefix(limits.Message, "Writes are refused and reads still served; "))
	})
}

func TestFollowersBehind(t *testing.T) {
	full := func(r registryv1alpha1.StorageReplicaStatus, set string) registryv1alpha1.StorageReplicaStatus {
		r.VerifiedSet = set
		return r
	}
	masters := []string{"master-0", "master-1", "master-2"}

	tests := []struct {
		name     string
		replicas []registryv1alpha1.StorageReplicaStatus
		holder   string
		nodes    []string
		want     bool
	}{
		{
			name: "every follower holds the leader's set",
			replicas: []registryv1alpha1.StorageReplicaStatus{
				full(leader("master-0", true, 414), "a"),
				full(follower("master-1", true, 414, "master-0"), "a"),
				full(follower("master-2", true, 414, "master-0"), "a"),
			},
			holder: "master-0", nodes: masters, want: false,
		},
		{
			// The lease moved to master-1. The set did not change, but the new leader may hold what
			// was pushed to it, and the followers have not copied from it: master-2 replicated from
			// the previous leader, master-0 led and replicated from nobody. Seen in multimaster-airgap.
			name: "the lease moved and the followers replicated from the previous leader",
			replicas: []registryv1alpha1.StorageReplicaStatus{
				full(follower("master-0", true, 414, ""), "a"),
				full(leader("master-1", true, 426), "a"),
				full(follower("master-2", true, 414, "master-0"), "a"),
			},
			holder: "master-1", nodes: masters, want: true,
		},
		{
			name: "after the lease moved, every follower replicated from the new leader",
			replicas: []registryv1alpha1.StorageReplicaStatus{
				full(follower("master-0", true, 426, "master-1"), "a"),
				full(leader("master-1", true, 426), "a"),
				full(follower("master-2", true, 426, "master-1"), "a"),
			},
			holder: "master-1", nodes: masters, want: false,
		},
		{
			name: "a follower is still replicating",
			replicas: []registryv1alpha1.StorageReplicaStatus{
				full(leader("master-0", true, 414), "a"),
				full(follower("master-1", true, 414, "master-0"), "a"),
				follower("master-2", false, 120, "master-0"),
			},
			holder: "master-0", nodes: masters, want: true,
		},
		{
			// A release or a module changed the set: the leader verifies the new one, the followers
			// were full against the old.
			name: "the followers are full against a set the leader has moved past",
			replicas: []registryv1alpha1.StorageReplicaStatus{
				full(leader("master-0", true, 430), "b"),
				full(follower("master-1", true, 414, "master-0"), "a"),
				full(follower("master-2", true, 414, "master-0"), "a"),
			},
			holder: "master-0", nodes: masters, want: true,
		},
		{
			name: "the leader is still filling",
			replicas: []registryv1alpha1.StorageReplicaStatus{
				leader("master-0", false, 200),
				full(follower("master-1", true, 414, "master-0"), "a"),
			},
			holder: "master-0", nodes: masters[:2], want: true,
		},
		{
			name: "a new master has not reported yet",
			replicas: []registryv1alpha1.StorageReplicaStatus{
				full(leader("master-0", true, 414), "a"),
				full(follower("master-1", true, 414, "master-0"), "a"),
			},
			holder: "master-0", nodes: masters, want: true,
		},
		{
			// The entry of a master that is gone stays in the status, and would otherwise keep every
			// follower replicating for as long as the cluster runs.
			name: "an entry left behind by a master that is gone",
			replicas: []registryv1alpha1.StorageReplicaStatus{
				full(leader("master-0", true, 414), "a"),
				full(follower("master-1", true, 414, "master-0"), "a"),
				follower("master-9", false, 10, "master-0"),
			},
			holder: "master-0", nodes: masters[:2], want: false,
		},
		{
			name: "a single master has no followers",
			replicas: []registryv1alpha1.StorageReplicaStatus{
				leader("master-0", false, 10),
			},
			holder: "master-0", nodes: masters[:1], want: false,
		},
		{
			name: "no leader, nothing to replicate from",
			replicas: []registryv1alpha1.StorageReplicaStatus{
				follower("master-1", false, 10, ""),
			},
			holder: "", nodes: masters, want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, FollowersBehind(tt.replicas, tt.holder, tt.nodes))
		})
	}
}

// TestStorageEndpointInUse is what keeps the agents where they are while the lease has no holder,
// which it has on every planned handover: the endpoint the node layouts carry, as long as it is still
// one of the replicas, and the same answer however the layouts happen to be listed.
func TestStorageEndpointInUse(t *testing.T) {
	addresses := []string{"10.0.0.1:5001", "10.0.0.2:5001", "10.0.0.3:5001"}

	pointedAt := func(host string) registryv1alpha1.RegistryNodeSpec {
		return registryv1alpha1.RegistryNodeSpec{
			Cache: true,
			Backends: []registryv1alpha1.Backend{
				{
					Name:     registryv1alpha1.BackendStorage,
					Endpoint: registryv1alpha1.Endpoint{Scheme: registryv1alpha1.SchemeHTTPS, Host: host},
				},
				{
					Name: registryv1alpha1.BackendUpstream,
					Endpoint: registryv1alpha1.Endpoint{
						Scheme: registryv1alpha1.SchemeHTTPS, Host: "registry.deckhouse.io",
					},
				},
			},
		}
	}
	direct := registryv1alpha1.RegistryNodeSpec{
		Backends: []registryv1alpha1.Backend{{
			Name:     registryv1alpha1.BackendUpstream,
			Endpoint: registryv1alpha1.Endpoint{Scheme: registryv1alpha1.SchemeHTTPS, Host: "10.0.0.1:5001"},
		}},
	}

	tests := []struct {
		name  string
		nodes []registryv1alpha1.RegistryNodeSpec
		want  string
	}{
		{
			name:  "the endpoint every layout carries",
			nodes: []registryv1alpha1.RegistryNodeSpec{pointedAt("10.0.0.3:5001"), pointedAt("10.0.0.3:5001")},
			want:  "10.0.0.3:5001",
		},
		{
			name: "no layouts yet",
			want: "",
		},
		{
			name:  "an endpoint that is no longer a replica",
			nodes: []registryv1alpha1.RegistryNodeSpec{pointedAt("10.0.0.9:5001")},
			want:  "",
		},
		{
			name:  "layouts without a storage backend say nothing, even naming a replica's host",
			nodes: []registryv1alpha1.RegistryNodeSpec{direct},
			want:  "",
		},
		{
			name: "the endpoint most layouts carry",
			nodes: []registryv1alpha1.RegistryNodeSpec{
				pointedAt("10.0.0.1:5001"), pointedAt("10.0.0.3:5001"), pointedAt("10.0.0.3:5001"),
			},
			want: "10.0.0.3:5001",
		},
		{
			name: "only replicas still in use are counted",
			nodes: []registryv1alpha1.RegistryNodeSpec{
				pointedAt("10.0.0.9:5001"), pointedAt("10.0.0.9:5001"), pointedAt("10.0.0.2:5001"),
			},
			want: "10.0.0.2:5001",
		},
		{
			name: "a tie goes to the smallest address",
			nodes: []registryv1alpha1.RegistryNodeSpec{
				pointedAt("10.0.0.3:5001"), pointedAt("10.0.0.2:5001"),
			},
			want: "10.0.0.2:5001",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, StorageEndpointInUse(tt.nodes, addresses))

			// The same layouts listed the other way round: the answer must not depend on it.
			reversed := make([]registryv1alpha1.RegistryNodeSpec, len(tt.nodes))
			for i := range tt.nodes {
				reversed[len(tt.nodes)-1-i] = tt.nodes[i]
			}
			assert.Equal(t, tt.want, StorageEndpointInUse(reversed, addresses))
		})
	}
}

// TestHoldsLeadersSet is the per-replica half of FollowersBehind on its own, as the storage update
// reads it to decide whether a follower can take the lease over.
func TestHoldsLeadersSet(t *testing.T) {
	withSet := func(r registryv1alpha1.StorageReplicaStatus, set string) *registryv1alpha1.StorageReplicaStatus {
		r.VerifiedSet = set
		return &r
	}
	failed := func(r *registryv1alpha1.StorageReplicaStatus) *registryv1alpha1.StorageReplicaStatus {
		r.Error = "reading the catalogue: input/output error"
		return r
	}

	tests := []struct {
		name    string
		replica *registryv1alpha1.StorageReplicaStatus
		leader  *registryv1alpha1.StorageReplicaStatus
		want    bool
		because string
	}{
		{
			name:    "full against the leader's set",
			replica: withSet(follower("master-1", true, 414, "master-0"), "b"),
			leader:  withSet(leader("master-0", true, 430), "b"),
			want:    true,
		},
		{
			name:    "full against the set before the update",
			replica: withSet(follower("master-1", true, 414, "master-0"), "a"),
			leader:  withSet(leader("master-0", true, 430), "b"),
			because: "a replaced follower reads its old Full back from the status before it replicates the new set",
		},
		{
			name:    "not full",
			replica: withSet(follower("master-1", false, 120, "master-0"), ""),
			leader:  withSet(leader("master-0", true, 430), "b"),
		},
		{
			name:    "its last pass failed",
			replica: failed(withSet(follower("master-1", true, 430, "master-0"), "b")),
			leader:  withSet(leader("master-0", true, 430), "b"),
		},
		{
			name:    "the leader names no set",
			replica: withSet(follower("master-1", true, 430, "master-0"), "b"),
			leader:  withSet(leader("master-0", false, 200), ""),
			because: "with nothing to compare with, no follower can be shown to be current",
		},
		{
			name:    "neither names a set",
			replica: withSet(follower("master-1", true, 430, "master-0"), ""),
			leader:  withSet(leader("master-0", true, 430), ""),
			because: "two empty fingerprints are not a match",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, HoldsLeadersSet(tt.replica, tt.leader), tt.because)
		})
	}
}
