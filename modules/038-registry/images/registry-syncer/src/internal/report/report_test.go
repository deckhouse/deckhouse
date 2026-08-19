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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	registryv1alpha1 "github.com/deckhouse/deckhouse/go_lib/registry/apis/deckhouse.io/v1alpha1"
)

func newPublisher(t *testing.T, objects ...client.Object) (*Publisher, client.Client) {
	t.Helper()

	scheme := runtime.NewScheme()
	require.NoError(t, registryv1alpha1.AddToScheme(scheme))

	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(objects...).
		WithStatusSubresource(&registryv1alpha1.RegistryStorage{}).
		Build()

	return &Publisher{Client: fakeClient}, fakeClient
}

func storageObject(replicas ...registryv1alpha1.StorageReplicaStatus) *registryv1alpha1.RegistryStorage {
	return &registryv1alpha1.RegistryStorage{
		ObjectMeta: metav1.ObjectMeta{Name: registryv1alpha1.SingletonName},
		Status:     registryv1alpha1.RegistryStorageStatus{Replicas: replicas},
	}
}

// getStorage reads the whole object, for the fields that are not the replica list.
func getStorage(t *testing.T, c client.Client) *registryv1alpha1.RegistryStorage {
	t.Helper()

	storage := &registryv1alpha1.RegistryStorage{}
	require.NoError(t, c.Get(context.Background(),
		types.NamespacedName{Name: registryv1alpha1.SingletonName}, storage))
	return storage
}

func getReplicas(t *testing.T, c client.Client) []registryv1alpha1.StorageReplicaStatus {
	t.Helper()

	storage := &registryv1alpha1.RegistryStorage{}
	require.NoError(t, c.Get(context.Background(),
		types.NamespacedName{Name: registryv1alpha1.SingletonName}, storage))
	return storage.Status.Replicas
}

func TestPublishAddsTheOwnEntry(t *testing.T) {
	publisher, c := newPublisher(t, storageObject())

	require.NoError(t, publisher.Publish(context.Background(), State{
		Node:            "master-0",
		Role:            registryv1alpha1.ReplicaRoleLeader,
		Full:            true,
		VerifiedDigests: 459,
	}))

	replicas := getReplicas(t, c)
	require.Len(t, replicas, 1)
	assert.Equal(t, "master-0", replicas[0].Node)
	assert.Equal(t, registryv1alpha1.ReplicaRoleLeader, replicas[0].Role)
	assert.True(t, replicas[0].Full)
	assert.EqualValues(t, 459, replicas[0].VerifiedDigests)
}

// TestTheTwoCountsAreSeparate pins that "how much of the set is here" and "how big the store is" are
// different numbers reported in different fields.
//
// They were one number, and the reading it invited was wrong in the most expensive direction: an
// air-gapped cluster showed `333/556` beside `full: true`, which looks like a transition authorised
// while a third of the images are missing. It was not — 333 IS the whole set the cluster declares, and
// 556 is what an operator measured in a bundle, which carries other modules, other editions and
// attestations. Only `verifiedDigests` may be compared with `full`; `totalDigests` decides nothing.
func TestTheTwoCountsAreSeparate(t *testing.T) {
	publisher, c := newPublisher(t, storageObject())

	require.NoError(t, publisher.Publish(context.Background(), State{
		Node:            "master-0",
		Role:            registryv1alpha1.ReplicaRoleLeader,
		Full:            true,
		VerifiedDigests: 333,
		TotalDigests:    556,
		DeclaredDigests: 405,
	}))

	replicas := getReplicas(t, c)
	require.Len(t, replicas, 1)
	assert.EqualValues(t, 333, replicas[0].VerifiedDigests,
		"the set the cluster declares, which is what full is decided from")
	assert.EqualValues(t, 556, replicas[0].TotalDigests,
		"everything the store holds, the set included, reported and not decided from")
	assert.EqualValues(t, 405, replicas[0].DeclaredDigests,
		"how big the set is, which is the only denominator a cluster with an upstream has")
	assert.True(t, replicas[0].Full,
		"holding more than the set does not make a replica incomplete")
}

// TestPublishLeavesOtherReplicasAlone is what lets several syncers write the same
// status without a coordinator between them.
func TestPublishLeavesOtherReplicasAlone(t *testing.T) {
	publisher, c := newPublisher(t, storageObject(
		registryv1alpha1.StorageReplicaStatus{
			Node: "master-1", Role: registryv1alpha1.ReplicaRoleFollower,
			Full: true, VerifiedDigests: 459, Source: "master-0",
		},
		registryv1alpha1.StorageReplicaStatus{
			Node: "master-2", Role: registryv1alpha1.ReplicaRoleFollower, VerifiedDigests: 100,
		},
	))

	require.NoError(t, publisher.Publish(context.Background(), State{
		Node: "master-0", Role: registryv1alpha1.ReplicaRoleLeader, Full: true, VerifiedDigests: 459,
	}))

	replicas := getReplicas(t, c)
	require.Len(t, replicas, 3)

	byNode := make(map[string]registryv1alpha1.StorageReplicaStatus, len(replicas))
	for _, replica := range replicas {
		byNode[replica.Node] = replica
	}

	assert.Equal(t, "master-0", byNode["master-1"].Source, "another replica's entry was rewritten")
	assert.True(t, byNode["master-1"].Full)
	assert.EqualValues(t, 100, byNode["master-2"].VerifiedDigests)
}

func TestPublishUpdatesInPlace(t *testing.T) {
	publisher, c := newPublisher(t, storageObject(
		registryv1alpha1.StorageReplicaStatus{
			Node: "master-0", Role: registryv1alpha1.ReplicaRoleLeader, VerifiedDigests: 312,
		},
	))

	require.NoError(t, publisher.Publish(context.Background(), State{
		Node: "master-0", Role: registryv1alpha1.ReplicaRoleLeader, Full: true, VerifiedDigests: 459,
	}))

	replicas := getReplicas(t, c)
	require.Len(t, replicas, 1, "the entry is replaced, not duplicated")
	assert.True(t, replicas[0].Full)
	assert.EqualValues(t, 459, replicas[0].VerifiedDigests)
}

// TestPublishIsIdempotent matters because every replica writes this object and the
// controller watches it: a needless write multiplies into a reconciliation of the
// whole layout.
func TestPublishIsIdempotent(t *testing.T) {
	publisher, c := newPublisher(t, storageObject())
	state := State{Node: "master-0", Role: registryv1alpha1.ReplicaRoleLeader, Full: true, VerifiedDigests: 459}
	ctx := context.Background()

	require.NoError(t, publisher.Publish(ctx, state))

	storage := &registryv1alpha1.RegistryStorage{}
	require.NoError(t, c.Get(ctx, types.NamespacedName{Name: registryv1alpha1.SingletonName}, storage))
	version := storage.ResourceVersion

	require.NoError(t, publisher.Publish(ctx, state))

	require.NoError(t, c.Get(ctx, types.NamespacedName{Name: registryv1alpha1.SingletonName}, storage))
	assert.Equal(t, version, storage.ResourceVersion)

	// And a real change is still written, so the assertion above is not vacuous.
	state.VerifiedDigests = 460
	require.NoError(t, publisher.Publish(ctx, state))
	require.NoError(t, c.Get(ctx, types.NamespacedName{Name: registryv1alpha1.SingletonName}, storage))
	assert.NotEqual(t, version, storage.ResourceVersion)
}

// TestPublishToleratesAMissingStorage covers the cache being turned off, or the
// controller not having created the object yet. A syncer crash-looping over that
// would be noise rather than information.
func TestPublishToleratesAMissingStorage(t *testing.T) {
	publisher, _ := newPublisher(t)

	assert.NoError(t, publisher.Publish(context.Background(), State{
		Node: "master-0", Role: registryv1alpha1.ReplicaRoleLeader,
	}))
}

func TestPublishRequiresANode(t *testing.T) {
	publisher, _ := newPublisher(t, storageObject())

	// An entry without a node has no key, so it would either be appended forever or
	// silently collide with another replica's.
	assert.Error(t, publisher.Publish(context.Background(), State{Full: true}))
}

func TestPublishClearsAFixedError(t *testing.T) {
	publisher, c := newPublisher(t, storageObject(
		registryv1alpha1.StorageReplicaStatus{
			Node: "master-0", Role: registryv1alpha1.ReplicaRoleLeader, Error: "401 from upstream",
		},
	))

	require.NoError(t, publisher.Publish(context.Background(), State{
		Node: "master-0", Role: registryv1alpha1.ReplicaRoleLeader, Full: true, VerifiedDigests: 459,
	}))

	replicas := getReplicas(t, c)
	require.Len(t, replicas, 1)
	assert.Empty(t, replicas[0].Error, "a fixed problem must stop being reported")
	assert.True(t, replicas[0].Full)
}

func TestMerge(t *testing.T) {
	t.Run("appends when absent", func(t *testing.T) {
		var replicas []registryv1alpha1.StorageReplicaStatus

		assert.True(t, Merge(&replicas, State{Node: "master-0", Full: true}))
		require.Len(t, replicas, 1)
		assert.Equal(t, "master-0", replicas[0].Node)
	})

	t.Run("reports no change for an identical entry", func(t *testing.T) {
		replicas := []registryv1alpha1.StorageReplicaStatus{
			{Node: "master-0", Role: registryv1alpha1.ReplicaRoleLeader, Full: true, VerifiedDigests: 459},
		}

		assert.False(t, Merge(&replicas, State{
			Node: "master-0", Role: registryv1alpha1.ReplicaRoleLeader, Full: true, VerifiedDigests: 459,
		}))
	})

	t.Run("detects a role change", func(t *testing.T) {
		// A follower being promoted is exactly the case where a missed change would
		// leave the status claiming there is no leader.
		replicas := []registryv1alpha1.StorageReplicaStatus{
			{Node: "master-1", Role: registryv1alpha1.ReplicaRoleFollower, Full: true, VerifiedDigests: 459},
		}

		assert.True(t, Merge(&replicas, State{
			Node: "master-1", Role: registryv1alpha1.ReplicaRoleLeader, Full: true, VerifiedDigests: 459,
		}))
		assert.Equal(t, registryv1alpha1.ReplicaRoleLeader, replicas[0].Role)
	})

	t.Run("keeps the order of the others", func(t *testing.T) {
		replicas := []registryv1alpha1.StorageReplicaStatus{
			{Node: "master-0"}, {Node: "master-1"}, {Node: "master-2"},
		}

		require.True(t, Merge(&replicas, State{Node: "master-1", Full: true}))
		assert.Equal(t, []string{"master-0", "master-1", "master-2"},
			[]string{replicas[0].Node, replicas[1].Node, replicas[2].Node})
	})
}

// TestMergeDoesNotEraseTheCollectionFields is the cross-clobbering the two reporters would
// otherwise do to each other. They run on different schedules and own different fields of one
// entry; a reporter that rebuilds the whole entry erases the other's work, and the two take
// turns doing it — which shows up as a status that flaps for no reason anyone can find.
func TestMergeDoesNotEraseTheCollectionFields(t *testing.T) {
	collected := metav1.NewTime(time.Now().Truncate(time.Second))
	replicas := []registryv1alpha1.StorageReplicaStatus{{
		Node:            "master-0",
		CollectedAt:     &collected,
		CollectionError: "the sweep failed",
	}}

	changed := Merge(&replicas, State{Node: "master-0", Role: "leader", Full: true, VerifiedDigests: 459})
	assert.True(t, changed)

	require.Len(t, replicas, 1)
	assert.True(t, replicas[0].Full, "the fill report did not land")
	assert.Equal(t, &collected, replicas[0].CollectedAt, "the fill report erased the collection time")
	assert.Equal(t, "the sweep failed", replicas[0].CollectionError)
}

// TestMergeCollectionDoesNotEraseTheFillFields is the same rule from the other side.
func TestMergeCollectionDoesNotEraseTheFillFields(t *testing.T) {
	replicas := []registryv1alpha1.StorageReplicaStatus{{
		Node:            "master-0",
		Role:            "leader",
		Full:            true,
		VerifiedDigests: 459,
		Address:         "10.0.0.1:5001",
	}}

	finished := time.Now().Truncate(time.Second)
	changed := MergeCollection(&replicas, State{Node: "master-0", CollectedAt: &finished})
	assert.True(t, changed)

	require.Len(t, replicas, 1)
	assert.True(t, replicas[0].Full, "the collection report erased the fill state")
	assert.EqualValues(t, 459, replicas[0].VerifiedDigests)
	assert.Equal(t, "10.0.0.1:5001", replicas[0].Address)
	require.NotNil(t, replicas[0].CollectedAt)
}

// TestMergeCollectionIsIdempotent keeps a needless write from multiplying: every replica writes
// this object and the controller watches it.
func TestMergeCollectionIsIdempotent(t *testing.T) {
	finished := time.Now().Truncate(time.Second)
	replicas := []registryv1alpha1.StorageReplicaStatus{{Node: "master-0"}}

	assert.True(t, MergeCollection(&replicas, State{Node: "master-0", CollectedAt: &finished}))
	assert.False(t, MergeCollection(&replicas, State{Node: "master-0", CollectedAt: &finished}),
		"an unchanged collection report was written again")
}

// TestMergeCollectionClearsAPastFailure: a run that succeeded has to remove the previous
// error, or an operator reads a failure that is no longer true.
func TestMergeCollectionClearsAPastFailure(t *testing.T) {
	finished := time.Now().Truncate(time.Second)
	replicas := []registryv1alpha1.StorageReplicaStatus{{
		Node:            "master-0",
		CollectionError: "the store is not writable",
	}}

	assert.True(t, MergeCollection(&replicas, State{Node: "master-0", CollectedAt: &finished}))
	assert.Empty(t, replicas[0].CollectionError)
}

// TestMergeCollectionCreatesAnEntry covers a replica that collected before it ever reported a
// fill, which is what happens on a cache that was already full when the module took over.
func TestMergeCollectionCreatesAnEntry(t *testing.T) {
	var replicas []registryv1alpha1.StorageReplicaStatus

	finished := time.Now().Truncate(time.Second)
	assert.True(t, MergeCollection(&replicas, State{
		Node: "master-0", Role: "follower", Address: "10.0.0.1:5001", CollectedAt: &finished,
	}))

	require.Len(t, replicas, 1)
	assert.Equal(t, "master-0", replicas[0].Node)
	assert.EqualValues(t, "follower", replicas[0].Role)
	require.NotNil(t, replicas[0].CollectedAt)
}

// TestOnlyOneReplicaCarriesTheLeaderRole is a status a live cluster produced: two entries claiming
// Leader at the same time.
//
// A replica writes only its own entry, so an entry outlives whatever it last said. One that led, lost
// the lease and then stopped publishing — its pass failing, or its process restarting — leaves
// `role: Leader` behind it indefinitely. Everything downstream reads this status to find the leader,
// including followers deciding what to replicate from, so a second claim is not untidiness: it is two
// answers to a question that has one.
func TestOnlyOneReplicaCarriesTheLeaderRole(t *testing.T) {
	replicas := []registryv1alpha1.StorageReplicaStatus{
		{Node: "master-2", Role: registryv1alpha1.ReplicaRoleLeader, VerifiedDigests: 333},
		{Node: "master-0", Role: registryv1alpha1.ReplicaRoleFollower, VerifiedDigests: 387},
	}

	// The replica that actually holds the lease now publishes.
	changed := Merge(&replicas, State{
		Node: "master-1", Role: registryv1alpha1.ReplicaRoleLeader, Full: true, VerifiedDigests: 333,
	})
	assert.True(t, changed)

	leaders := map[string]int32{}
	for _, replica := range replicas {
		if replica.Role == registryv1alpha1.ReplicaRoleLeader {
			leaders[replica.Node] = replica.VerifiedDigests
		}
	}
	assert.Equal(t, map[string]int32{"master-1": 333}, leaders,
		"the lease-holder leads, and the memory of an earlier one is demoted")

	// Demoted, not deleted: what that replica held is still true and still reported.
	for _, replica := range replicas {
		if replica.Node == "master-2" {
			assert.EqualValues(t, 333, replica.VerifiedDigests)
			assert.Equal(t, registryv1alpha1.ReplicaRoleFollower, replica.Role)
		}
	}
}

// TestDemotingAStaleLeaderCountsAsAChange: the write is skipped when nothing changed, so a demotion
// that reported "nothing changed" would never reach the API — and the second claim would survive.
func TestDemotingAStaleLeaderCountsAsAChange(t *testing.T) {
	entry := registryv1alpha1.StorageReplicaStatus{
		Node: "master-1", Role: registryv1alpha1.ReplicaRoleLeader, VerifiedDigests: 5,
	}
	replicas := []registryv1alpha1.StorageReplicaStatus{
		{Node: "master-2", Role: registryv1alpha1.ReplicaRoleLeader},
		entry,
	}

	// The publishing replica's own entry is identical to what it would write — only the other
	// replica's role changes.
	changed := Merge(&replicas, State{
		Node: "master-1", Role: registryv1alpha1.ReplicaRoleLeader, VerifiedDigests: 5,
	})

	assert.True(t, changed, "a demotion elsewhere is still a change worth writing")
	assert.Equal(t, registryv1alpha1.ReplicaRoleFollower, replicas[0].Role)
}

// TestALeaderThatIsNotFullWithdrawsPermissionToDropTheUpstream is the interlock between two writers of
// one object.
//
// `safeToDropUpstream` is derived by the controller from the leader's report, and the air-gap
// transition is gated on it — but the derivation and the report are written by different processes, so
// the conclusion can outlive the fact. Measured on a cluster: `safeToDropUpstream: true` while the
// leader's own entry, in the same object, said it did not hold the set. In that window the cluster
// could be cut off from its upstream on evidence that no longer existed.
func TestALeaderThatIsNotFullWithdrawsPermissionToDropTheUpstream(t *testing.T) {
	storage := storageObject(registryv1alpha1.StorageReplicaStatus{
		Node: "master-0", Role: registryv1alpha1.ReplicaRoleLeader, Full: true, VerifiedDigests: 459,
	})
	storage.Status.SafeToDropUpstream = true
	storage.Status.AllReplicasFull = true

	publisher, c := newPublisher(t, storage)

	// The same replica, now reporting that it does not hold the set.
	require.NoError(t, publisher.Publish(context.Background(), State{
		Node: "master-0", Role: registryv1alpha1.ReplicaRoleLeader, Full: false, VerifiedDigests: 333,
	}))

	updated := getStorage(t, c)
	assert.False(t, updated.Status.SafeToDropUpstream,
		"permission cannot outlive the completeness it was derived from")
	assert.False(t, updated.Status.AllReplicasFull)
}

// TestOnlyWithdrawing keeps the interlock one-directional: granting permission stays the controller's,
// which is what keeps one decision in one place.
func TestOnlyWithdrawing(t *testing.T) {
	storage := storageObject()
	publisher, c := newPublisher(t, storage)

	require.NoError(t, publisher.Publish(context.Background(), State{
		Node: "master-0", Role: registryv1alpha1.ReplicaRoleLeader, Full: true, VerifiedDigests: 459,
	}))

	updated := getStorage(t, c)
	assert.False(t, updated.Status.SafeToDropUpstream,
		"a full leader states its fullness; whether that is permission is not its call")
}

// TestAFollowerDoesNotWithdrawIt: completeness of a follower says nothing about the leader, and a
// follower that is behind must not veto a transition the leader has earned.
func TestAFollowerDoesNotWithdrawIt(t *testing.T) {
	storage := storageObject(registryv1alpha1.StorageReplicaStatus{
		Node: "master-0", Role: registryv1alpha1.ReplicaRoleLeader, Full: true,
	})
	storage.Status.SafeToDropUpstream = true

	publisher, c := newPublisher(t, storage)

	require.NoError(t, publisher.Publish(context.Background(), State{
		Node: "master-1", Role: registryv1alpha1.ReplicaRoleFollower, Full: false, VerifiedDigests: 12,
	}))

	assert.True(t, getStorage(t, c).Status.SafeToDropUpstream)
}
