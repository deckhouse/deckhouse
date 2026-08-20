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

package cache

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/deckhouse/deckhouse/dhctl/pkg/app/options"
	"github.com/deckhouse/deckhouse/dhctl/pkg/state"
	utilcache "github.com/deckhouse/deckhouse/dhctl/pkg/util/cache"
	"github.com/deckhouse/deckhouse/dhctl/pkg/util/stringsutil"
)

func tombstonedCacheDir(t *testing.T, identity string) string {
	t.Helper()

	dir := t.TempDir()
	stateDir := filepath.Join(dir, stringsutil.Sha256Encode(identity))

	require.NoError(t, os.MkdirAll(stateDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(stateDir, state.TombstoneKey), nil, 0o600))

	return dir
}

func TestChoiceCacheReturnsNoCacheWhenItCannotOpenOne(t *testing.T) {
	dir := tombstonedCacheDir(t, "some-cluster")

	c, err := choiceCache(t.Context(), "some-cluster", CacheOptions{Cache: options.CacheOptions{Dir: dir}})

	require.Error(t, err)

	// Compared against the untyped nil rather than asserted with require.Nil, which reports a
	// typed nil pointer inside an interface as nil and would pass on the defect being pinned.
	require.True(t, c == nil, "a (*StateCache, error) pair returned straight out of this signature makes a non-nil interface around a nil pointer")
}

// TestInitCacheKeepsTheDummyCacheWhenItFails pins the diagnosis a completed bootstrap used to lose:
// the second bootstrap on one machine met the tombstone of the first, and the cache it could not
// open was installed globally anyway, so the deferred Finalize of the caller already returning the
// error walked a nil directory and the run died of SIGSEGV instead of saying why.
//
// This is the only initCache call in the package's tests on purpose: the package-level sync.Once is
// shared state, and a prior successful init would turn this one into a no-op returning nil.
func TestInitCacheKeepsTheDummyCacheWhenItFails(t *testing.T) {
	ctx := t.Context()
	dir := tombstonedCacheDir(t, "some-cluster")

	err := initCache(ctx, "some-cluster", CacheOptions{Cache: options.CacheOptions{Dir: dir}})

	require.ErrorContains(t, err, "marked as exhausted", "the refusal has to reach the user")

	// The four StateCache methods that touch s.dir without a nil guard. Iterate is the one the
	// deferred Finalize of a bootstrap reaches through ExtractDhctlState.
	require.NotPanics(t, func() {
		require.NoError(t, Global().Iterate(ctx, func(string, []byte) error { return nil }))
		Global().Clean(ctx)
		Global().CleanWithExceptions(ctx)
		Global().Dir()
	})

	require.IsType(t, &utilcache.DummyCache{}, Global(), "a failed init must not replace the global cache")
}
