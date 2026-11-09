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

package distribution

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"syscall"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// refuseIn makes the probe fail with errno in the directories named, and pass everywhere else.
func refuseIn(t *testing.T, errno syscall.Errno, dirs ...string) *[]string {
	t.Helper()
	var probed []string
	probeWrite = func(dir string) error {
		probed = append(probed, dir)
		for _, refused := range dirs {
			if dir == refused {
				return &fs.PathError{Op: "open", Path: filepath.Join(dir, ".write-probe-1"), Err: errno}
			}
		}
		return nil
	}
	t.Cleanup(func() { probeWrite = defaultProbeWrite })
	return &probed
}

var defaultProbeWrite = probeWrite

func TestAWritableStorePassesAndLeavesNothingBehind(t *testing.T) {
	dir := t.TempDir()
	v2 := filepath.Join(dir, "docker", "registry", "v2")
	require.NoError(t, os.MkdirAll(filepath.Join(v2, "blobs"), 0o755))

	require.NoError(t, CheckWritable(dir, "/opt/deckhouse/registry/local_data"))

	for _, d := range []string{dir, v2, filepath.Join(v2, "blobs")} {
		entries, err := os.ReadDir(d)
		require.NoError(t, err)
		for _, entry := range entries {
			assert.True(t, entry.IsDir(), "the probe left %s in %s", entry.Name(), d)
		}
	}
}

// appendOnly makes removeFile refuse with EPERM, as an append-only directory does: it takes new files
// and keeps them. The real attribute takes chattr and a filesystem that keeps it, which a test run has
// neither of.
func appendOnly(t *testing.T) {
	t.Helper()
	removeFile = func(name string) error {
		if _, err := os.Lstat(name); err != nil {
			return err
		}
		return &fs.PathError{Op: "remove", Path: name, Err: syscall.EPERM}
	}
	t.Cleanup(func() { removeFile = os.Remove })
}

// TestAnAppendOnlyDirectoryIsWritable: the registry only creates files in the directories probed, so a
// directory that refuses to remove them takes its writes. Reported, the store would read as NotWritable
// and the alert would fire on a store that fills.
func TestAnAppendOnlyDirectoryIsWritable(t *testing.T) {
	dir := t.TempDir()
	v2 := filepath.Join(dir, "docker", "registry", "v2")
	require.NoError(t, os.MkdirAll(filepath.Join(v2, "blobs"), 0o755))
	appendOnly(t)

	require.NoError(t, CheckWritable(dir, "/opt/deckhouse/registry/local_data"))
	assert.FileExists(t, filepath.Join(v2, probeFile), "the directory keeps the file it took")

	require.NoError(t, CheckWritable(dir, "/opt/deckhouse/registry/local_data"),
		"the file left behind shows the directory takes files")
	entries, err := os.ReadDir(v2)
	require.NoError(t, err)
	var files []string
	for _, entry := range entries {
		if !entry.IsDir() {
			files = append(files, entry.Name())
		}
	}
	assert.Equal(t, []string{probeFile}, files, "one file left behind, not one for every probe")
}

// TestAProbeFileLeftBehindIsRemoved: once the directory lets go of the file, the next probe removes it
// and probes as usual.
func TestAProbeFileLeftBehindIsRemoved(t *testing.T) {
	dir := t.TempDir()
	left := filepath.Join(dir, probeFile)
	require.NoError(t, os.WriteFile(left, nil, 0o600))

	require.NoError(t, CheckWritable(dir, ""))
	assert.NoFileExists(t, left)
}

func TestOnlyTheDirectoriesThatExistAreProbed(t *testing.T) {
	dir := t.TempDir()
	probed := refuseIn(t, 0)
	require.NoError(t, CheckWritable(dir, ""))
	assert.Equal(t, []string{dir}, *probed, "an empty store: the registry has created nothing under it yet")

	docker := filepath.Join(dir, "docker")
	require.NoError(t, os.MkdirAll(docker, 0o755))
	*probed = nil
	require.NoError(t, CheckWritable(dir, ""))
	assert.Equal(t, []string{docker}, *probed, "the rest of the tree is created under the deepest that exists")

	v2 := filepath.Join(docker, "registry", "v2")
	require.NoError(t, os.MkdirAll(filepath.Join(v2, "blobs"), 0o755))
	require.NoError(t, os.MkdirAll(filepath.Join(v2, "repositories"), 0o755))
	*probed = nil
	require.NoError(t, CheckWritable(dir, ""))
	assert.Equal(t, []string{v2, filepath.Join(v2, "blobs"), filepath.Join(v2, "repositories")}, *probed)
}

// TestAStoreDirectoryAboveAWritableTreeIsNotARefusal: once the tree is there the registry writes only
// under it, so a store directory chowned to another user — the top alone, as `chown` without -R does —
// over a root-owned tree is a store that fills. Reported, it would read as one that refuses writes, and
// the alert would fire on a store that works.
func TestAStoreDirectoryAboveAWritableTreeIsNotARefusal(t *testing.T) {
	dir := t.TempDir()
	v2 := filepath.Join(dir, "docker", "registry", "v2")
	require.NoError(t, os.MkdirAll(filepath.Join(v2, "blobs"), 0o755))
	refuseIn(t, syscall.EACCES, dir)

	assert.NoError(t, CheckWritable(dir, ""))

	require.NoError(t, os.RemoveAll(filepath.Join(dir, "docker")))
	assert.Error(t, CheckWritable(dir, ""), "an empty store: the registry has to create its tree there")
}

// TestARefusedDirectoryIsDescribed: the registry's own answer is a 500 that names no file, so the
// message has to carry everything — which directory, on the node too, and whose it is.
func TestARefusedDirectoryIsDescribed(t *testing.T) {
	dir := t.TempDir()
	blobs := filepath.Join(dir, "docker", "registry", "v2", "blobs")
	require.NoError(t, os.MkdirAll(blobs, 0o755))
	refuseIn(t, syscall.EACCES, blobs)

	err := CheckWritable(dir, "/opt/deckhouse/registry/local_data")
	var refused *NotWritableError
	require.True(t, errors.As(err, &refused), "got %v", err)
	assert.Equal(t, blobs, refused.Dir)
	assert.Equal(t, "/opt/deckhouse/registry/local_data/docker/registry/v2/blobs", refused.HostDir)
	assert.True(t, refused.Described)
	assert.Equal(t, uint32(os.Getuid()), refused.Owner)

	text := err.Error()
	assert.Contains(t, text, "the store is not writable")
	assert.Contains(t, text, "/opt/deckhouse/registry/local_data/docker/registry/v2/blobs on the node")
	assert.Contains(t, text, "permission denied")
	assert.Contains(t, text, fmt.Sprintf("uid %d, gid %d, mode drwxr-xr-x", os.Getuid(), os.Getgid()))
	assert.NotContains(t, text, ".write-probe", "the probe's own file is noise")
}

func TestARefusalWithoutTheNodesPathNamesNone(t *testing.T) {
	dir := t.TempDir()
	refuseIn(t, syscall.EACCES, dir)

	err := CheckWritable(dir, "")
	var refused *NotWritableError
	require.True(t, errors.As(err, &refused), "got %v", err)
	assert.Empty(t, refused.HostDir)
	assert.NotContains(t, err.Error(), "on the node)")
}

// TestTheAdviceFollowsWhatRefusedTheWrite: chown is the answer to a directory root does not own, and
// only to that — a root-owned directory without the owner's write bit wants chmod, and a refusal no
// owner or mode explains wants neither.
func TestTheAdviceFollowsWhatRefusedTheWrite(t *testing.T) {
	const hostPath = "/opt/deckhouse/registry/local_data"
	const hostDir = hostPath + "/docker/registry/v2/blobs"
	refused := func(errno syscall.Errno, owner uint32, mode fs.FileMode, described bool) *NotWritableError {
		return &NotWritableError{
			Dir: "/data/docker/registry/v2/blobs", HostDir: hostDir, HostPath: hostPath,
			Err:   &fs.PathError{Op: "open", Path: "/data/docker/registry/v2/blobs/.write-probe-1", Err: errno},
			Owner: owner, Group: owner, Mode: fs.ModeDir | mode, Described: described,
		}
	}

	cases := map[string]struct {
		err         *NotWritableError
		contains    []string
		notContains []string
	}{
		"owned by another user": {
			err:         refused(syscall.EACCES, 1000, 0o755, true),
			contains:    []string{"root without capabilities", "chown -R root:root " + hostPath},
			notContains: []string{"chmod"},
		},
		"owned by root without the owner's write bit": {
			err:         refused(syscall.EACCES, 0, 0o555, true),
			contains:    []string{"uid 0, gid 0, mode dr-xr-xr-x", "chmod -R u+rwX " + hostPath},
			notContains: []string{"chown"},
		},
		"owned by another user without the owner's write bit": {
			err:      refused(syscall.EACCES, 1000, 0o500, true),
			contains: []string{"chown -R root:root " + hostPath + " && chmod -R u+rwX " + hostPath},
		},
		"not described": {
			err:      refused(syscall.EACCES, 0, 0, false),
			contains: []string{"chown -R root:root " + hostPath + " && chmod -R u+rwX " + hostPath},
		},
		"owned by root and writable by it": {
			err:         refused(syscall.EACCES, 0, 0o755, true),
			contains:    []string{"security module", "getfacl -p " + hostDir},
			notContains: []string{"chown", "chmod"},
		},
		"an attribute no owner changes": {
			err:         refused(syscall.EPERM, 0, 0o755, true),
			contains:    []string{"operation not permitted", "lsattr -d " + hostDir, "chattr -i to clear it"},
			notContains: []string{"chown", "chmod -R"},
		},
		"an attribute, on a directory of another user too": {
			err:         refused(syscall.EPERM, 1000, 0o755, true),
			contains:    []string{"lsattr -d " + hostDir},
			notContains: []string{"chown"},
		},
	}

	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			text := c.err.Error()
			for _, s := range c.contains {
				assert.Contains(t, text, s)
			}
			for _, s := range c.notContains {
				assert.NotContains(t, text, s)
			}
		})
	}

	withoutHostPath := refused(syscall.EACCES, 1000, 0o755, true)
	withoutHostPath.HostPath, withoutHostPath.HostDir = "", ""
	assert.Contains(t, withoutHostPath.Error(), "chown -R root:root the store's directory on the node")
	withoutHostPath.Err = syscall.EPERM
	assert.Contains(t, withoutHostPath.Error(), "lsattr -d /data/docker/registry/v2/blobs",
		"the container's path, when the node's is not known")
}

// TestAReadOnlyFilesystemIsNotAnOwnershipProblem: chown cannot fix a read-only mount.
func TestAReadOnlyFilesystemIsNotAnOwnershipProblem(t *testing.T) {
	dir := t.TempDir()
	refuseIn(t, syscall.EROFS, dir)

	err := CheckWritable(dir, "/opt/deckhouse/registry/local_data")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "read-only file system")
	assert.Contains(t, err.Error(), "mounted read-write")
	assert.NotContains(t, err.Error(), "chown")
}

// TestOtherFailuresAreNotThisChecksToReport: a full disk is the reserve's to report, and is.
func TestOtherFailuresAreNotThisChecksToReport(t *testing.T) {
	dir := t.TempDir()
	refuseIn(t, syscall.ENOSPC, dir)
	assert.NoError(t, CheckWritable(dir, ""))

	assert.NoError(t, CheckWritable(filepath.Join(dir, "absent"), ""), "nothing to probe yet")
}
