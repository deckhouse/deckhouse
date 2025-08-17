/*
Copyright 2021 Flant JSC

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

package cr

import (
	"strings"
	"sync"
	"time"

	crv1 "github.com/google/go-containerregistry/pkg/v1"
)

const (
	// Default maximum number of entries in each cache map
	defaultMaxCacheSize = 1000
)

// cacheEntry holds cached data with timestamp for LRU eviction
type cacheEntry struct {
	value     any
	timestamp time.Time
}

// imageCache provides thread-safe caching for container registry operations
// with size limits to prevent memory leaks
type imageCache struct {
	mu      sync.RWMutex
	digests map[string]*cacheEntry // tag -> digest mapping
	images  map[string]*cacheEntry // digest -> image mapping
	maxSize int
}

// newImageCache creates a new image cache instance
func newImageCache() *imageCache {
	return &imageCache{
		digests: make(map[string]*cacheEntry),
		images:  make(map[string]*cacheEntry),
		maxSize: defaultMaxCacheSize,
	}
}

// getDigest retrieves cached digest for a given tag
func (c *imageCache) getDigest(tag string) (string, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	entry, exists := c.digests[tag]
	if !exists {
		return "", false
	}
	// Update timestamp for LRU
	entry.timestamp = time.Now()
	return entry.value.(string), true
}

// setDigest stores digest for a given tag in cache
func (c *imageCache) setDigest(tag, digest string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Evict old entries if cache is full
	if len(c.digests) >= c.maxSize {
		c.evictOldestDigest()
	}

	c.digests[tag] = &cacheEntry{
		value:     digest,
		timestamp: time.Now(),
	}
}

// getImage retrieves cached image for a given digest
func (c *imageCache) getImage(digest string) (crv1.Image, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	entry, exists := c.images[digest]
	if !exists {
		return nil, false
	}
	// Update timestamp for LRU
	entry.timestamp = time.Now()
	return entry.value.(crv1.Image), true
}

// setImage stores image for a given digest in cache
func (c *imageCache) setImage(digest string, image crv1.Image) {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Evict old entries if cache is full
	if len(c.images) >= c.maxSize {
		c.evictOldestImage()
	}

	c.images[digest] = &cacheEntry{
		value:     image,
		timestamp: time.Now(),
	}
}

// clear removes all cached data
func (c *imageCache) clear() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.digests = make(map[string]*cacheEntry)
	c.images = make(map[string]*cacheEntry)
}

// evictOldestDigest removes the oldest digest entry from cache
// Must be called with lock held
func (c *imageCache) evictOldestDigest() {
	if len(c.digests) == 0 {
		return
	}

	var oldestKey string
	var oldestTime time.Time
	first := true

	for key, entry := range c.digests {
		if first || entry.timestamp.Before(oldestTime) {
			oldestKey = key
			oldestTime = entry.timestamp
			first = false
		}
	}

	delete(c.digests, oldestKey)
}

// evictOldestImage removes the oldest image entry from cache
// Must be called with lock held
func (c *imageCache) evictOldestImage() {
	if len(c.images) == 0 {
		return
	}

	var oldestKey string
	var oldestTime time.Time
	first := true

	for key, entry := range c.images {
		if first || entry.timestamp.Before(oldestTime) {
			oldestKey = key
			oldestTime = entry.timestamp
			first = false
		}
	}

	delete(c.images, oldestKey)
}

// shouldCacheTag determines if a tag should be cached based on registry path
// Caches ALL tags from:
// - release directories (path contains "/release")
// - dev registry (dev-registry.deckhouse.io)
// Does NOT cache anything else
func shouldCacheTag(registryURL, tag string) bool {
	// Cache ALL tags from release directories
	if strings.Contains(registryURL, "/release") {
		return true
	}
	
	// Cache dev images from dev-registry.deckhouse.io
	if strings.Contains(registryURL, "dev-registry.deckhouse.io") {
		return true
	}
	
	return false
}
