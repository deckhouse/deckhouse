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
	"time"

	"github.com/go-logr/logr"
	"github.com/prometheus/client_golang/prometheus"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
	ctrlmetrics "sigs.k8s.io/controller-runtime/pkg/metrics"
)

var (
	cacheObjectCount = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "cache_object_count",
		Help: "Number of objects in the informer cache by GVK",
	}, []string{"gvk"})

	cacheSyncStatus = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "cache_sync_status",
		Help: "Whether the informer cache is synced (1) or not (0) by GVK",
	}, []string{"gvk"})
)

func init() {
	ctrlmetrics.Registry.MustRegister(cacheObjectCount, cacheSyncStatus)
}

type watchedType struct {
	gvk string
	obj client.Object
}

// CacheMetricsRunnable periodically collects cache object counts and sync status.
type CacheMetricsRunnable struct {
	cache  cache.Cache
	scheme *runtime.Scheme
	types  []watchedType
}

// NewCacheMetricsRunnable creates a new runnable that collects cache metrics.
func NewCacheMetricsRunnable(c cache.Cache, scheme *runtime.Scheme, types map[string]client.Object) *CacheMetricsRunnable {
	wt := make([]watchedType, 0, len(types))
	for gvk, obj := range types {
		wt = append(wt, watchedType{gvk: gvk, obj: obj})
	}
	return &CacheMetricsRunnable{cache: c, scheme: scheme, types: wt}
}

// Start implements manager.Runnable.
func (r *CacheMetricsRunnable) Start(ctx context.Context) error {
	logger := log.FromContext(ctx).WithName("cache-metrics")
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
			r.collect(ctx, logger)
		}
	}
}

// NeedLeaderElection implements manager.LeaderElectionRunnable.
func (r *CacheMetricsRunnable) NeedLeaderElection() bool {
	return false
}

func (r *CacheMetricsRunnable) collect(ctx context.Context, logger logr.Logger) {
	for _, wt := range r.types {
		count, err := r.listCount(ctx, wt)
		if err != nil {
			logger.Error(err, "failed to list objects for cache metrics", "gvk", wt.gvk)
			continue
		}
		cacheObjectCount.WithLabelValues(wt.gvk).Set(float64(count))

		synced, err := r.isSynced(ctx, wt)
		if err != nil {
			logger.Error(err, "failed to get informer sync status", "gvk", wt.gvk)
			continue
		}
		if synced {
			cacheSyncStatus.WithLabelValues(wt.gvk).Set(1)
		} else {
			cacheSyncStatus.WithLabelValues(wt.gvk).Set(0)
		}
	}
}

func (r *CacheMetricsRunnable) listCount(ctx context.Context, wt watchedType) (int, error) {
	var gvk schema.GroupVersionKind
	if u, ok := wt.obj.(*unstructured.Unstructured); ok {
		gvk = u.GroupVersionKind()
	} else {
		gvk = r.gvkForObj(wt.obj)
	}

	list := &unstructured.UnstructuredList{}
	list.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   gvk.Group,
		Version: gvk.Version,
		Kind:    gvk.Kind + "List",
	})
	if err := r.cache.List(ctx, list); err != nil {
		return 0, err
	}
	return len(list.Items), nil
}

func (r *CacheMetricsRunnable) isSynced(ctx context.Context, wt watchedType) (bool, error) {
	informer, err := r.cache.GetInformer(ctx, wt.obj)
	if err != nil {
		return false, err
	}
	return informer.HasSynced(), nil
}

func (r *CacheMetricsRunnable) gvkForObj(obj client.Object) schema.GroupVersionKind {
	gvks, _, _ := r.scheme.ObjectKinds(obj)
	if len(gvks) == 0 {
		return schema.GroupVersionKind{}
	}
	return gvks[0]
}
