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

package infrastructureprovider

import (
	"context"
	"fmt"
	"sync"

	"github.com/name212/govalue"

	"github.com/deckhouse/deckhouse/dhctl/pkg/config"
	"github.com/deckhouse/deckhouse/dhctl/pkg/infrastructure"
	"github.com/deckhouse/deckhouse/dhctl/pkg/log"
)

var defaultProvidersCache = newCloudProvidersMapCache()

// CleanupProvidersFromDefaultCache - warning! is not tread-safe to avoid deadlock
func CleanupProvidersFromDefaultCache(logger log.Logger) {
	defaultProvidersCache.finalizedMutex.Lock()
	defer defaultProvidersCache.finalizedMutex.Unlock()

	for _, provider := range defaultProvidersCache.cloudProvidersCache {
		logger.LogDebugF("CleanupProvidersFromDefaultCache called. Cleanup provider %s from default cache\n", provider.String())
		if err := provider.Cleanup(); err != nil {
			logger.LogWarnF("Failed to cleanup provider %s from default cache: %v\n", provider.String(), err)
		}
	}

	defaultProvidersCache.finalized = true
}

type (
	ProviderCreatorForCache func(ctx context.Context, clusterUUID string, metaConfig *config.MetaConfig, logger log.Logger) (infrastructure.CloudProvider, error)

	CloudProvidersCacheIteratorFunc func(key string, provider infrastructure.CloudProvider)
)

type CloudProvidersCache interface {
	GetOrAdd(ctx context.Context, uuid string, metaConfig *config.MetaConfig, logger log.Logger, creator ProviderCreatorForCache) (infrastructure.CloudProvider, error)
	Get(uuid string, metaConfig *config.MetaConfig, logger log.Logger) (infrastructure.CloudProvider, bool, error)
	IterateOverCache(iteratorFunc CloudProvidersCacheIteratorFunc) error
}

type cloudProvidersMapCache struct {
	cloudProvidersCacheMutex sync.Mutex
	cloudProvidersCache      map[string]infrastructure.CloudProvider

	finalizedMutex sync.Mutex
	finalized      bool
}

func newCloudProvidersMapCache() *cloudProvidersMapCache {
	return &cloudProvidersMapCache{
		cloudProvidersCache: make(map[string]infrastructure.CloudProvider),
		finalized:           false,
	}
}

func (c *cloudProvidersMapCache) GetOrAdd(ctx context.Context, clusterUUID string, metaConfig *config.MetaConfig, logger log.Logger, creator ProviderCreatorForCache) (infrastructure.CloudProvider, error) {
	if creator == nil {
		return nil, fmt.Errorf("Provider creator is nil for cluster %s", clusterUUID)
	}

	create := func(ctx context.Context, clusterUUID string, metaConfig *config.MetaConfig, logger log.Logger) (infrastructure.CloudProvider, error) {
		provider, err := creator(ctx, clusterUUID, metaConfig, logger)
		if err != nil {
			return nil, err
		}

		if govalue.IsNil(provider) {
			return nil, fmt.Errorf("Do not store nil provider for cluster %s in cache", clusterUUID)
		}

		return provider, nil
	}

	if metaConfig.ProviderName == "" {
		logger.LogDebugF("Do not store provider for empty provider for cluster %s probably it is static cluster and do not need any cleanup\n", clusterUUID)
		return create(ctx, clusterUUID, metaConfig, logger)
	}

	c.finalizedMutex.Lock()
	defer c.finalizedMutex.Unlock()

	cacheKey := getKey(clusterUUID, metaConfig)

	if c.finalized {
		return nil, fmt.Errorf("Cache finalized! Do not add provider for cluster wit key %s to finalized cache!", cacheKey)
	}

	c.cloudProvidersCacheMutex.Lock()
	defer c.cloudProvidersCacheMutex.Unlock()

	cachedProvider, ok := c.cloudProvidersCache[cacheKey]
	if ok {
		logger.LogDebugF("Found existing provider for cluster %s in cache by key %s. Returns from cache\n", cachedProvider.String(), cacheKey)
		return cachedProvider, nil
	}

	provider, err := create(ctx, clusterUUID, metaConfig, logger)
	if err != nil {
		return nil, err
	}

	afterCleanup := func(l log.Logger) {
		c.cloudProvidersCacheMutex.Lock()
		defer c.cloudProvidersCacheMutex.Unlock()

		p, ok := c.cloudProvidersCache[cacheKey]
		if !ok {
			l.LogDebugF("Provider with key %s not found. Skip cleaning\n", cacheKey)
			return
		}

		delete(c.cloudProvidersCache, cacheKey)
		l.LogDebugF("Provider %s found in cache by key %s and deleted\n", p.String(), cacheKey)
		p = nil
	}

	// add 'z' letter for calling cleanup after all cleanup groups
	// because we can have deps in another groups for provider, for example in
	// stop executor in Runner
	provider.AddAfterCleanupFunc("zCloudProviderCacheCleaner", afterCleanup)

	c.cloudProvidersCache[cacheKey] = provider
	logger.LogDebugF("Store %s in cache with key %s\n", provider.String(), cacheKey)

	return provider, nil
}

func (c *cloudProvidersMapCache) Get(clusterUUID string, metaConfig *config.MetaConfig, logger log.Logger) (infrastructure.CloudProvider, bool, error) {
	if metaConfig.ProviderName == "" {
		logger.LogDebugF("Do not store provider for cluster %s in cache with empty provider name\n", clusterUUID)
		return nil, false, nil
	}

	cacheKey := getKey(clusterUUID, metaConfig)

	c.finalizedMutex.Lock()
	defer c.finalizedMutex.Unlock()

	if c.finalized {
		return nil, false, fmt.Errorf("Cache finalized! Do not get provider with key %s to finalized cache!", cacheKey)
	}

	c.cloudProvidersCacheMutex.Lock()
	defer c.cloudProvidersCacheMutex.Unlock()

	provider, ok := c.cloudProvidersCache[cacheKey]
	if !ok {
		logger.LogDebugF("Provider with key %s not found.\n", cacheKey)
		return nil, false, nil
	}

	if govalue.IsNil(provider) {
		logger.LogDebugF("Provider with key %s is nil.\n", cacheKey)
		return nil, false, nil
	}

	logger.LogDebugF("Found existing provider for cluster %s in cache by key %s. Returns it.\n", provider.String(), cacheKey)

	return provider, true, nil
}

func (c *cloudProvidersMapCache) IterateOverCache(f CloudProvidersCacheIteratorFunc) error {
	c.finalizedMutex.Lock()
	defer c.finalizedMutex.Unlock()

	if c.finalized {
		return fmt.Errorf("Cache finalized! Do not iterate over cache!")
	}

	c.cloudProvidersCacheMutex.Lock()
	defer c.cloudProvidersCacheMutex.Unlock()

	for key, provider := range c.cloudProvidersCache {
		f(key, provider)
	}

	return nil
}

func getKey(clusterUUID string, metaConfig *config.MetaConfig) string {
	return fmt.Sprintf(
		"%s/%s/%s/%s",
		metaConfig.ClusterPrefix,
		clusterUUID,
		metaConfig.ProviderName,
		metaConfig.Layout,
	)
}
