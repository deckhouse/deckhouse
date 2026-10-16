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
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"

	registryv1alpha1 "github.com/deckhouse/deckhouse/go_lib/registry/apis/deckhouse.io/v1alpha1"
	"github.com/deckhouse/registry-syncer/internal/fill"
	"github.com/deckhouse/registry-syncer/internal/gc"
	"github.com/deckhouse/registry-syncer/internal/report"
)

// TestAnAirGappedLeaderDoesNotHoldTheCollectionBack: needSync stays up for the followers while the
// leader is short of the set, and an air-gapped leader has nothing to fill from. Read as "a fill is
// owed here", that held its collection back for good — and a store over its budget could then neither
// be reclaimed nor take the rest of the set.
func TestAnAirGappedLeaderDoesNotHoldTheCollectionBack(t *testing.T) {
	storage := storageWith(registryv1alpha1.RegistryStorageSpec{NeedSync: true})

	loop, _, _ := newLoop(t, true, nil)
	assert.True(t, loop.OwesCopy(), "undecided, a replica holds the collection back")

	c := releaseAwareClient(t, storage, deployedRelease("v1.76.6"))
	loop.Client = c
	loop.Publisher = &report.Publisher{Client: c}
	loop.DataDir = t.TempDir()
	loop.lastStoreCheck.Store(time.Now().UnixNano())

	require.NoError(t, loop.once(context.Background()))
	assert.False(t, loop.Complete(), "short of the set")
	assert.False(t, loop.OwesCopy(), "but with nothing to copy from, it holds no collection back")
}

// TestAnIdlePassEndsASilence: the registry's silence is timed across the passes that wanted it. One
// that wanted nothing ends the series, so that a silence days old does not make the next ordinary
// restart of the registry an error at once.
func TestAnIdlePassEndsASilence(t *testing.T) {
	set := SetKey(gc.Releases{Deployed: "v1.76.6"}, nil, nil)
	storage := storageWith(registryv1alpha1.RegistryStorageSpec{})
	storage.Status.Replicas = []registryv1alpha1.StorageReplicaStatus{
		{Node: "master-1", Role: registryv1alpha1.ReplicaRoleLeader, Full: true, VerifiedSet: set,
			Address: closedAddress(t)},
		{Node: "master-0", Role: registryv1alpha1.ReplicaRoleFollower, Full: true, VerifiedSet: set,
			Source: "master-1"},
	}

	loop, _, _ := newLoop(t, false, nil)
	c := releaseAwareClient(t, storage, deployedRelease("v1.76.6"))
	loop.Client = c
	loop.Publisher = &report.Publisher{Client: c}
	loop.lastStoreCheck.Store(time.Now().UnixNano())
	loop.registryDownSince.Store(time.Now().Add(-72 * time.Hour).UnixNano())

	require.NoError(t, loop.once(context.Background()))
	assert.Zero(t, loop.registryDownSince.Load())
}

// TestACopyAheadStopsWhenItsSourceIsNoLongerThis: started under one arrangement — a role, a leader, an
// upstream and its account — a copy ahead does not go on under another.
func TestACopyAheadStopsWhenItsSourceIsNoLongerThis(t *testing.T) {
	loop, _, _ := newLoop(t, true, nil)
	block := func(ctx context.Context) fill.Report {
		<-ctx.Done()
		return fill.Report{}
	}

	upstream := &registryv1alpha1.RegistryStorageSpec{Upstream: &registryv1alpha1.Upstream{
		Endpoint: registryv1alpha1.Endpoint{Host: "registry.example.com", Path: "/deckhouse/ee"},
	}}
	scope := aheadScope(upstream, true, nil)
	loop.startAhead(context.Background(), block, "pending", scope)
	require.True(t, loop.Filling())

	loop.stopAheadOutside(scope)
	assert.True(t, loop.Filling(), "the same arrangement: it goes on")

	moved := upstream.DeepCopy()
	moved.Upstream.Auth = &registryv1alpha1.Auth{Username: "another"}
	assert.NotEqual(t, scope, aheadScope(moved, true, nil), "another account is another arrangement")
	assert.NotEqual(t, scope, aheadScope(upstream, false, &Leader{Node: "master-1"}), "so is following")
	assert.NotEqual(t, scope, aheadScope(&registryv1alpha1.RegistryStorageSpec{}, true, nil),
		"and so is the upstream dropped")

	loop.stopAheadOutside(aheadScope(moved, true, nil))
	assert.False(t, loop.Filling(), "another one: it stops")
	assert.Empty(t, loop.heldAhead("pending", ""), "and holds nothing")
}

// TestTheCopyAheadLeavesRoomForTheSet: none starts into a store with less than a fifth of its budget
// free — or of its filesystem, without a budget — or one that refuses writes.
func TestTheCopyAheadLeavesRoomForTheSet(t *testing.T) {
	const gib = int64(1) << 30
	for _, tc := range []struct {
		name  string
		store *registryv1alpha1.StoreUsage
		room  bool
	}{
		{"not measured", nil, true},
		{"refusing writes", &registryv1alpha1.StoreUsage{Writable: false, FilesystemFreeBytes: 100 * gib}, false},
		{"budget mostly free", &registryv1alpha1.StoreUsage{Writable: true, BudgetBytes: 100 * gib,
			UsedBytes: 50 * gib, FilesystemFreeBytes: 500 * gib, FilesystemCapacityBytes: 1000 * gib}, true},
		{"budget nearly spent", &registryv1alpha1.StoreUsage{Writable: true, BudgetBytes: 100 * gib,
			UsedBytes: 85 * gib, FilesystemFreeBytes: 500 * gib, FilesystemCapacityBytes: 1000 * gib}, false},
		{"budget free, disk not", &registryv1alpha1.StoreUsage{Writable: true, BudgetBytes: 100 * gib,
			UsedBytes: 10 * gib, FilesystemFreeBytes: 12 * gib, ReserveBytes: 2 * gib,
			FilesystemCapacityBytes: 1000 * gib}, false},
		{"no budget, disk free", &registryv1alpha1.StoreUsage{Writable: true,
			FilesystemFreeBytes: 400 * gib, ReserveBytes: 10 * gib, FilesystemCapacityBytes: 1000 * gib}, true},
		{"no budget, disk nearly full", &registryv1alpha1.StoreUsage{Writable: true,
			FilesystemFreeBytes: 150 * gib, ReserveBytes: 10 * gib, FilesystemCapacityBytes: 1000 * gib}, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.room, roomAhead(tc.store))
		})
	}
}

// TestAnUpdateFromAnUnreadableSourceDoesNotStopTheFill: the source of a pending module update cannot
// be read — its credentials are a placeholder. The set is copied all the same, and the update is a
// miss of the copy ahead rather than the end of the fill. A module the cluster runs from such a source
// still is the end of it: that set cannot be copied.
func TestAnUpdateFromAnUnreadableSourceDoesNotStopTheFill(t *testing.T) {
	loop, _, _ := newLoop(t, true, nil)
	spec := &registryv1alpha1.RegistryStorageSpec{Upstream: &registryv1alpha1.Upstream{
		Endpoint: registryv1alpha1.Endpoint{Scheme: registryv1alpha1.SchemeHTTP, Host: closedAddress(t),
			Path: "/deckhouse/ee"},
	}}
	loop.WriteAddress = closedAddress(t)
	releases := gc.Releases{Deployed: "v1.76.6"}
	unreadable := fill.ModuleSource{Name: "vendor", Repository: "vendor.example.com/modules", DockerCfg: "<hidden>"}

	copier, err := loop.copier(spec, releases, []fill.ModuleRef{
		{Name: "vendor-module", Version: "v1.1.0", Source: unreadable, Pending: true},
	})
	require.NoError(t, err)
	require.Len(t, copier.AheadUnread, 1)
	assert.Contains(t, copier.AheadUnread[0], "vendor-module")

	_, err = loop.copier(spec, releases, []fill.ModuleRef{
		{Name: "vendor-module", Version: "v1.0.0", Source: unreadable},
	})
	assert.Error(t, err)
}

// TestAFollowerTakesThePartsItsLeaderHolds is the review's finding on a single fingerprint for all of
// what is ahead of the set: an air-gapped leader holds the pushed release, and a pending module update
// whose package never made it into the bundle. Counted as one, the leader held nothing ahead and its
// follower copied nothing — not even the release. Counted by part, the follower takes the release.
func TestAFollowerTakesThePartsItsLeaderHolds(t *testing.T) {
	leaderStorage := startRegistry(t)
	followerStorage := startRegistry(t)

	running := pushDigest(t, leaderStorage, "system/deckhouse")
	pushImage(t, leaderStorage, "system/deckhouse:v1.76.6")
	pushInstaller(t, leaderStorage, "system/deckhouse/install:v1.76.6", []string{running})
	update := pushDigest(t, leaderStorage, "system/deckhouse")
	pushImage(t, leaderStorage, "system/deckhouse:v1.77.0")
	pushInstaller(t, leaderStorage, "system/deckhouse/install:v1.77.0", []string{update})

	pending := deployedRelease("v1.77.0")
	pending.Object["status"] = map[string]any{"phase": "Pending"}
	// Never pushed.
	module := &unstructured.Unstructured{Object: map[string]any{
		"spec":   map[string]any{"moduleName": "snapshot-controller", "version": "0.1.21"},
		"status": map[string]any{"phase": "Pending"},
	}}
	module.SetGroupVersionKind(schema.GroupVersionKind{Group: "deckhouse.io", Version: "v1alpha1", Kind: "ModuleRelease"})
	module.SetName("snapshot-controller-v0.1.21")

	set := SetKey(gc.Releases{Deployed: "v1.76.6"}, nil, nil)
	release := fill.PlatformPart("v1.77.0")

	storage := storageWith(registryv1alpha1.RegistryStorageSpec{})
	storage.Status.Replicas = []registryv1alpha1.StorageReplicaStatus{
		{Node: "master-1", Role: registryv1alpha1.ReplicaRoleLeader, Full: true, VerifiedSet: set,
			AheadSet: release, Address: leaderStorage},
		{Node: "master-0", Role: registryv1alpha1.ReplicaRoleFollower, Full: true, VerifiedSet: set,
			Source: "master-1"},
	}

	loop, _, _ := newLoop(t, false, nil)
	c := releaseAwareClient(t, storage, deployedRelease("v1.76.6"), pending, module)
	loop.Client = c
	loop.Publisher = &report.Publisher{Client: c}
	loop.LocalAddress = followerStorage
	loop.WriteAddress = followerStorage
	loop.ReportedAddress = followerStorage
	loop.lastStoreCheck.Store(time.Now().UnixNano())

	_, current, err := CurrentKeys(context.Background(), c)
	require.NoError(t, err)
	require.Len(t, splitParts(current), 2, "the release and the module update are two parts")

	require.NoError(t, loop.once(context.Background()))
	loop.waitAhead()
	require.NoError(t, loop.once(context.Background()))

	assert.Equal(t, release, replicaOf(t, c, "master-0").AheadSet,
		"the follower holds the release its leader holds, and does not claim the module neither has")
	assert.Equal(t, resolveTag(t, leaderStorage, "system/deckhouse", "v1.77.0"),
		resolveTag(t, followerStorage, "system/deckhouse", "v1.77.0"))
}

// TestHeldPartsFollowWhatIsPending: a part reported before a restart still counts; one no longer
// pending falls away; and after the collection took what is ahead back, nothing reported counts.
func TestHeldPartsFollowWhatIsPending(t *testing.T) {
	loop, _, _ := newLoop(t, true, nil)
	assert.Equal(t, "a,b", loop.heldAhead(joinParts([]string{"b", "a", "c"}), "b,a"))
	assert.Equal(t, "a", loop.heldAhead("a", "a,b"), "b is no longer pending")
	assert.Empty(t, loop.heldAhead("", "a"), "nothing pending, nothing ahead")

	loop.ForgetAhead()
	assert.Empty(t, loop.heldAhead("a", "a"), "given back to the collection: no longer held")
}
