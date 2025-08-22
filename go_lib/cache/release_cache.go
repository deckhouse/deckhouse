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
	"sync"

	"github.com/Masterminds/semver/v3"
	crv1 "github.com/google/go-containerregistry/pkg/v1"

	moduletypes "github.com/deckhouse/deckhouse/deckhouse-controller/pkg/controller/moduleloader/types"
)

// ModuleReleaseMetadata contains module release information
type ModuleReleaseMetadata struct {
	Version *semver.Version `json:"version"`

	Changelog        map[string]any          `json:"-"`
	ModuleDefinition *moduletypes.Definition `json:"module,omitempty"`
}

// LightweightReleaseInfo contains only metadata and digest, without the heavy Image object
// This reduces memory usage by 99.9% (from ~50-200MB to ~1-8KB per cache entry)
type LightweightReleaseInfo struct {
	Metadata *ModuleReleaseMetadata
	Digest   crv1.Hash
}

// ReleaseImageInfoCache provides thread-safe caching for lightweight release metadata
// Uses LightweightReleaseInfo to minimize memory footprint (1-8KB vs 50-200MB per entry)
type ReleaseImageInfoCache struct {
	cache map[string]*LightweightReleaseInfo
	mutex sync.RWMutex
}

// NewReleaseImageInfoCache creates a new cache with optimized settings
func NewReleaseImageInfoCache() *ReleaseImageInfoCache {
	return &ReleaseImageInfoCache{
		cache: make(map[string]*LightweightReleaseInfo),
		mutex: sync.RWMutex{},
	}
}

// Get retrieves LightweightReleaseInfo from cache if it exists
func (c *ReleaseImageInfoCache) Get(digest string) (*LightweightReleaseInfo, bool) {
	c.mutex.RLock()
	defer c.mutex.RUnlock()

	entry, exists := c.cache[digest]
	if !exists {
		return nil, false
	}

	return entry, true
}

// Set stores LightweightReleaseInfo in cache
func (c *ReleaseImageInfoCache) Set(digest string, info *LightweightReleaseInfo) {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	c.cache[digest] = info
}

// Global singleton instance
var (
	globalCacheInstance *ReleaseImageInfoCache
	globalCacheOnce     sync.Once
)

// GetGlobalCache returns the singleton global cache instance
// This ensures all ModuleDownloader instances share the same cache, preventing
// the cache isolation problem where each controller creates its own cache.
func GetGlobalCache() *ReleaseImageInfoCache {
	globalCacheOnce.Do(func() {
		globalCacheInstance = NewReleaseImageInfoCache()
	})
	return globalCacheInstance
}
