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

package report

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	registryv1alpha1 "github.com/deckhouse/deckhouse/go_lib/registry/apis/deckhouse.io/v1alpha1"
)

func TestTheIntegrityFieldsHaveOneWriter(t *testing.T) {
	replicas := []registryv1alpha1.StorageReplicaStatus{{Node: "master-0", Role: registryv1alpha1.ReplicaRoleLeader, Full: true}}
	scrubbed := time.Date(2026, 11, 10, 12, 0, 0, 500, time.UTC)
	images := &registryv1alpha1.ReplicaImages{Present: 400, Corrupted: 1}

	require.True(t, MergeIntegrity(&replicas, Integrity{
		Node: "master-0", CorruptedBlobs: []string{"sha256:aa"}, Images: images, ScrubbedAt: &scrubbed,
	}))
	assert.Equal(t, []string{"sha256:aa"}, replicas[0].CorruptedBlobs)
	assert.Equal(t, images, replicas[0].Images)
	assert.True(t, replicas[0].Full, "nothing else is touched")
	assert.False(t, MergeIntegrity(&replicas, Integrity{
		Node: "master-0", CorruptedBlobs: []string{"sha256:aa"}, Images: images, ScrubbedAt: &scrubbed,
	}), "the same report writes nothing")

	// A pass report rebuilds the entry and keeps what it does not own.
	Merge(&replicas, State{Node: "master-0", Role: registryv1alpha1.ReplicaRoleLeader, Full: false})
	assert.Equal(t, []string{"sha256:aa"}, replicas[0].CorruptedBlobs)
	assert.Equal(t, images, replicas[0].Images)
	require.NotNil(t, replicas[0].ScrubbedAt)

	// Repaired: the list goes, the last count and scrub stay when not given.
	require.True(t, MergeIntegrity(&replicas, Integrity{Node: "master-0"}))
	assert.Nil(t, replicas[0].CorruptedBlobs)
	assert.Equal(t, images, replicas[0].Images)

	assert.False(t, MergeIntegrity(&replicas, Integrity{Node: "master-9", CorruptedBlobs: []string{"sha256:aa"}}),
		"no entry is created for a replica that has not reported a pass")
	assert.Len(t, replicas, 1)
}

// TestABrokenBlobOfTheSetTakesFullBackInTheSameWrite: left to the next pass, Full stood beside the finding
// for half a minute on a stand, and a leader stepping down could hand the lease to this replica.
func TestABrokenBlobOfTheSetTakesFullBackInTheSameWrite(t *testing.T) {
	storage := storageObject(
		registryv1alpha1.StorageReplicaStatus{Node: "master-0", Role: registryv1alpha1.ReplicaRoleLeader, Full: true},
		registryv1alpha1.StorageReplicaStatus{Node: "master-1", Role: registryv1alpha1.ReplicaRoleFollower, Full: true},
	)
	storage.Status.SafeToDropUpstream, storage.Status.AllReplicasFull = true, true
	publisher, c := newPublisher(t, storage)
	ctx := context.Background()

	// A broken blob of an image nobody declares leaves the set whole.
	require.NoError(t, publisher.PublishIntegrity(ctx, Integrity{Node: "master-1", CorruptedBlobs: []string{"sha256:aa"}}))
	got := getStorage(t, c)
	assert.True(t, got.Status.Replicas[1].Full)

	require.NoError(t, publisher.PublishIntegrity(ctx, Integrity{
		Node: "master-1", CorruptedBlobs: []string{"sha256:aa"}, ShortOfSet: true,
	}))
	got = getStorage(t, c)
	assert.False(t, got.Status.Replicas[1].Full)
	assert.True(t, got.Status.SafeToDropUpstream, "a follower does not speak for the gates")

	require.NoError(t, publisher.PublishIntegrity(ctx, Integrity{
		Node: "master-0", CorruptedBlobs: []string{"sha256:bb"}, ShortOfSet: true,
	}))
	got = getStorage(t, c)
	assert.False(t, got.Status.Replicas[0].Full)
	assert.False(t, got.Status.SafeToDropUpstream, "a leader short of the set withdraws the permission")
	assert.False(t, got.Status.AllReplicasFull)

	// Repaired, the integrity report does not give Full back: the pass that reads the store does.
	require.NoError(t, publisher.PublishIntegrity(ctx, Integrity{Node: "master-0"}))
	assert.False(t, getStorage(t, c).Status.Replicas[0].Full)
}
