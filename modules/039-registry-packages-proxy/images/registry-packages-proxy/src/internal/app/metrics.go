// Copyright 2024 Flant JSC
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

package app

import (
	"github.com/prometheus/client_golang/prometheus"

	"registry-packages-proxy/internal/cache"
)

func RegisterMetrics() *cache.Metrics {
	cacheSize := prometheus.NewGauge(prometheus.GaugeOpts{
		Namespace: "registry_packages_proxy",
		Subsystem: "cache",
		Name:      "size",
		Help:      "Size of the cache in bytes",
	})
	prometheus.MustRegister(cacheSize)

	cacheMetrics := &cache.Metrics{
		CacheSize: cacheSize,
	}
	return cacheMetrics
}
