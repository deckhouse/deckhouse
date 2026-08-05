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
			name:              "the cache is authoritative once no upstream is left",
			spec:              airGapSpec(),
			replicas:          []registryv1alpha1.StorageReplicaStatus{leader("master-0", true, 459)},
			wantPhase:         registryv1alpha1.StoragePhaseReady,
			wantLeader:        "master-0",
			wantSafeToDrop:    true,
			wantAllFull:       true,
			wantAuthoritative: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Aggregate(tt.spec, tt.replicas)

			assert.Equal(t, tt.wantPhase, got.Phase)
			assert.Equal(t, tt.wantLeader, got.Leader)
			assert.Equal(t, tt.wantSafeToDrop, got.SafeToDropUpstream)
			assert.Equal(t, tt.wantAllFull, got.AllReplicasFull)
			assert.Equal(t, tt.wantAuthoritative, got.Authoritative)
		})
	}
}

func TestAggregateFillProgress(t *testing.T) {
	got := Aggregate(passThroughSpec(), []registryv1alpha1.StorageReplicaStatus{leader("master-0", false, 312)})

	require.NotNil(t, got.Fill)
	assert.EqualValues(t, 312, got.Fill.Filled)
	assert.EqualValues(t, 459, got.Fill.Total)

	// Without an expected count there is no progress to report, and a "312 of 0"
	// would be worse than nothing.
	noSource := Aggregate(
		&registryv1alpha1.RegistryStorageSpec{Upstream: testUpstream("registry.deckhouse.io")},
		[]registryv1alpha1.StorageReplicaStatus{leader("master-0", false, 312)},
	)
	assert.Nil(t, noSource.Fill)
}

// TestLeaderFull covers the gate the air-gap transition reads. It is deliberately
// derived from the reports rather than from the aggregate, so a stale or
// hand-edited aggregate cannot authorize the transition.
func TestLeaderFull(t *testing.T) {
	tests := []struct {
		name     string
		replicas []registryv1alpha1.StorageReplicaStatus
		want     bool
	}{
		{name: "no replicas", want: false},
		{
			name:     "the leader is full",
			replicas: []registryv1alpha1.StorageReplicaStatus{leader("master-0", true, 459)},
			want:     true,
		},
		{
			name:     "the leader is filling",
			replicas: []registryv1alpha1.StorageReplicaStatus{leader("master-0", false, 312)},
			want:     false,
		},
		{
			name: "the leader claims full but reports an error",
			replicas: []registryv1alpha1.StorageReplicaStatus{
				{Node: "master-0", Role: registryv1alpha1.ReplicaRoleLeader, Full: true, Error: "verification failed"},
			},
			// A replica that both claims completeness and reports a failure is not
			// trustworthy enough to cut the cluster off its upstream.
			want: false,
		},
		{
			name: "only full followers",
			replicas: []registryv1alpha1.StorageReplicaStatus{
				follower("master-1", true, 459, "master-0"),
			},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, LeaderFull(tt.replicas))
		})
	}
}
