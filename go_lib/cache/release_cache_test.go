// Copyright 2025 Flant JSC
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
	"testing"

	crv1 "github.com/google/go-containerregistry/pkg/v1"
)

func TestReleaseImageInfoCache(t *testing.T) {
	// Test cache creation
	cache := NewReleaseImageInfoCache()

	// Test setting and getting
	info1 := &LightweightReleaseInfo{
		Digest: crv1.Hash{Algorithm: "sha256", Hex: "test1"},
	}
	cache.Set("digest1", info1)

	if cached, found := cache.Get("digest1"); !found {
		t.Error("Expected to find digest1 in cache")
	} else if cached != info1 {
		t.Error("Expected to get the same info from cache")
	}

	// Test cache miss
	if _, found := cache.Get("nonexistent"); found {
		t.Error("Expected cache miss for nonexistent key")
	}

	// Test multiple entries
	info2 := &LightweightReleaseInfo{
		Digest: crv1.Hash{Algorithm: "sha256", Hex: "test2"},
	}
	info3 := &LightweightReleaseInfo{
		Digest: crv1.Hash{Algorithm: "sha256", Hex: "test3"},
	}

	cache.Set("digest2", info2)
	cache.Set("digest3", info3)

	// Verify all entries exist
	if cached, found := cache.Get("digest1"); !found || cached != info1 {
		t.Error("Expected to find digest1 after adding other entries")
	}
	if cached, found := cache.Get("digest2"); !found || cached != info2 {
		t.Error("Expected to find digest2 in cache")
	}
	if cached, found := cache.Get("digest3"); !found || cached != info3 {
		t.Error("Expected to find digest3 in cache")
	}
}

func TestGetGlobalCache(t *testing.T) {
	// Test that GetGlobalCache returns the same instance
	cache1 := GetGlobalCache()
	cache2 := GetGlobalCache()

	if cache1 != cache2 {
		t.Error("Expected GetGlobalCache to return the same instance")
	}

	// Test that the global cache works
	info := &LightweightReleaseInfo{
		Digest: crv1.Hash{Algorithm: "sha256", Hex: "global"},
	}
	cache1.Set("global-test", info)

	if cached, found := cache2.Get("global-test"); !found {
		t.Error("Expected to find entry in shared global cache")
	} else if cached != info {
		t.Error("Expected to get the same info from shared global cache")
	}
}
