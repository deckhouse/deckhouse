/*
Copyright 2026 Flant JSC

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

package quota

import (
	"fmt"
	"slices"
	"sync"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/discovery"
)

// countVerbs are the verbs the resource quota controller of kube-controller-manager needs to count a
// resource.
var countVerbs = []string{"create", "list", "watch", "delete"}

// ReconcileInvalidateEvery bounds how often a miss of the controller invalidates its cached discovery:
// an invalidation drops all of it, and a quota that keeps a key of a group not served yet misses on
// every reconcile.
const ReconcileInvalidateEvery = 10 * time.Second

// APIDiscovery looks resources up in the discovery of the API server. A miss invalidates the cache and
// looks again once, so a CRD installed a moment ago is found; with InvalidateEvery, misses within it of
// the last invalidation take the cache as it is.
type APIDiscovery struct {
	Client discovery.CachedDiscoveryInterface
	// InvalidateEvery is zero for a webhook, which answers a user who may have installed the CRD a
	// moment ago, and ReconcileInvalidateEvery for the controller.
	InvalidateEvery time.Duration
	// Now is time.Now unless a test sets it.
	Now func() time.Time

	mu              sync.Mutex
	lastInvalidated time.Time
}

// Lookup tells how the API server serves the resource of the group, "" being the core group.
func (d *APIDiscovery) Lookup(group, resource string) (Served, error) {
	served, err := d.lookup(group, resource)
	if err != nil || served != ServedResourceMissing && served != ServedGroupNotServed {
		return served, err
	}
	if !d.mayInvalidate() {
		return served, nil
	}
	d.Client.Invalidate()
	return d.lookup(group, resource)
}

// mayInvalidate takes the turn to invalidate the cache when the last invalidation is old enough.
func (d *APIDiscovery) mayInvalidate() bool {
	now := time.Now()
	if d.Now != nil {
		now = d.Now()
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	if !d.lastInvalidated.IsZero() && now.Sub(d.lastInvalidated) < d.InvalidateEvery {
		return false
	}
	d.lastInvalidated = now
	return true
}

func (d *APIDiscovery) lookup(group, resource string) (Served, error) {
	groups, err := d.Client.ServerGroups()
	if err != nil {
		return ServedUnknown, fmt.Errorf("list the API groups: %w", err)
	}
	index := slices.IndexFunc(groups.Groups, func(g metav1.APIGroup) bool { return g.Name == group })
	if index < 0 {
		return ServedGroupNotServed, nil
	}
	version := groups.Groups[index].PreferredVersion.GroupVersion
	resources, err := d.Client.ServerResourcesForGroupVersion(version)
	if err != nil {
		return ServedUnknown, fmt.Errorf("list the resources of %s: %w", version, err)
	}
	for _, r := range resources.APIResources {
		if r.Name != resource {
			continue
		}
		if !r.Namespaced {
			return ServedClusterScoped, nil
		}
		for _, verb := range countVerbs {
			if !slices.Contains(r.Verbs, verb) {
				return ServedNotCountable, nil
			}
		}
		return ServedNamespaced, nil
	}
	return ServedResourceMissing, nil
}
