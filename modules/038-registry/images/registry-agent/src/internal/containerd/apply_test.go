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

package containerd

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newWriter(t *testing.T) *Writer {
	t.Helper()
	return &Writer{Root: filepath.Join(t.TempDir(), "registry.d")}
}

func desiredFor(hosts ...string) Desired {
	desired := Desired{Files: map[string][]byte{}, Hosts: hosts}
	for _, host := range hosts {
		desired.Files[filepath.Join(host, "hosts.toml")] = []byte("[host] # " + host)
	}
	return desired
}

func TestApplyWritesTheConfiguration(t *testing.T) {
	writer := newWriter(t)

	result, err := writer.Apply(desiredFor("registry.d8-system.svc:5001"))
	require.NoError(t, err)
	assert.True(t, result.Changed())
	assert.Contains(t, result.Written, filepath.Join("registry.d8-system.svc:5001", "hosts.toml"))

	content, err := os.ReadFile(filepath.Join(writer.Root, "registry.d8-system.svc:5001", "hosts.toml"))
	require.NoError(t, err)
	assert.Contains(t, string(content), "[host]")

	// World-readable: containerd reads these and does not run as this process. A
	// directory the runtime cannot read is a node that cannot pull.
	info, err := os.Stat(filepath.Join(writer.Root, "registry.d8-system.svc:5001", "hosts.toml"))
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0o644), info.Mode().Perm())
}

// TestApplyIsIdempotent is what keeps a scheduled re-render from touching the
// directory for nothing, which would defeat any change detection above it.
func TestApplyIsIdempotent(t *testing.T) {
	writer := newWriter(t)
	desired := desiredFor("registry.d8-system.svc:5001")

	first, err := writer.Apply(desired)
	require.NoError(t, err)
	require.True(t, first.Changed())

	second, err := writer.Apply(desired)
	require.NoError(t, err)
	assert.False(t, second.Changed())
	assert.Empty(t, second.Written)
}

// TestApplyRemovesItsOwnStaleHost covers a registry being removed from the
// configuration: its directory has to go, or the runtime keeps reaching it with
// credentials that may have been revoked.
func TestApplyRemovesItsOwnStaleHost(t *testing.T) {
	writer := newWriter(t)

	_, err := writer.Apply(desiredFor("registry.d8-system.svc:5001", "ghcr.io"))
	require.NoError(t, err)
	require.DirExists(t, filepath.Join(writer.Root, "ghcr.io"))

	result, err := writer.Apply(desiredFor("registry.d8-system.svc:5001"))
	require.NoError(t, err)

	assert.Equal(t, []string{"ghcr.io"}, result.RemovedHosts)
	assert.NoDirExists(t, filepath.Join(writer.Root, "ghcr.io"))
	assert.DirExists(t, filepath.Join(writer.Root, "registry.d8-system.svc:5001"))
}

// TestApplyLeavesStrangersAlone is the property that lets the agent own this
// directory without owning everything in it. Deleting a configuration an
// administrator put there by hand would be a worse failure than leaving one behind.
func TestApplyLeavesStrangersAlone(t *testing.T) {
	writer := newWriter(t)
	require.NoError(t, os.MkdirAll(filepath.Join(writer.Root, "someone-elses.example.com"), 0o755))
	require.NoError(t, os.WriteFile(
		filepath.Join(writer.Root, "someone-elses.example.com", "hosts.toml"), []byte("[host]"), 0o644))

	_, err := writer.Apply(desiredFor("registry.d8-system.svc:5001"))
	require.NoError(t, err)

	assert.FileExists(t, filepath.Join(writer.Root, "someone-elses.example.com", "hosts.toml"))
}

// TestApplyDoesNotRemoveAHostItStillWants guards the obvious catastrophe: taking the
// in-cluster endpoint away from a node that is using it.
func TestApplyDoesNotRemoveAHostItStillWants(t *testing.T) {
	writer := newWriter(t)
	desired := desiredFor("registry.d8-system.svc:5001", "ghcr.io")

	_, err := writer.Apply(desired)
	require.NoError(t, err)
	_, err = writer.Apply(desired)
	require.NoError(t, err)

	assert.DirExists(t, filepath.Join(writer.Root, "registry.d8-system.svc:5001"))
	assert.DirExists(t, filepath.Join(writer.Root, "ghcr.io"))
}

func TestApplyTracksItsHostsAcrossRestarts(t *testing.T) {
	writer := newWriter(t)

	_, err := writer.Apply(desiredFor("registry.d8-system.svc:5001", "ghcr.io"))
	require.NoError(t, err)

	// A fresh writer, as after the agent restarts: it has to learn what it created
	// from the state file, since nothing else distinguishes its directories.
	restarted := &Writer{Root: writer.Root}
	result, err := restarted.Apply(desiredFor("registry.d8-system.svc:5001"))
	require.NoError(t, err)

	assert.Equal(t, []string{"ghcr.io"}, result.RemovedHosts)
}

// TestApplyWithAnUnreadableStateFileKeepsEverything errs the safe way: on a parsing
// error the agent leaves directories behind rather than deleting configuration it
// cannot account for.
func TestApplyWithAnUnreadableStateFileKeepsEverything(t *testing.T) {
	writer := newWriter(t)

	_, err := writer.Apply(desiredFor("registry.d8-system.svc:5001", "ghcr.io"))
	require.NoError(t, err)

	require.NoError(t, os.WriteFile(
		filepath.Join(writer.Root, StateFile), []byte("this is not json"), 0o644))

	result, err := writer.Apply(desiredFor("registry.d8-system.svc:5001"))
	require.NoError(t, err)

	assert.Empty(t, result.RemovedHosts)
	assert.DirExists(t, filepath.Join(writer.Root, "ghcr.io"))
}

func TestApplyRewritesChangedContent(t *testing.T) {
	writer := newWriter(t)
	require.NoError(t, first(writer.Apply(desiredFor("ghcr.io"))))

	changed := desiredFor("ghcr.io")
	changed.Files[filepath.Join("ghcr.io", "hosts.toml")] = []byte("[host] # new credentials")

	result, err := writer.Apply(changed)
	require.NoError(t, err)
	assert.True(t, result.Changed())

	content, err := os.ReadFile(filepath.Join(writer.Root, "ghcr.io", "hosts.toml"))
	require.NoError(t, err)
	assert.Contains(t, string(content), "new credentials")
}

// TestApplyLeavesNoTemporaryFiles matters because containerd reads this directory on
// every pull: a half-written file next to a configuration is at best confusing and at
// worst read.
func TestApplyLeavesNoTemporaryFiles(t *testing.T) {
	writer := newWriter(t)

	for i := range 3 {
		desired := desiredFor("ghcr.io")
		desired.Files[filepath.Join("ghcr.io", "hosts.toml")] = []byte{byte(i)}
		require.NoError(t, first(writer.Apply(desired)))
	}

	entries, err := os.ReadDir(filepath.Join(writer.Root, "ghcr.io"))
	require.NoError(t, err)
	require.Len(t, entries, 1)
	assert.Equal(t, "hosts.toml", entries[0].Name())
}

func TestApplyEmptyDesiredRemovesEverythingItOwns(t *testing.T) {
	writer := newWriter(t)
	require.NoError(t, first(writer.Apply(desiredFor("registry.d8-system.svc:5001", "ghcr.io"))))

	// An Unmanaged node: the agent owns nothing on the pull path any more.
	result, err := writer.Apply(Desired{Files: map[string][]byte{}})
	require.NoError(t, err)

	assert.ElementsMatch(t, []string{"registry.d8-system.svc:5001", "ghcr.io"}, result.RemovedHosts)
	hosts, err := writer.HostsOf()
	require.NoError(t, err)
	assert.Empty(t, hosts)
}

// TestWritableReportsAnImmutableRoot is the check that makes an immutable operating
// system fail with a stated reason instead of a bare permission error on the first
// write — which is what the existing bashible steps do today.
func TestWritableReportsAnImmutableRoot(t *testing.T) {
	writer := newWriter(t)
	require.NoError(t, writer.Writable())

	readOnly := filepath.Join(t.TempDir(), "read-only")
	require.NoError(t, os.MkdirAll(readOnly, 0o500))
	t.Cleanup(func() { _ = os.Chmod(readOnly, 0o700) })

	blocked := &Writer{Root: filepath.Join(readOnly, "registry.d")}
	err := blocked.Writable()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "immutable root filesystem")
}

func TestHostsOf(t *testing.T) {
	writer := newWriter(t)

	hosts, err := writer.HostsOf()
	require.NoError(t, err)
	assert.Empty(t, hosts, "a directory that does not exist yet is not an error")

	require.NoError(t, first(writer.Apply(desiredFor("ghcr.io", "registry.d8-system.svc:5001"))))

	hosts, err = writer.HostsOf()
	require.NoError(t, err)
	assert.Equal(t, []string{"ghcr.io", "registry.d8-system.svc:5001"}, hosts)
}

func first(_ Result, err error) error { return err }

// TestApplyTakesOverFromThePreviousImplementation is the handover, and it matters
// more than it looks: an explicit host directory takes precedence over `_default`,
// so the per-registry directories the bashible step of the previous implementation
// wrote would route every pull around the agent. They have to go the moment the agent
// takes ownership.
//
// It works because the state file is a contract shared with that step, not something
// the agent invented: what the predecessor created is exactly what it recorded.
func TestApplyTakesOverFromThePreviousImplementation(t *testing.T) {
	writer := newWriter(t)

	// What the previous implementation left behind.
	for _, host := range []string{"registry.d8-system.svc:5001", "registry.deckhouse.io"} {
		require.NoError(t, os.MkdirAll(filepath.Join(writer.Root, host), 0o755))
		require.NoError(t, os.WriteFile(
			filepath.Join(writer.Root, host, "hosts.toml"), []byte("[host] # written by bashible"), 0o644))
	}
	require.NoError(t, os.WriteFile(filepath.Join(writer.Root, StateFile),
		[]byte(`["registry.d8-system.svc:5001","registry.deckhouse.io"]`), 0o644))

	result, err := writer.Apply(desiredFor(DefaultHost))
	require.NoError(t, err)

	assert.ElementsMatch(t,
		[]string{"registry.d8-system.svc:5001", "registry.deckhouse.io"}, result.RemovedHosts)
	assert.NoDirExists(t, filepath.Join(writer.Root, "registry.d8-system.svc:5001"))
	assert.NoDirExists(t, filepath.Join(writer.Root, "registry.deckhouse.io"))
	assert.DirExists(t, filepath.Join(writer.Root, DefaultHost))

	hosts, err := writer.HostsOf()
	require.NoError(t, err)
	assert.Equal(t, []string{DefaultHost}, hosts)
}

// TestApplyKeepsAnAdministratorsHostDirectory is the other side of the handover, and
// the reason removal is driven by the state file rather than by "everything that is
// not _default". A directory written by hand takes precedence over the fallback on
// purpose — that is how a custom registry configuration keeps working — and deleting
// it would break exactly the case the precedence exists for.
func TestApplyKeepsAnAdministratorsHostDirectory(t *testing.T) {
	writer := newWriter(t)
	require.NoError(t, os.MkdirAll(filepath.Join(writer.Root, "internal.example.com"), 0o755))
	require.NoError(t, os.WriteFile(
		filepath.Join(writer.Root, "internal.example.com", "hosts.toml"), []byte("[host]"), 0o644))

	_, err := writer.Apply(desiredFor(DefaultHost))
	require.NoError(t, err)

	assert.FileExists(t, filepath.Join(writer.Root, "internal.example.com", "hosts.toml"))
}
