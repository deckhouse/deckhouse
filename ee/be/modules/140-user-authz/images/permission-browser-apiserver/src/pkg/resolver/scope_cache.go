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

	"github.com/deckhouse/deckhouse/go_lib/user-authz/rules"

	"permission-browser-apiserver/pkg/authorizer/multitenancy"
)

var _ multitenancy.ResourceScope = (*ResourceScopeCache)(nil)

const (
	// defaultRefreshInterval is how often the scope cache refreshes from discovery.
	defaultRefreshInterval = 5 * time.Minute
	// bootstrapRefreshInterval is used until the cache is populated at least once.
	// This avoids keeping the apiserver not-ready for a long time if discovery
	// fails transiently during startup.
	bootstrapRefreshInterval = 10 * time.Second
	// missRefreshInterval bounds how often a lookup the snapshot cannot answer may pull the next
	// refresh forward.
	//
	// This exists because the two consumers of the shared decision were reading discovery on
	// schedules an order of magnitude apart. The authorization webhook re-lists a group within ten
	// seconds of being asked about something it does not hold; this cache waited out its five
	// minute cycle. So for up to five minutes after a CRD was installed, what this apiserver
	// reported and what the API server enforced were different answers to the same question - the
	// exact drift the shared decision was written to end. The direction was the safe one (a
	// resource nobody has heard of is treated like a namespaced one, so the report understates
	// access rather than overstating it), but "safe" is not "the same".
	//
	// A miss now schedules one refresh, at most this often, and the request is answered from the
	// snapshot in hand rather than waiting for it. The rate is what keeps this from becoming an
	// amplifier: refresh() lists every group in the cluster, which is far heavier than the
	// webhook's single-group listing.
	missRefreshInterval = 30 * time.Second
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

	// mu protects scopeMap and unavailableGroups.
	// Key format: "apiGroup/resource" (core group is empty string).
	mu       sync.RWMutex
	scopeMap map[string]bool // true = namespaced, false = cluster-scoped
	// unavailableGroups are the groups the last refresh could not read, whose entries were carried
	// over from the previous snapshot. A resource missing from one of them is missing because we
	// could not look, which is a different answer from "it does not exist".
	unavailableGroups map[string]struct{}

	// muMiss guards the miss-triggered refresh: when it last ran, and whether one is running now.
	muMiss      sync.Mutex
	lastMiss    time.Time
	missPending bool

	// now is the clock, so the interval above can be exercised without sleeping.
	now func() time.Time
}

// clock reads the cache's clock, defaulting to the real one.
func (c *ResourceScopeCache) clock() time.Time {
	if c.now != nil {
		return c.now()
	}
	return time.Now()
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
		unavailableGroups: make(map[string]struct{}),
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

// Scope reports whether the resource is namespaced and whether the snapshot
// contains it. Unlike IsNamespaced, a miss is not coerced to cluster-scoped:
// multi-tenancy uses !known as "treat like namespaced" so a discovery hole
// cannot fail-open. IsNamespaced stays fail-closed-as-cluster-scoped for
// AccessibleNamespaces (a false namespaced=true would list every namespace).
//
// A nil *ResourceScopeCache answers like an empty one, so a typed nil handed to an interface value
// is as safe as a nil interface.
func (c *ResourceScopeCache) Scope(group, resource string) (bool, bool) {
	if c == nil {
		return false, false
	}

	c.mu.RLock()
	defer c.mu.RUnlock()

	namespaced, known := c.scopeMap[group+"/"+resource]
	return namespaced, known
}

// HasResource reports whether the discovery snapshot serves the resource at
// all. An empty or stale-in-the-negative cache answers false: a caller uses
// this to make a claim the plain RBAC answer would not support, and a claim
// about a resource we have never seen is worse than no claim.
func (c *ResourceScopeCache) HasResource(group, resource string) bool {
	if c == nil {
		return false
	}

	c.mu.RLock()
	defer c.mu.RUnlock()

	_, ok := c.scopeMap[group+"/"+resource]

	return ok
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

// GroupUnavailable reports whether the last refresh failed to read this group. A resource that is
// not in the snapshot is only genuinely absent when its group was read successfully; otherwise the
// snapshot has a hole there and the caller must keep failing closed.
func (c *ResourceScopeCache) GroupUnavailable(group string) bool {
	if c == nil {
		return false
	}
	c.mu.RLock()
	defer c.mu.RUnlock()
	_, unavailable := c.unavailableGroups[group]
	return unavailable
}

// ScopeOf answers the whole question the multi-tenancy decision asks, in the type it consumes.
//
// The derivation used to live in the caller, assembled from Scope, HasData and GroupUnavailable.
// That put the line that decides whether a discovery hole fails open or closed away from the data
// it reasons about, and made every test fake responsible for reproducing it - a fake that got it
// wrong left the suite green and the behaviour unsafe.
//
// It also took three separate read locks, so the three answers could come from either side of a
// refresh: a resource could be reported missing from a snapshot while HasData described the next
// one. One lock now, one snapshot, one answer.
//
// Version-agnostic, like the map behind it: see the ResourceScope interface in the multitenancy
// package for why the webhook's per-version lookup and this one agree in every case the platform
// produces.
func (c *ResourceScopeCache) ScopeOf(group, resource string) rules.ResourceScope {
	if c == nil {
		return rules.ResourceScope{}
	}

	c.mu.RLock()
	defer c.mu.RUnlock()

	namespaced, known := c.scopeMap[group+"/"+resource]
	if known {
		return rules.ResourceScope{Known: true, Namespaced: namespaced}
	}

	// Not in the snapshot. Either the resource does not exist, or it was created since the last
	// refresh - a CRD installed a minute ago - and the snapshot is simply behind. Ask for a
	// refresh, rate-limited, and answer from what is held: waiting for discovery here would put a
	// cluster-wide listing on the request path.
	c.noteMiss()

	// That is an answer only if there IS a snapshot and it did read this group; otherwise nobody
	// looked, and the caller has to keep failing closed.
	_, unavailable := c.unavailableGroups[group]
	return rules.ResourceScope{Absent: len(c.scopeMap) > 0 && !unavailable}
}

// noteMiss schedules a refresh because a lookup asked about something the snapshot does not hold.
//
// It never blocks the caller and never runs two refreshes at once: the miss is a hint that the
// snapshot is behind, not a request the caller waits on. ScopeOf holds a read lock when it calls
// this, which is why the refresh runs in its own goroutine - refresh() takes the write lock at the
// end.
func (c *ResourceScopeCache) noteMiss() {
	c.muMiss.Lock()
	if c.missPending || c.clock().Sub(c.lastMiss) < missRefreshInterval {
		c.muMiss.Unlock()
		return
	}
	c.missPending = true
	c.lastMiss = c.clock()
	c.muMiss.Unlock()

	go func() {
		defer func() {
			c.muMiss.Lock()
			c.missPending = false
			c.muMiss.Unlock()
		}()
		c.refresh()
	}()
}

// HasData returns true if the cache has been populated with any entries.
// This can be used for readiness checks: an empty cache means we could not
// fetch discovery data yet and would treat all unknown resources as cluster-scoped.
func (c *ResourceScopeCache) HasData() bool {
	if c == nil {
		return false
	}

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

// refresh fetches the served resources from discovery and merges them into the scope map.
//
// ServerGroupsAndResources lists every version of every group, subresources included (the
// preferred-resources call drops the names with a "/"), so HasNamespacedResourceMatching sees the
// same resources RBAC does. On a partial failure the call returns the lists it did fetch together
// with an ErrGroupDiscoveryFailed naming the GroupVersions it could not. The entries of those groups
// are carried over from the previous snapshot: a miss in the map now decides an authorization
// answer (Scope), so an APIService that is down for a minute must not evict its resources. A group
// that is neither returned nor reported as failed has left the cluster and its entries go. Any other
// error, or an empty result, preserves the whole snapshot.
func (c *ResourceScopeCache) refresh() {
	if c.discoveryClient == nil {
		klog.V(4).Info("ResourceScopeCache: no discovery client, skipping refresh")
		return
	}

	_, resourceLists, err := c.discoveryClient.ServerGroupsAndResources()

	// failedGroups are the groups whose entries are kept from the previous snapshot.
	failedGroups := map[string]struct{}{}
	if err != nil {
		failedVersions, partial := discovery.GroupDiscoveryFailedErrorGroups(err)
		if !partial || len(resourceLists) == 0 {
			klog.Warningf("ResourceScopeCache: discovery failed: %v, preserving existing cache", err)
			return
		}
		for gv := range failedVersions {
			failedGroups[gv.Group] = struct{}{}
		}
		klog.V(4).Infof("ResourceScopeCache: discovery returned partial results, keeping the previous entries of %d groups: %v", len(failedGroups), err)
	}

	newMap := make(map[string]bool)

	c.mu.RLock()
	for key, namespaced := range c.scopeMap {
		group, _, _ := strings.Cut(key, "/")
		if _, keep := failedGroups[group]; keep {
			newMap[key] = namespaced
		}
	}
	c.mu.RUnlock()

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
	c.unavailableGroups = failedGroups
	c.mu.Unlock()

	klog.V(4).Infof("ResourceScopeCache: refreshed with %d resources", len(newMap))
}
