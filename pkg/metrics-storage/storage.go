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

package metricsstorage

import (
	"net/http"

	"github.com/deckhouse/deckhouse/pkg/metrics-storage/collectors"
	"github.com/deckhouse/deckhouse/pkg/metrics-storage/operation"
	"github.com/prometheus/client_golang/prometheus"
)

type Registerer interface {
	RegisterCounter(metric string, labels map[string]string) *prometheus.CounterVec
	RegisterGauge(metric string, labels map[string]string) *prometheus.GaugeVec
	RegisterHistogram(metric string, labels map[string]string, buckets []float64) *prometheus.HistogramVec
}

type Collector interface {
	Counter(metric string, labels map[string]string) *prometheus.CounterVec
	CounterAdd(metric string, value float64, labels map[string]string)

	Gauge(metric string, labels map[string]string) *prometheus.GaugeVec
	GaugeAdd(metric string, value float64, labels map[string]string)
	GaugeSet(metric string, value float64, labels map[string]string)

	Histogram(metric string, labels map[string]string, buckets []float64) *prometheus.HistogramVec
	HistogramObserve(metric string, value float64, labels map[string]string, buckets []float64)
}

type Storage interface {
	Registerer
	Collector

	ApplyOperation(op operation.MetricOperation, commonLabels map[string]string)
	ApplyBatchOperations(ops []operation.MetricOperation, labels map[string]string) error

	Grouped() GroupedStorage
	Collector() prometheus.Collector
	Handler() http.Handler
}

type GroupedStorage interface {
	Collector() prometheus.Collector
	Registerer() prometheus.Registerer

	ExpireGroupMetrics(group string)
	ExpireGroupMetricByName(group, name string)

	CounterAdd(group string, name string, value float64, labels map[string]string)
	GaugeSet(group string, name string, value float64, labels map[string]string)

	RegisterCounterCollector(name string, labelNames []string) (*collectors.ConstCounterCollector, error)
	RegisterGaugeCollector(name string, labelNames []string) (*collectors.ConstGaugeCollector, error)
}
