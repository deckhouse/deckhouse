// Copyright 2021 Flant CJSC
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

package tests

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/deckhouse/deckhouse/dhctl/pkg/state"
)

func RunStateCacheTests(t *testing.T, stateCache state.Cache) {
	var err error

	err = stateCache.Save("test", []byte(`test-1`))
	require.NoError(t, err)

	err = stateCache.Save("test.tfstate", []byte(`test-2`))
	require.NoError(t, err)

	err = stateCache.Save("test2.tfstate", []byte(`test-3`))
	require.NoError(t, err)

	require.Equal(t, true, stateCache.InCache("test"))
	require.Equal(t, true, stateCache.InCache("test.tfstate"))
	require.Equal(t, true, stateCache.InCache("test2.tfstate"))

	require.Equal(t, []byte("test-1"), stateCache.Load("test"))
	require.Equal(t, []byte("test-2"), stateCache.Load("test.tfstate"))
	require.Equal(t, []byte("test-3"), stateCache.Load("test2.tfstate"))

	structForTest := map[string]int{"abc": 10, "def": 1000, "xyz": 10}
	err = stateCache.SaveStruct("test-struct", structForTest)
	require.NoError(t, err)

	var test map[string]int
	err = stateCache.LoadStruct("test-struct", &test)
	require.NoError(t, err)

	require.Equal(t, structForTest, test)

	var objectsInCache []string
	err = stateCache.Iterate(func(s string, _ []byte) error {
		objectsInCache = append(objectsInCache, s)
		return nil
	})
	require.NoError(t, err)

	require.Equal(t, []string{"test", "test-struct", "test.tfstate", "test2.tfstate"}, objectsInCache)

	stateCache.Delete("test")
	var objectsInCacheAfterDelete []string
	err = stateCache.Iterate(func(s string, _ []byte) error {
		objectsInCacheAfterDelete = append(objectsInCacheAfterDelete, s)
		return nil
	})
	require.NoError(t, err)

	require.Equal(t, []string{"test-struct", "test.tfstate", "test2.tfstate"}, objectsInCacheAfterDelete)

	stateCache.Clean()

	var objectsInCacheAfterClean []string
	err = stateCache.Iterate(func(s string, _ []byte) error {
		objectsInCacheAfterClean = append(objectsInCacheAfterClean, s)
		return nil
	})
	require.NoError(t, err)
	require.Equal(t, objectsInCacheAfterClean, []string{".tombstone"})
}
