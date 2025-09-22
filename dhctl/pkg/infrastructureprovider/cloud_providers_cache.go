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
	"fmt"
	"sync"

	"github.com/deckhouse/deckhouse/dhctl/pkg/config"
	"github.com/deckhouse/deckhouse/dhctl/pkg/infrastructure"
	"github.com/deckhouse/deckhouse/dhctl/pkg/log"
	"github.com/deckhouse/deckhouse/dhctl/pkg/util/interfaces"
)

var defaultProvidersCache = newCloudProvidersMapCache()

type CloudProvidersCacheIteratorFunc func(key string, provider infrastructure.CloudProvider)

type CloudProvidersCache interface {
	Add(uuid string, metaConfig *config.MetaConfig, provider infrastructure.CloudProvider, logger log.Logger) infrastructure.CloudProvider
	Get(uuid string, metaConfig *config.MetaConfig, logger log.Logger) (infrastructure.CloudProvider, bool)
	IterateOverCache(iteratorFunc CloudProvidersCacheIteratorFunc)
}

type cloudProvidersMapCache struct {
	cloudProvidersCacheMutex sync.Mutex
	cloudProvidersCache      map[string]infrastructure.CloudProvider
}

func newCloudProvidersMapCache() *cloudProvidersMapCache {
	return &cloudProvidersMapCache{
		cloudProvidersCache: make(map[string]infrastructure.CloudProvider),
	}
}

func (c *cloudProvidersMapCache) Add(clusterUUID string, metaConfig *config.MetaConfig, provider infrastructure.CloudProvider, logger log.Logger) infrastructure.CloudProvider {
	if interfaces.IsNil(provider) {
		logger.LogWarnF("Do not store nil provider for cluster %s in cache\n", clusterUUID)
		return provider
	}

	if metaConfig.ProviderName == "" {
		logger.LogDebugF("Do not store provider for empty provider for cluster %s probably it is static cluster and do not need any cleanup\n", clusterUUID)
		return provider
	}

	cacheKey := c.getKey(clusterUUID, metaConfig)

	c.cloudProvidersCacheMutex.Lock()
	defer c.cloudProvidersCacheMutex.Unlock()

	cachedProvider, ok := c.cloudProvidersCache[cacheKey]
	if ok {
		logger.LogDebugF("Found existing provider for cluster %s in cache by key %s. Returns from cache\n", provider.String(), cacheKey)
		return cachedProvider
	}

	c.cloudProvidersCache[cacheKey] = provider
	logger.LogDebugF("Store %s in cache with key %s\n", provider.String(), cacheKey)

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

	provider.SetAfterCleanupFunc(afterCleanup)
	return provider
}

func (c *cloudProvidersMapCache) Get(clusterUUID string, metaConfig *config.MetaConfig, logger log.Logger) (infrastructure.CloudProvider, bool) {
	if metaConfig.ProviderName == "" {
		logger.LogDebugF("Do not store provider for cluster %s in cache with empty provider name\n", clusterUUID)
		return nil, false
	}

	cacheKey := c.getKey(clusterUUID, metaConfig)

	c.cloudProvidersCacheMutex.Lock()
	defer c.cloudProvidersCacheMutex.Unlock()

	provider, ok := c.cloudProvidersCache[cacheKey]
	if !ok {
		logger.LogDebugF("Provider with key %s not found.\n", cacheKey)
		return nil, false
	}

	if interfaces.IsNil(provider) {
		logger.LogDebugF("Provider with key %s is nil.\n", cacheKey)
		return nil, false
	}

	logger.LogDebugF("Found existing provider for cluster %s in cache by key %s. Returns it.\n", provider.String(), cacheKey)

	return provider, true
}

func (c *cloudProvidersMapCache) IterateOverCache(f CloudProvidersCacheIteratorFunc) {
	c.cloudProvidersCacheMutex.Lock()
	defer c.cloudProvidersCacheMutex.Unlock()

	for key, provider := range c.cloudProvidersCache {
		f(key, provider)
	}
}

func (c *cloudProvidersMapCache) getKey(clusterUUID string, metaConfig *config.MetaConfig) string {
	return fmt.Sprintf(
		"%s/%s/%s/%s",
		metaConfig.ClusterPrefix,
		clusterUUID,
		metaConfig.ProviderName,
		metaConfig.Layout,
	)
}
