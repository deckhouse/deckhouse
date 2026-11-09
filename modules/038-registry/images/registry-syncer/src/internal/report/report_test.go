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
	"strings"
	"testing"
	"time"
	"unicode/utf8"

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
// air-gapped cluster reads as a fraction of a bundle beside `full: true`, which looks like a
// transition authorised while images are missing. It is not: the numerator is the whole set the
// cluster declares, while the bundle's own number carries other modules, other editions and
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
		Address:         "10.0.0.1:4281",
	}}

	finished := time.Now().Truncate(time.Second)
	changed := MergeCollection(&replicas, State{Node: "master-0", CollectedAt: &finished})
	assert.True(t, changed)

	require.Len(t, replicas, 1)
	assert.True(t, replicas[0].Full, "the collection report erased the fill state")
	assert.EqualValues(t, 459, replicas[0].VerifiedDigests)
	assert.Equal(t, "10.0.0.1:4281", replicas[0].Address)
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
		Node: "master-0", Role: "follower", Address: "10.0.0.1:4281", CollectedAt: &finished,
	}))

	require.Len(t, replicas, 1)
	assert.Equal(t, "master-0", replicas[0].Node)
	assert.EqualValues(t, "follower", replicas[0].Role)
	require.NotNil(t, replicas[0].CollectedAt)
}

// TestOnlyOneReplicaCarriesTheLeaderRole is the status this guards against: two entries claiming
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
// the conclusion can outlive the fact: `safeToDropUpstream: true` beside a leader entry, in the same
// object, saying it does not hold the set. In that window the cluster could be cut off from its
// upstream on evidence that no longer exists.
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

// The announcement exists so that the FIRST fill of a store is visible while it runs: the ordinary
// report is published at the end of a pass, and until then a controller with no reports calls the
// storage `Idle`.
func TestAnnounceSpeaksWhenNobodyHasReportedYet(t *testing.T) {
	publisher, fakeClient := newPublisher(t, storageObject())

	require.NoError(t, publisher.Announce(context.Background(), State{
		Node:            "master-0",
		Role:            registryv1alpha1.ReplicaRoleLeader,
		Address:         "10.0.0.1:4281",
		DeclaredDigests: 443,
	}))

	replicas := getReplicas(t, fakeClient)
	require.Len(t, replicas, 1)
	assert.Equal(t, "master-0", replicas[0].Node)
	assert.Equal(t, registryv1alpha1.ReplicaRoleLeader, replicas[0].Role)
	// The denominator is what makes the announcement worth reading: a phase with no numbers beside it
	// says no more than the phase it replaces.
	assert.Equal(t, int32(443), replicas[0].DeclaredDigests)
	assert.False(t, replicas[0].Full)
	assert.Zero(t, replicas[0].VerifiedDigests)
}

// Create-only, and this is the rule that makes it safe to call at the start of every fill: a replica
// that has already published holds real numbers, and an announcement carries none.
func TestAnnounceLeavesAnExistingReportAlone(t *testing.T) {
	existing := registryv1alpha1.StorageReplicaStatus{
		Node:            "master-0",
		Role:            registryv1alpha1.ReplicaRoleLeader,
		Full:            true,
		VerifiedDigests: 410,
		DeclaredDigests: 410,
		TotalDigests:    455,
	}
	publisher, fakeClient := newPublisher(t, storageObject(existing))

	require.NoError(t, publisher.Announce(context.Background(), State{
		Node:            "master-0",
		Role:            registryv1alpha1.ReplicaRoleLeader,
		DeclaredDigests: 443,
	}))

	replicas := getReplicas(t, fakeClient)
	require.Len(t, replicas, 1)
	assert.Equal(t, existing, replicas[0])
}

// `Full` is never asserted by an announcement, whatever the caller passed: it is the field the
// transition is gated on, and it has to be earned by reading the store.
func TestAnnounceNeverClaimsCompleteness(t *testing.T) {
	publisher, fakeClient := newPublisher(t, storageObject())

	require.NoError(t, publisher.Announce(context.Background(), State{
		Node: "master-0",
		Role: registryv1alpha1.ReplicaRoleLeader,
		Full: true,
	}))

	replicas := getReplicas(t, fakeClient)
	require.Len(t, replicas, 1)
	assert.False(t, replicas[0].Full)
}

// And a leader announcing a fill takes back a permission derived from an earlier report — the same
// interlock Publish carries, because the conclusion must not outlive the fact.
func TestAnnounceWithdrawsAStandingPermission(t *testing.T) {
	storage := storageObject()
	storage.Status.SafeToDropUpstream = true
	storage.Status.AllReplicasFull = true
	publisher, fakeClient := newPublisher(t, storage)

	require.NoError(t, publisher.Announce(context.Background(), State{
		Node: "master-0",
		Role: registryv1alpha1.ReplicaRoleLeader,
	}))

	after := getStorage(t, fakeClient)
	assert.False(t, after.Status.SafeToDropUpstream)
	assert.False(t, after.Status.AllReplicasFull)
}

func TestAnnounceRequiresANode(t *testing.T) {
	publisher, _ := newPublisher(t, storageObject())

	require.Error(t, publisher.Announce(context.Background(), State{Role: registryv1alpha1.ReplicaRoleLeader}))
}

// TestMergeKeepsTheLastStoreMeasurement: a report that did not measure the store must not erase
// the last measurement, or the status flaps between having one and not.
func TestMergeKeepsTheLastStoreMeasurement(t *testing.T) {
	measured := &registryv1alpha1.StoreUsage{UsedBytes: 1 << 30, ReserveBytes: 5 << 30, Writable: true}
	replicas := []registryv1alpha1.StorageReplicaStatus{{Node: "master-0", Store: measured}}

	assert.False(t, Merge(&replicas, State{Node: "master-0"}), "nothing new was said")
	require.NotNil(t, replicas[0].Store)
	assert.Equal(t, int64(1<<30), replicas[0].Store.UsedBytes)

	full := &registryv1alpha1.StoreUsage{
		UsedBytes: 2 << 30, ReserveBytes: 5 << 30,
		Reason: registryv1alpha1.StoreBudgetExhausted,
	}
	assert.True(t, Merge(&replicas, State{Node: "master-0", Store: full}))
	assert.False(t, replicas[0].Store.Writable)
	assert.Equal(t, registryv1alpha1.StoreBudgetExhausted, replicas[0].Store.Reason)
}

func TestMergeStoreTouchesOnlyTheStore(t *testing.T) {
	replicas := []registryv1alpha1.StorageReplicaStatus{
		{Node: "master-0", Full: true, VerifiedDigests: 459, Error: "3 references failed"},
		{Node: "master-1", Full: true},
	}
	usage := &registryv1alpha1.StoreUsage{UsedBytes: 1 << 30, ReserveBytes: 5 << 30, Writable: true}

	assert.True(t, MergeStore(&replicas, State{Node: "master-0", Store: usage}))
	assert.True(t, replicas[0].Full)
	assert.EqualValues(t, 459, replicas[0].VerifiedDigests)
	assert.Equal(t, "3 references failed", replicas[0].Error)
	assert.Equal(t, int64(1<<30), replicas[0].Store.UsedBytes)
	assert.Nil(t, replicas[1].Store)

	assert.False(t, MergeStore(&replicas, State{Node: "master-0", Store: usage.DeepCopy()}),
		"the same figures write nothing")

	assert.True(t, MergeStore(&replicas, State{Node: "master-2", Store: usage}))
	require.Len(t, replicas, 3)
	assert.Equal(t, "master-2", replicas[2].Node)
}

// TestProgressClearsTheLastPassesError is the other defect a live cluster found: the leader's first
// attempt failed, the next one worked, and the status said Failed for the forty minutes it ran.
func TestProgressClearsTheLastPassesError(t *testing.T) {
	replicas := []registryv1alpha1.StorageReplicaStatus{{
		Node: "master-0", Role: registryv1alpha1.ReplicaRoleLeader, Full: false,
		VerifiedDigests: 120, Error: "reading the image set of main: MANIFEST_UNKNOWN",
	}}

	assert.True(t, MergeProgress(&replicas, State{Node: "master-0", VerifiedDigests: 40, DeclaredDigests: 459}))
	assert.Empty(t, replicas[0].Error, "a pass that is copying is past what failed the last one")
	assert.EqualValues(t, 120, replicas[0].VerifiedDigests, "a pass counting from zero must not read as an emptied store")
	assert.EqualValues(t, 459, replicas[0].DeclaredDigests)
	assert.Equal(t, registryv1alpha1.ReplicaRoleLeader, replicas[0].Role)

	assert.True(t, MergeProgress(&replicas, State{Node: "master-0", VerifiedDigests: 300, DeclaredDigests: 459}))
	assert.EqualValues(t, 300, replicas[0].VerifiedDigests)
	assert.False(t, replicas[0].Full, "progress does not make a replica full; its own report does")

	assert.False(t, MergeProgress(&replicas, State{Node: "master-0", VerifiedDigests: 300, DeclaredDigests: 459}),
		"the same progress writes nothing")
}

// TestProgressCarriesWhatIsWrongWithTheStore: a store the registry cannot write to still counts every
// reference it holds as done, so a pass over it reports progress. The error that says the store is not
// writable is what keeps a replica `full` from an earlier pass from being elected or copied from, and
// a progress report that cleared it left such a store looking healthy for as long as the pass ran.
func TestProgressCarriesWhatIsWrongWithTheStore(t *testing.T) {
	const notWritable = "the store is not writable: creating a file in /data/docker/registry/v2/blobs failed"
	replicas := []registryv1alpha1.StorageReplicaStatus{{
		Node: "master-0", Role: registryv1alpha1.ReplicaRoleLeader, Full: true, VerifiedSet: "abc",
		VerifiedDigests: 459, DeclaredDigests: 459, Error: notWritable,
	}}

	assert.False(t, MergeProgress(&replicas, State{Node: "master-0", VerifiedDigests: 1, DeclaredDigests: 459,
		Error: notWritable}), "the same error and a lower count write nothing")
	assert.Equal(t, notWritable, replicas[0].Error, "kept while the store is still not writable")
	assert.True(t, replicas[0].Full, "progress leaves Full alone, so only the error keeps this one from counting")

	assert.True(t, MergeProgress(&replicas, State{Node: "master-0", VerifiedDigests: 2, DeclaredDigests: 459,
		Error: "something else"}))
	assert.Equal(t, "something else", replicas[0].Error, "replaced, not appended to")

	assert.True(t, MergeProgress(&replicas, State{Node: "master-0", VerifiedDigests: 3, DeclaredDigests: 459}))
	assert.Empty(t, replicas[0].Error, "nothing wrong while copying: cleared")
	assert.True(t, replicas[0].Full)
}

func TestMergeWithdrawTakesBackOnlyFull(t *testing.T) {
	replicas := []registryv1alpha1.StorageReplicaStatus{{
		Node: "master-0", Role: registryv1alpha1.ReplicaRoleLeader, Full: true,
		VerifiedDigests: 459, DeclaredDigests: 459, VerifiedSet: "abc",
	}}

	assert.True(t, MergeWithdraw(&replicas, State{Node: "master-0"}))
	assert.False(t, replicas[0].Full)
	assert.Empty(t, replicas[0].VerifiedSet)
	assert.EqualValues(t, 459, replicas[0].VerifiedDigests, "what the store holds does not change")
	assert.False(t, MergeWithdraw(&replicas, State{Node: "master-0"}), "twice is once")
	assert.False(t, MergeWithdraw(&replicas, State{Node: "master-1"}), "nothing to withdraw for an absent replica")
}

// TestMergeDemoteTakesBackWhatTheEntryClaims: a replica whose store was found short of the set takes
// back everything its entry says it holds, and nothing else — not what the health watch, the store's
// measurement or the collection wrote — and claims nothing about any other entry, a leader's included.
func TestMergeDemoteTakesBackWhatTheEntryClaims(t *testing.T) {
	collected := metav1.NewTime(time.Date(2026, 10, 30, 3, 0, 0, 0, time.UTC))
	store := &registryv1alpha1.StoreUsage{UsedBytes: 4096, Writable: true}
	replicas := []registryv1alpha1.StorageReplicaStatus{
		{
			Node: "master-0", Role: registryv1alpha1.ReplicaRoleLeader, Full: true,
			VerifiedDigests: 453, DeclaredDigests: 453, TotalDigests: 517, VerifiedSet: "abc", AheadSet: "def",
			Source: "master-2", Address: "10.0.0.1:5001", Error: "a pass failed", HealthError: "a disk failed",
			Store: store, CollectedAt: &collected, CollectionError: "a collection failed",
		},
		{Node: "master-1", Role: registryv1alpha1.ReplicaRoleLeader, Full: true, VerifiedSet: "abc"},
	}

	assert.True(t, MergeDemote(&replicas, State{Node: "master-0"}))
	assert.Equal(t, registryv1alpha1.StorageReplicaStatus{
		Node: "master-0", Role: registryv1alpha1.ReplicaRoleFollower,
		DeclaredDigests: 453, TotalDigests: 517, Address: "10.0.0.1:5001",
		Error: "a pass failed", HealthError: "a disk failed",
		Store: store, CollectedAt: &collected, CollectionError: "a collection failed",
	}, replicas[0])
	assert.Equal(t, registryv1alpha1.StorageReplicaStatus{
		Node: "master-1", Role: registryv1alpha1.ReplicaRoleLeader, Full: true, VerifiedSet: "abc",
	}, replicas[1], "another entry is never touched, the leader's least of all")

	assert.False(t, MergeDemote(&replicas, State{Node: "master-0"}), "twice is once")
	assert.False(t, MergeDemote(&replicas, State{Node: "master-3"}), "nothing to take back for an absent replica")
	assert.Len(t, replicas, 2)
}

// TestDemoteWithdrawsThePermissions: the demotion is the leader's report taken back, and the two
// permissions the controller derived from it go in the same write.
func TestDemoteWithdrawsThePermissions(t *testing.T) {
	storage := storageObject(
		registryv1alpha1.StorageReplicaStatus{
			Node: "master-0", Role: registryv1alpha1.ReplicaRoleLeader, Full: true, VerifiedDigests: 453, VerifiedSet: "abc",
		},
		registryv1alpha1.StorageReplicaStatus{Node: "master-1", Full: true, VerifiedDigests: 453, VerifiedSet: "abc"},
	)
	storage.Status.SafeToDropUpstream = true
	storage.Status.AllReplicasFull = true
	publisher, c := newPublisher(t, storage)

	require.NoError(t, publisher.Demote(context.Background(), State{Node: "master-0"}))

	updated := getStorage(t, c)
	assert.False(t, updated.Status.SafeToDropUpstream)
	assert.False(t, updated.Status.AllReplicasFull)
	assert.False(t, updated.Status.Replicas[0].Full)
	assert.Zero(t, updated.Status.Replicas[0].VerifiedDigests)
	assert.Equal(t, registryv1alpha1.ReplicaRoleFollower, updated.Status.Replicas[0].Role)
	assert.True(t, updated.Status.Replicas[1].Full)

	assert.Error(t, publisher.Demote(context.Background(), State{}), "a demotion names its replica")
}

// TestMergeTakeBackKeepsTheRole: a leader that keeps the lease with nobody else full takes back what
// Demote takes back — and only that — and keeps the role it reported: it is still the one that fills.
func TestMergeTakeBackKeepsTheRole(t *testing.T) {
	collected := metav1.NewTime(time.Date(2026, 10, 30, 3, 0, 0, 0, time.UTC))
	stamp := metav1.NewTime(time.Date(2026, 10, 30, 9, 0, 0, 0, time.UTC))
	store := &registryv1alpha1.StoreUsage{UsedBytes: 4096, Writable: true}
	replicas := []registryv1alpha1.StorageReplicaStatus{
		{
			Node: "master-0", Role: registryv1alpha1.ReplicaRoleLeader, Full: true,
			VerifiedDigests: 453, DeclaredDigests: 453, TotalDigests: 517, VerifiedSet: "abc", AheadSet: "def",
			Source: "master-2", Address: "10.0.0.1:5001", Error: "a pass failed", HealthError: "a disk failed",
			Store: store, CollectedAt: &collected, CollectionError: "a collection failed", SyncerStartedAt: &stamp,
		},
		{Node: "master-1", Role: registryv1alpha1.ReplicaRoleFollower, Full: true, VerifiedSet: "abc"},
	}

	assert.True(t, MergeTakeBack(&replicas, State{Node: "master-0", Role: registryv1alpha1.ReplicaRoleLeader}))
	assert.Equal(t, registryv1alpha1.StorageReplicaStatus{
		Node: "master-0", Role: registryv1alpha1.ReplicaRoleLeader,
		DeclaredDigests: 453, TotalDigests: 517, Address: "10.0.0.1:5001",
		Error: "a pass failed", HealthError: "a disk failed",
		Store: store, CollectedAt: &collected, CollectionError: "a collection failed", SyncerStartedAt: &stamp,
	}, replicas[0])
	assert.Equal(t, registryv1alpha1.StorageReplicaStatus{
		Node: "master-1", Role: registryv1alpha1.ReplicaRoleFollower, Full: true, VerifiedSet: "abc",
	}, replicas[1], "another entry is never touched")

	assert.False(t, MergeTakeBack(&replicas, State{Node: "master-0"}), "twice is once")
	assert.False(t, MergeTakeBack(&replicas, State{Node: "master-3"}), "nothing to take back for an absent replica")
	assert.Len(t, replicas, 2)

	// A demotion afterwards still owes the role, and makes it.
	assert.True(t, MergeDemote(&replicas, State{Node: "master-0"}))
	assert.Equal(t, registryv1alpha1.ReplicaRoleFollower, replicas[0].Role)
	assert.False(t, MergeDemote(&replicas, State{Node: "master-0"}))
}

// TestTakeBackWithdrawsThePermissions: the leader's report taken back, the two permissions the
// controller derived from it go in the same write, as a demotion takes them — and the role stays.
func TestTakeBackWithdrawsThePermissions(t *testing.T) {
	storage := storageObject(
		registryv1alpha1.StorageReplicaStatus{
			Node: "master-0", Role: registryv1alpha1.ReplicaRoleLeader, Full: true, VerifiedDigests: 453, VerifiedSet: "abc",
		},
		registryv1alpha1.StorageReplicaStatus{Node: "master-1", VerifiedDigests: 300},
	)
	storage.Status.SafeToDropUpstream = true
	storage.Status.AllReplicasFull = true
	publisher, c := newPublisher(t, storage)
	ctx := context.Background()

	require.NoError(t, publisher.TakeBack(ctx, State{Node: "master-0", Role: registryv1alpha1.ReplicaRoleLeader}))

	updated := getStorage(t, c)
	assert.False(t, updated.Status.SafeToDropUpstream)
	assert.False(t, updated.Status.AllReplicasFull)
	assert.False(t, updated.Status.Replicas[0].Full)
	assert.Zero(t, updated.Status.Replicas[0].VerifiedDigests)
	assert.Empty(t, updated.Status.Replicas[0].VerifiedSet)
	assert.Equal(t, registryv1alpha1.ReplicaRoleLeader, updated.Status.Replicas[0].Role)
	assert.EqualValues(t, 300, updated.Status.Replicas[1].VerifiedDigests)

	version := updated.ResourceVersion
	require.NoError(t, publisher.TakeBack(ctx, State{Node: "master-0", Role: registryv1alpha1.ReplicaRoleLeader}))
	assert.Equal(t, version, getStorage(t, c).ResourceVersion, "taken back twice writes once")

	assert.Error(t, publisher.TakeBack(ctx, State{}), "a take-back names its replica")
}

func TestMergeCarriesTheVerifiedSet(t *testing.T) {
	var replicas []registryv1alpha1.StorageReplicaStatus
	Merge(&replicas, State{Node: "master-0", Full: true, VerifiedSet: "abc"})
	require.Len(t, replicas, 1)
	assert.Equal(t, "abc", replicas[0].VerifiedSet)
}

func TestClipKeepsAMessageWithinTheLimit(t *testing.T) {
	short := "the sweep failed"
	assert.Equal(t, short, Clip(short))

	// Three bytes a character, so a cut at the limit lands inside one.
	long := strings.Repeat("€", MessageLimit)
	clipped := Clip(long)
	assert.LessOrEqual(t, len(clipped), MessageLimit)
	assert.True(t, utf8.ValidString(clipped), "cut on a character boundary")
	assert.True(t, strings.HasSuffix(clipped, "(truncated)"))
}

func TestMergeCollectionClipsTheError(t *testing.T) {
	replicas := []registryv1alpha1.StorageReplicaStatus{{Node: "master-0"}}
	now := time.Now()

	MergeCollection(&replicas, State{Node: "master-0", CollectedAt: &now, CollectionError: strings.Repeat("x", 200000)})

	assert.LessOrEqual(t, len(replicas[0].CollectionError), MessageLimit)
}

// TestMergeHealthTouchesOnlyItsOwnError: the health watch writes the error and healthError fields and
// nothing else, and takes back from the error only what it wrote — an error a pass reported is the
// pass's. healthError is the watch's alone and always says what it found.
func TestMergeHealthTouchesOnlyItsOwnError(t *testing.T) {
	const failing = "this replica's registry is not answering: 10.0.0.1:5001: connection refused"
	health := func(found string) State { return State{Node: "master-0", Error: found, HealthError: found} }
	replicas := []registryv1alpha1.StorageReplicaStatus{
		{Node: "master-0", Role: registryv1alpha1.ReplicaRoleLeader, Full: true, VerifiedSet: "abc", VerifiedDigests: 459},
		{Node: "master-1", Full: true},
	}

	assert.True(t, MergeHealth(&replicas, health(failing), ""))
	assert.Equal(t, failing, replicas[0].Error)
	assert.Equal(t, failing, replicas[0].HealthError)
	assert.True(t, replicas[0].Full, "completeness is left alone and comes back with the registry")
	assert.Equal(t, "abc", replicas[0].VerifiedSet)
	assert.EqualValues(t, 459, replicas[0].VerifiedDigests)
	assert.Empty(t, replicas[1].Error)
	assert.Empty(t, replicas[1].HealthError)

	assert.False(t, MergeHealth(&replicas, health(failing), failing), "the same error writes nothing")

	const worse = failing + "; this replica's store does not accept writes: read-only file system"
	assert.True(t, MergeHealth(&replicas, health(worse), failing), "its own error, replaced")
	assert.Equal(t, worse, replicas[0].Error)
	assert.Equal(t, worse, replicas[0].HealthError)

	assert.True(t, MergeHealth(&replicas, health(""), worse), "and taken back")
	assert.Empty(t, replicas[0].Error)
	assert.Empty(t, replicas[0].HealthError)
	assert.False(t, MergeHealth(&replicas, health(""), ""), "nothing to take back")

	const copyFailed = "3 of 459 references could not be copied"
	replicas[0].Error = copyFailed
	assert.True(t, MergeHealth(&replicas, health(failing), ""),
		"an error the pass reported is not replaced, and healthError is set beside it")
	assert.Equal(t, copyFailed, replicas[0].Error)
	assert.Equal(t, failing, replicas[0].HealthError)
	assert.True(t, MergeHealth(&replicas, health(""), failing), "healthError taken back")
	assert.Equal(t, copyFailed, replicas[0].Error, "and the pass's error not")
	assert.Empty(t, replicas[0].HealthError)

	assert.True(t, MergeHealth(&replicas, State{Node: "master-2", Error: failing, HealthError: failing}, ""),
		"a replica failing before its first report gets an entry")
	require.Len(t, replicas, 3)
	assert.Equal(t, failing, replicas[2].Error)
	assert.Equal(t, failing, replicas[2].HealthError)
	assert.False(t, MergeHealth(&replicas, State{Node: "master-3"}, ""), "and a healthy one without a report gets none")
}

// TestAHealthErrorTakesBackOnlyThePermission: a leader whose registry or store fails takes back
// safeToDropUpstream, which the controller then keeps back, since it needs a leader without an error. It
// leaves allReplicasFull alone: the controller derives that from Full, which the health watch does not
// touch, and put it back after every round — the field turned over every half a minute for as long as
// the failure lasted.
func TestAHealthErrorTakesBackOnlyThePermission(t *testing.T) {
	const failing = "this replica's registry is not answering: 10.0.0.1:4283: connection refused"
	storage := storageObject(
		registryv1alpha1.StorageReplicaStatus{Node: "master-0", Role: registryv1alpha1.ReplicaRoleLeader, Full: true},
		registryv1alpha1.StorageReplicaStatus{Node: "master-1", Role: registryv1alpha1.ReplicaRoleFollower, Full: true},
	)
	storage.Status.SafeToDropUpstream = true
	storage.Status.AllReplicasFull = true
	publisher, fakeClient := newPublisher(t, storage)

	leader := State{Node: "master-0", Role: registryv1alpha1.ReplicaRoleLeader, Error: failing, HealthError: failing}
	require.NoError(t, publisher.PublishHealth(context.Background(), leader, ""))

	after := getStorage(t, fakeClient)
	assert.False(t, after.Status.SafeToDropUpstream)
	assert.True(t, after.Status.AllReplicasFull, "every replica still holds the set")
	assert.True(t, after.Status.Replicas[0].Full)

	written := after.ResourceVersion
	require.NoError(t, publisher.PublishHealth(context.Background(), leader, failing))
	assert.Equal(t, written, getStorage(t, fakeClient).ResourceVersion,
		"the next round, with the failure unchanged, writes nothing")
}

// TestMergeKeepsTheHealthError: healthError is the health watch's, and no other report touches it. A pass
// that went well does not know the registry answers, and one that rebuilt the entry without it would
// make a replica that cannot serve a candidate again until the watch's next round.
func TestMergeKeepsTheHealthError(t *testing.T) {
	const failing = "this replica's store does not accept writes: read-only file system"
	replicas := []registryv1alpha1.StorageReplicaStatus{{
		Node: "master-0", Role: registryv1alpha1.ReplicaRoleLeader, Full: true, VerifiedSet: "abc",
		VerifiedDigests: 459, Error: failing, HealthError: failing,
	}}

	assert.True(t, Merge(&replicas, State{Node: "master-0", Role: registryv1alpha1.ReplicaRoleLeader,
		VerifiedDigests: 460, Error: "the upstream refused the credentials"}))
	assert.Equal(t, failing, replicas[0].HealthError, "a pass's report")

	MergeProgress(&replicas, State{Node: "master-0", VerifiedDigests: 461})
	assert.Equal(t, failing, replicas[0].HealthError, "progress")

	replicas[0].Full, replicas[0].VerifiedSet = true, "abc"
	assert.True(t, MergeWithdraw(&replicas, State{Node: "master-0"}))
	assert.Equal(t, failing, replicas[0].HealthError, "a withdrawal")

	assert.False(t, MergeStore(&replicas, State{Node: "master-0", Store: replicas[0].Store}))
	assert.True(t, MergeStore(&replicas, State{Node: "master-0", Store: &registryv1alpha1.StoreUsage{Writable: true}}))
	assert.Equal(t, failing, replicas[0].HealthError, "a store measurement")

	// And through the API: an announcement and a pass's report, written as the syncer writes them.
	publisher, fakeClient := newPublisher(t, storageObject(replicas[0]))
	require.NoError(t, publisher.Announce(context.Background(), State{Node: "master-0", DeclaredDigests: 446}))
	require.NoError(t, publisher.Publish(context.Background(), State{Node: "master-0", VerifiedDigests: 462}))
	stored := getReplicas(t, fakeClient)
	require.Len(t, stored, 1)
	assert.EqualValues(t, 462, stored[0].VerifiedDigests)
	assert.Equal(t, failing, stored[0].HealthError, "an announcement and a published report")
}

// TestProgressCarriesTheHealthError: progress clears the previous pass's error and puts the health
// watch's in its place, when there is one.
func TestProgressCarriesTheHealthError(t *testing.T) {
	const failing = "this replica's store does not accept writes: read-only file system"
	replicas := []registryv1alpha1.StorageReplicaStatus{{
		Node: "master-0", Error: "reading the image set of main: MANIFEST_UNKNOWN",
	}}

	assert.True(t, MergeProgress(&replicas, State{Node: "master-0", VerifiedDigests: 40, Error: failing}))
	assert.Equal(t, failing, replicas[0].Error)
	assert.False(t, MergeProgress(&replicas, State{Node: "master-0", VerifiedDigests: 40, Error: failing}))
}

// TestMergeStampsTheSyncerStart: the report says which process made it, to the second, once that
// process has read its store, and nothing when it has not — so the controller does not take the
// previous process's Full for the running one's.
func TestMergeStampsTheSyncerStart(t *testing.T) {
	started := time.Date(2026, 10, 30, 9, 0, 0, 750_000_000, time.UTC)
	var replicas []registryv1alpha1.StorageReplicaStatus

	assert.True(t, Merge(&replicas, State{Node: "master-0", Full: true, SyncerStartedAt: &started}))
	require.NotNil(t, replicas[0].SyncerStartedAt)
	assert.Equal(t, time.Date(2026, 10, 30, 9, 0, 0, 0, time.UTC), replicas[0].SyncerStartedAt.UTC(),
		"to the second, as the API server keeps it")
	assert.False(t, Merge(&replicas, State{Node: "master-0", Full: true, SyncerStartedAt: &started}),
		"the same stamp writes nothing")

	assert.True(t, Merge(&replicas, State{Node: "master-0", Full: true}))
	assert.Nil(t, replicas[0].SyncerStartedAt, "a report this process has not vouched for carries no stamp")
}

// TestPublishingTheSameStampTwiceWritesNothing: read back from the API server, the stamp has lost its
// fraction of a second; were it kept, every pass would rewrite the status, and every syncer and the
// controller watch it.
func TestPublishingTheSameStampTwiceWritesNothing(t *testing.T) {
	publisher, c := newPublisher(t, storageObject())
	started := time.Date(2026, 10, 30, 9, 0, 0, 750_000_000, time.Local)
	state := State{Node: "master-0", Role: registryv1alpha1.ReplicaRoleLeader, Full: true, SyncerStartedAt: &started}
	ctx := context.Background()

	require.NoError(t, publisher.Publish(ctx, state))
	version := getStorage(t, c).ResourceVersion

	require.NoError(t, publisher.Publish(ctx, state))
	assert.Equal(t, version, getStorage(t, c).ResourceVersion)
}

// TestTheOtherReportersKeepTheStamp: a demotion, progress and the health watch write their own fields
// and leave the stamp as the last report of the pass made it.
func TestTheOtherReportersKeepTheStamp(t *testing.T) {
	stamp := metav1.NewTime(time.Date(2026, 10, 30, 9, 0, 0, 0, time.UTC))
	replicas := []registryv1alpha1.StorageReplicaStatus{{
		Node: "master-0", Role: registryv1alpha1.ReplicaRoleLeader, Full: true, VerifiedDigests: 453,
		SyncerStartedAt: &stamp,
	}}

	assert.True(t, MergeDemote(&replicas, State{Node: "master-0"}))
	assert.True(t, MergeProgress(&replicas, State{Node: "master-0", VerifiedDigests: 460}))
	assert.True(t, MergeHealth(&replicas, State{Node: "master-0", HealthError: "a disk failed"}, ""))

	require.NotNil(t, replicas[0].SyncerStartedAt)
	assert.True(t, stamp.Equal(replicas[0].SyncerStartedAt))
}

// TestPublishAddressCorrectsOnlyTheAddress is a replaced replica coming up on another port: its
// entry still carries what the previous pod reported, and the address is the one thing the new pod
// can say before its first pass ends.
func TestPublishAddressCorrectsOnlyTheAddress(t *testing.T) {
	previous := registryv1alpha1.StorageReplicaStatus{
		Node: "master-1", Role: registryv1alpha1.ReplicaRoleLeader, Full: true,
		VerifiedDigests: 410, DeclaredDigests: 410, Address: "10.0.0.2:5001",
	}
	other := registryv1alpha1.StorageReplicaStatus{Node: "master-0", Address: "10.0.0.1:5001"}
	storage := storageObject(other, previous)
	storage.Status.SafeToDropUpstream = true
	publisher, fakeClient := newPublisher(t, storage)

	require.NoError(t, publisher.PublishAddress(context.Background(), State{
		Node: "master-1", Role: registryv1alpha1.ReplicaRoleFollower, Address: "10.0.0.2:4281",
	}))

	after := getStorage(t, fakeClient)
	want := previous
	want.Address = "10.0.0.2:4281"
	assert.Equal(t, []registryv1alpha1.StorageReplicaStatus{other, want}, after.Status.Replicas,
		"the role, the counts and Full stay what the last pass found")
	assert.True(t, after.Status.SafeToDropUpstream, "an address says nothing about the permission")
}

func TestMergeAddress(t *testing.T) {
	t.Run("an entry that already says it writes nothing", func(t *testing.T) {
		replicas := []registryv1alpha1.StorageReplicaStatus{{Node: "master-0", Address: "10.0.0.1:4281"}}
		assert.False(t, MergeAddress(&replicas, State{Node: "master-0", Address: "10.0.0.1:4281"}))
	})

	t.Run("no entry is not created: Announce and Publish do that with the numbers", func(t *testing.T) {
		replicas := []registryv1alpha1.StorageReplicaStatus{{Node: "master-0", Address: "10.0.0.1:4281"}}
		assert.False(t, MergeAddress(&replicas, State{Node: "master-1", Address: "10.0.0.2:4281"}))
		assert.Len(t, replicas, 1)
	})

	t.Run("an empty address erases nothing", func(t *testing.T) {
		replicas := []registryv1alpha1.StorageReplicaStatus{{Node: "master-0", Address: "10.0.0.1:5001"}}
		assert.False(t, MergeAddress(&replicas, State{Node: "master-0"}))
		assert.Equal(t, "10.0.0.1:5001", replicas[0].Address)
	})
}

func TestPublishAddressRequiresANode(t *testing.T) {
	publisher, _ := newPublisher(t, storageObject())

	require.Error(t, publisher.PublishAddress(context.Background(), State{Address: "10.0.0.1:4281"}))
}
