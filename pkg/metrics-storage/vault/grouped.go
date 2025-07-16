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
	"sync"

	"github.com/deckhouse/deckhouse/pkg/log"
	"github.com/prometheus/client_golang/prometheus"

	"github.com/deckhouse/deckhouse/pkg/metrics-storage/collectors"
	labelspkg "github.com/deckhouse/deckhouse/pkg/metrics-storage/labels"
)

var _ prometheus.Collector = (*GroupedVault)(nil)

type GroupedVault struct {
	collectors map[string]collectors.ConstCollector

	mu         sync.Mutex
	registry   *prometheus.Registry
	registerer prometheus.Registerer

	resolveMetricNameFunc func(name string) string

	logger *log.Logger
}

type Option func(*Options)

type Options struct {
	logger   *log.Logger
	registry *prometheus.Registry
}

func NewOptions(opts ...Option) *Options {
	v := &Options{
		logger: log.NewLogger().Named("grouped-vault"),
	}

	for _, option := range opts {
		option(v)
	}

	return v
}

// WithLogger sets the logger for the GroupedVault.
func WithLogger(logger *log.Logger) Option {
	return func(v *Options) {
		v.logger = logger
	}
}

// WithRegistry sets an existing registry for the GroupedVault.
func WithRegistry(registry *prometheus.Registry) Option {
	return func(v *Options) {
		v.registry = registry
	}
}

// WithNewRegistry creates a new registry for the GroupedVault.
func WithNewRegistry() Option {
	return func(v *Options) {
		v.registry = prometheus.NewRegistry()
	}
}

// NewGroupedVault creates and initializes a new GroupedVault instance.
// It initializes the vault with an empty collectors map and the default Prometheus registerer.
// If you need to use a custom registerer, you can provide it through the WithRegisterer or WithNewRegisterer option.
//
// The GroupedVault is a specialized collector that manages multiple Prometheus metrics
// under a single registration point, grouped by name. It handles the registration and
// deregistration of metrics with the Prometheus collector registry.
//
// Parameters:
//   - resolveMetricNameFunc: A function that transforms a metric name string. This can be used
//     for adding prefixes, standardizing format, or other name transformations.
//   - options: Optional configuration options that modify the behavior of the vault.
//     These are applied in order after the default configuration is set.//
//
// Options include:
//   - WithNewRegistry: Creates a new isolated Prometheus registry for the metrics
//   - WithRegistry: Uses a provided Prometheus registry
//   - WithLogger: Sets a custom logger for the metrics storage
func NewGroupedVault(resolveMetricNameFunc func(name string) string, opts ...Option) *GroupedVault {
	vault := &GroupedVault{
		collectors:            make(map[string]collectors.ConstCollector),
		registerer:            prometheus.DefaultRegisterer,
		resolveMetricNameFunc: resolveMetricNameFunc,

		logger: log.NewLogger().Named("grouped-vault"),
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

func (v *GroupedVault) Registerer() prometheus.Registerer {
	return v.registerer
}

// Collector returns collector of MetricStorage
// it can be useful to collect metrics in external registerer
func (v *GroupedVault) Collector() prometheus.Collector {
	if v.registry != nil {
		return v.registry
	}

	return v
}

// ExpireGroupMetrics takes each collector in collectors and clear all metrics by group.
func (v *GroupedVault) ExpireGroupMetrics(group string) {
	v.mu.Lock()
	for _, collector := range v.collectors {
		collector.ExpireGroupMetrics(group)
	}
	v.mu.Unlock()
}

// ExpireGroupMetricByName gets a collector by its name and clears all metrics inside the collector by the group.
func (v *GroupedVault) ExpireGroupMetricByName(group, name string) {
	metricName := v.resolveMetricNameFunc(name)
	v.mu.Lock()
	collector, ok := v.collectors[metricName]
	if ok {
		collector.ExpireGroupMetrics(group)
	}
	v.mu.Unlock()
}

func (v *GroupedVault) RegisterCounterCollector(name string, labelNames []string) (*collectors.ConstCounterCollector, error) {
	metricName := v.resolveMetricNameFunc(name)
	v.mu.Lock()
	defer v.mu.Unlock()

	collector, ok := v.collectors[metricName]
	if !ok {
		collector = collectors.NewConstCounterCollector(metricName, labelNames)

		if err := v.registerer.Register(collector); err != nil {
			return nil, fmt.Errorf("counter '%s' %v registration: %v", metricName, labelNames, err)
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

func (v *GroupedVault) RegisterGaugeCollector(name string, labelNames []string) (*collectors.ConstGaugeCollector, error) {
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

func (v *GroupedVault) CounterAdd(group string, name string, value float64, labels map[string]string) {
	metricName := v.resolveMetricNameFunc(name)

	c, err := v.RegisterCounterCollector(metricName, labelspkg.LabelNames(labels))
	if err != nil {
		v.logger.Error("CounterAdd", log.Err(err))

		return
	}

	c.Add(value, labels, collectors.WithGroup(group))
}

func (v *GroupedVault) GaugeSet(group string, name string, value float64, labels map[string]string) {
	metricName := v.resolveMetricNameFunc(name)

	c, err := v.RegisterGaugeCollector(metricName, labelspkg.LabelNames(labels))
	if err != nil {
		v.logger.Error("GaugeSet", log.Err(err))

		return
	}

	c.Set(value, labels, collectors.WithGroup(group))
}

func (v *GroupedVault) Describe(ch chan<- *prometheus.Desc) {
	v.mu.Lock()
	defer v.mu.Unlock()

	for _, collector := range v.collectors {
		collector.Describe(ch)
	}
}

func (v *GroupedVault) Collect(ch chan<- prometheus.Metric) {
	v.mu.Lock()
	defer v.mu.Unlock()

	for _, collector := range v.collectors {
		collector.Collect(ch)
	}
}
