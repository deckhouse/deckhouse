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

package store

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestUsageOfAnAbsentPath is the ordinary case: a node that never ran a storage replica.
// Not an error, and nothing to report.
func TestUsageOfAnAbsentPath(t *testing.T) {
	size, err := Usage(filepath.Join(t.TempDir(), "absent"))
	require.NoError(t, err)
	assert.Zero(t, size)
}

func TestUsageOfAnEmptyPath(t *testing.T) {
	size, err := Usage("")
	require.NoError(t, err)
	assert.Zero(t, size)
}

func TestUsageCountsTheBlobs(t *testing.T) {
	root := t.TempDir()
	blobs := filepath.Join(root, "docker", "registry", "v2", "blobs", "sha256", "ab")
	require.NoError(t, os.MkdirAll(blobs, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(blobs, "data"), make([]byte, 4096), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(root, "top"), make([]byte, 100), 0o644))

	size, err := Usage(root)
	require.NoError(t, err)
	assert.EqualValues(t, 4196, size)
}

// TestUsageDoesNotDoubleCountASymlink keeps the number honest on a store where one blob is
// linked from several places, which is how a registry lays out tags.
func TestUsageDoesNotDoubleCountASymlink(t *testing.T) {
	root := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(root, "data"), make([]byte, 512), 0o644))
	require.NoError(t, os.Symlink(filepath.Join(root, "data"), filepath.Join(root, "link")))

	size, err := Usage(root)
	require.NoError(t, err)
	assert.EqualValues(t, 512, size)
}

// TestUsageOfAFile: the path is expected to be the store's directory, and something else
// in its place is not the store — reporting its size would report the wrong thing.
func TestUsageOfAFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "not-a-directory")
	require.NoError(t, os.WriteFile(path, make([]byte, 10), 0o644))

	size, err := Usage(path)
	require.NoError(t, err)
	assert.Zero(t, size)
}

// TestUsageToleratesAnUnreadableSubtree: an underestimate still answers the question the
// number exists for, and an error would answer nothing.
func TestUsageToleratesAnUnreadableSubtree(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root reads everything, so there is no unreadable subtree to make")
	}

	root := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(root, "readable"), make([]byte, 256), 0o644))

	locked := filepath.Join(root, "locked")
	require.NoError(t, os.MkdirAll(filepath.Join(locked, "inner"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(locked, "inner", "blob"), make([]byte, 1024), 0o644))
	require.NoError(t, os.Chmod(locked, 0o000))
	t.Cleanup(func() { _ = os.Chmod(locked, 0o755) })

	size, err := Usage(root)
	require.NoError(t, err)
	assert.EqualValues(t, 256, size)
}
