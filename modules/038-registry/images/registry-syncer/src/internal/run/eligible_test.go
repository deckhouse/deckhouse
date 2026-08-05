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

	// And once it has moved, the replica that stood aside has something to copy from.
	// Before the move it would have had nothing: a follower does not replicate from an
	// incomplete leader, which is exactly the stall being avoided.
	assert.Equal(t, ActionReplicate,
		Decide(airGap, false, &Leader{Node: "master-1", Address: "10.0.0.2:5001", Full: true}))
	assert.Equal(t, ActionCountCatalogue,
		Decide(airGap, false, &Leader{Node: "master-0", Address: "10.0.0.1:5001", Full: false}))
}
