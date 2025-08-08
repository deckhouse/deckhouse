// Copyright 2022 Flant JSC
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

package registry

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"path"
	"strings"
	"sync"
	"time"

	v1 "github.com/google/go-containerregistry/pkg/v1"
	regTransport "github.com/google/go-containerregistry/pkg/v1/remote/transport"
	"github.com/iancoleman/strcase"
	"gopkg.in/yaml.v2"

	modRelease "github.com/deckhouse/deckhouse/deckhouse-controller/pkg/controller/module-controllers/downloader"
	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/controller/module-controllers/utils"
	moduletypes "github.com/deckhouse/deckhouse/deckhouse-controller/pkg/controller/moduleloader/types"
	"github.com/deckhouse/deckhouse/go_lib/dependency"
	"github.com/deckhouse/deckhouse/go_lib/dependency/cr"
	"github.com/deckhouse/deckhouse/pkg/log"
)

// Cache configuration for module registry service
type ModuleCacheConfig struct {
	TTL             time.Duration
	MaxEntries      int
	CleanupInterval time.Duration
}

// Cache entry with TTL
type cacheEntry[T any] struct {
	value     T
	createdAt time.Time
	ttl       time.Duration
}

// Cache keys
type digestKey struct {
	moduleName     string
	releaseChannel string
}

type metadataKey struct {
	moduleName     string
	releaseChannel string
	digest         string
}

// Performance metrics for monitoring
type ModuleCacheMetrics struct {
	DigestHits     int64
	DigestMisses   int64
	MetadataHits   int64
	MetadataMisses int64
	Evictions      int64
	TotalRequests  int64
}

// Module registry cache implementing digest-first optimization
type ModuleRegistryCache struct {
	digestCache   map[digestKey]*cacheEntry[string]
	metadataCache map[metadataKey]*cacheEntry[*modRelease.ModuleReleaseMetadata]
	mu            sync.RWMutex
	config        ModuleCacheConfig
	metrics       ModuleCacheMetrics
	stopCleanup   chan struct{}
	cleanupDone   chan struct{}
	logger        *log.Logger
}

type moduleReleaseService struct {
	dc dependency.Container

	registry        string
	registryOptions []cr.Option

	logger *log.Logger

	// Cache with lazy initialization
	cache     *ModuleRegistryCache
	cacheOnce sync.Once
}

func newModuleReleaseService(registryAddress string, registryConfig *utils.RegistryConfig, logger *log.Logger) *moduleReleaseService {
	return &moduleReleaseService{
		dc:              dependency.NewDependencyContainer(),
		registry:        registryAddress,
		registryOptions: utils.GenerateRegistryOptions(registryConfig, logger),
		logger:          logger,
	}
}

func (svc *moduleReleaseService) ListModules(ctx context.Context) ([]string, error) {
	regCli, err := svc.dc.GetRegistryClient(svc.registry, svc.registryOptions...)
	if err != nil {
		return nil, fmt.Errorf("get registry client: %w", err)
	}

	ls, err := regCli.ListTags(ctx)
	if err != nil {
		return nil, fmt.Errorf("list tags: %w", err)
	}

	return ls, err
}

var (
	ErrChannelIsNotFound = errors.New("channel is not found")
	ErrModuleIsNotFound  = errors.New("module is not found")
)

func (svc *moduleReleaseService) ListModuleTags(ctx context.Context, moduleName string) ([]string, error) {
	regCli, err := svc.dc.GetRegistryClient(path.Join(svc.registry, moduleName), svc.registryOptions...)
	if err != nil {
		return nil, fmt.Errorf("get registry client: %w", err)
	}

	ls, err := regCli.ListTags(ctx)
	if err != nil {
		if strings.Contains(err.Error(), string(regTransport.NameUnknownErrorCode)) {
			err = errors.Join(err, ErrModuleIsNotFound)
		}

		return nil, fmt.Errorf("list tags: %w", err)
	}

	return ls, err
}

// getCache initializes cache lazily
func (svc *moduleReleaseService) getCache() *ModuleRegistryCache {
	svc.cacheOnce.Do(func() {
		config := ModuleCacheConfig{
			TTL:             15 * time.Minute,
			MaxEntries:      1000,
			CleanupInterval: 5 * time.Minute,
		}
		svc.cache = newModuleRegistryCache(config, svc.logger)
	})
	return svc.cache
}

// GetModuleRelease with digest-first optimization and caching
func (svc *moduleReleaseService) GetModuleRelease(ctx context.Context, moduleName, releaseChannel string) (*modRelease.ModuleReleaseMetadata, error) {
	cache := svc.getCache()

	// Step 1: Get registry client
	regCli, err := svc.dc.GetRegistryClient(path.Join(svc.registry, moduleName, "release"), svc.registryOptions...)
	if err != nil {
		return nil, fmt.Errorf("get registry client: %w", err)
	}

	// Step 2: Get current digest from registry (lightweight operation)
	currentDigest, err := regCli.Digest(ctx, strcase.ToKebab(releaseChannel))
	if err != nil {
		if strings.Contains(err.Error(), string(regTransport.ManifestUnknownErrorCode)) {
			err = errors.Join(err, ErrChannelIsNotFound)
		}

		svc.logger.Warn("Failed to get module digest, falling back to full image download",
			slog.String("module", moduleName),
			slog.String("channel", releaseChannel),
			log.Err(err))

		// Fallback to original behavior if digest call fails
		return svc.getModuleReleaseFallback(ctx, regCli, moduleName, releaseChannel)
	}

	// Step 3: Check cached digest
	cachedDigest, digestCacheHit := cache.GetDigest(moduleName, releaseChannel)

	// Step 4: Check if we have cached metadata for this digest
	if digestCacheHit && cachedDigest == currentDigest {
		if metadata, metadataHit := cache.GetMetadata(moduleName, releaseChannel, currentDigest); metadataHit {
			svc.logger.Debug("Cache hit for module release",
				slog.String("module", moduleName),
				slog.String("channel", releaseChannel),
				slog.String("digest", currentDigest))
			return metadata, nil
		}
	}

	// Step 5: Cache miss - fetch image and extract metadata
	svc.logger.Debug("Cache miss - fetching module image",
		slog.String("module", moduleName),
		slog.String("channel", releaseChannel),
		slog.String("current_digest", currentDigest),
		slog.String("cached_digest", cachedDigest))

	img, err := regCli.Image(ctx, strcase.ToKebab(releaseChannel))
	if err != nil {
		if strings.Contains(err.Error(), string(regTransport.ManifestUnknownErrorCode)) {
			err = errors.Join(err, ErrChannelIsNotFound)
		}
		return nil, fmt.Errorf("fetch image error: %w", err)
	}

	// Verify digest consistency between calls
	imageDigest, err := img.Digest()
	if err == nil && currentDigest != imageDigest.String() {
		svc.logger.Warn("Module image digest inconsistency between digest and image calls",
			slog.String("digest_call", currentDigest),
			slog.String("image_digest", imageDigest.String()),
			slog.String("module", moduleName),
			slog.String("channel", releaseChannel))
	}

	moduleMetadata, err := svc.fetchModuleReleaseMetadata(img)
	if err != nil {
		return nil, fmt.Errorf("fetch module release metadata error: %w", err)
	}

	if moduleMetadata.Version == nil {
		return nil, fmt.Errorf("module release %q metadata malformed: no version found", moduleName)
	}

	// Step 6: Cache both digest and metadata
	cache.SetDigest(moduleName, releaseChannel, currentDigest)
	cache.SetMetadata(moduleName, releaseChannel, currentDigest, moduleMetadata)

	return moduleMetadata, nil
}

// getModuleReleaseFallback implements the original behavior when digest optimization fails
func (svc *moduleReleaseService) getModuleReleaseFallback(ctx context.Context, regCli cr.Client, moduleName, releaseChannel string) (*modRelease.ModuleReleaseMetadata, error) {
	svc.logger.Debug("Using fallback module retrieval method",
		slog.String("module", moduleName),
		slog.String("channel", releaseChannel))

	img, err := regCli.Image(ctx, strcase.ToKebab(releaseChannel))
	if err != nil {
		if strings.Contains(err.Error(), string(regTransport.ManifestUnknownErrorCode)) {
			err = errors.Join(err, ErrChannelIsNotFound)
		}

		return nil, fmt.Errorf("fetch image error: %w", err)
	}

	moduleMetadata, err := svc.fetchModuleReleaseMetadata(img)
	if err != nil {
		return nil, fmt.Errorf("fetch module release metadata error: %w", err)
	}

	if moduleMetadata.Version == nil {
		return nil, fmt.Errorf("module release %q metadata malformed: no version found", moduleName)
	}

	return moduleMetadata, nil
}

func (svc *moduleReleaseService) fetchModuleReleaseMetadata(img v1.Image) (*modRelease.ModuleReleaseMetadata, error) {
	var meta = new(modRelease.ModuleReleaseMetadata)

	rc, err := cr.Extract(img)
	if err != nil {
		return nil, err
	}
	defer rc.Close()

	rr := &releaseReader{
		versionReader:   bytes.NewBuffer(nil),
		changelogReader: bytes.NewBuffer(nil),
		moduleReader:    bytes.NewBuffer(nil),
	}

	err = rr.untarMetadata(rc)
	if err != nil {
		return nil, err
	}

	if rr.versionReader.Len() > 0 {
		err = json.NewDecoder(rr.versionReader).Decode(&meta)
		if err != nil {
			return nil, err
		}
	}

	if rr.changelogReader.Len() > 0 {
		var changelog map[string]any

		err = yaml.NewDecoder(rr.changelogReader).Decode(&changelog)
		if err != nil {
			// if changelog build failed - warn about it but don't fail the release
			svc.logger.Warn("Unmarshal CHANGELOG yaml failed", log.Err(err))

			meta.Changelog = make(map[string]any)

			return nil, nil
		}

		meta.Changelog = changelog
	}

	if rr.moduleReader.Len() > 0 {
		var ModuleDefinition moduletypes.Definition
		err = yaml.NewDecoder(rr.moduleReader).Decode(&ModuleDefinition)
		if err != nil {
			// if module.yaml decode failed - warn about it but don't fail the release
			svc.logger.Warn("Unmarshal module yaml failed", log.Err(err))

			meta.ModuleDefinition = nil

			return meta, nil
		}

		meta.ModuleDefinition = &ModuleDefinition
	}

	return meta, nil
}

// Cache implementation for module registry

// newModuleRegistryCache creates a new cache instance
func newModuleRegistryCache(config ModuleCacheConfig, logger *log.Logger) *ModuleRegistryCache {
	cache := &ModuleRegistryCache{
		digestCache:   make(map[digestKey]*cacheEntry[string]),
		metadataCache: make(map[metadataKey]*cacheEntry[*modRelease.ModuleReleaseMetadata]),
		config:        config,
		stopCleanup:   make(chan struct{}),
		cleanupDone:   make(chan struct{}),
		logger:        logger,
	}

	// Start background cleanup goroutine
	go cache.cleanupExpired()

	return cache
}

// GetDigest retrieves cached digest for module and release channel
func (c *ModuleRegistryCache) GetDigest(moduleName, releaseChannel string) (string, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	key := digestKey{moduleName: moduleName, releaseChannel: releaseChannel}
	entry, exists := c.digestCache[key]

	c.metrics.TotalRequests++

	if !exists || c.isExpired(entry) {
		c.metrics.DigestMisses++
		return "", false
	}

	c.metrics.DigestHits++
	return entry.value, true
}

// GetMetadata retrieves cached metadata for module, channel, and digest
func (c *ModuleRegistryCache) GetMetadata(moduleName, releaseChannel, digest string) (*modRelease.ModuleReleaseMetadata, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	key := metadataKey{moduleName: moduleName, releaseChannel: releaseChannel, digest: digest}
	entry, exists := c.metadataCache[key]

	c.metrics.TotalRequests++

	if !exists || c.isExpiredMetadata(entry) {
		c.metrics.MetadataMisses++
		return nil, false
	}

	c.metrics.MetadataHits++
	return entry.value, true
}

// SetDigest caches a digest for module and release channel
func (c *ModuleRegistryCache) SetDigest(moduleName, releaseChannel, digest string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	key := digestKey{moduleName: moduleName, releaseChannel: releaseChannel}

	// Check size limits and evict if needed
	if len(c.digestCache) >= c.config.MaxEntries {
		c.evictLRUFromDigestCache()
	}

	c.digestCache[key] = &cacheEntry[string]{
		value:     digest,
		createdAt: time.Now(),
		ttl:       c.config.TTL,
	}
}

// SetMetadata caches metadata for module, channel, and digest
func (c *ModuleRegistryCache) SetMetadata(moduleName, releaseChannel, digest string, metadata *modRelease.ModuleReleaseMetadata) {
	c.mu.Lock()
	defer c.mu.Unlock()

	key := metadataKey{moduleName: moduleName, releaseChannel: releaseChannel, digest: digest}

	// Check size limits and evict if needed
	if len(c.metadataCache) >= c.config.MaxEntries {
		c.evictLRUFromMetadataCache()
	}

	c.metadataCache[key] = &cacheEntry[*modRelease.ModuleReleaseMetadata]{
		value:     metadata,
		createdAt: time.Now(),
		ttl:       c.config.TTL,
	}
}

// Background cleanup of expired entries
func (c *ModuleRegistryCache) cleanupExpired() {
	ticker := time.NewTicker(c.config.CleanupInterval)
	defer ticker.Stop()
	defer close(c.cleanupDone)

	for {
		select {
		case <-ticker.C:
			c.removeExpired()
		case <-c.stopCleanup:
			return
		}
	}
}

// Remove expired entries
func (c *ModuleRegistryCache) removeExpired() {
	c.mu.Lock()
	defer c.mu.Unlock()

	now := time.Now()
	evicted := 0

	// Clean digest cache
	for key, entry := range c.digestCache {
		if now.Sub(entry.createdAt) > entry.ttl {
			delete(c.digestCache, key)
			evicted++
		}
	}

	// Clean metadata cache
	for key, entry := range c.metadataCache {
		if now.Sub(entry.createdAt) > entry.ttl {
			delete(c.metadataCache, key)
			evicted++
		}
	}

	c.metrics.Evictions += int64(evicted)

	if evicted > 0 {
		c.logger.Debug("Module cache cleanup completed",
			slog.Int("evicted_entries", evicted),
			slog.Int("digest_cache_size", len(c.digestCache)),
			slog.Int("metadata_cache_size", len(c.metadataCache)))
	}
}

// evictLRUFromDigestCache evicts oldest entry from digest cache
func (c *ModuleRegistryCache) evictLRUFromDigestCache() {
	if len(c.digestCache) == 0 {
		return
	}

	var oldestKey digestKey
	var oldestTime time.Time
	first := true

	for key, entry := range c.digestCache {
		if first || entry.createdAt.Before(oldestTime) {
			oldestKey = key
			oldestTime = entry.createdAt
			first = false
		}
	}

	delete(c.digestCache, oldestKey)
	c.metrics.Evictions++
}

// evictLRUFromMetadataCache evicts oldest entry from metadata cache
func (c *ModuleRegistryCache) evictLRUFromMetadataCache() {
	if len(c.metadataCache) == 0 {
		return
	}

	var oldestKey metadataKey
	var oldestTime time.Time
	first := true

	for key, entry := range c.metadataCache {
		if first || entry.createdAt.Before(oldestTime) {
			oldestKey = key
			oldestTime = entry.createdAt
			first = false
		}
	}

	delete(c.metadataCache, oldestKey)
	c.metrics.Evictions++
}

// isExpiredString checks if string cache entry is expired
func (c *ModuleRegistryCache) isExpired(entry *cacheEntry[string]) bool {
	return time.Since(entry.createdAt) > entry.ttl
}

// isExpiredMetadata checks if metadata cache entry is expired
func (c *ModuleRegistryCache) isExpiredMetadata(entry *cacheEntry[*modRelease.ModuleReleaseMetadata]) bool {
	return time.Since(entry.createdAt) > entry.ttl
}

// GetMetrics returns current cache performance metrics
func (c *ModuleRegistryCache) GetMetrics() ModuleCacheMetrics {
	c.mu.RLock()
	defer c.mu.RUnlock()

	// Make a copy to avoid race conditions
	return c.metrics
}

// Close gracefully shuts down the cache
func (c *ModuleRegistryCache) Close() {
	close(c.stopCleanup)
	<-c.cleanupDone
}
