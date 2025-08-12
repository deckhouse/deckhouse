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
	_ ConstCollector = (*ConstCounterCollector)(nil)
)

type GroupedCounterMetric struct {
	Value       *MetricValue[uint64]
	LabelValues []string
	Group       string
}

type ConstCounterCollector struct {
	mtx sync.RWMutex

	collection map[uint64]GroupedCounterMetric
	desc       *prometheus.Desc
	name       string
	labelNames []string
}

func NewConstCounterCollector(desc MetricDescription) *ConstCounterCollector {
	d := prometheus.NewDesc(
		desc.Name,
		desc.Help,
		desc.LabelNames,
		prometheus.Labels(desc.ConstLabels),
	)

	return &ConstCounterCollector{
		name:       desc.Name,
		labelNames: desc.LabelNames,
		desc:       d,
		collection: make(map[uint64]GroupedCounterMetric),
	}
}

// Add increases a counter metric by a value. Metric is identified by label values and a group.
func (c *ConstCounterCollector) Add(value float64, labels map[string]string, opts ...ConstCollectorOption) {
	c.mtx.Lock()
	defer c.mtx.Unlock()

	options := NewConstCollectorOptions(opts...)

	labelValues := labelspkg.LabelValues(labels, c.labelNames)
	metricHash := HashMetric(options.Group, labelValues)

	storedMetric, ok := c.collection[metricHash]
	if !ok {
		storedMetric = GroupedCounterMetric{
			Value:       NewMetricValue(uint64(value)),
			LabelValues: labelValues,
			Group:       options.Group,
		}
	} else {
		storedMetric.Value.Add(uint64(value))
	}

	c.collection[metricHash] = storedMetric
}

func (c *ConstCounterCollector) Describe(ch chan<- *prometheus.Desc) {
	ch <- c.desc
}

func (c *ConstCounterCollector) Collect(ch chan<- prometheus.Metric) {
	c.mtx.RLock()
	defer c.mtx.RUnlock()

	for _, s := range c.collection {
		ch <- prometheus.MustNewConstMetric(c.desc, prometheus.CounterValue, float64(s.Value.Get()), s.LabelValues...)
	}
}

func (c *ConstCounterCollector) Type() string {
	return "counter"
}

func (c *ConstCounterCollector) LabelNames() []string {
	return c.labelNames
}

func (c *ConstCounterCollector) Name() string {
	return c.name
}

// ExpireGroupMetrics deletes all metrics from collection with matched group.
func (c *ConstCounterCollector) ExpireGroupMetrics(group string) {
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
func (c *ConstCounterCollector) UpdateLabels(labels []string) {
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
	newCollection := make(map[uint64]GroupedCounterMetric)

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
		newCollection[newLabelsHash] = GroupedCounterMetric{
			Value:       metric.Value,
			LabelValues: newLabelsValues,
			Group:       metric.Group,
		}
	}

	c.collection = newCollection
}
