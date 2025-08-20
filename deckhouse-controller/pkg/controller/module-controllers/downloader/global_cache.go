// Copyright 2024 Flant JSC
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

package downloader

import (
	"sync"
)

// GlobalReleaseImageInfoCache is a singleton instance of ReleaseImageInfoCache
// that should be shared across all ModuleDownloader instances to avoid
// cache isolation between different controllers
var (
	globalCacheInstance *ReleaseImageInfoCache
	globalCacheOnce     sync.Once
)

// GetGlobalReleaseImageInfoCache returns the singleton global cache instance
// This ensures all ModuleDownloader instances share the same cache, preventing
// the cache isolation problem where each controller creates its own cache.
//
// TTL is not implemented here because:
// 1. Cache lifetime equals controller process lifetime 
// 2. On controller restart, cache is automatically cleared
// 3. Module metadata doesn't change during controller runtime
// 4. TTL would add unnecessary complexity and performance overhead
//
// Usage example:
//   globalCache := GetGlobalReleaseImageInfoCache()
//   md := NewModuleDownloader(..., globalCache)
func GetGlobalReleaseImageInfoCache() *ReleaseImageInfoCache {
	globalCacheOnce.Do(func() {
		globalCacheInstance = NewReleaseImageInfoCache()
	})
	return globalCacheInstance
}
