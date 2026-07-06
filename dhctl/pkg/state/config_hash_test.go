// Copyright 2026 Flant JSC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package state

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

// writeBlobsToRandomTempFiles writes each blob to a freshly created, randomly
// named temp file (mirroring dhctl-server's os.CreateTemp("", "*") behaviour)
// and returns the resulting paths. Files live under a t.TempDir so they are
// cleaned up automatically.
func writeBlobsToRandomTempFiles(t *testing.T, blobs [][]byte) []string {
	t.Helper()

	dir := t.TempDir()
	paths := make([]string, 0, len(blobs))
	for _, blob := range blobs {
		f, err := os.CreateTemp(dir, "*")
		require.NoError(t, err)
		_, err = f.Write(blob)
		require.NoError(t, err)
		require.NoError(t, f.Close())
		paths = append(paths, f.Name())
	}
	return paths
}

// TestConfigHashDeterministicAcrossRandomTempNames is the regression test for
// the preflight-skip-on-restart bug: dhctl-server writes each config blob to a
// randomly named temp file, so two invocations with byte-identical config used
// to produce different hashes (the old ConfigHash sorted by path). That flipped
// the preflight cache salt every run and re-ran preflights on restart.
func TestConfigHashDeterministicAcrossRandomTempNames(t *testing.T) {
	ctx := context.Background()

	blobs := [][]byte{
		[]byte("apiVersion: deckhouse.io/v1\nkind: ClusterConfiguration\n"),
		[]byte("apiVersion: deckhouse.io/v1\nkind: StaticClusterConfiguration\n"),
		[]byte("apiVersion: deckhouse.io/v1\nkind: Resource\n"),
	}

	// Run several times: temp names are random, so a single pass could match by
	// luck. Every run must hash equal to the first.
	first := ConfigHash(ctx, writeBlobsToRandomTempFiles(t, blobs))
	for range 10 {
		got := ConfigHash(ctx, writeBlobsToRandomTempFiles(t, blobs))
		require.Equal(t, first, got, "identical config content must hash equally regardless of random temp file names")
	}
}

// TestConfigHashIndependentOfPathOrder ensures the slice order of paths does not
// affect the hash (same files, shuffled argument order).
func TestConfigHashIndependentOfPathOrder(t *testing.T) {
	ctx := context.Background()

	paths := writeBlobsToRandomTempFiles(t, [][]byte{
		[]byte("blob-A"),
		[]byte("blob-B"),
		[]byte("blob-C"),
	})

	reversed := make([]string, len(paths))
	for i, p := range paths {
		reversed[len(paths)-1-i] = p
	}

	require.Equal(t, ConfigHash(ctx, paths), ConfigHash(ctx, reversed))
}

// TestConfigHashChangesOnContentChange ensures the salt still invalidates when
// any config actually changes (e.g. a changed master IP / CIDR), so preflights
// correctly re-run.
func TestConfigHashChangesOnContentChange(t *testing.T) {
	ctx := context.Background()

	base := [][]byte{
		[]byte("master:\n  ip: 10.0.0.1\n"),
		[]byte("apiVersion: deckhouse.io/v1\nkind: StaticClusterConfiguration\n"),
	}
	changed := [][]byte{
		[]byte("master:\n  ip: 10.0.0.2\n"), // IP changed
		[]byte("apiVersion: deckhouse.io/v1\nkind: StaticClusterConfiguration\n"),
	}

	require.NotEqual(t,
		ConfigHash(ctx, writeBlobsToRandomTempFiles(t, base)),
		ConfigHash(ctx, writeBlobsToRandomTempFiles(t, changed)),
		"changed config content must change the hash so preflights re-run")
}

// TestConfigHashStableForSameFile is a sanity check that a single fixed file
// hashes to the same value across calls.
func TestConfigHashStableForSameFile(t *testing.T) {
	ctx := context.Background()

	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	require.NoError(t, os.WriteFile(path, []byte("kind: ClusterConfiguration\n"), 0o600))

	require.Equal(t, ConfigHash(ctx, []string{path}), ConfigHash(ctx, []string{path}))
}
