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

package vault

import (
	"fmt"
	"log/slog"
	"sync"

	"github.com/deckhouse/deckhouse/pkg/log"
	"github.com/prometheus/client_golang/prometheus"

	"github.com/deckhouse/deckhouse/pkg/metrics-storage/collectors"
	labelspkg "github.com/deckhouse/deckhouse/pkg/metrics-storage/labels"
)

var _ prometheus.Collector = (*Vault)(nil)

type Vault struct {
	collectors map[string]collectors.ConstCollector

	mu         sync.Mutex
	registry   *prometheus.Registry
	registerer prometheus.Registerer

	resolveMetricNameFunc func(name string) string

	logger *log.Logger
}

// NewVault creates and initializes a new Vault instance.
// It initializes the vault with an empty collectors map and the default Prometheus registerer.
// If you need to use a custom registerer, you can provide it through the WithRegistry or WithNewRegistry option.
//
// The Vault is a specialized collector that manages multiple Prometheus metrics
// under a single registration point. It handles the registration and
// deregistration of metrics with the Prometheus collector registry.
//
// Parameters:
//   - resolveMetricNameFunc: A function that transforms a metric name string. This can be used
//     for adding prefixes, standardizing format, or other name transformations.
//   - options: Optional configuration options that modify the behavior of the vault.
//     These are applied in order after the default configuration is set.
//
// Options include:
//   - WithNewRegistry: Creates a new isolated Prometheus registry for the metrics
//   - WithRegistry: Uses a provided Prometheus registry
//   - WithLogger: Sets a custom logger for the metrics storage
func NewVault(resolveMetricNameFunc func(name string) string, opts ...Option) *Vault {
	vault := &Vault{
		collectors:            make(map[string]collectors.ConstCollector),
		registerer:            prometheus.DefaultRegisterer,
		resolveMetricNameFunc: resolveMetricNameFunc,

		logger: log.NewLogger().Named("vault"),
	}

	options := NewOptions(opts...)

	if options.registry != nil {
		vault.registry = options.registry
		vault.registerer = options.registry
	}

	if options.logger != nil {
		vault.logger = options.logger
	}

	return vault
}

func (v *Vault) Registerer() prometheus.Registerer {
	return v.registerer
}

// Collector returns collector of MetricStorage
// it can be useful to collect metrics in external registerer
func (v *Vault) Collector() prometheus.Collector {
	if v.registry != nil {
		return v.registry
	}

	return v
}

func (v *Vault) RegisterCounter(name string, labelNames []string) (*collectors.ConstCounterCollector, error) {
	metricName := v.resolveMetricNameFunc(name)
	v.mu.Lock()
	defer v.mu.Unlock()

	collector, ok := v.collectors[metricName]
	if !ok {
		collector = collectors.NewConstCounterCollector(metricName, labelNames)

		if err := v.registerer.Register(collector); err != nil {
			return nil, fmt.Errorf("counter '%s' %v registration: %w", metricName, labelNames, err)
		}

		v.collectors[metricName] = collector
	}

	if ok && !labelspkg.IsSubset(collector.LabelNames(), labelNames) {
		collector.UpdateLabels(labelNames)
	}

	counter, ok := collector.(*collectors.ConstCounterCollector)
	if !ok {
		return nil, fmt.Errorf("counter %v collector requested, but %s %v collector exists",
			labelNames, collector.Type(), collector.LabelNames())
	}

	return counter, nil
}

func (v *Vault) RegisterGauge(name string, labelNames []string) (*collectors.ConstGaugeCollector, error) {
	metricName := v.resolveMetricNameFunc(name)
	v.mu.Lock()
	defer v.mu.Unlock()

	collector, ok := v.collectors[metricName]
	if !ok {
		collector = collectors.NewConstGaugeCollector(metricName, labelNames)

		if err := v.registerer.Register(collector); err != nil {
			return nil, fmt.Errorf("gauge '%s' %v registration: %v", metricName, labelNames, err)
		}

		v.collectors[metricName] = collector
	}

	if ok && !labelspkg.IsSubset(collector.LabelNames(), labelNames) {
		collector.UpdateLabels(labelNames)
	}

	gauge, ok := collector.(*collectors.ConstGaugeCollector)
	if !ok {
		return nil, fmt.Errorf("gauge %v collector requested, but %s %v collector exists",
			labelNames, collector.Type(), collector.LabelNames())
	}

	return gauge, nil
}

func (v *Vault) RegisterHistogram(name string, labelNames []string, buckets []float64) (*collectors.ConstHistogramCollector, error) {
	metricName := v.resolveMetricNameFunc(name)
	v.mu.Lock()
	defer v.mu.Unlock()

	collector, ok := v.collectors[metricName]
	if !ok {
		collector = collectors.NewConstHistogramCollector(metricName, labelNames, buckets)

		if err := v.registerer.Register(collector); err != nil {
			return nil, fmt.Errorf("histogram '%s' %v registration: %v", metricName, labelNames, err)
		}

		v.collectors[metricName] = collector
	}

	if ok && !labelspkg.IsSubset(collector.LabelNames(), labelNames) {
		collector.UpdateLabels(labelNames)
	}

	histogram, ok := collector.(*collectors.ConstHistogramCollector)
	if !ok {
		return nil, fmt.Errorf("histogram %v collector requested, but %s %v collector exists",
			labelNames, collector.Type(), collector.LabelNames())
	}

	return histogram, nil
}

func (v *Vault) CounterAdd(name string, value float64, labels map[string]string) {
	c, err := v.RegisterCounter(name, labelspkg.LabelNames(labels))
	if err != nil {
		v.logger.Error(
			"CounterAdd",
			slog.String("name", name),
			slog.Any("labels", labels),
			log.Err(err),
		)

		return
	}

	c.Add(value, labels)
}

func (v *Vault) GaugeSet(name string, value float64, labels map[string]string) {
	metricName := v.resolveMetricNameFunc(name)

	c, err := v.RegisterGauge(metricName, labelspkg.LabelNames(labels))
	if err != nil {
		v.logger.Error(
			"GaugeSet",
			slog.String("name", name),
			slog.Any("labels", labels),
			log.Err(err),
		)

		return
	}

	c.Set(value, labels)
}

func (v *Vault) GaugeAdd(name string, value float64, labels map[string]string) {
	c, err := v.RegisterGauge(name, labelspkg.LabelNames(labels))
	if err != nil {
		v.logger.Error(
			"GaugeAdd",
			slog.String("name", name),
			slog.Any("labels", labels),
			log.Err(err),
		)

		return
	}

	c.Add(value, labels)
}

func (v *Vault) HistogramObserve(name string, value float64, labels map[string]string, buckets []float64) {
	c, err := v.RegisterHistogram(name, labelspkg.LabelNames(labels), buckets)
	if err != nil {
		v.logger.Error(
			"HistogramObserve",
			slog.String("name", name),
			slog.Any("labels", labels),
			slog.Any("buckets", buckets),
			log.Err(err),
		)

		return
	}

	c.Observe(value, labels)
}

// Reset clears all collectors from the vault and unregisters them
func (v *Vault) Reset() error {
	v.mu.Lock()
	defer v.mu.Unlock()

	var lastErr error
	for _, collector := range v.collectors {
		v.registry.Unregister(collector)
	}

	v.collectors = make(map[string]collectors.ConstCollector)
	return lastErr
}

func (v *Vault) Describe(ch chan<- *prometheus.Desc) {
	v.mu.Lock()
	defer v.mu.Unlock()

	for _, collector := range v.collectors {
		collector.Describe(ch)
	}
}

func (v *Vault) Collect(ch chan<- prometheus.Metric) {
	v.mu.Lock()
	defer v.mu.Unlock()

	for _, collector := range v.collectors {
		collector.Collect(ch)
	}
}
