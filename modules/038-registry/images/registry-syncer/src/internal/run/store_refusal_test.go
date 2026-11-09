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
	"io"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"sync/atomic"
	"syscall"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	registryv1alpha1 "github.com/deckhouse/deckhouse/go_lib/registry/apis/deckhouse.io/v1alpha1"

	"github.com/deckhouse/registry-syncer/internal/report"
)

// TestAStoreOverItsBudgetRefusesWrites is what lets a collection start on a store a fill can no longer
// write to: waiting for that fill would wait for the room only the collection gives back.
func TestAStoreOverItsBudgetRefusesWrites(t *testing.T) {
	dir := t.TempDir()
	blobs := filepath.Join(dir, "docker", "registry", "v2", "blobs", "sha256", "ab", "abcd")
	require.NoError(t, os.MkdirAll(blobs, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(blobs, "data"), make([]byte, 64<<10), 0o644))

	loop := &Loop{Log: slog.New(slog.NewTextHandler(io.Discard, nil)), DataDir: dir}

	assert.False(t, loop.StoreRefusesWrites(), "nothing read yet is not a refusal")

	loop.desired.Store(&registryv1alpha1.RegistryStorageSpec{Store: registryv1alpha1.StorageStore{Size: "1Ki"}})
	assert.True(t, loop.StoreRefusesWrites(), "64 KiB in a 1 KiB budget")

	unmeasured := &Loop{Log: loop.Log}
	unmeasured.desired.Store(&registryv1alpha1.RegistryStorageSpec{Store: registryv1alpha1.StorageStore{Size: "1Ki"}})
	assert.False(t, unmeasured.StoreRefusesWrites(), "a store that cannot be measured is not a refusal")
}

// TestAStoreThatIsNotWritableIsPublishedAsRefusingWrites: the measurement knows only space, so a store
// nobody could write to read writable and no alert fired. The store watcher probes before it measures,
// so it says NotWritable within a tick — not when a pass that may take an hour ends — and takes it back
// once the directory is fixed.
func TestAStoreThatIsNotWritableIsPublishedAsRefusingWrites(t *testing.T) {
	storage := storageWith(registryv1alpha1.RegistryStorageSpec{
		Store: registryv1alpha1.StorageStore{Size: "1Gi"},
	})
	storage.Status.Replicas = []registryv1alpha1.StorageReplicaStatus{{Node: "master-0", Full: true}}
	loop, c, _ := newLoop(t, true, storage)
	loop.DataDir = t.TempDir()
	loop.StoreHostPath = storeHostPath
	loop.Publisher = &report.Publisher{Client: c}
	loop.desired.Store(&storage.Spec)

	var refused atomic.Bool
	refused.Store(true)
	loop.checkWritable = func(dataDir, _ string) error {
		if refused.Load() {
			return notWritable(dataDir)
		}
		return nil
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go loop.watchStore(ctx, 20*time.Millisecond)

	require.Eventually(t, func() bool {
		store := replicaOf(t, c, "master-0").Store
		return store != nil && !store.Writable
	}, 5*time.Second, 20*time.Millisecond)
	store := replicaOf(t, c, "master-0").Store
	assert.Equal(t, registryv1alpha1.StoreNotWritable, store.Reason)
	assert.Equal(t, int64(1<<30), store.BudgetBytes, "the rest of the measurement is published as it is")

	// Back to what the measurement says, which on the disk the test runs on may be a limit.
	refused.Store(false)
	require.Eventually(t, func() bool {
		store := replicaOf(t, c, "master-0").Store
		return store != nil && store.Reason != registryv1alpha1.StoreNotWritable
	}, 5*time.Second, 20*time.Millisecond)
}

// TestNotWritableWinsOverALimit: a collection or a larger budget gives room to a store the registry
// still cannot write to, so the reason published is the one only the operator's fix answers.
func TestNotWritableWinsOverALimit(t *testing.T) {
	loop := &Loop{Log: slog.New(slog.NewTextHandler(io.Discard, nil)), DataDir: t.TempDir()}
	loop.checkWritable = func(dataDir, _ string) error { return notWritable(dataDir) }
	require.Error(t, loop.probeWritable())

	measured := &registryv1alpha1.StoreUsage{UsedBytes: 2 << 30, BudgetBytes: 1 << 30,
		Reason: registryv1alpha1.StoreBudgetExhausted}
	published := loop.publishedStore(measured)
	assert.False(t, published.Writable)
	assert.Equal(t, registryv1alpha1.StoreNotWritable, published.Reason)
	assert.Equal(t, registryv1alpha1.StoreBudgetExhausted, measured.Reason, "the measurement itself is left as it is")
	assert.Nil(t, loop.publishedStore(nil), "nothing measured, ever, and nothing published")
}

// roomyStore is a measurement with room to spare under both limits: what the disk a test runs on has
// left is no figure to assert on.
func roomyStore(_ string, spec *registryv1alpha1.RegistryStorageSpec, dedicated bool) (*registryv1alpha1.StoreUsage, error) {
	return &registryv1alpha1.StoreUsage{
		UsedBytes: 1 << 30, BudgetBytes: 100 << 30, ReserveBytes: 10 << 30,
		FilesystemFreeBytes: 400 << 30, FilesystemCapacityBytes: 500 << 30,
		DedicatedFilesystem: dedicated, Writable: true,
	}, nil
}

// TestNotWritableLeavesTheCollectionAlone: only what is published says NotWritable. What the collection
// on this replica asks — whether there is room ahead of the set, whether the store is at a limit — stays
// the measurement's: a store with room and a directory that refuses has nothing a collection gives back,
// and releasing what is ahead of the set, or sweeping without waiting for the fill, would only lose it.
func TestNotWritableLeavesTheCollectionAlone(t *testing.T) {
	loop := &Loop{Log: slog.New(slog.NewTextHandler(io.Discard, nil)), DataDir: t.TempDir(), measure: roomyStore}
	loop.desired.Store(&registryv1alpha1.RegistryStorageSpec{Store: registryv1alpha1.StorageStore{Size: "100Gi"}})
	loop.checkWritable = func(dataDir, _ string) error { return notWritable(dataDir) }
	require.Error(t, loop.probeWritable())

	published := loop.publishedStore(loop.measureStore(loop.Desired()))
	require.NotNil(t, published)
	assert.False(t, published.Writable, "published as NotWritable")
	assert.Equal(t, registryv1alpha1.StoreNotWritable, published.Reason)

	assert.True(t, loop.RoomAhead(),
		"so the collection keeps what is ahead of the set, as it would for this room (ReleaseAhead)")
	assert.False(t, loop.StoreRefusesWrites(),
		"so the collection waits for the fill and does not sweep early, as it would for this room (SweepAnyway, FillPending)")
}

// TestAStoreThatCannotBeMeasuredIsStillNotWritable: the syncer is held to the same mode bits as the
// registry, so a tree the registry cannot write to is often one the syncer cannot walk either. A failed
// measurement leaves the status as it is, which said writable for exactly that store; it is published
// from the last measurement instead, and with that one's figures.
func TestAStoreThatCannotBeMeasuredIsStillNotWritable(t *testing.T) {
	storage := storageWith(registryv1alpha1.RegistryStorageSpec{
		Store: registryv1alpha1.StorageStore{Size: "100Gi"},
	})
	storage.Status.Replicas = []registryv1alpha1.StorageReplicaStatus{{Node: "master-0", Full: true}}
	loop, c, _ := newLoop(t, true, storage)
	loop.DataDir = t.TempDir()
	loop.StoreHostPath = storeHostPath
	loop.Publisher = &report.Publisher{Client: c}
	loop.desired.Store(&storage.Spec)

	var unreadable atomic.Bool
	loop.measure = func(dataDir string, spec *registryv1alpha1.RegistryStorageSpec, dedicated bool) (*registryv1alpha1.StoreUsage, error) {
		if unreadable.Load() {
			return nil, &fs.PathError{Op: "open", Path: dataDir, Err: syscall.EACCES}
		}
		return roomyStore(dataDir, spec, dedicated)
	}
	loop.checkWritable = func(dataDir, _ string) error {
		if unreadable.Load() {
			return notWritable(dataDir)
		}
		return nil
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go loop.watchStore(ctx, 20*time.Millisecond)

	require.Eventually(t, func() bool {
		store := replicaOf(t, c, "master-0").Store
		return store != nil && store.Writable
	}, 5*time.Second, 20*time.Millisecond, "measured and writable first")

	unreadable.Store(true)
	require.Eventually(t, func() bool {
		store := replicaOf(t, c, "master-0").Store
		return store != nil && !store.Writable
	}, 5*time.Second, 20*time.Millisecond)
	store := replicaOf(t, c, "master-0").Store
	assert.Equal(t, registryv1alpha1.StoreNotWritable, store.Reason)
	assert.Equal(t, int64(1<<30), store.UsedBytes, "the figures are the last measurement's")

	// Readable and writable again: measured, and published as it is.
	unreadable.Store(false)
	require.Eventually(t, func() bool {
		store := replicaOf(t, c, "master-0").Store
		return store != nil && store.Writable
	}, 5*time.Second, 20*time.Millisecond)
}

// TestARestartedSyncerStartsFromWhatTheReplicaReported: a syncer restarted over a store it can neither
// measure nor write to has measured nothing yet, and starts from what the replica last reported — but
// not from a NotWritable a syncer before it reported, which says nothing about the store's room.
func TestARestartedSyncerStartsFromWhatTheReplicaReported(t *testing.T) {
	refusing := func(reported *registryv1alpha1.StoreUsage) *Loop {
		storage := storageWith(registryv1alpha1.RegistryStorageSpec{})
		storage.Status.Replicas = []registryv1alpha1.StorageReplicaStatus{{Node: "master-0", Store: reported}}
		loop, _, _ := newLoop(t, true, storage)
		loop.DataDir = t.TempDir()
		loop.recallStore(context.Background())
		loop.checkWritable = func(dataDir, _ string) error { return notWritable(dataDir) }
		require.Error(t, loop.probeWritable())
		return loop
	}

	loop := refusing(&registryv1alpha1.StoreUsage{UsedBytes: 5 << 30, BudgetBytes: 10 << 30, Writable: true})
	published := loop.publishedStore(nil)
	require.NotNil(t, published, "nothing measured, and the replica's report to start from")
	assert.Equal(t, registryv1alpha1.StoreNotWritable, published.Reason)
	assert.Equal(t, int64(5<<30), published.UsedBytes)

	loop = refusing(&registryv1alpha1.StoreUsage{UsedBytes: 5 << 30, Reason: registryv1alpha1.StoreNotWritable})
	assert.Nil(t, loop.publishedStore(nil), "the status already says NotWritable, and is left as it is")
}
