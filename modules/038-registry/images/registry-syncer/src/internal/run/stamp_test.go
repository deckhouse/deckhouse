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
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	registryv1alpha1 "github.com/deckhouse/deckhouse/go_lib/registry/apis/deckhouse.io/v1alpha1"

	"github.com/deckhouse/registry-syncer/internal/gc"
	"github.com/deckhouse/registry-syncer/internal/report"
)

// The syncer stamps its report with when its process started, once that process has read its store —
// see Loop.storeRead. The controller drops the upstream only on a lease holder's report stamped no
// earlier than its syncer container started, so a Full the previous process left cannot do it.

// processStart is when the process under test started: long enough ago that its registry is past
// StartupGrace, and with a fraction of a second the report does not keep.
var processStart = time.Date(2026, 10, 1, 9, 0, 0, 750_000_000, time.UTC)

// previousStart is the stamp a previous process left in the status.
var previousStart = metav1.NewTime(time.Date(2026, 10, 1, 8, 0, 0, 0, time.UTC))

func stampOf(t *testing.T, c client.Client, node string) *time.Time {
	t.Helper()
	stamp := replicaOf(t, c, node).SyncerStartedAt
	if stamp == nil {
		return nil
	}
	at := stamp.UTC()
	return &at
}

// reportOf is a replica's entry without the measurement of its store: that follows the free space of
// the host's filesystem, which other processes move across a rounding step at any moment, and is
// written whenever it does — see distribution.MeasureStore.
func reportOf(t *testing.T, c client.Client, node string) registryv1alpha1.StorageReplicaStatus {
	t.Helper()
	entry := replicaOf(t, c, node)
	entry.Store = nil
	return entry
}

// TestTheStampWaitsForTheProcessToReadItsStore: a new process over a status its predecessor stamped.
// Its first idle pass, past StartupGrace, rechecks the store and cannot read the set: the replica's
// own registry does not answer yet (a closed address). An upstream that does not answer behind the
// pull-through ends the same way. So it publishes the Full it carried over, and no stamp: that Full is
// the predecessor's. The recheck that reads the store stamps the
// report, and the idle passes after it keep the stamp and change nothing of the report.
func TestTheStampWaitsForTheProcessToReadItsStore(t *testing.T) {
	local := startRegistry(t)
	one := pushDigest(t, local, "system/deckhouse")
	two := pushDigest(t, local, "system/deckhouse")
	pushImage(t, local, "system/deckhouse:v1.76.6")
	pushInstaller(t, local, "system/deckhouse/install:v1.76.6", []string{one, two})
	set := SetKey(gc.Releases{Deployed: "v1.76.6"}, nil, nil)

	storage := storageWith(registryv1alpha1.RegistryStorageSpec{})
	storage.Status.Replicas = []registryv1alpha1.StorageReplicaStatus{
		{Node: "master-1", Role: registryv1alpha1.ReplicaRoleLeader, Full: true, VerifiedSet: set,
			Address: closedAddress(t)},
		{Node: "master-0", Role: registryv1alpha1.ReplicaRoleFollower, Full: true, VerifiedSet: set,
			VerifiedDigests: 3, Source: "master-1", SyncerStartedAt: &previousStart},
	}

	loop, _, _ := newLoop(t, false, nil)
	c := releaseAwareClient(t, storage, deployedRelease("v1.76.6"))
	loop.Client = c
	loop.Publisher = &report.Publisher{Client: c}
	loop.LocalAddress = closedAddress(t)
	loop.WriteAddress = loop.LocalAddress
	loop.ReportedAddress = local
	loop.DataDir = t.TempDir()
	loop.started = processStart
	// The store holds the set from the start: the first pass cannot read the set, but its count of the
	// store must not refute the Full it carries over — see refutedByCount.
	release := resolveTag(t, local, "system/deckhouse", "v1.76.6")
	heldOnDisk(t, loop.DataDir, one, two, release)
	tagOnDisk(t, loop.DataDir, "v1.76.6", release)

	require.NoError(t, loop.once(context.Background()))
	assert.True(t, replicaOf(t, c, "master-0").Full, "carried over, as before")
	assert.Nil(t, stampOf(t, c, "master-0"), "the previous process's stamp goes, and this one has none yet")

	// The registry answers; the next recheck reads the store.
	loop.LocalAddress = local
	loop.WriteAddress = local
	loop.lastStoreCheck.Store(time.Now().Add(-storeRecheck).UnixNano())

	require.NoError(t, loop.once(context.Background()))
	assert.True(t, replicaOf(t, c, "master-0").Full)
	require.NotNil(t, stampOf(t, c, "master-0"), "a recheck that read the store stamps the report")
	assert.Equal(t, processStart.Truncate(time.Second), *stampOf(t, c, "master-0"))

	before := reportOf(t, c, "master-0")
	require.NoError(t, loop.once(context.Background()))
	assert.Equal(t, processStart.Truncate(time.Second), *stampOf(t, c, "master-0"),
		"an idle pass carries this process's reading over, and its stamp with it")
	assert.Equal(t, before, reportOf(t, c, "master-0"), "and changes nothing of the report")
}

// TestACopyStampsTheReport: a replication puts the set into the store and counts what it holds.
func TestACopyStampsTheReport(t *testing.T) {
	leaderStorage := startRegistry(t)
	followerStorage := startRegistry(t)
	one := pushDigest(t, leaderStorage, "system/deckhouse")
	pushImage(t, leaderStorage, "system/deckhouse:v1.76.6")
	pushInstaller(t, leaderStorage, "system/deckhouse/install:v1.76.6", []string{one})

	storage := storageWith(registryv1alpha1.RegistryStorageSpec{
		Source:   &registryv1alpha1.StorageSource{ExpectedDigests: 3},
		NeedSync: true,
	})
	storage.Status.Replicas = []registryv1alpha1.StorageReplicaStatus{
		{Node: "master-1", Role: registryv1alpha1.ReplicaRoleLeader, Full: true, VerifiedDigests: 3,
			Address: leaderStorage},
		{Node: "master-0", Role: registryv1alpha1.ReplicaRoleFollower, SyncerStartedAt: &previousStart},
	}
	loop, c, _ := newLoop(t, false, storage, deployedRelease("v1.76.6"))
	loop.LocalAddress = followerStorage
	loop.WriteAddress = followerStorage
	loop.ReportedAddress = followerStorage
	loop.started = processStart

	require.NoError(t, loop.once(context.Background()))

	require.NotNil(t, stampOf(t, c, "master-0"))
	assert.Equal(t, processStart.Truncate(time.Second), *stampOf(t, c, "master-0"))
}

// TestAnAirGapCountStampsTheReport: in air-gap the leader counts its store on every pass. An empty
// store is counted all the same, and that reading is this process's: Full false, stamped.
func TestAnAirGapCountStampsTheReport(t *testing.T) {
	store := startRegistry(t)
	running := pushDigest(t, store, "system/deckhouse")
	pushImage(t, store, "system/deckhouse:v1.76.6")
	pushInstaller(t, store, "system/deckhouse/install:v1.76.6", []string{running})

	storage := storageWith(registryv1alpha1.RegistryStorageSpec{})
	storage.Status.Replicas = []registryv1alpha1.StorageReplicaStatus{{
		Node: "master-0", Role: registryv1alpha1.ReplicaRoleLeader, Full: true, SyncerStartedAt: &previousStart,
	}}
	loop, _, _ := newLoop(t, true, nil)
	c := releaseAwareClient(t, storage, deployedRelease("v1.76.6"))
	loop.Client = c
	loop.Publisher = &report.Publisher{Client: c}
	loop.LocalAddress = store
	loop.WriteAddress = store
	loop.DataDir = t.TempDir()
	loop.started = processStart

	require.NoError(t, loop.once(context.Background()))

	assert.False(t, replicaOf(t, c, "master-0").Full, "nothing on the disk")
	require.NotNil(t, stampOf(t, c, "master-0"))
	assert.Equal(t, processStart.Truncate(time.Second), *stampOf(t, c, "master-0"))
}

// TestTheTransitionRecountStampsTheReport: while the air-gap transition is asked for, the leader's
// pass, idle or not, ends with a recount of its store — see StoreIsAuthority — which is the reading the
// controller waits for after the leader restarts. One that failed stamps nothing.
func TestTheTransitionRecountStampsTheReport(t *testing.T) {
	transition := func(t *testing.T, store string) (*Loop, client.Client) {
		t.Helper()
		storage := storageWith(registryv1alpha1.RegistryStorageSpec{
			Upstream: &registryv1alpha1.Upstream{
				Endpoint: registryv1alpha1.Endpoint{Scheme: registryv1alpha1.SchemeHTTP, Host: "held.example.invalid"},
			},
			Publish:         true,
			AirGapRequested: true,
		})
		storage.Status.Replicas = []registryv1alpha1.StorageReplicaStatus{{
			Node: "master-0", Role: registryv1alpha1.ReplicaRoleLeader, Full: true, VerifiedDigests: 459,
			SyncerStartedAt: &previousStart,
		}}
		loop, c, _ := newLoop(t, true, storage, deployedRelease("v1.76.6"))
		set, err := CurrentSetKey(context.Background(), c)
		require.NoError(t, err)
		stored := statusOf(t, c)
		stored.Status.Replicas[0].VerifiedSet = set
		require.NoError(t, c.Status().Update(context.Background(), stored))
		loop.LocalAddress = store
		loop.WriteAddress = store
		loop.DataDir = t.TempDir()
		loop.started = processStart
		// The idle pass's own recheck is not what this is about.
		loop.lastStoreCheck.Store(time.Now().UnixNano())
		return loop, c
	}

	t.Run("a recount that read the store", func(t *testing.T) {
		store := startRegistry(t)
		running := pushDigest(t, store, "system/deckhouse")
		pushImage(t, store, "system/deckhouse:v1.76.6")
		pushInstaller(t, store, "system/deckhouse/install:v1.76.6", []string{running})
		loop, c := transition(t, store)
		release := resolveTag(t, store, "system/deckhouse", "v1.76.6")
		heldOnDisk(t, loop.DataDir, running, release)
		tagOnDisk(t, loop.DataDir, "v1.76.6", release)

		require.NoError(t, loop.once(context.Background()))

		assert.True(t, replicaOf(t, c, "master-0").Full)
		require.NotNil(t, stampOf(t, c, "master-0"))
		assert.Equal(t, processStart.Truncate(time.Second), *stampOf(t, c, "master-0"))
	})

	t.Run("a recount that failed", func(t *testing.T) {
		loop, c := transition(t, startRegistry(t))

		require.NoError(t, loop.once(context.Background()))

		assert.NotEmpty(t, replicaOf(t, c, "master-0").Error, "the recount failed and says so")
		assert.Nil(t, stampOf(t, c, "master-0"), "a reading that failed is no reading")
	})
}
