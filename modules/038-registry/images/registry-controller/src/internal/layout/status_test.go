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
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

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

func leader(node string, full bool, digests int32) registryv1alpha1.StorageReplicaStatus {
	return registryv1alpha1.StorageReplicaStatus{
		Node: node, Role: registryv1alpha1.ReplicaRoleLeader, Full: full, VerifiedDigests: digests,
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
			// The case measured on a live cluster: the lease moved in eight seconds while
			// the departed replica's entry went on saying Leader. Two entries claim it,
			// and only the lease can say which claim is current.
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
			got := Aggregate(tt.spec, tt.replicas, tt.holder)

			assert.Equal(t, tt.wantPhase, got.Phase)
			assert.Equal(t, tt.wantLeader, got.Leader)
			assert.Equal(t, tt.wantSafeToDrop, got.SafeToDropUpstream)
			assert.Equal(t, tt.wantAllFull, got.AllReplicasFull)
			assert.Equal(t, tt.wantAuthoritative, got.Authoritative)
		})
	}
}

func TestAggregateFillProgress(t *testing.T) {
	got := Aggregate(passThroughSpec(), []registryv1alpha1.StorageReplicaStatus{leader("master-0", false, 312)}, "master-0")

	require.NotNil(t, got.Fill)
	assert.EqualValues(t, 312, got.Fill.Filled)
	assert.EqualValues(t, 459, got.Fill.Total)

	// Without an expected count there is no progress to report, and a "312 of 0"
	// would be worse than nothing.
	noSource := Aggregate(
		&registryv1alpha1.RegistryStorageSpec{Upstream: testUpstream("registry.deckhouse.io")},
		[]registryv1alpha1.StorageReplicaStatus{leader("master-0", false, 312)},
		"master-0",
	)
	assert.Nil(t, noSource.Fill)
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
