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

package storage

import (
	"fmt"
	"log/slog"
	"sync"

	"github.com/prometheus/client_golang/prometheus"

	"github.com/deckhouse/deckhouse/pkg/log"

	"github.com/deckhouse/deckhouse/pkg/metrics-storage/collectors"
	labelspkg "github.com/deckhouse/deckhouse/pkg/metrics-storage/labels"
	"github.com/deckhouse/deckhouse/pkg/metrics-storage/options"
)

var _ prometheus.Collector = (*GroupedVault)(nil)

type GroupedVault struct {
	collectors map[string]collectors.ConstCollector

	mu         sync.Mutex
	registry   *prometheus.Registry
	registerer prometheus.Registerer

	logger *log.Logger
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
func NewGroupedVault(opts ...options.VaultOption) *GroupedVault {
	vault := &GroupedVault{
		collectors: make(map[string]collectors.ConstCollector),
		registerer: prometheus.DefaultRegisterer,

		logger: log.NewLogger().Named("vault-grouped"),
	}

	o := options.NewVaultOptions(opts...)

	if o.Registry != nil {
		vault.registry = o.Registry
		vault.registerer = o.Registry
	}

	if o.Logger != nil {
		vault.logger = o.Logger
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
	v.mu.Lock()
	collector, ok := v.collectors[name]
	if ok {
		collector.ExpireGroupMetrics(group)
	}
	v.mu.Unlock()
}

func (v *GroupedVault) RegisterCounter(name string, labelNames []string, opts ...options.RegisterOption) (*collectors.ConstCounterCollector, error) {
	v.mu.Lock()
	defer v.mu.Unlock()

	o := options.NewRegisterOptions(opts...)

	if o.Help == "" {
		o.Help = name
	}

	collector, ok := v.collectors[name]
	if !ok {
		collector = collectors.NewConstCounterCollector(collectors.MetricDescription{
			Name:        name,
			Help:        o.Help,
			LabelNames:  labelNames,
			ConstLabels: o.ConstantLabels,
		})

		if err := v.registerer.Register(collector); err != nil {
			return nil, fmt.Errorf("counter '%s' %v registration: %v", name, labelNames, err)
		}

		v.collectors[name] = collector
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

func (v *GroupedVault) RegisterGauge(name string, labelNames []string, opts ...options.RegisterOption) (*collectors.ConstGaugeCollector, error) {
	v.mu.Lock()
	defer v.mu.Unlock()

	options := options.NewRegisterOptions(opts...)

	if options.Help == "" {
		options.Help = name
	}

	collector, ok := v.collectors[name]
	if !ok {
		collector = collectors.NewConstGaugeCollector(&collectors.MetricDescription{
			Name:        name,
			Help:        options.Help,
			LabelNames:  labelNames,
			ConstLabels: options.ConstantLabels,
		})

		if err := v.registerer.Register(collector); err != nil {
			return nil, fmt.Errorf("gauge '%s' %v registration: %v", name, labelNames, err)
		}

		v.collectors[name] = collector
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

func (v *GroupedVault) RegisterHistogram(name string, labelNames []string, buckets []float64, opts ...options.RegisterOption) (*collectors.ConstHistogramCollector, error) {
	v.mu.Lock()
	defer v.mu.Unlock()

	o := options.NewRegisterOptions(opts...)

	if o.Help == "" {
		o.Help = name
	}

	collector, ok := v.collectors[name]
	if !ok {
		collector = collectors.NewConstHistogramCollector(&collectors.MetricDescription{
			Name:        name,
			Help:        o.Help,
			LabelNames:  labelNames,
			ConstLabels: o.ConstantLabels,
		}, buckets)

		if err := v.registerer.Register(collector); err != nil {
			return nil, fmt.Errorf("histogram '%s' %v registration: %v", name, labelNames, err)
		}

		v.collectors[name] = collector
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

func (v *GroupedVault) CounterAdd(group string, name string, value float64, labels map[string]string) {
	c, err := v.RegisterCounter(name, labelspkg.LabelNames(labels))
	if err != nil {
		v.logger.Error(
			"CounterAdd",
			slog.String("group", group),
			slog.String("name", name),
			slog.Any("labels", labels),
			log.Err(err),
		)

		return
	}

	c.Add(value, labels, collectors.WithGroup(group))
}

func (v *GroupedVault) GaugeSet(group string, name string, value float64, labels map[string]string) {
	c, err := v.RegisterGauge(name, labelspkg.LabelNames(labels))
	if err != nil {
		v.logger.Error(
			"GaugeSet",
			slog.String("group", group),
			slog.String("name", name),
			slog.Any("labels", labels),
			log.Err(err),
		)

		return
	}

	c.Set(value, labels, collectors.WithGroup(group))
}

func (v *GroupedVault) GaugeAdd(group string, name string, value float64, labels map[string]string) {
	c, err := v.RegisterGauge(name, labelspkg.LabelNames(labels))
	if err != nil {
		v.logger.Error(
			"GaugeAdd",
			slog.String("group", group),
			slog.String("name", name),
			slog.Any("labels", labels),
			log.Err(err),
		)

		return
	}

	c.Add(value, labels, collectors.WithGroup(group))
}

func (v *GroupedVault) HistogramObserve(group string, name string, value float64, labels map[string]string, buckets []float64) {
	c, err := v.RegisterHistogram(name, labelspkg.LabelNames(labels), buckets)
	if err != nil {
		v.logger.Error(
			"HistogramObserve",
			slog.String("group", group),
			slog.String("name", name),
			slog.Any("labels", labels),
			slog.Any("buckets", buckets),
			log.Err(err),
		)

		return
	}

	c.Observe(value, labels, collectors.WithGroup(group))
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
