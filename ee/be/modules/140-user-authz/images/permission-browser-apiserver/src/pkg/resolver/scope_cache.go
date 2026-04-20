/*
Copyright 2025 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package resolver

import (
	"strings"
	"sync"
	"time"

	"k8s.io/client-go/discovery"
	"k8s.io/klog/v2"
)

const (
	// defaultRefreshInterval is how often the scope cache refreshes from discovery.
	defaultRefreshInterval = 5 * time.Minute
	// bootstrapRefreshInterval is used until the cache is populated at least once.
	// This avoids keeping the apiserver not-ready for a long time if discovery
	// fails transiently during startup.
	bootstrapRefreshInterval = 10 * time.Second
)

// ResourceScopeCache provides O(1) lookups for whether a resource is namespaced or cluster-scoped.
// It periodically refreshes its data from the API server discovery endpoint in the background,
// eliminating the need for live discovery calls during request processing.
type ResourceScopeCache struct {
	discoveryClient discovery.DiscoveryInterface
	refreshInterval time.Duration
	// bootstrapInterval is used until the cache is populated at least once.
	// If zero, bootstrapRefreshInterval is used.
	bootstrapInterval time.Duration

	// mu protects scopeMap.
	// Key format: "apiGroup/resource" (core group is empty string).
	mu       sync.RWMutex
	scopeMap map[string]bool // true = namespaced, false = cluster-scoped
}

// NewResourceScopeCache creates a new cache and performs initial population from discovery.
// If the initial discovery call fails, the cache starts empty and will be populated
// on the next refresh cycle.
func NewResourceScopeCache(discoveryClient discovery.DiscoveryInterface) *ResourceScopeCache {
	c := &ResourceScopeCache{
		discoveryClient:   discoveryClient,
		refreshInterval:   defaultRefreshInterval,
		bootstrapInterval: bootstrapRefreshInterval,
		scopeMap:          make(map[string]bool),
	}

	// Perform initial population
	c.refresh()

	return c
}

// IsNamespaced returns whether the given resource is namespaced.
// For unknown resources (not found in cache), returns false (fail-closed: assume cluster-scoped).
//
// IMPORTANT: A false positive here (returning true for a cluster-scoped resource) would cause
// the NamespaceResolver to treat the user as having namespaced access, potentially listing
// all namespaces (info leak). Therefore, unknown resources are assumed cluster-scoped.
func (c *ResourceScopeCache) IsNamespaced(group, resource string) bool {
	key := group + "/" + resource

	c.mu.RLock()
	defer c.mu.RUnlock()

	namespaced, ok := c.scopeMap[key]
	if !ok {
		klog.V(5).Infof("ResourceScopeCache: resource %s not found in cache, assuming cluster-scoped", key)
		return false
	}
	return namespaced
}

// HasData returns true if the cache has been populated with any entries.
// This can be used for readiness checks: an empty cache means we could not
// fetch discovery data yet and would treat all unknown resources as cluster-scoped.
func (c *ResourceScopeCache) HasData() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return len(c.scopeMap) > 0
}

// StartRefreshLoop starts the background refresh loop. Blocks until stopCh is closed.
func (c *ResourceScopeCache) StartRefreshLoop(stopCh <-chan struct{}) {
	for {
		interval := c.refreshInterval
		bootstrap := c.bootstrapInterval
		if bootstrap <= 0 {
			bootstrap = bootstrapRefreshInterval
		}
		// While the cache is empty, refresh more aggressively, but never slower than refreshInterval.
		// This keeps fast test intervals (milliseconds) intact.
		if !c.HasData() && interval > bootstrap {
			interval = bootstrap
		}

		timer := time.NewTimer(interval)
		select {
		case <-timer.C:
			c.refresh()
		case <-stopCh:
			timer.Stop()
			klog.Info("ResourceScopeCache: refresh loop stopped")
			return
		}
		timer.Stop()
	}
}

// refresh fetches all API resources from discovery and rebuilds the scope map.
// On error, the existing cache is preserved (stale data is better than no data).
func (c *ResourceScopeCache) refresh() {
	if c.discoveryClient == nil {
		klog.V(4).Info("ResourceScopeCache: no discovery client, skipping refresh")
		return
	}

	// ServerPreferredResources returns resources for all groups in one call.
	// It may return partial results along with an error for some groups.
	resourceLists, err := c.discoveryClient.ServerPreferredResources()
	if err != nil {
		// ServerPreferredResources may return partial results with an error.
		// If we got some results, use them; otherwise preserve the old cache.
		if len(resourceLists) == 0 {
			klog.Warningf("ResourceScopeCache: discovery failed completely: %v, preserving existing cache", err)
			return
		}
		klog.V(4).Infof("ResourceScopeCache: discovery returned partial results: %v", err)
	}

	newMap := make(map[string]bool)

	for _, resourceList := range resourceLists {
		if resourceList == nil {
			continue
		}

		// Parse the GroupVersion from the resource list.
		// Format is "group/version" or just "version" for core API.
		group := ""
		if gv := resourceList.GroupVersion; gv != "" {
			parts := strings.SplitN(gv, "/", 2)
			if len(parts) == 2 {
				group = parts[0]
			}
			// If len(parts) == 1, it's core API (e.g., "v1"), group stays ""
		}

		for _, res := range resourceList.APIResources {
			// Skip subresources (e.g., "pods/status")
			if strings.Contains(res.Name, "/") {
				continue
			}

			key := group + "/" + res.Name
			newMap[key] = res.Namespaced
		}
	}

	if len(newMap) == 0 {
		klog.Warning("ResourceScopeCache: refresh produced empty map, preserving existing cache")
		return
	}

	c.mu.Lock()
	c.scopeMap = newMap
	c.mu.Unlock()

	klog.V(4).Infof("ResourceScopeCache: refreshed with %d resources", len(newMap))
}
