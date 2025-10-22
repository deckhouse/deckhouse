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

package tests

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/deckhouse/deckhouse/dhctl/pkg/state"
)

func assertCacheKeys(t *testing.T, stateCache state.Cache, expectedKeys []string) {
	var objectsInCacheAfterClean []string
	err := stateCache.Iterate(func(s string, _ []byte) error {
		objectsInCacheAfterClean = append(objectsInCacheAfterClean, s)
		return nil
	})
	require.NoError(t, err)
	require.Equal(t, objectsInCacheAfterClean, expectedKeys)
}

func RunStateCacheTests(t *testing.T, stateCache state.Cache) {
	var err error

	type testCase struct {
		key   string
		value []byte
	}

	simpleKeys := []testCase{
		{key: "test", value: []byte(`test-1`)},
		{key: "test.tfstate", value: []byte(`test-2`)},
		{key: "test2.tfstate", value: []byte(`test-3`)},
	}

	for _, k := range simpleKeys {
		err = stateCache.Save(k.key, k.value)
		require.NoError(t, err)

		ok, err := stateCache.InCache(k.key)
		require.NoError(t, err)
		require.True(t, ok)

		content, err := stateCache.Load(k.key)

		require.NoError(t, err)
		require.Equal(t, k.value, content)
	}

	var iterateValues []testCase
	err = stateCache.Iterate(func(k string, v []byte) error {
		iterateValues = append(iterateValues, testCase{
			key:   k,
			value: v,
		})

		return nil
	})
	require.NoError(t, err)
	require.Equal(t, iterateValues, simpleKeys)

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
	assertCacheKeys(t, stateCache, []string{"test-struct", "test.tfstate", "test2.tfstate"})

	stateCache.Clean()
	assertCacheKeys(t, stateCache, []string{".tombstone"})

	stateCache.Delete(".tombstone")
	err = stateCache.Save("a", []byte("a-test"))
	require.NoError(t, err)

	err = stateCache.Save("b", []byte("b-test"))
	require.NoError(t, err)

	err = stateCache.Save("c", []byte("c-test"))
	require.NoError(t, err)

	stateCache.CleanWithExceptions("b")
	assertCacheKeys(t, stateCache, []string{".tombstone", "b"})
}
