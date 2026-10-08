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
	"errors"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	dhlog "github.com/deckhouse/lib-dhctl/pkg/logger"
)

const lockHelperPathEnv = "DHCTL_TEST_LOCK_HELPER_PATH"

// startHelperProcess runs this test binary again with only the helper test and env. A helper
// still running when the test ends is killed, and a failed test logs the helper's output.
func startHelperProcess(t *testing.T, helper, env string) *exec.Cmd {
	t.Helper()

	var output bytes.Buffer
	cmd := exec.Command(os.Args[0], "-test.run=^"+helper+"$", "-test.count=1")
	cmd.Env = append(os.Environ(), env)
	cmd.Stdout = &output
	cmd.Stderr = &output
	require.NoError(t, cmd.Start())

	t.Cleanup(func() {
		if cmd.ProcessState == nil {
			if err := cmd.Process.Kill(); err != nil && !errors.Is(err, os.ErrProcessDone) {
				t.Errorf("kill helper process %s: %v", helper, err)
			}
			t.Logf("helper process %s stopped: %v", helper, cmd.Wait())
		}
		if t.Failed() {
			t.Logf("helper process %s output:\n%s", helper, output.String())
		}
	})

	return cmd
}

// TestLockHelperProcess is not a test: it is this binary run again to hold a lock in a process
// of its own until that process is killed.
func TestLockHelperProcess(t *testing.T) {
	path := os.Getenv(lockHelperPathEnv)
	if path == "" {
		t.Skip("runs only as a helper process")
	}

	_, locked, err := lockFile(t.Context(), path)
	require.NoError(t, err)
	require.True(t, locked)
	require.NoError(t, os.WriteFile(path+".held", nil, 0o644))

	time.Sleep(time.Minute)
}

// A process that dies with the lock must not block anyone: the kernel drops the lock with it,
// and the lock file it leaves on disk means nothing.
func TestLockIsReleasedWhenItsHolderIsKilled(t *testing.T) {
	path := filepath.Join(t.TempDir(), "candi@sha256:x.lock.0")

	holder := startHelperProcess(t, "TestLockHelperProcess", lockHelperPathEnv+"="+path)

	require.Eventually(t, func() bool {
		_, err := os.Stat(path + ".held")
		return err == nil
	}, 10*time.Second, 20*time.Millisecond, "the helper process must take the lock")

	require.NoError(t, holder.Process.Kill())
	require.ErrorContains(t, holder.Wait(), "signal: killed")
	require.FileExists(t, path, "the lock file outlives its holder")

	started := time.Now()
	release, locked, err := lockFile(t.Context(), path)
	require.NoError(t, err)
	require.True(t, locked)
	require.NoError(t, release())
	require.Less(t, time.Since(started), 5*time.Second, "a dead holder must not be waited for")
}

func TestLockDirGivesUpOnAStuckHolder(t *testing.T) {
	origWait := lockWait
	lockWait = 200 * time.Millisecond
	t.Cleanup(func() { lockWait = origWait })

	dir := filepath.Join(t.TempDir(), "candi@sha256:x")
	for _, name := range []string{dir + ".lock.0", dir + ".lock.1"} {
		release, locked, err := lockFile(t.Context(), name)
		require.NoError(t, err)
		require.True(t, locked)
		t.Cleanup(func() { require.NoError(t, release()) })
	}

	var logs bytes.Buffer
	ctx := dhlog.ToContext(t.Context(), slog.New(slog.NewTextHandler(&logs, nil)))

	started := time.Now()
	unlock, first := LockDir(ctx, dir)
	require.False(t, first, "stuck holders are given up on, the caller goes on without the lock")
	require.NoError(t, unlock())
	require.GreaterOrEqual(t, time.Since(started), 2*lockWait)

	for _, name := range []string{dir + ".lock.0", dir + ".lock.1"} {
		require.Contains(t, logs.String(), name, "giving up on a holder must be visible in the log")
	}
	require.Contains(t, logs.String(), "level=WARN")
}
