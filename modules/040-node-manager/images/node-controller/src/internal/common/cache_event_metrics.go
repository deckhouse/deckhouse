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

	"github.com/prometheus/client_golang/prometheus"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/util/workqueue"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	ctrlmetrics "sigs.k8s.io/controller-runtime/pkg/metrics"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

var (
	cacheEventsTotal = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "cache_events_total",
		Help: "Total number of cache events by GVK and event type",
	}, []string{"gvk", "event_type"})

	cacheEventDuration = prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "cache_event_duration_seconds",
		Help:    "Duration of cache event handler processing",
		Buckets: []float64{.0001, .0005, .001, .005, .01, .05, .1, .5, 1},
	}, []string{"gvk", "event_type"})
)

func init() {
	ctrlmetrics.Registry.MustRegister(cacheEventsTotal, cacheEventDuration)
}

// GVKLabel returns a string label for the given object's GVK.
func GVKLabel(scheme *runtime.Scheme, obj client.Object) string {
	if u, ok := obj.(*unstructured.Unstructured); ok {
		return formatGVK(u.GroupVersionKind())
	}
	gvks, _, err := scheme.ObjectKinds(obj)
	if err != nil || len(gvks) == 0 {
		return "Unknown"
	}
	return formatGVK(gvks[0])
}

func formatGVK(gvk schema.GroupVersionKind) string {
	if gvk.Group == "" {
		return gvk.Version + "." + gvk.Kind
	}
	return gvk.Group + "/" + gvk.Version + "." + gvk.Kind
}

// InstrumentEventHandler wraps an EventHandler with metrics collection.
func InstrumentEventHandler(gvk string, h handler.EventHandler) handler.EventHandler {
	return &instrumentedEventHandler{inner: h, gvk: gvk}
}

type instrumentedEventHandler struct {
	inner handler.EventHandler
	gvk   string
}

func (h *instrumentedEventHandler) Create(ctx context.Context, e event.TypedCreateEvent[client.Object], q workqueue.TypedRateLimitingInterface[reconcile.Request]) {
	start := time.Now()
	h.inner.Create(ctx, e, q)
	cacheEventsTotal.WithLabelValues(h.gvk, "create").Inc()
	cacheEventDuration.WithLabelValues(h.gvk, "create").Observe(time.Since(start).Seconds())
}

func (h *instrumentedEventHandler) Update(ctx context.Context, e event.TypedUpdateEvent[client.Object], q workqueue.TypedRateLimitingInterface[reconcile.Request]) {
	start := time.Now()
	h.inner.Update(ctx, e, q)
	cacheEventsTotal.WithLabelValues(h.gvk, "update").Inc()
	cacheEventDuration.WithLabelValues(h.gvk, "update").Observe(time.Since(start).Seconds())
}

func (h *instrumentedEventHandler) Delete(ctx context.Context, e event.TypedDeleteEvent[client.Object], q workqueue.TypedRateLimitingInterface[reconcile.Request]) {
	start := time.Now()
	h.inner.Delete(ctx, e, q)
	cacheEventsTotal.WithLabelValues(h.gvk, "delete").Inc()
	cacheEventDuration.WithLabelValues(h.gvk, "delete").Observe(time.Since(start).Seconds())
}

func (h *instrumentedEventHandler) Generic(ctx context.Context, e event.TypedGenericEvent[client.Object], q workqueue.TypedRateLimitingInterface[reconcile.Request]) {
	start := time.Now()
	h.inner.Generic(ctx, e, q)
	cacheEventsTotal.WithLabelValues(h.gvk, "generic").Inc()
	cacheEventDuration.WithLabelValues(h.gvk, "generic").Observe(time.Since(start).Seconds())
}
