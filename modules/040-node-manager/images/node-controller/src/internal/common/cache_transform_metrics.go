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
	"time"

	"github.com/prometheus/client_golang/prometheus"
	toolscache "k8s.io/client-go/tools/cache"
	ctrlmetrics "sigs.k8s.io/controller-runtime/pkg/metrics"
)

var (
	cacheTransformDuration = prometheus.NewHistogram(prometheus.HistogramOpts{
		Name:    "cache_transform_duration_seconds",
		Help:    "Duration of cache transform function execution",
		Buckets: []float64{.00001, .00005, .0001, .0005, .001, .005, .01, .05},
	})

	cacheTransformErrors = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "cache_transform_errors_total",
		Help: "Total number of cache transform errors",
	})
)

func init() {
	ctrlmetrics.Registry.MustRegister(cacheTransformDuration, cacheTransformErrors)
}

// InstrumentTransform wraps a cache TransformFunc with duration and error metrics.
func InstrumentTransform(fn toolscache.TransformFunc) toolscache.TransformFunc {
	return func(obj interface{}) (interface{}, error) {
		start := time.Now()
		result, err := fn(obj)
		cacheTransformDuration.Observe(time.Since(start).Seconds())
		if err != nil {
			cacheTransformErrors.Inc()
		}
		return result, err
	}
}
