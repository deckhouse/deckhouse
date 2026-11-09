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
	"bytes"
	"context"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	registryv1alpha1 "github.com/deckhouse/deckhouse/go_lib/registry/apis/deckhouse.io/v1alpha1"

	"github.com/deckhouse/registry-syncer/internal/distribution"
	"github.com/deckhouse/registry-syncer/internal/report"
)

const storeHostPath = "/opt/deckhouse/registry/local_data"

func TestFilesystemWarning(t *testing.T) {
	dedicated := distribution.Filesystem{Dedicated: true}
	shared := distribution.Filesystem{}

	cases := map[string]struct {
		before    *registryv1alpha1.StoreUsage
		now       distribution.Filesystem
		holdsData bool
		contains  string
	}{
		"the disk the last run was on is not mounted this time": {
			before:   &registryv1alpha1.StoreUsage{DedicatedFilesystem: true, UsedBytes: 5 << 30},
			now:      shared,
			contains: "the store is empty and on the node's own filesystem",
		},
		"the disk is still not mounted while the store fills the node's filesystem": {
			before:    &registryv1alpha1.StoreUsage{DedicatedFilesystem: true, UsedBytes: 5 << 30},
			now:       shared,
			holdsData: true,
			contains:  "the disk meant for " + storeHostPath + " is probably not mounted",
		},
		"a disk mounted over a store that was filling the node's filesystem": {
			before:   &registryv1alpha1.StoreUsage{UsedBytes: 5 << 30},
			now:      dedicated,
			contains: "hidden under the disk mounted at " + storeHostPath,
		},
		"a store filling the disk mounted over the node's directory": {
			before:    &registryv1alpha1.StoreUsage{UsedBytes: 5 << 30},
			now:       dedicated,
			holdsData: true,
		},
		"the same disk as last time": {
			before: &registryv1alpha1.StoreUsage{DedicatedFilesystem: true, UsedBytes: 5 << 30},
			now:    dedicated,
		},
		"the node's filesystem as last time": {
			before: &registryv1alpha1.StoreUsage{UsedBytes: 5 << 30},
			now:    shared,
		},
		"a new disk under a store that never held anything": {
			before: &registryv1alpha1.StoreUsage{},
			now:    dedicated,
		},
		"nothing reported before": {
			now: shared,
		},
	}

	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			warning := filesystemWarning(c.before, c.now, storeHostPath, c.holdsData)
			if c.contains == "" {
				assert.Empty(t, warning)
				return
			}
			assert.Contains(t, warning, c.contains)
		})
	}

	assert.Contains(t, filesystemWarning(&registryv1alpha1.StoreUsage{DedicatedFilesystem: true}, shared, "", false),
		"the disk meant for the store's directory on the node")
	assert.NotContains(t, filesystemWarning(&registryv1alpha1.StoreUsage{DedicatedFilesystem: true}, shared, "", true),
		"empty", "a store that fills is not said to be empty")
	assert.Contains(t, filesystemWarning(&registryv1alpha1.StoreUsage{DedicatedFilesystem: true}, shared, "", true),
		"delete "+storeMemoryFile, "the operator who keeps the store there is told how")
}

// TestAStoreOffItsDiskIsReported: on a master whose disk was not mounted, the store starts empty on the
// node's filesystem — with nothing to fill from on an air-gapped one, and filling the node's filesystem
// from the upstream or the leader on any other. The replica says why for as long as the disk is not
// there, filled or not, and stops once it is.
func TestAStoreOffItsDiskIsReported(t *testing.T) {
	storage := storageWith(registryv1alpha1.RegistryStorageSpec{
		Source: &registryv1alpha1.StorageSource{ExpectedDigests: 4},
	})
	storage.Status.Replicas = []registryv1alpha1.StorageReplicaStatus{{
		Node: "master-0",
		Store: &registryv1alpha1.StoreUsage{
			UsedBytes: 5 << 30, DedicatedFilesystem: true, Writable: true,
		},
	}}
	loop, c, _ := newLoop(t, true, storage, deployedRelease("v1.70.1"))
	loop.LocalAddress = startRegistry(t)
	loop.WriteAddress = loop.LocalAddress
	loop.DataDir = t.TempDir()
	loop.StoreHostPath = storeHostPath
	ctx := context.Background()

	loop.recallStore(ctx)
	require.NoError(t, loop.once(ctx))

	replica := replicaOf(t, c, "master-0")
	assert.Contains(t, replica.Error, "is probably not mounted")
	require.NotNil(t, replica.Store)
	assert.False(t, replica.Store.DedicatedFilesystem, "the status says where the store is now")

	require.NoError(t, loop.once(ctx))
	assert.Contains(t, replicaOf(t, c, "master-0").Error, "is probably not mounted", "still empty")

	blobOnDisk(t, loop.DataDir, "sha256:0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef", []byte("held"))
	require.NoError(t, loop.once(ctx))
	assert.Contains(t, replicaOf(t, c, "master-0").Error, "is probably not mounted",
		"filled, and filling the node's filesystem")

	// The disk is back.
	loop.Applier.Options.StoreFilesystem = distribution.Filesystem{Dedicated: true}
	require.NoError(t, loop.once(ctx))
	assert.NotContains(t, replicaOf(t, c, "master-0").Error, "not mounted",
		"a store that holds something on its disk is the store the replica works with now")
	assert.Nil(t, loop.lastRun)
}

// TestTheWarningOutlivesARestart: the first pass reports the store as it is now, over what the last run
// reported, so a pod restarted with the disk still not mounted has only the store's own memory to go
// by — and must still say it, for as long as the disk is not there, whatever the store holds by then.
func TestTheWarningOutlivesARestart(t *testing.T) {
	storage := storageWith(registryv1alpha1.RegistryStorageSpec{
		Source: &registryv1alpha1.StorageSource{ExpectedDigests: 4},
	})
	storage.Status.Replicas = []registryv1alpha1.StorageReplicaStatus{{
		Node:  "master-0",
		Store: &registryv1alpha1.StoreUsage{UsedBytes: 5 << 30, DedicatedFilesystem: true, Writable: true},
	}}
	dataDir := t.TempDir()
	ctx := context.Background()

	start := func(storage *registryv1alpha1.RegistryStorage) (*Loop, client.Client) {
		loop, c, _ := newLoop(t, true, storage, deployedRelease("v1.70.1"))
		loop.LocalAddress = startRegistry(t)
		loop.WriteAddress = loop.LocalAddress
		loop.DataDir = dataDir
		loop.StoreHostPath = storeHostPath
		loop.recallStore(ctx)
		return loop, c
	}

	first, c := start(storage)
	require.NoError(t, first.once(ctx))
	assert.Contains(t, replicaOf(t, c, "master-0").Error, "is probably not mounted")
	assert.FileExists(t, filepath.Join(dataDir, storeMemoryFile))

	// Restarted: what the replica reported now says the store is empty and on the node's filesystem.
	reported := &registryv1alpha1.RegistryStorage{}
	require.NoError(t, c.Get(ctx, types.NamespacedName{Name: registryv1alpha1.SingletonName}, reported))
	require.False(t, replicaOf(t, c, "master-0").Store.DedicatedFilesystem)
	reported.ResourceVersion = ""

	second, c := start(reported)
	require.NoError(t, second.once(ctx))
	assert.Contains(t, replicaOf(t, c, "master-0").Error, "is probably not mounted", "still empty, still off its disk")

	blobOnDisk(t, dataDir, "sha256:0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef", []byte("held"))
	require.NoError(t, second.once(ctx))
	assert.Contains(t, replicaOf(t, c, "master-0").Error, "is probably not mounted", "filled, still off its disk")
	assert.FileExists(t, filepath.Join(dataDir, storeMemoryFile))

	// Restarted again, the store full by now: the memory is all that still says where it belongs.
	reported = &registryv1alpha1.RegistryStorage{}
	require.NoError(t, c.Get(ctx, types.NamespacedName{Name: registryv1alpha1.SingletonName}, reported))
	reported.ResourceVersion = ""
	third, c := start(reported)
	require.NoError(t, third.once(ctx))
	assert.Contains(t, replicaOf(t, c, "master-0").Error, "is probably not mounted", "filled, still off its disk")

	// The operator keeps the store on the node's filesystem: the memory and the pod are deleted.
	require.NoError(t, os.Remove(filepath.Join(dataDir, storeMemoryFile)))
	reported = &registryv1alpha1.RegistryStorage{}
	require.NoError(t, c.Get(ctx, types.NamespacedName{Name: registryv1alpha1.SingletonName}, reported))
	reported.ResourceVersion = ""
	fourth, c := start(reported)
	require.NoError(t, fourth.once(ctx))
	assert.NotContains(t, replicaOf(t, c, "master-0").Error, "not mounted")
	assert.NoFileExists(t, filepath.Join(dataDir, storeMemoryFile))
}

// TestAStoreThatCannotBeMeasuredIsNotSaidToBeEmpty: the warning about a disk mounted over what the store
// held says the store is empty, which a walk that failed does not tell. What was said from the last
// measure stands until the next.
func TestAStoreThatCannotBeMeasuredIsNotSaidToBeEmpty(t *testing.T) {
	loop, _, _ := newLoop(t, true, nil)
	loop.DataDir = t.TempDir()
	loop.StoreHostPath = storeHostPath
	loop.checkWritable = func(string, string) error { return nil }
	loop.Applier.Options.StoreFilesystem = distribution.Filesystem{Dedicated: true}
	loop.lastRun = &registryv1alpha1.StoreUsage{UsedBytes: 5 << 30}

	assert.Empty(t, loop.storeProblems(nil), "not measured yet")
	assert.Contains(t, loop.storeProblems(&registryv1alpha1.StoreUsage{}), "hidden under the disk")
	assert.Contains(t, loop.storeProblems(nil), "hidden under the disk", "not measured this time")
	assert.Empty(t, loop.storeProblems(&registryv1alpha1.StoreUsage{UsedBytes: 16 << 20}))
	assert.Empty(t, loop.storeProblems(nil))
}

// TestAStoreTheRegistryCannotWriteToIsReported: the registry answers such a store's writes with a 500
// that names no file, so the replica's error has to say which directory, whose, and what to run. The
// refusal itself is distribution's to test: these tests may run as root with every capability, which
// no mode bit stops.
func TestAStoreTheRegistryCannotWriteToIsReported(t *testing.T) {
	loop, c, _ := newLoop(t, true, storageWith(registryv1alpha1.RegistryStorageSpec{
		Source: &registryv1alpha1.StorageSource{ExpectedDigests: 4},
	}), deployedRelease("v1.70.1"))
	loop.LocalAddress = startRegistry(t)
	loop.WriteAddress = loop.LocalAddress
	loop.DataDir = t.TempDir()
	loop.StoreHostPath = storeHostPath

	var refusal error
	loop.checkWritable = func(dataDir, hostPath string) error {
		assert.Equal(t, loop.DataDir, dataDir)
		assert.Equal(t, storeHostPath, hostPath)
		return refusal
	}
	refusal = notWritable(loop.DataDir)
	ctx := context.Background()

	loop.inspectStore()
	require.NoError(t, loop.once(ctx))

	replica := replicaOf(t, c, "master-0")
	assert.True(t, strings.HasPrefix(replica.Error, "the store is not writable"),
		"in front of the pass's own error: %s", replica.Error)
	assert.Contains(t, replica.Error, "uid 1000, gid 1000")
	assert.Contains(t, replica.Error, "chown -R root:root "+storeHostPath)
	require.NotNil(t, replica.Store)
	assert.False(t, replica.Store.Writable, "published as refusing writes, for the condition and the alert")
	assert.Equal(t, registryv1alpha1.StoreNotWritable, replica.Store.Reason)

	refusal = nil
	require.NoError(t, loop.once(ctx))
	replica = replicaOf(t, c, "master-0")
	assert.NotContains(t, replica.Error, "not writable", "fixed, and no longer said")
	// Whatever the space on the disk the test runs on says: that is the measurement's.
	assert.NotEqual(t, registryv1alpha1.StoreNotWritable, replica.Store.Reason)
}

// notWritable is the refusal of a store directory owned by another user, as CheckWritable reports it.
func notWritable(dataDir string) error {
	return &distribution.NotWritableError{
		Dir: dataDir, HostDir: storeHostPath, HostPath: storeHostPath,
		Err:   &fs.PathError{Op: "open", Path: dataDir, Err: syscall.EACCES},
		Owner: 1000, Group: 1000, Mode: fs.ModeDir | 0o755, Described: true,
	}
}

// TestAPassUnderWayKeepsTheStoreNotWritable: a store the registry cannot write to still counts every
// reference it holds as done, so a pass over it reports progress. Those reports took the error off the
// status for the whole pass — and on a replica `full` from an earlier pass, which a progress report
// leaves alone, the error is the one thing that keeps it from being elected or copied from.
func TestAPassUnderWayKeepsTheStoreNotWritable(t *testing.T) {
	storage := storageWith(registryv1alpha1.RegistryStorageSpec{})
	storage.Status.Replicas = []registryv1alpha1.StorageReplicaStatus{{
		Node: "master-0", Role: registryv1alpha1.ReplicaRoleLeader, Full: true, VerifiedSet: "abc",
		VerifiedDigests: 459, DeclaredDigests: 459, Error: "the pass before",
	}}
	loop, c, _ := newLoop(t, true, storage)
	loop.DataDir = t.TempDir()
	loop.StoreHostPath = storeHostPath
	var refusal error
	loop.checkWritable = func(string, string) error { return refusal }
	refusal = notWritable(loop.DataDir)

	progress := loop.progress(context.Background(), report.State{Node: "master-0"}, "fill")
	progress(1, 459) // the first reference, found already there

	replica := replicaOf(t, c, "master-0")
	assert.True(t, strings.HasPrefix(replica.Error, "the store is not writable"), "got %q", replica.Error)
	assert.NotContains(t, replica.Error, "the pass before", "the previous pass's error is behind this one")
	assert.True(t, replica.Full, "progress leaves Full alone, so only the error keeps this replica out")

	// Fixed while the pass runs: the next report clears it.
	refusal = nil
	progress = loop.progress(context.Background(), report.State{Node: "master-0"}, "fill")
	progress(2, 459)
	assert.Empty(t, replicaOf(t, c, "master-0").Error)
}

// TestAPassUnderWayKeepsTheStoreOffItsDisk: a disk not mounted after a reboot, and the store filling
// again from the upstream or the leader onto the node's filesystem. The first progress report took the
// warning off the status, removed the store's memory of its disk, and nothing said it again — not the
// end of the pass, not a restart. It stands for the whole fill and after it, until the disk is back.
func TestAPassUnderWayKeepsTheStoreOffItsDisk(t *testing.T) {
	storage := storageWith(registryv1alpha1.RegistryStorageSpec{})
	storage.Status.Replicas = []registryv1alpha1.StorageReplicaStatus{{Node: "master-0"}}
	loop, c, _ := newLoop(t, true, storage)
	var logged bytes.Buffer
	loop.Log = slog.New(slog.NewTextHandler(&logged, nil))
	loop.DataDir = t.TempDir()
	loop.StoreHostPath = storeHostPath
	loop.checkWritable = func(string, string) error { return nil }
	loop.lastRun = &registryv1alpha1.StoreUsage{DedicatedFilesystem: true, UsedBytes: 5 << 30}

	require.Contains(t, loop.storeProblems(&registryv1alpha1.StoreUsage{}), "is probably not mounted")
	require.FileExists(t, filepath.Join(loop.DataDir, storeMemoryFile))

	loop.progress(context.Background(), report.State{Node: "master-0"}, "fill")(1, 459)

	assert.FileExists(t, filepath.Join(loop.DataDir, storeMemoryFile), "remembered for a restart")
	assert.NotNil(t, loop.lastRun)
	assert.Contains(t, replicaOf(t, c, "master-0").Error, "is probably not mounted")
	assert.Contains(t, loop.storeProblems(&registryv1alpha1.StoreUsage{UsedBytes: 16 << 20}),
		"is probably not mounted", "the pass ends with the store full on the node's filesystem")

	// The disk is back, under a store that held something there.
	loop.Applier.Options.StoreFilesystem = distribution.Filesystem{Dedicated: true}
	loop.progress(context.Background(), report.State{Node: "master-0"}, "fill")(1, 459)

	assert.NoFileExists(t, filepath.Join(loop.DataDir, storeMemoryFile))
	assert.Nil(t, loop.lastRun)
	assert.Contains(t, logged.String(), "the store no longer needs attention")
	assert.Empty(t, replicaOf(t, c, "master-0").Error)
}
