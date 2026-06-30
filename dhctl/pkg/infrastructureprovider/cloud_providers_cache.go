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
	"strings"
	"sync"

	"github.com/name212/govalue"

	dhlog "github.com/deckhouse/lib-dhctl/pkg/logger"

	"github.com/deckhouse/deckhouse/dhctl/pkg/config"
	"github.com/deckhouse/deckhouse/dhctl/pkg/infrastructure"
)

var defaultProvidersCache = newCloudProvidersMapCache()

// CleanupProvidersFromDefaultCache - warning! is not tread-safe to avoid deadlock
func CleanupProvidersFromDefaultCache() {
	defaultProvidersCache.finalizedMutex.Lock()
	defer defaultProvidersCache.finalizedMutex.Unlock()

	ctx := context.Background()

	for _, provider := range defaultProvidersCache.cloudProvidersCache {
		logDebugF(ctx, "CleanupProvidersFromDefaultCache called. Cleaning up provider %s from default cache\n", provider.String())
		if err := provider.Cleanup(); err != nil {
			logWarnF(ctx, "Failed to cleanup provider %s from default cache: %v\n", provider.String(), err)
		}
	}

	defaultProvidersCache.finalized = true
}

type (
	ProviderCreatorForCache func(ctx context.Context, clusterUUID string, metaConfig *config.MetaConfig) (infrastructure.CloudProvider, error)

	CloudProvidersCacheIteratorFunc func(key string, provider infrastructure.CloudProvider)
)

type CloudProvidersCache interface {
	GetOrAdd(ctx context.Context, uuid string, metaConfig *config.MetaConfig, creator ProviderCreatorForCache) (infrastructure.CloudProvider, error)
	Get(uuid string, metaConfig *config.MetaConfig) (infrastructure.CloudProvider, bool, error)
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

func (c *cloudProvidersMapCache) GetOrAdd(ctx context.Context, clusterUUID string, metaConfig *config.MetaConfig, creator ProviderCreatorForCache) (infrastructure.CloudProvider, error) {
	if creator == nil {
		return nil, fmt.Errorf("Provider creator is nil for cluster %s", clusterUUID)
	}

	create := func(ctx context.Context, clusterUUID string, metaConfig *config.MetaConfig) (infrastructure.CloudProvider, error) {
		provider, err := creator(ctx, clusterUUID, metaConfig)
		if err != nil {
			return nil, err
		}

		if govalue.IsNil(provider) {
			return nil, fmt.Errorf("Will not store nil provider for cluster %s in cache", clusterUUID)
		}

		return provider, nil
	}

	if metaConfig.ProviderName == "" {
		dhlog.FromContext(ctx).DebugContext(ctx, fmt.Sprintf("Not storing provider for empty provider for cluster %s, probably it is a static cluster and does not need any cleanup", clusterUUID))
		return create(ctx, clusterUUID, metaConfig)
	}

	c.finalizedMutex.Lock()
	defer c.finalizedMutex.Unlock()

	cacheKey := getKey(clusterUUID, metaConfig)

	if c.finalized {
		return nil, fmt.Errorf("Cache finalized! Cannot add provider for cluster with key %s to a finalized cache!", cacheKey)
	}

	c.cloudProvidersCacheMutex.Lock()
	defer c.cloudProvidersCacheMutex.Unlock()

	cachedProvider, ok := c.cloudProvidersCache[cacheKey]
	if ok {
		dhlog.FromContext(ctx).DebugContext(ctx, fmt.Sprintf("Found existing provider for cluster %s in cache by key %s. Returning from cache", cachedProvider.String(), cacheKey))
		return cachedProvider, nil
	}

	provider, err := create(ctx, clusterUUID, metaConfig)
	if err != nil {
		return nil, err
	}

	afterCleanup := func() {
		c.cloudProvidersCacheMutex.Lock()
		defer c.cloudProvidersCacheMutex.Unlock()

		p, ok := c.cloudProvidersCache[cacheKey]
		if !ok {
			dhlog.FromContext(ctx).DebugContext(ctx, fmt.Sprintf("Provider with key %s not found. Skipping cleanup", cacheKey))
			return
		}

		delete(c.cloudProvidersCache, cacheKey)
		dhlog.FromContext(ctx).DebugContext(ctx, fmt.Sprintf("Provider %s found in cache by key %s and deleted", p.String(), cacheKey))
		p = nil
	}

	// add 'z' letter for calling cleanup after all cleanup groups
	// because we can have deps in another groups for provider, for example in
	// stop executor in Runner
	provider.AddAfterCleanupFunc("zCloudProviderCacheCleaner", afterCleanup)

	c.cloudProvidersCache[cacheKey] = provider
	dhlog.FromContext(ctx).DebugContext(ctx, fmt.Sprintf("Storing %s in cache with key %s", provider.String(), cacheKey))

	return provider, nil
}

func (c *cloudProvidersMapCache) Get(clusterUUID string, metaConfig *config.MetaConfig) (infrastructure.CloudProvider, bool, error) {
	ctx := context.Background()

	if metaConfig.ProviderName == "" {
		dhlog.FromContext(ctx).DebugContext(ctx, fmt.Sprintf("Not storing provider for cluster %s in cache with empty provider name", clusterUUID))
		return nil, false, nil
	}

	cacheKey := getKey(clusterUUID, metaConfig)

	c.finalizedMutex.Lock()
	defer c.finalizedMutex.Unlock()

	if c.finalized {
		return nil, false, fmt.Errorf("Cache finalized! Cannot get provider with key %s from a finalized cache!", cacheKey)
	}

	c.cloudProvidersCacheMutex.Lock()
	defer c.cloudProvidersCacheMutex.Unlock()

	provider, ok := c.cloudProvidersCache[cacheKey]
	if !ok {
		dhlog.FromContext(ctx).DebugContext(ctx, fmt.Sprintf("Provider with key %s not found.", cacheKey))
		return nil, false, nil
	}

	if govalue.IsNil(provider) {
		dhlog.FromContext(ctx).DebugContext(ctx, fmt.Sprintf("Provider with key %s is nil.", cacheKey))
		return nil, false, nil
	}

	dhlog.FromContext(ctx).DebugContext(ctx, fmt.Sprintf("Found existing provider for cluster %s in cache by key %s. Returning it.", provider.String(), cacheKey))

	return provider, true, nil
}

func (c *cloudProvidersMapCache) IterateOverCache(f CloudProvidersCacheIteratorFunc) error {
	c.finalizedMutex.Lock()
	defer c.finalizedMutex.Unlock()

	if c.finalized {
		return fmt.Errorf("Cache finalized! Cannot iterate over a finalized cache!")
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

func logWarnF(ctx context.Context, f string, args ...any) {
	dhlog.FromContext(ctx).WarnContext(ctx, strings.TrimRight(fmt.Sprintf(f, args...), "\n"))
}

func logDebugF(ctx context.Context, f string, args ...any) {
	dhlog.FromContext(ctx).DebugContext(ctx, strings.TrimRight(fmt.Sprintf(f, args...), "\n"))
}
