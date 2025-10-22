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

	"github.com/prometheus/client_golang/prometheus"

	"github.com/deckhouse/deckhouse/pkg/metrics-storage/collectors"
	"github.com/deckhouse/deckhouse/pkg/metrics-storage/operation"
	"github.com/deckhouse/deckhouse/pkg/metrics-storage/options"
)

type Registerer interface {
	RegisterCounter(metric string, labelNames []string, opts ...options.RegisterOption) (*collectors.ConstCounterCollector, error)
	RegisterGauge(metric string, labelNames []string, opts ...options.RegisterOption) (*collectors.ConstGaugeCollector, error)
	RegisterHistogram(metric string, labelNames []string, buckets []float64, opts ...options.RegisterOption) (*collectors.ConstHistogramCollector, error)
}

type Collector interface {
	Counter(metric string, labels map[string]string) *collectors.ConstCounterCollector
	CounterAdd(metric string, value float64, labels map[string]string)

	Gauge(metric string, labels map[string]string) *collectors.ConstGaugeCollector
	GaugeAdd(metric string, value float64, labels map[string]string)
	GaugeSet(metric string, value float64, labels map[string]string)

	Histogram(metric string, labels map[string]string, buckets []float64) *collectors.ConstHistogramCollector
	HistogramObserve(metric string, value float64, labels map[string]string, buckets []float64)
}

type CollectorFunc func(s Storage)

type Storage interface {
	Registerer
	Collector

	ApplyOperation(op operation.MetricOperation, commonLabels map[string]string) error
	ApplyBatchOperations(ops []operation.MetricOperation, labels map[string]string) error

	Grouped() GroupedStorage
	Collector() prometheus.Collector
	Handler() http.Handler
}

type GroupedCollector interface {
	CounterAdd(group string, metric string, value float64, labels map[string]string)

	GaugeAdd(group string, metric string, value float64, labels map[string]string)
	GaugeSet(group string, metric string, value float64, labels map[string]string)

	HistogramObserve(group string, metric string, value float64, labels map[string]string, buckets []float64)
}

type GroupedStorage interface {
	Registerer
	GroupedCollector

	Collector() prometheus.Collector

	ExpireGroupMetrics(group string)
	ExpireGroupMetricByName(group, name string)
}
