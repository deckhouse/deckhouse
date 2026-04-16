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

package common

import (
	"context"
	"fmt"
	"reflect"
	"sort"
	"sync"
	"sync/atomic"
	"time"

	"github.com/go-logr/logr"
	coordinationv1 "k8s.io/api/coordination/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	toolscache "k8s.io/client-go/tools/cache"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"

	v1 "github.com/deckhouse/node-controller/api/deckhouse.io/v1"
)

// cacheCounter tracks the number of objects loaded into the informer cache per type.
type cacheCounter struct {
	mu     sync.RWMutex
	counts map[string]*atomic.Int64
	logger logr.Logger
}

func (c *cacheCounter) increment(key string) {
	c.mu.RLock()
	counter, ok := c.counts[key]
	c.mu.RUnlock()

	if ok {
		counter.Add(1)
		return
	}

	c.mu.Lock()
	counter, ok = c.counts[key]
	if !ok {
		counter = &atomic.Int64{}
		c.counts[key] = counter
	}
	c.mu.Unlock()
	counter.Add(1)
}

func (c *cacheCounter) reportAndReset() {
	c.mu.RLock()
	keys := make([]string, 0, len(c.counts))
	for k := range c.counts {
		keys = append(keys, k)
	}
	c.mu.RUnlock()

	if len(keys) == 0 {
		return
	}

	sort.Strings(keys)
	keysAndValues := make([]interface{}, 0, len(keys)*2)
	for _, k := range keys {
		c.mu.RLock()
		counter := c.counts[k]
		c.mu.RUnlock()
		val := counter.Swap(0)
		if val > 0 {
			keysAndValues = append(keysAndValues, k, val)
		}
	}

	if len(keysAndValues) > 0 {
		c.logger.Info("cache objects loaded since last report", keysAndValues...)
	}
}

func (c *cacheCounter) reportLoop(ctx context.Context) {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			c.reportAndReset()
		}
	}
}

// objectTypeName returns a human-readable name for a cached object.
func objectTypeName(obj interface{}) string {
	if accessor, err := meta.Accessor(obj); err == nil {
		ns := accessor.GetNamespace()
		kind := reflect.TypeOf(obj).Elem().Name()
		if kind == "" {
			kind = fmt.Sprintf("%T", obj)
		}
		if ns != "" {
			return fmt.Sprintf("%s/%s", kind, ns)
		}
		return kind
	}
	return fmt.Sprintf("%T", obj)
}

// CacheTransformWithLogging returns a TransformFunc that counts objects loaded into
// the informer cache by type and periodically logs the counts.
// It also strips managedFields to reduce memory usage.
func CacheTransformWithLogging(ctx context.Context, logger logr.Logger) toolscache.TransformFunc {
	counter := &cacheCounter{
		counts: make(map[string]*atomic.Int64),
		logger: logger.WithName("cache-monitor"),
	}

	go counter.reportLoop(ctx)

	strip := cache.TransformStripManagedFields()
	return func(obj interface{}) (interface{}, error) {
		key := objectTypeName(obj)
		counter.increment(key)
		return strip(obj)
	}
}

// cachedType describes a typed or unstructured object list to probe in the cache.
type cachedType struct {
	name string
	list client.ObjectList
}

// knownCachedTypes returns the list of object types that controllers may cache.
func knownCachedTypes() []cachedType {
	unstrList := func(group, version, kind string) client.ObjectList {
		u := &unstructured.UnstructuredList{}
		u.SetGroupVersionKind(schema.GroupVersionKind{Group: group, Version: version, Kind: kind})
		return u
	}

	return []cachedType{
		{"v1/Node", &corev1.NodeList{}},
		{"v1/Pod", &corev1.PodList{}},
		{"v1/Secret", &corev1.SecretList{}},
		{"v1/Endpoints", &corev1.EndpointsList{}},
		{"coordination.k8s.io/v1/Lease", &coordinationv1.LeaseList{}},
		{"deckhouse.io/v1/NodeGroup", &v1.NodeGroupList{}},
		{"machine.sapcloud.io/v1alpha1/Machine", unstrList("machine.sapcloud.io", "v1alpha1", "MachineList")},
		{"machine.sapcloud.io/v1alpha1/MachineDeployment", unstrList("machine.sapcloud.io", "v1alpha1", "MachineDeploymentList")},
		{"cluster.x-k8s.io/v1beta2/Machine", unstrList("cluster.x-k8s.io", "v1beta2", "MachineList")},
		{"cluster.x-k8s.io/v1beta2/MachineDeployment", unstrList("cluster.x-k8s.io", "v1beta2", "MachineDeploymentList")},
	}
}

// LogCacheContents logs the number of objects in the cache for each known type.
// Call this after the cache has synced to get a snapshot of what's consuming memory.
func LogCacheContents(ctx context.Context, c cache.Cache, logger logr.Logger) {
	log := logger.WithName("cache-monitor")

	type cacheEntry struct {
		name  string
		count int
	}
	var entries []cacheEntry

	for _, ct := range knownCachedTypes() {
		list := ct.list.DeepCopyObject().(client.ObjectList)
		if err := c.List(ctx, list); err != nil {
			continue
		}

		items, err := meta.ExtractList(list)
		if err != nil {
			continue
		}

		count := len(items)
		if count == 0 {
			continue
		}

		entries = append(entries, cacheEntry{name: ct.name, count: count})
	}

	sort.Slice(entries, func(i, j int) bool {
		return entries[i].count > entries[j].count
	})

	keysAndValues := make([]interface{}, 0, len(entries)*2)
	for _, e := range entries {
		keysAndValues = append(keysAndValues, e.name, e.count)
	}

	if len(keysAndValues) > 0 {
		log.Info("cache synced: object counts", keysAndValues...)
	}
}
