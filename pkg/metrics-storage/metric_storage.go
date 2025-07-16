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
	"fmt"
	"log/slog"
	"net/http"
	"strings"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	"github.com/deckhouse/deckhouse/pkg/log"

	"github.com/deckhouse/deckhouse/pkg/metrics-storage/collectors"
	labelspkg "github.com/deckhouse/deckhouse/pkg/metrics-storage/labels"
	"github.com/deckhouse/deckhouse/pkg/metrics-storage/operation"
	"github.com/deckhouse/deckhouse/pkg/metrics-storage/options"
	"github.com/deckhouse/deckhouse/pkg/metrics-storage/storage"
)

var _ Storage = (*MetricStorage)(nil)

const (
	PrefixTemplate = "{PREFIX}"
)

// TODO:
//  1. Add context support to retrieve Storage from context
//  2. Implement partial label support to avoid specifying all values for all labels
//     (retrieve default labels from context or use predefined defaults)
//  3. Improve prefix handling by using explicit options or builder functions instead of string substitution

// MetricStorage is used to register metric values.
type MetricStorage struct {
	Prefix string

	vault        *storage.Vault
	groupedVault *storage.GroupedVault

	registry   *prometheus.Registry
	gatherer   prometheus.Gatherer
	registerer prometheus.Registerer

	logger *log.Logger
}

// Option represents a MetricStorage option
type Option func(*MetricStorage)

// WithNewRegistry is an option to create a new prometheus registry
func WithNewRegistry() Option {
	return func(m *MetricStorage) {
		m.registry = prometheus.NewRegistry()
		m.gatherer = m.registry
		m.registerer = m.registry
	}
}

// WithRegistry is an option to set a custom prometheus registry
func WithRegistry(registry *prometheus.Registry) Option {
	return func(m *MetricStorage) {
		m.registry = registry
		m.gatherer = registry
		m.registerer = registry
	}
}

// WithLogger is an option to set a custom logger
func WithLogger(logger *log.Logger) Option {
	return func(m *MetricStorage) {
		m.logger = logger
	}
}

// NewMetricStorage creates and initializes a new MetricStorage instance with the specified prefix.
// It sets up the storage with empty maps for gauges, counters, and histograms, and the default Prometheus registerer.
// If you need to use a custom registerer, you can provide it through the WithRegisterer or WithNewRegisterer option.
//
// The MetricStorage provides a centralized way to manage Prometheus metrics with consistent naming
// and label handling. It supports counters, gauges, histograms, and grouped metrics operations.
//
// Parameters:
//   - prefix: A string prefix to apply to metric names. Can be referenced in metric names using {PREFIX}.
//   - opts: Optional configuration options that customize the behavior of the storage.
//     These are applied in order after the default configuration is established.
//
// Options include:
//   - WithNewRegistry: Creates a new isolated Prometheus registry for the metrics
//   - WithRegistry: Uses a provided Prometheus registry
//   - WithLogger: Sets a custom logger for the metrics storage
func NewMetricStorage(prefix string, opts ...Option) *MetricStorage {
	m := &MetricStorage{
		Prefix: prefix,

		gatherer:   prometheus.DefaultGatherer,
		registerer: prometheus.DefaultRegisterer,

		logger: log.NewLogger().Named("metrics-storage"),
	}

	// Apply provided options
	for _, opt := range opts {
		opt(m)
	}

	m.vault = storage.NewVault(
		m.resolveMetricName,
		options.WithRegistry(m.registry),
		options.WithLogger(m.logger.Named("vault")),
	)

	m.groupedVault = storage.NewGroupedVault(
		m.resolveMetricName,
		options.WithRegistry(m.registry),
		options.WithLogger(m.logger.Named("grouped-vault")),
	)

	return m
}

func (m *MetricStorage) Grouped() GroupedStorage {
	return m.groupedVault
}

func (m *MetricStorage) resolveMetricName(name string) string {
	if strings.Contains(name, PrefixTemplate) {
		return strings.Replace(name, PrefixTemplate, m.Prefix, 1)
	}

	return name
}

func (m *MetricStorage) RegisterCounter(metric string, labelNames []string, opts ...options.RegisterOption) (*collectors.ConstCounterCollector, error) {
	c, err := m.vault.RegisterCounter(metric, labelNames, opts...)
	if err != nil {
		return nil, err
	}

	return c, nil
}

func (m *MetricStorage) RegisterGauge(metric string, labelNames []string, opts ...options.RegisterOption) (*collectors.ConstGaugeCollector, error) {
	c, err := m.vault.RegisterGauge(metric, labelNames, opts...)
	if err != nil {
		return nil, err
	}

	return c, nil
}

func (m *MetricStorage) RegisterHistogram(metric string, labelNames []string, buckets []float64, opts ...options.RegisterOption) (*collectors.ConstHistogramCollector, error) {
	c, err := m.vault.RegisterHistogram(metric, labelNames, buckets, opts...)
	if err != nil {
		return nil, err
	}

	return c, nil
}

func (m *MetricStorage) CounterAdd(metric string, value float64, labels map[string]string) {
	if m == nil {
		return
	}

	m.Counter(metric, labels).Add(value, labels)
}

func (m *MetricStorage) GaugeSet(metric string, value float64, labels map[string]string) {
	if m == nil {
		return
	}

	m.vault.GaugeSet(metric, value, labels)
}

func (m *MetricStorage) GaugeAdd(metric string, value float64, labels map[string]string) {
	if m == nil {
		return
	}

	m.vault.GaugeAdd(metric, value, labels)
}

func (m *MetricStorage) HistogramObserve(metric string, value float64, labels map[string]string, buckets []float64) {
	if m == nil {
		return
	}

	m.Histogram(metric, labels, buckets).Observe(value, labels)
}

func (m *MetricStorage) Counter(metric string, labels map[string]string) *collectors.ConstCounterCollector {
	c, err := m.vault.RegisterCounter(metric, labelspkg.LabelNames(labels))
	if err != nil {
		m.logger.Error(
			"Counter",
			slog.String("name", metric),
			slog.Any("labels", labels),
			log.Err(err),
		)
	}

	return c
}

// Gauge return saved or register a new gauge.
func (m *MetricStorage) Gauge(metric string, labels map[string]string) *collectors.ConstGaugeCollector {
	c, err := m.vault.RegisterGauge(metric, labelspkg.LabelNames(labels))
	if err != nil {
		m.logger.Error(
			"Gauge",
			slog.String("name", metric),
			slog.Any("labels", labels),
			log.Err(err),
		)
	}

	return c
}

func (m *MetricStorage) Histogram(metric string, labels map[string]string, buckets []float64) *collectors.ConstHistogramCollector {
	c, err := m.vault.RegisterHistogram(metric, labelspkg.LabelNames(labels), buckets)
	if err != nil {
		m.logger.Error(
			"Histogram",
			slog.String("name", metric),
			slog.Any("labels", labels),
			log.Err(err),
		)
	}

	return c
}

// Batch operations for metrics from hooks

// ApplyBatchOperations processes a batch of metric operations with optional labels.
//
// This method handles metric operations in two categories:
//  1. Grouped operations - Operations with the same Group value are processed together.
//     All existing metrics in a group are expired before new operations are applied.
//  2. Non-grouped operations - Operations without a Group value are processed individually.
//
// The method first validates all operations, then organizes them by group,
// and finally applies them with the provided labels.
//
// Parameters:
//   - ops: A slice of MetricOperation objects to be applied
//   - labels: A map of string key-value pairs to be attached to all metrics
//
// If the MetricStorage receiver is nil, the method safely returns nil without performing any operations.
func (m *MetricStorage) ApplyBatchOperations(ops []operation.MetricOperation, labels map[string]string) error {
	if m == nil {
		return nil
	}

	err := operation.ValidateOperations(ops)
	if err != nil {
		return err
	}

	// Group operations by 'Group' value.
	groupedOps := make(map[string][]operation.MetricOperation)
	nonGroupedOps := make([]operation.MetricOperation, 0)

	for _, op := range ops {
		if op.Group == "" {
			nonGroupedOps = append(nonGroupedOps, op)
			continue
		}

		if _, ok := groupedOps[op.Group]; !ok {
			groupedOps[op.Group] = make([]operation.MetricOperation, 0)
		}

		groupedOps[op.Group] = append(groupedOps[op.Group], op)
	}

	// Expire each group and apply new metric operations.
	for group, ops := range groupedOps {
		m.applyGroupedOperations(group, ops, labels)
	}

	err = m.applyNonGroupedBatchOperations(nonGroupedOps, labels)
	if err != nil {
		return err
	}

	return nil
}

// ApplyOperation applies the specified metric operation to the metric storage.
//
// It processes different types of metric operations:
// - For ActionAdd: Increases a counter metric by the specified value
// - For ActionSet: Sets a gauge metric to the specified value
// - For ActionObserve: Records an observation in a histogram with the specified buckets
//
// The function merges operation-specific labels with common labels before applying the operation.
//
// Parameters:
// - op: The MetricOperation to apply (containing action type, metric name, value, labels, etc.)
// - commonLabels: Additional labels to apply to the metric
//
// If the MetricStorage instance is nil, the function returns without performing any operation.
func (m *MetricStorage) ApplyOperation(op operation.MetricOperation, commonLabels map[string]string) {
	if m == nil {
		return
	}

	labels := labelspkg.MergeLabels(op.Labels, commonLabels)

	if op.Action == operation.ActionAdd && op.Value != nil {
		m.CounterAdd(op.Name, *op.Value, labels)
		return
	}

	if op.Action == operation.ActionSet && op.Value != nil {
		m.GaugeSet(op.Name, *op.Value, labels)
		return
	}

	if op.Action == operation.ActionObserve && op.Value != nil && op.Buckets != nil {
		m.HistogramObserve(op.Name, *op.Value, labels, op.Buckets)
	}
}

// applyGroupedOperations processes a batch of MetricOperations that belong to the same group.
// It first expires all existing metrics for the group, then applies each operation,
// adding the provided common labels to each metric.
func (m *MetricStorage) applyGroupedOperations(group string, ops []operation.MetricOperation, commonLabels map[string]string) {
	if m == nil {
		return
	}

	// Implicitly expire all metrics for group.
	m.groupedVault.ExpireGroupMetrics(group)

	// Apply metric operations one-by-one.
	for _, op := range ops {
		if op.Action == operation.ActionExpire {
			m.groupedVault.ExpireGroupMetrics(group)
			continue
		}

		labels := labelspkg.MergeLabels(op.Labels, commonLabels)

		if op.Action == operation.ActionAdd && op.Value != nil {
			m.groupedVault.CounterAdd(group, op.Name, *op.Value, labels)
		}

		if op.Action == operation.ActionSet && op.Value != nil {
			m.groupedVault.GaugeSet(group, op.Name, *op.Value, labels)
		}
	}
}

// applyNonGroupedBatchOperations processes a batch of MetricOperations that are not grouped.
// It applies each operation in the batch to the metric storage, adding the provided labels to each metric.
func (m *MetricStorage) applyNonGroupedBatchOperations(ops []operation.MetricOperation, labels map[string]string) error {
	if m == nil {
		return nil
	}

	// Apply metric operations
	for _, metricOp := range ops {
		labels := labelspkg.MergeLabels(metricOp.Labels, labels)

		if metricOp.Action == operation.ActionAdd && metricOp.Value != nil {
			m.vault.CounterAdd(metricOp.Name, *metricOp.Value, labels)
			continue
		}

		if metricOp.Action == operation.ActionSet && metricOp.Value != nil {
			m.vault.GaugeSet(metricOp.Name, *metricOp.Value, labels)
			continue
		}

		if metricOp.Action == operation.ActionObserve && metricOp.Value != nil && metricOp.Buckets != nil {
			m.vault.HistogramObserve(metricOp.Name, *metricOp.Value, labels, metricOp.Buckets)
			continue
		}

		return fmt.Errorf("no operation in metric from module hook, name=%s", metricOp.Name)
	}

	return nil
}

// Collector returns collector of MetricStorage
// it can be useful to collect metrics in external registerer
func (m *MetricStorage) Collector() prometheus.Collector {
	if m.registry != nil {
		return m.registry
	}

	return m
}

// Handler returns handler of MetricStorage
// returns default prometheus handler if MetricStorage created without Registry options
func (m *MetricStorage) Handler() http.Handler {
	if m.registry == nil {
		return promhttp.Handler()
	}

	return promhttp.HandlerFor(m.registry, promhttp.HandlerOpts{
		Registry: m.registry,
	})
}

func (m *MetricStorage) Describe(ch chan<- *prometheus.Desc) {
	if m.vault != nil {
		m.vault.Collector().Describe(ch)
	}

	if m.groupedVault != nil {
		m.groupedVault.Collector().Describe(ch)
	}
}

func (m *MetricStorage) Collect(ch chan<- prometheus.Metric) {
	if m.vault != nil {
		m.vault.Collector().Collect(ch)
	}

	if m.groupedVault != nil {
		m.groupedVault.Collector().Collect(ch)
	}
}
