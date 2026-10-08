// Copyright 2026 Flant JSC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//	http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package image

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	dhlog "github.com/deckhouse/lib-dhctl/pkg/logger"

	"github.com/deckhouse/deckhouse/dhctl/pkg/infrastructureprovider/providerdir"
)

const testUnpackDigest = "sha256:1111111111111111111111111111111111111111111111111111111111111111"

func testUnpackRequest(t *testing.T, registryCalls *atomic.Int32) UnpackRequest {
	t.Helper()

	return UnpackRequest{
		Root:   t.TempDir(),
		Name:   "candi",
		Digest: testUnpackDigest,
		Registry: func(context.Context) (*RegistryConfig, error) {
			if registryCalls != nil {
				registryCalls.Add(1)
			}
			return NewRegistryConfig("HTTPS", "r.example.com/test", "u", "p", "")
		},
	}
}

// stubUnpackDownload writes one file into the directory it is given and records the reference.
func stubUnpackDownload(calls *atomic.Int32, gotRef *string) DownloadFunc {
	return func(_ context.Context, imageRef, dir string, _ RegistryConfig, _ bool) error {
		calls.Add(1)
		if gotRef != nil {
			*gotRef = imageRef
		}
		return os.WriteFile(filepath.Join(dir, "payload"), []byte("new"), 0o644)
	}
}

// unpackRootEntries lists what an unpack left in root, lock files aside: those stay on purpose.
func unpackRootEntries(t *testing.T, root string) []string {
	t.Helper()
	entries, err := os.ReadDir(root)
	require.NoError(t, err)

	names := make([]string, 0, len(entries))
	for _, entry := range entries {
		if !strings.Contains(entry.Name(), ".lock.") {
			names = append(names, entry.Name())
		}
	}

	return names
}

func TestEnsureUnpackedExistingDirNeedsNeitherRegistryNorDownload(t *testing.T) {
	var registryCalls, downloads atomic.Int32
	request := testUnpackRequest(t, &registryCalls)
	dir := filepath.Join(request.Root, "candi@"+testUnpackDigest)
	require.NoError(t, os.MkdirAll(dir, 0o755))

	got, err := EnsureUnpacked(t.Context(), request, EnsureUnpackedOptions{Download: stubUnpackDownload(&downloads, nil)})
	require.NoError(t, err)
	require.Equal(t, dir, got)
	require.Zero(t, registryCalls.Load(), "an unpacked digest must not ask for the registry")
	require.Zero(t, downloads.Load())

	entries, err := os.ReadDir(request.Root)
	require.NoError(t, err)
	require.Len(t, entries, 1, "an unpacked digest takes no lock either")
}

func TestEnsureUnpackedDownloadsIntoPlace(t *testing.T) {
	var downloads atomic.Int32
	var gotRef, gotStaging, gotFinal string
	request := testUnpackRequest(t, nil)

	got, err := EnsureUnpacked(t.Context(), request, EnsureUnpackedOptions{
		Download: stubUnpackDownload(&downloads, &gotRef),
		Prepare: func(_ context.Context, stagingDir, finalDir string) error {
			gotStaging, gotFinal = stagingDir, finalDir
			return nil
		},
	})
	require.NoError(t, err)
	require.Equal(t, filepath.Join(request.Root, "candi@"+testUnpackDigest), got)
	require.Equal(t, int32(1), downloads.Load())
	require.Equal(t, "r.example.com/test@"+testUnpackDigest, gotRef, "without an explicit image the registry repo is pinned by digest")
	require.True(t, strings.HasPrefix(gotStaging, got+".partial-"), "prepare must see the staging copy, got %s", gotStaging)
	require.Equal(t, got, gotFinal)
	require.FileExists(t, filepath.Join(got, "payload"))
	require.Equal(t, []string{filepath.Base(got)}, unpackRootEntries(t, request.Root), "nothing but the unpacked directory may stay behind")

	info, err := os.Stat(got)
	require.NoError(t, err)
	require.Equal(t, os.FileMode(0o755), info.Mode().Perm())
}

// The lock is not a guarantee, so two processes can still unpack the same digest at once. The
// one that loses the rename must take the winner's result.
func TestEnsureUnpackedAnotherProcessFinishedFirst(t *testing.T) {
	request := testUnpackRequest(t, nil)
	dir := filepath.Join(request.Root, "candi@"+testUnpackDigest)

	got, err := EnsureUnpacked(t.Context(), request, EnsureUnpackedOptions{
		Download: func(_ context.Context, _, staging string, _ RegistryConfig, _ bool) error {
			require.NoError(t, os.WriteFile(filepath.Join(staging, "payload"), []byte("mine"), 0o644))
			require.NoError(t, os.MkdirAll(dir, 0o755))
			return os.WriteFile(filepath.Join(dir, "payload"), []byte("theirs"), 0o644)
		},
	})
	require.NoError(t, err)
	require.Equal(t, dir, got)

	content, err := os.ReadFile(filepath.Join(dir, "payload"))
	require.NoError(t, err)
	require.Equal(t, "theirs", string(content))
	require.Equal(t, []string{filepath.Base(dir)}, unpackRootEntries(t, request.Root), "the losing staging copy must be removed")
}

// Staging copies of this digest older than providerdir.StagingDirMaxAge were left by killed
// downloads and go. Young ones may belong to a live process and stay out of this unpack, other
// digests belong to other unpacks.
func TestEnsureUnpackedRemovesStaleStagingDirsOfItsDigest(t *testing.T) {
	var downloads atomic.Int32
	request := testUnpackRequest(t, nil)
	staging := func(name string, age time.Duration) string {
		path := filepath.Join(request.Root, name)
		require.NoError(t, os.MkdirAll(path, 0o755))
		old := time.Now().Add(-age)
		require.NoError(t, os.Chtimes(path, old, old))
		return path
	}
	stale := staging("candi@"+testUnpackDigest+".partial-1", 2*providerdir.StagingDirMaxAge)
	young := staging("candi@"+testUnpackDigest+".partial-2", 0)
	require.NoError(t, os.WriteFile(filepath.Join(young, "garbage"), []byte("x"), 0o644))
	otherDigest := staging("candi@sha256:2222222222222222222222222222222222222222222222222222222222222222.partial-3", 2*providerdir.StagingDirMaxAge)

	got, err := EnsureUnpacked(t.Context(), request, EnsureUnpackedOptions{Download: stubUnpackDownload(&downloads, nil)})
	require.NoError(t, err)
	require.NoDirExists(t, stale)
	require.DirExists(t, young)
	require.NoFileExists(t, filepath.Join(got, "garbage"))
	require.DirExists(t, otherDigest)
}

func TestEnsureUnpackedUsesExplicitImage(t *testing.T) {
	var downloads atomic.Int32
	var gotRef string

	_, err := EnsureUnpacked(t.Context(), testUnpackRequest(t, nil), EnsureUnpackedOptions{
		Image:    "modules.example.com/cloud-provider-dvp@" + testUnpackDigest,
		Download: stubUnpackDownload(&downloads, &gotRef),
	})
	require.NoError(t, err)
	require.Equal(t, "modules.example.com/cloud-provider-dvp@"+testUnpackDigest, gotRef)
}

func TestEnsureUnpackedFailedDownloadLeavesNoDir(t *testing.T) {
	request := testUnpackRequest(t, nil)

	_, err := EnsureUnpacked(t.Context(), request, EnsureUnpackedOptions{
		Download: func(context.Context, string, string, RegistryConfig, bool) error {
			return errors.New("registry refused")
		},
	})
	require.ErrorContains(t, err, "registry refused")
	require.ErrorContains(t, err, "r.example.com/test@"+testUnpackDigest)
	require.Empty(t, unpackRootEntries(t, request.Root), "a failed download must leave neither the directory nor its staging copy")
}

func TestEnsureUnpackedLogsTheDownload(t *testing.T) {
	var downloads atomic.Int32
	request := testUnpackRequest(t, nil)
	var logs bytes.Buffer
	ctx := dhlog.ToContext(t.Context(), slog.New(slog.NewTextHandler(&logs, &slog.HandlerOptions{Level: slog.LevelDebug})))

	dir, err := EnsureUnpacked(ctx, request, EnsureUnpackedOptions{Download: stubUnpackDownload(&downloads, nil)})
	require.NoError(t, err)
	require.Contains(t, logs.String(), "level=DEBUG")
	require.Contains(t, logs.String(), "r.example.com/test@"+testUnpackDigest)
	require.Contains(t, logs.String(), dir)
}

func TestEnsureUnpackedFailedPrepareLeavesNoDir(t *testing.T) {
	var downloads atomic.Int32
	request := testUnpackRequest(t, nil)

	_, err := EnsureUnpacked(t.Context(), request, EnsureUnpackedOptions{
		Download: stubUnpackDownload(&downloads, nil),
		Prepare:  func(context.Context, string, string) error { return errors.New("no schema inside") },
	})
	require.ErrorContains(t, err, "no schema inside")
	require.Empty(t, unpackRootEntries(t, request.Root), "an image that failed its check must not become visible")
}

func TestEnsureUnpackedRegistryErrorIsReturned(t *testing.T) {
	request := testUnpackRequest(t, nil)
	request.Registry = func(context.Context) (*RegistryConfig, error) {
		return nil, errors.New("cluster unreachable")
	}

	_, err := EnsureUnpacked(t.Context(), request, EnsureUnpackedOptions{})
	require.ErrorContains(t, err, "get registry for candi@"+testUnpackDigest+": cluster unreachable")
}

func TestEnsureUnpackedConcurrentCallsDownloadOnce(t *testing.T) {
	const callers = 8
	var downloads, started atomic.Int32
	request := testUnpackRequest(t, nil)
	release := make(chan struct{})
	held := func(ctx context.Context, imageRef, dir string, registry RegistryConfig, showProgress bool) error {
		<-release
		return stubUnpackDownload(&downloads, nil)(ctx, imageRef, dir, registry, showProgress)
	}

	var wg sync.WaitGroup
	for range callers {
		wg.Go(func() {
			started.Add(1)
			_, err := EnsureUnpacked(t.Context(), request, EnsureUnpackedOptions{Download: held})
			assert.NoError(t, err)
		})
	}
	// No download finishes before every caller has started, so none can find the dir in place.
	require.Eventually(t, func() bool { return started.Load() == callers }, 10*time.Second, time.Millisecond)
	close(release)
	wg.Wait()

	require.Equal(t, int32(1), downloads.Load())
}

func TestEnsureUnpackedRejectsIncompleteRequest(t *testing.T) {
	for _, tc := range []struct {
		name    string
		modify  func(request *UnpackRequest)
		wantErr string
	}{
		{name: "empty root", modify: func(r *UnpackRequest) { r.Root = "" }, wantErr: "root is required"},
		{name: "empty name", modify: func(r *UnpackRequest) { r.Name = "" }, wantErr: "name is required"},
		{name: "empty digest", modify: func(r *UnpackRequest) { r.Digest = "" }, wantErr: "digest is required"},
		{name: "no registry getter", modify: func(r *UnpackRequest) { r.Registry = nil }, wantErr: "registry getter is required"},
		{
			name: "registry getter returns nothing",
			modify: func(r *UnpackRequest) {
				r.Registry = func(context.Context) (*RegistryConfig, error) { return nil, nil }
			},
			wantErr: "registry getter returned no registry",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var downloads atomic.Int32
			request := testUnpackRequest(t, nil)
			tc.modify(&request)

			_, err := EnsureUnpacked(t.Context(), request, EnsureUnpackedOptions{Download: stubUnpackDownload(&downloads, nil)})
			require.ErrorContains(t, err, tc.wantErr)
			require.Zero(t, downloads.Load())
		})
	}
}

// The digest becomes a path: one that is not a digest could point the lock files, the staging dir
// and the unpacked image outside root.
func TestEnsureUnpackedRejectsDigestEscapingRoot(t *testing.T) {
	var downloads atomic.Int32
	base := t.TempDir()
	request := testUnpackRequest(t, nil)
	request.Root = filepath.Join(base, "download")
	request.Digest = "sha256:../../../escaped"

	_, err := EnsureUnpacked(t.Context(), request, EnsureUnpackedOptions{Download: stubUnpackDownload(&downloads, nil)})
	require.ErrorContains(t, err, "is not a directory right under")
	require.Zero(t, downloads.Load())

	escaped, err := filepath.Glob(filepath.Join(base, "escaped*"))
	require.NoError(t, err)
	require.Empty(t, escaped, "nothing may be created outside root")
}

// A holder that is alive but stuck must not block the rest for ever, and those who gave up on it
// must not all download at once: they line up behind the second lock.
func TestEnsureUnpackedStuckLockHolderCostsOneMoreDownload(t *testing.T) {
	origWait := lockWait
	lockWait = 300 * time.Millisecond
	t.Cleanup(func() { lockWait = origWait })

	var downloads atomic.Int32
	request := testUnpackRequest(t, nil)
	dir := filepath.Join(request.Root, "candi@"+testUnpackDigest)

	release, locked, err := lockFile(t.Context(), dir+".lock.0")
	require.NoError(t, err)
	require.True(t, locked)
	t.Cleanup(func() { require.NoError(t, release()) })

	// Every waiter warns that it gives up on the stuck holder; that is not what this test checks.
	ctx := dhlog.ToContext(t.Context(), slog.New(slog.DiscardHandler))

	started := time.Now()
	var wg sync.WaitGroup
	for range 8 {
		wg.Go(func() {
			_, err := EnsureUnpacked(ctx, request, EnsureUnpackedOptions{Download: stubUnpackDownload(&downloads, nil)})
			assert.NoError(t, err)
		})
	}
	wg.Wait()

	require.Equal(t, int32(1), downloads.Load())
	require.GreaterOrEqual(t, time.Since(started), lockWait, "the stuck holder is waited for before it is given up on")
	require.FileExists(t, filepath.Join(dir, "payload"))
}

// A stuck first-lock holder is alive and writes deep inside its staging dir, which leaves the dir's
// own mtime old. Whoever gave up on that holder must not take the dir for one a killed download left.
func TestEnsureUnpackedKeepsOldStagingDirsWhileTheFirstLockIsHeld(t *testing.T) {
	origWait := lockWait
	lockWait = 300 * time.Millisecond
	t.Cleanup(func() { lockWait = origWait })

	var downloads atomic.Int32
	request := testUnpackRequest(t, nil)
	dir := filepath.Join(request.Root, "candi@"+testUnpackDigest)

	release, locked, err := lockFile(t.Context(), dir+".lock.0")
	require.NoError(t, err)
	require.True(t, locked)
	t.Cleanup(func() { require.NoError(t, release()) })

	holderStaging := dir + ".partial-old"
	require.NoError(t, os.MkdirAll(holderStaging, 0o755))
	old := time.Now().Add(-2 * providerdir.StagingDirMaxAge)
	require.NoError(t, os.Chtimes(holderStaging, old, old))

	// Giving up on the stuck holder warns; that is not what this test checks.
	ctx := dhlog.ToContext(t.Context(), slog.New(slog.DiscardHandler))

	_, err = EnsureUnpacked(ctx, request, EnsureUnpackedOptions{Download: stubUnpackDownload(&downloads, nil)})
	require.NoError(t, err)
	require.DirExists(t, holderStaging)
}

func TestEnsureUnpackedStopsWaitingForTheLockWhenTheContextIsDone(t *testing.T) {
	var downloads atomic.Int32
	request := testUnpackRequest(t, nil)
	dir := filepath.Join(request.Root, "candi@"+testUnpackDigest)
	release, locked, err := lockFile(t.Context(), dir+".lock.0")
	require.NoError(t, err)
	require.True(t, locked)
	t.Cleanup(func() { require.NoError(t, release()) })

	ctx, cancel := context.WithTimeout(t.Context(), 200*time.Millisecond)
	defer cancel()

	started := time.Now()
	_, err = EnsureUnpacked(ctx, request, EnsureUnpackedOptions{Download: stubUnpackDownload(&downloads, nil)})
	require.ErrorIs(t, err, context.DeadlineExceeded)
	require.Less(t, time.Since(started), lockWait)
	require.Zero(t, downloads.Load())
}

// The lock only saves downloads: a download root where it cannot be taken (NFS without lockd, FUSE)
// must still unpack.
func TestEnsureUnpackedGoesOnWhenTheLockFails(t *testing.T) {
	var downloads atomic.Int32
	request := testUnpackRequest(t, nil)
	dir := filepath.Join(request.Root, "candi@"+testUnpackDigest)
	for _, lock := range []string{dir + ".lock.0", dir + ".lock.1"} {
		require.NoError(t, os.MkdirAll(lock, 0o755), "a directory cannot be opened as a lock file")
	}

	var logs bytes.Buffer
	ctx := dhlog.ToContext(t.Context(), slog.New(slog.NewTextHandler(&logs, nil)))

	got, err := EnsureUnpacked(ctx, request, EnsureUnpackedOptions{Download: stubUnpackDownload(&downloads, nil)})
	require.NoError(t, err)
	require.FileExists(t, filepath.Join(got, "payload"))
	require.Contains(t, logs.String(), "level=WARN", "going on without the lock must be visible in the log")
}

const unpackHelperRootEnv = "DHCTL_TEST_UNPACK_HELPER_ROOT"

// TestUnpackHelperProcess is not a test: the test below runs this binary again to get separate
// processes, the way dhctl server runs every request.
func TestUnpackHelperProcess(t *testing.T) {
	root := os.Getenv(unpackHelperRootEnv)
	if root == "" {
		t.Skip("runs only as a helper process")
	}

	started, err := os.CreateTemp(filepath.Join(root, "started"), "process-")
	require.NoError(t, err)
	require.NoError(t, started.Close())

	_, err = EnsureUnpacked(t.Context(), UnpackRequest{
		Root:   filepath.Join(root, "download"),
		Name:   "candi",
		Digest: testUnpackDigest,
		Registry: func(context.Context) (*RegistryConfig, error) {
			return NewRegistryConfig("HTTPS", "r.example.com/test", "u", "p", "")
		},
	}, EnsureUnpackedOptions{
		Download: func(_ context.Context, _, dir string, _ RegistryConfig, _ bool) error {
			mark, err := os.CreateTemp(filepath.Join(root, "downloads"), "download-")
			if err != nil {
				return err
			}
			if err := mark.Close(); err != nil {
				return err
			}
			require.Eventually(t, func() bool {
				_, err := os.Stat(filepath.Join(root, "release"))
				return err == nil
			}, time.Minute, 10*time.Millisecond, "the test releases the download")
			return os.WriteFile(filepath.Join(dir, "payload"), []byte("new"), 0o644)
		},
	})
	require.NoError(t, err)
}

func TestEnsureUnpackedManyProcessesDownloadOnce(t *testing.T) {
	root := t.TempDir()
	for _, name := range []string{"started", "downloads"} {
		require.NoError(t, os.MkdirAll(filepath.Join(root, name), 0o755))
	}

	const processes = 12
	commands := make([]*exec.Cmd, 0, processes)
	for range processes {
		commands = append(commands, startHelperProcess(t, "TestUnpackHelperProcess", unpackHelperRootEnv+"="+root))
	}
	// No download finishes before every process has started, so none can find the dir in place.
	require.Eventually(t, func() bool {
		started, err := os.ReadDir(filepath.Join(root, "started"))
		return err == nil && len(started) == processes
	}, time.Minute, 10*time.Millisecond, "every helper process must start")
	require.NoError(t, os.WriteFile(filepath.Join(root, "release"), nil, 0o644))
	for _, cmd := range commands {
		require.NoError(t, cmd.Wait())
	}

	marks, err := os.ReadDir(filepath.Join(root, "downloads"))
	require.NoError(t, err)
	require.Len(t, marks, 1, "one process downloads, the rest wait for it and take its result")
	require.FileExists(t, filepath.Join(root, "download", "candi@"+testUnpackDigest, "payload"))
}
