// Copyright 2021 Flant JSC
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

	"github.com/deckhouse/deckhouse/dhctl/pkg/app"
	"github.com/deckhouse/deckhouse/dhctl/pkg/log"
	"github.com/deckhouse/deckhouse/dhctl/pkg/tests"
)

func TestNewTempStateCache(t *testing.T) {
	log.InitLogger("json")

	dir, err := os.MkdirTemp(os.TempDir(), "dhctl-test-cache-*")
	require.NoError(t, err)
	app.CacheDir = dir

	defer os.RemoveAll(dir)

	t.Run("Save load delete operations", func(t *testing.T) {
		stateCache, err := NewTempStateCache("test-identity")
		require.NoError(t, err)

		require.Equal(t,
			"f2883e125771cadcd9918a8d991d8002c8b5b0c52dbe9811f89f3f4eee53ccab",
			filepath.Base(stateCache.dir),
		)

		tests.RunStateCacheTests(t, stateCache)

		ok, err := stateCache.InCache(".tombstone")

		require.NoError(t, err)
		require.True(t, ok)
	})
}
