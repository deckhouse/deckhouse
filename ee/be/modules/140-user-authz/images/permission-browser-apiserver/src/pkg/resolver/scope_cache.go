/*
Copyright 2025 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package resolver

import (
	"slices"
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

	// mu protects scopeMap and partial.
	// Key format: "apiGroup/resource" (core group is empty string).
	mu       sync.RWMutex
	scopeMap map[string]bool // true = namespaced, false = cluster-scoped
	// partial records that the snapshot was built from an incomplete discovery
	// response, which ServerPreferredResources returns whenever an aggregated
	// APIService is unavailable. Callers that enumerate the snapshot report less
	// than the truth while this holds, so they need a way to say so.
	partial bool
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

// Scope reports whether the resource is namespaced and whether the snapshot knows
// it at all.
//
// IsNamespaced collapses "cluster-scoped" and "unknown" into the same false
// because for namespace resolution both must fail closed. Callers that treat a
// false as a reason to drop something need the two apart: dropping a row because
// discovery was incomplete makes the answer quietly smaller than the truth.
func (c *ResourceScopeCache) Scope(group, resource string) (bool, bool) {
	key := group + "/" + resource

	c.mu.RLock()
	defer c.mu.RUnlock()

	namespaced, known := c.scopeMap[key]

	return namespaced, known
}

// Partial reports whether the current snapshot was built from an incomplete
// discovery response.
func (c *ResourceScopeCache) Partial() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()

	return c.partial
}

// HasNamespacedResourceMatching reports whether the discovery snapshot
// contains at least one namespaced resource matched by the RBAC apiGroups and
// resources fields. Both top-level resources and subresources participate so
// wildcard rules are evaluated with the same semantics as Kubernetes RBAC.
func (c *ResourceScopeCache) HasNamespacedResourceMatching(apiGroups, resources []string) bool {
	c.mu.RLock()
	defer c.mu.RUnlock()

	for key, namespaced := range c.scopeMap {
		if !namespaced {
			continue
		}

		group, resource, ok := strings.Cut(key, "/")
		if !ok {
			continue
		}
		if !matchesAPIGroup(apiGroups, group) {
			continue
		}
		if matchesResource(resources, resource) {
			return true
		}
	}

	return false
}

// GroupResource is a discovered resource identified by its API group and name.
// Name may carry a subresource ("pods/log"), mirroring discovery output.
type GroupResource struct {
	Group      string
	Resource   string
	Namespaced bool
}

// ResourcesMatching returns the discovered resources matched by the RBAC
// apiGroups and resources fields of a single PolicyRule.
//
// It is the expansion counterpart of HasNamespacedResourceMatching: instead of
// answering "does anything match", it enumerates what matches, so a wildcard
// rule can be turned into concrete rows. Matching uses the same semantics as
// Kubernetes RBAC, including "*/subresource" rules.
//
// Subresources are only returned when the rule names them explicitly (either
// as "resource/subresource" or "*/subresource"): a bare "*" resources rule
// grants top-level resources, and listing every subresource of the cluster
// would bury the meaningful rows.
//
// The result is sorted for deterministic output.
func (c *ResourceScopeCache) ResourcesMatching(apiGroups, resources []string) []GroupResource {
	wantsSubresources := false
	for _, ruleResource := range resources {
		if strings.Contains(ruleResource, "/") {
			wantsSubresources = true
			break
		}
	}

	c.mu.RLock()
	matched := make([]GroupResource, 0, len(c.scopeMap))
	for key, namespaced := range c.scopeMap {
		group, resource, ok := strings.Cut(key, "/")
		if !ok {
			continue
		}
		if !wantsSubresources && strings.Contains(resource, "/") {
			continue
		}
		if !matchesAPIGroup(apiGroups, group) {
			continue
		}
		if !matchesResource(resources, resource) {
			continue
		}
		matched = append(matched, GroupResource{Group: group, Resource: resource, Namespaced: namespaced})
	}
	c.mu.RUnlock()

	slices.SortFunc(matched, func(a, b GroupResource) int {
		if cmp := strings.Compare(a.Group, b.Group); cmp != 0 {
			return cmp
		}
		return strings.Compare(a.Resource, b.Resource)
	})

	return matched
}

func matchesAPIGroup(ruleGroups []string, group string) bool {
	for _, ruleGroup := range ruleGroups {
		if ruleGroup == "*" || ruleGroup == group {
			return true
		}
	}
	return false
}

func matchesResource(ruleResources []string, resource string) bool {
	subresource := ""
	if _, value, ok := strings.Cut(resource, "/"); ok {
		subresource = value
	}

	for _, ruleResource := range ruleResources {
		if ruleResource == "*" || ruleResource == resource {
			return true
		}
		if subresource != "" && ruleResource == "*/"+subresource {
			return true
		}
	}
	return false
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
	partial := false
	if err != nil {
		// ServerPreferredResources may return partial results with an error.
		// If we got some results, use them; otherwise preserve the old cache.
		if len(resourceLists) == 0 {
			klog.Warningf("ResourceScopeCache: discovery failed completely: %v, preserving existing cache", err)
			return
		}
		klog.V(4).Infof("ResourceScopeCache: discovery returned partial results: %v", err)
		partial = true
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
	c.partial = partial
	c.mu.Unlock()

	klog.V(4).Infof("ResourceScopeCache: refreshed with %d resources", len(newMap))
}
