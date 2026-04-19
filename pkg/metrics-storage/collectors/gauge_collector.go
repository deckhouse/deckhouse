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

package collectors

import (
	"sort"
	"sync"

	"github.com/prometheus/client_golang/prometheus"

	labelspkg "github.com/deckhouse/deckhouse/pkg/metrics-storage/labels"
)

var (
	_ ConstCollector = (*ConstGaugeCollector)(nil)
)

type GroupedGaugeMetric struct {
	Value       *MetricValue[float64]
	LabelValues []string
	Group       string
}

type ConstGaugeCollector struct {
	mtx sync.RWMutex

	name       string
	labelNames []string
	desc       *prometheus.Desc
	collection map[uint64]GroupedGaugeMetric
}

func NewConstGaugeCollector(desc *MetricDescription) *ConstGaugeCollector {
	d := prometheus.NewDesc(
		desc.Name,
		desc.Help,
		desc.LabelNames,
		prometheus.Labels(desc.ConstLabels),
	)

	return &ConstGaugeCollector{
		name:       desc.Name,
		labelNames: desc.LabelNames,
		desc:       d,
		collection: make(map[uint64]GroupedGaugeMetric),
	}
}

func (c *ConstGaugeCollector) Add(value float64, labels map[string]string, opts ...ConstCollectorOption) {
	c.mtx.Lock()
	defer c.mtx.Unlock()

	options := NewConstCollectorOptions(opts...)

	labelValues := labelspkg.LabelValues(labels, c.labelNames)
	metricHash := HashMetric(options.Group, labelValues)

	storedMetric, ok := c.collection[metricHash]
	if !ok {
		storedMetric = GroupedGaugeMetric{
			Value:       NewMetricValue(value),
			LabelValues: labelValues,
			Group:       options.Group,
		}
	} else {
		storedMetric.Value.Add(value)
	}

	c.collection[metricHash] = storedMetric
}

func (c *ConstGaugeCollector) Set(value float64, labels map[string]string, opts ...ConstCollectorOption) {
	c.mtx.Lock()
	defer c.mtx.Unlock()

	options := NewConstCollectorOptions(opts...)

	labelValues := labelspkg.LabelValues(labels, c.labelNames)
	metricHash := HashMetric(options.Group, labelValues)

	storedMetric, ok := c.collection[metricHash]
	if !ok {
		storedMetric = GroupedGaugeMetric{
			Value:       NewMetricValue(value),
			LabelValues: labelValues,
			Group:       options.Group,
		}
	}

	storedMetric.Value.Set(value)
	c.collection[metricHash] = storedMetric
}

func (c *ConstGaugeCollector) Describe(ch chan<- *prometheus.Desc) {
	ch <- c.desc
}

func (c *ConstGaugeCollector) Collect(ch chan<- prometheus.Metric) {
	c.mtx.RLock()
	defer c.mtx.RUnlock()

	for _, s := range c.collection {
		ch <- prometheus.MustNewConstMetric(c.desc, prometheus.GaugeValue, s.Value.Get(), s.LabelValues...)
	}
}

func (c *ConstGaugeCollector) Type() string {
	return "gauge"
}

func (c *ConstGaugeCollector) LabelNames() []string {
	return c.labelNames
}

func (c *ConstGaugeCollector) Name() string {
	return c.name
}

// ExpireGroupMetrics deletes all metrics from collection with matched group.
func (c *ConstGaugeCollector) ExpireGroupMetrics(group string) {
	c.mtx.Lock()
	defer c.mtx.Unlock()

	for hash, m := range c.collection {
		if m.Group == group {
			delete(c.collection, hash)
		}
	}
}

// UpdateLabels checks if any new labels are provided to the controller and updates its description, labelNames list and collection.
// The collection is recalculated in accordance with new label list.
func (c *ConstGaugeCollector) UpdateLabels(labels []string) {
	c.mtx.Lock()
	defer c.mtx.Unlock()

	// Create a map of current labels for quick lookup
	previousLabelsMap := make(map[string]int, len(c.labelNames))
	for idx, label := range c.labelNames {
		previousLabelsMap[label] = idx
	}

	// Check if we need to update labels
	var mustUpdate bool
	for _, label := range labels {
		if _, found := previousLabelsMap[label]; !found {
			mustUpdate = true
			c.labelNames = append(c.labelNames, label)
		}
	}

	// If no new labels, return early
	if !mustUpdate {
		return
	}

	// Sort labels for consistency
	sort.Strings(c.labelNames)

	// Create new description and collection with updated labels
	c.desc = prometheus.NewDesc(c.name, c.name, c.labelNames, nil)
	newCollection := make(map[uint64]GroupedGaugeMetric)

	// Update each metric in the collection
	for _, metric := range c.collection {
		newLabelsValues := make([]string, len(c.labelNames))

		for i, labelName := range c.labelNames {
			if idx, found := previousLabelsMap[labelName]; found && idx < len(metric.LabelValues) {
				newLabelsValues[i] = metric.LabelValues[idx]
			} else {
				newLabelsValues[i] = ""
			}
		}

		newLabelsHash := HashMetric(metric.Group, newLabelsValues)
		newCollection[newLabelsHash] = GroupedGaugeMetric{
			Value:       metric.Value,
			LabelValues: newLabelsValues,
			Group:       metric.Group,
		}
	}

	c.collection = newCollection
}
