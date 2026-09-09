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

package layout

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newCache(t *testing.T) *Cache {
	t.Helper()
	return &Cache{Path: filepath.Join(t.TempDir(), "state", "layout.json")}
}

func TestCacheRoundTrip(t *testing.T) {
	cache := newCache(t)
	stored := Stored{Generation: 7, Spec: &nodeLayout().Spec}

	require.NoError(t, cache.Save(stored))

	loaded, err := cache.Load()
	require.NoError(t, err)
	assert.Equal(t, stored.Spec, loaded.Spec)
	assert.EqualValues(t, 7, loaded.Generation)
}

// TestCacheIsReadableByNobodyElse is the narrow security property that actually
// holds. The copy carries registry credentials because routing without the API
// requires them; what can be done about it is keeping it out of reach of everything
// but the agent, unlike the runtime configuration which containerd must read.
func TestCacheIsReadableByNobodyElse(t *testing.T) {
	cache := newCache(t)
	require.NoError(t, cache.Save(Stored{Generation: 7, Spec: &nodeLayout().Spec}))

	info, err := os.Stat(cache.Path)
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0o600), info.Mode().Perm())

	directory, err := os.Stat(filepath.Dir(cache.Path))
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0o700), directory.Mode().Perm())
}

func TestCacheLoadWithoutACopy(t *testing.T) {
	// A node that has never reached the API. Saying "nothing" plainly beats an error
	// the caller has to classify.
	loaded, err := newCache(t).Load()
	require.NoError(t, err)
	assert.Nil(t, loaded)
}

func TestCacheIsIdempotent(t *testing.T) {
	cache := newCache(t)
	stored := Stored{Generation: 7, Spec: &nodeLayout().Spec}

	require.NoError(t, cache.Save(stored))
	before, err := os.Stat(cache.Path)
	require.NoError(t, err)

	require.NoError(t, cache.Save(stored))
	after, err := os.Stat(cache.Path)
	require.NoError(t, err)

	assert.Equal(t, before.ModTime(), after.ModTime(), "an unchanged layout must not be rewritten")
}

func TestCacheOverwrites(t *testing.T) {
	cache := newCache(t)
	require.NoError(t, cache.Save(Stored{Generation: 7, Spec: &nodeLayout().Spec}))

	updated := nodeLayout().Spec
	updated.Cache = false
	updated.Backends = updated.Backends[:1]
	require.NoError(t, cache.Save(Stored{Generation: 8, Spec: &updated}))

	loaded, err := cache.Load()
	require.NoError(t, err)
	assert.False(t, loaded.Spec.Cache)
	assert.Len(t, loaded.Spec.Backends, 1)
	assert.EqualValues(t, 8, loaded.Generation)
}

// TestCacheLeavesNoTemporaryFiles matters because a leftover temporary copy would
// also carry credentials, and would outlive the write that made it.
func TestCacheLeavesNoTemporaryFiles(t *testing.T) {
	cache := newCache(t)

	for i := range 3 {
		spec := nodeLayout().Spec
		spec.Backends = spec.Backends[:1]
		spec.Backends[0].Path = string(rune('a' + i))
		require.NoError(t, cache.Save(Stored{Generation: int64(i), Spec: &spec}))
	}

	entries, err := os.ReadDir(filepath.Dir(cache.Path))
	require.NoError(t, err)
	require.Len(t, entries, 1)
	assert.Equal(t, filepath.Base(cache.Path), entries[0].Name())
}

func TestCacheRejectsNothingToSave(t *testing.T) {
	assert.Error(t, newCache(t).Save(Stored{}))
}

func TestCacheDefaultPath(t *testing.T) {
	// Under /var rather than /etc: it is state the agent maintains, and on an
	// immutable operating system /etc may not be writable at all.
	assert.Equal(t, "/var/lib/deckhouse/registry-agent/layout.json", (&Cache{}).path())
	assert.Equal(t, "/tmp/custom.json", (&Cache{Path: "/tmp/custom.json"}).path())
}
