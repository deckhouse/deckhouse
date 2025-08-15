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
	_ ConstCollector = (*ConstHistogramCollector)(nil)
)

type GroupedHistogramMetric struct {
	Buckets     []uint64              // Count of observations in each bucket
	Sum         *MetricValue[float64] // Sum of all observed values
	Count       *MetricValue[uint64]  // Total count of observations
	LabelValues []string
	Group       string
}

type ConstHistogramCollector struct {
	mtx sync.RWMutex

	name       string
	labelNames []string
	desc       *prometheus.Desc
	buckets    []float64
	collection map[uint64]GroupedHistogramMetric
	bucketsMtx sync.RWMutex
}

func NewConstHistogramCollector(desc *MetricDescription, buckets []float64) *ConstHistogramCollector {
	if len(buckets) == 0 {
		buckets = prometheus.DefBuckets
	}

	// Ensure buckets are sorted
	sortedBuckets := make([]float64, len(buckets))
	copy(sortedBuckets, buckets)
	sort.Float64s(sortedBuckets)

	d := prometheus.NewDesc(
		desc.Name,
		desc.Help,
		desc.LabelNames,
		prometheus.Labels(desc.ConstLabels),
	)

	return &ConstHistogramCollector{
		name:       desc.Name,
		labelNames: desc.LabelNames,
		desc:       d,
		buckets:    sortedBuckets,
		collection: make(map[uint64]GroupedHistogramMetric),
	}
}

func (c *ConstHistogramCollector) Observe(value float64, labels map[string]string, opts ...ConstCollectorOption) {
	c.mtx.Lock()
	defer c.mtx.Unlock()

	options := NewConstCollectorOptions(opts...)

	labelValues := labelspkg.LabelValues(labels, c.labelNames)
	metricHash := HashMetric(options.Group, labelValues)

	// Get or create metric
	storedMetric, ok := c.collection[metricHash]
	if !ok {
		storedMetric = GroupedHistogramMetric{
			Buckets:     make([]uint64, len(c.buckets)),
			Sum:         NewMetricValue(0.0),
			Count:       NewMetricValue(uint64(0)),
			LabelValues: labelValues,
			Group:       options.Group,
		}
	}

	// Update sum and count
	storedMetric.Sum.Add(value)
	storedMetric.Count.Add(1)

	// If value is negative, we do not count it in buckets
	if value < 0 {
		c.collection[metricHash] = storedMetric
		return
	}

	// Update buckets
	for i, upperBound := range c.buckets {
		if value <= upperBound {
			storedMetric.Buckets[i]++
		}
	}

	c.collection[metricHash] = storedMetric
}

func (c *ConstHistogramCollector) Describe(ch chan<- *prometheus.Desc) {
	ch <- c.desc
}

func (c *ConstHistogramCollector) Collect(ch chan<- prometheus.Metric) {
	c.mtx.RLock()
	defer c.mtx.RUnlock()

	for _, metric := range c.collection {
		// Convert uint64 buckets to map[float64]uint64 for prometheus
		buckets := make(map[float64]uint64)
		for i, bucket := range c.buckets {
			buckets[bucket] = metric.Buckets[i]
		}

		// Create histogram metric
		h := prometheus.MustNewConstHistogram(
			c.desc,
			metric.Count.Get(),
			metric.Sum.Get(),
			buckets,
			metric.LabelValues...,
		)
		ch <- h
	}
}

func (c *ConstHistogramCollector) Type() string {
	return "histogram"
}

func (c *ConstHistogramCollector) LabelNames() []string {
	c.mtx.RLock()
	defer c.mtx.RUnlock()
	return c.labelNames
}

func (c *ConstHistogramCollector) Name() string {
	return c.name
}

func (c *ConstHistogramCollector) Buckets() []float64 {
	c.bucketsMtx.RLock()
	defer c.bucketsMtx.RUnlock()

	result := make([]float64, len(c.buckets))
	copy(result, c.buckets)
	return result
}

func (c *ConstHistogramCollector) UpdateBuckets(buckets []float64) {
	c.mtx.Lock()
	c.bucketsMtx.Lock()
	defer c.mtx.Unlock()
	defer c.bucketsMtx.Unlock()

	if len(buckets) == 0 {
		return
	}

	// Sort new buckets
	sortedBuckets := make([]float64, len(buckets))
	copy(sortedBuckets, buckets)
	sort.Float64s(sortedBuckets)

	// Check if buckets actually changed
	if len(sortedBuckets) == len(c.buckets) {
		same := true
		for i, bucket := range sortedBuckets {
			if bucket != c.buckets[i] {
				same = false
				break
			}
		}
		if same {
			return
		}
	}

	// Update buckets
	oldBuckets := c.buckets
	c.buckets = sortedBuckets

	// Rebuild collection with new bucket structure
	newCollection := make(map[uint64]GroupedHistogramMetric)
	for hash, metric := range c.collection {
		newMetric := GroupedHistogramMetric{
			Buckets:     make([]uint64, len(c.buckets)),
			Sum:         metric.Sum,
			Count:       metric.Count,
			LabelValues: metric.LabelValues,
			Group:       metric.Group,
		}

		// Recalculate bucket counts for new bucket boundaries
		// This is an approximation - we lose some precision when changing buckets
		for i, newBucket := range c.buckets {
			for j, oldBucket := range oldBuckets {
				if j < len(metric.Buckets) && newBucket >= oldBucket {
					if i == 0 || (i > 0 && newBucket > c.buckets[i-1]) {
						newMetric.Buckets[i] += metric.Buckets[j]
					}
				}
			}
		}

		newCollection[hash] = newMetric
	}

	c.collection = newCollection
}

// ExpireGroupMetrics deletes all metrics from collection with matched group.
func (c *ConstHistogramCollector) ExpireGroupMetrics(group string) {
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
func (c *ConstHistogramCollector) UpdateLabels(labels []string) {
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
	newCollection := make(map[uint64]GroupedHistogramMetric)

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
		newCollection[newLabelsHash] = GroupedHistogramMetric{
			Buckets:     metric.Buckets,
			Sum:         metric.Sum,
			Count:       metric.Count,
			LabelValues: newLabelsValues,
			Group:       metric.Group,
		}
	}

	c.collection = newCollection
}

// Reset clears all observations for a specific group and labels combination
func (c *ConstHistogramCollector) Reset(labels map[string]string, opts ...ConstCollectorOption) {
	c.mtx.Lock()
	defer c.mtx.Unlock()

	options := NewConstCollectorOptions(opts...)

	labelValues := labelspkg.LabelValues(labels, c.labelNames)
	metricHash := HashMetric(options.Group, labelValues)

	if metric, ok := c.collection[metricHash]; ok && metric.Group == options.Group {
		// Reset all buckets to zero
		for i := range metric.Buckets {
			metric.Buckets[i] = 0
		}
		metric.Sum.Set(0.0)
		metric.Count.Set(0)
		c.collection[metricHash] = metric
	}
}

// GetObservationCount returns the total number of observations for a specific metric
func (c *ConstHistogramCollector) GetObservationCount(labels map[string]string, opts ...ConstCollectorOption) uint64 {
	c.mtx.RLock()
	defer c.mtx.RUnlock()

	options := NewConstCollectorOptions(opts...)

	labelValues := labelspkg.LabelValues(labels, c.labelNames)
	metricHash := HashMetric(options.Group, labelValues)

	if metric, ok := c.collection[metricHash]; ok && metric.Group == options.Group {
		return metric.Count.Get()
	}
	return 0
}

// GetSum returns the sum of all observations for a specific metric
func (c *ConstHistogramCollector) GetSum(labels map[string]string, opts ...ConstCollectorOption) float64 {
	c.mtx.RLock()
	defer c.mtx.RUnlock()

	options := NewConstCollectorOptions(opts...)

	labelValues := labelspkg.LabelValues(labels, c.labelNames)
	metricHash := HashMetric(options.Group, labelValues)

	if metric, ok := c.collection[metricHash]; ok && metric.Group == options.Group {
		return metric.Sum.Get()
	}
	return 0.0
}
