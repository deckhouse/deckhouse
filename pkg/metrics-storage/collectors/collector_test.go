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

package collectors_test

import (
	"sort"
	"strconv"
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/deckhouse/deckhouse/pkg/metrics-storage/collectors"
)

// TestConstCounterCollectorCreation tests the creation of a counter collector
func TestConstCounterCollectorCreation(t *testing.T) {
	// Create a counter collector with some label names
	labelNames := []string{"label1", "label2"}
	collector := collectors.NewConstCounterCollector("test_counter", labelNames)

	// Verify the collector was created correctly
	assert.NotNil(t, collector)
	assert.Equal(t, "test_counter", collector.Name())
	assert.Equal(t, "counter", collector.Type())
	assert.ElementsMatch(t, labelNames, collector.LabelNames())
}

// TestConstGaugeCollectorCreation tests the creation of a gauge collector
func TestConstGaugeCollectorCreation(t *testing.T) {
	// Create a gauge collector with some label names
	labelNames := []string{"label1", "label2"}
	collector := collectors.NewConstGaugeCollector("test_gauge", labelNames)

	// Verify the collector was created correctly
	assert.NotNil(t, collector)
	assert.Equal(t, "test_gauge", collector.Name())
	assert.Equal(t, "gauge", collector.Type())
	assert.ElementsMatch(t, labelNames, collector.LabelNames())
}

// TestConstCounterCollectorAdd tests the Add method of counter collector
func TestConstCounterCollectorAdd(t *testing.T) {
	// Create a counter collector
	labelNames := []string{"label1", "label2"}
	collector := collectors.NewConstCounterCollector("test_counter", labelNames)

	// Add some metrics
	group := "test_group"
	collector.Add(group, 10, map[string]string{"label1": "value1", "label2": "value2"})
	collector.Add(group, 5, map[string]string{"label1": "value1", "label2": "value2"})

	// Collect the metrics
	metrics := collectMetrics(collector)

	// Verify the metrics
	require.Len(t, metrics, 1)
	verifyCounterValue(t, metrics[0], 15)
	verifyLabels(t, metrics[0], map[string]string{"label1": "value1", "label2": "value2"})
}

// TestConstCounterCollectorAddMultipleGroups tests adding metrics with different groups
func TestConstCounterCollectorAddMultipleGroups(t *testing.T) {
	// Create a counter collector
	labelNames := []string{"label1", "label2"}
	collector := collectors.NewConstCounterCollector("test_counter", labelNames)

	// Add metrics with different groups but same labels
	group1 := "group1"
	group2 := "group2"
	collector.Add(group1, 10, map[string]string{"label1": "value1", "label2": "value2"})
	collector.Add(group2, 5, map[string]string{"label1": "value1", "label2": "value2"})

	// Since the labels are the same, we should end up with a single metric
	// This tests that groups don't affect the metric identity for collection
	metrics := collectMetrics(collector)
	require.Len(t, metrics, 1)
	verifyCounterValue(t, metrics[0], 15)
}

// TestConstGaugeCollectorSet tests the Set method of gauge collector
func TestConstGaugeCollectorSet(t *testing.T) {
	// Create a gauge collector
	labelNames := []string{"label1", "label2"}
	collector := collectors.NewConstGaugeCollector("test_gauge", labelNames)

	// Set some metrics
	group := "test_group"
	collector.Set(group, 10, map[string]string{"label1": "value1", "label2": "value2"})

	// Verify the initial value
	metrics := collectMetrics(collector)
	require.Len(t, metrics, 1)
	verifyGaugeValue(t, metrics[0], 10)

	// Update the value
	collector.Set(group, 20, map[string]string{"label1": "value1", "label2": "value2"})

	// Verify the updated value
	metrics = collectMetrics(collector)
	require.Len(t, metrics, 1)
	verifyGaugeValue(t, metrics[0], 20)
}

// TestConstGaugeCollectorSetMultipleLabels tests setting metrics with different label values
func TestConstGaugeCollectorSetMultipleLabels(t *testing.T) {
	// Create a gauge collector
	labelNames := []string{"label1", "label2"}
	collector := collectors.NewConstGaugeCollector("test_gauge", labelNames)

	// Set metrics with different labels
	group := "test_group"
	collector.Set(group, 10, map[string]string{"label1": "value1", "label2": "value2"})
	collector.Set(group, 20, map[string]string{"label1": "value3", "label2": "value4"})

	// Verify we have two metrics
	metrics := collectMetrics(collector)
	require.Len(t, metrics, 2)

	// Sort metrics by value for deterministic testing
	sort.Slice(metrics, func(i, j int) bool {
		return getMetricValue(t, metrics[i]) < getMetricValue(t, metrics[j])
	})

	// Verify the values and labels
	verifyGaugeValue(t, metrics[0], 10)
	verifyLabels(t, metrics[0], map[string]string{"label1": "value1", "label2": "value2"})
	verifyGaugeValue(t, metrics[1], 20)
	verifyLabels(t, metrics[1], map[string]string{"label1": "value3", "label2": "value4"})
}

// TestExpireGroupMetricsCounter tests the ExpireGroupMetrics method of counter collector
func TestExpireGroupMetricsCounter(t *testing.T) {
	// Create a counter collector
	labelNames := []string{"label1", "label2"}
	collector := collectors.NewConstCounterCollector("test_counter", labelNames)

	// Add metrics with different groups
	group1 := "group1"
	group2 := "group2"
	collector.Add(group1, 10, map[string]string{"label1": "value1", "label2": "value2"})
	collector.Add(group2, 20, map[string]string{"label1": "value3", "label2": "value4"})

	// Verify both metrics exist
	metrics := collectMetrics(collector)
	require.Len(t, metrics, 2)

	// Expire one group
	collector.ExpireGroupMetrics(group1)

	// Verify only the other group's metric remains
	metrics = collectMetrics(collector)
	require.Len(t, metrics, 1)
	verifyCounterValue(t, metrics[0], 20)
	verifyLabels(t, metrics[0], map[string]string{"label1": "value3", "label2": "value4"})
}

// TestExpireGroupMetricsGauge tests the ExpireGroupMetrics method of gauge collector
func TestExpireGroupMetricsGauge(t *testing.T) {
	// Create a gauge collector
	labelNames := []string{"label1", "label2"}
	collector := collectors.NewConstGaugeCollector("test_gauge", labelNames)

	// Set metrics with different groups
	group1 := "group1"
	group2 := "group2"
	collector.Set(group1, 10, map[string]string{"label1": "value1", "label2": "value2"})
	collector.Set(group2, 20, map[string]string{"label1": "value3", "label2": "value4"})

	// Verify both metrics exist
	metrics := collectMetrics(collector)
	require.Len(t, metrics, 2)

	// Expire one group
	collector.ExpireGroupMetrics(group1)

	// Verify only the other group's metric remains
	metrics = collectMetrics(collector)
	require.Len(t, metrics, 1)
	verifyGaugeValue(t, metrics[0], 20)
	verifyLabels(t, metrics[0], map[string]string{"label1": "value3", "label2": "value4"})
}

// TestUpdateLabelsCounter tests the UpdateLabels method of counter collector
func TestUpdateLabelsCounter(t *testing.T) {
	// Create a counter collector with initial labels
	initialLabelNames := []string{"label1", "label2"}
	collector := collectors.NewConstCounterCollector("test_counter", initialLabelNames)

	// Add a metric
	group := "test_group"
	collector.Add(group, 10, map[string]string{"label1": "value1", "label2": "value2"})

	// Verify the initial state
	metrics := collectMetrics(collector)
	require.Len(t, metrics, 1)
	verifyCounterValue(t, metrics[0], 10)
	verifyLabels(t, metrics[0], map[string]string{"label1": "value1", "label2": "value2"})

	// Update the labels to add a new one
	newLabelNames := []string{"label1", "label2", "label3"}
	collector.UpdateLabels(newLabelNames)

	// Verify the updated state
	assert.ElementsMatch(t, newLabelNames, collector.LabelNames())

	// Collect the metrics and verify
	metrics = collectMetrics(collector)
	require.Len(t, metrics, 1)
	verifyCounterValue(t, metrics[0], 10)

	// The new label should be added with an empty value
	labels := extractLabels(t, metrics[0])
	assert.Equal(t, "value1", labels["label1"])
	assert.Equal(t, "value2", labels["label2"])
	assert.Equal(t, "", labels["label3"])
}

// TestUpdateLabelsGauge tests the UpdateLabels method of gauge collector
func TestUpdateLabelsGauge(t *testing.T) {
	// Create a gauge collector with initial labels
	initialLabelNames := []string{"label1", "label2"}
	collector := collectors.NewConstGaugeCollector("test_gauge", initialLabelNames)

	// Set a metric
	group := "test_group"
	collector.Set(group, 10, map[string]string{"label1": "value1", "label2": "value2"})

	// Verify the initial state
	metrics := collectMetrics(collector)
	require.Len(t, metrics, 1)
	verifyGaugeValue(t, metrics[0], 10)
	verifyLabels(t, metrics[0], map[string]string{"label1": "value1", "label2": "value2"})

	// Update the labels to add a new one
	newLabelNames := []string{"label1", "label2", "label3"}
	collector.UpdateLabels(newLabelNames)

	// Verify the updated state
	assert.ElementsMatch(t, newLabelNames, collector.LabelNames())

	// Collect the metrics and verify
	metrics = collectMetrics(collector)
	require.Len(t, metrics, 1)
	verifyGaugeValue(t, metrics[0], 10)

	// The new label should be added with an empty value
	labels := extractLabels(t, metrics[0])
	assert.Equal(t, "value1", labels["label1"])
	assert.Equal(t, "value2", labels["label2"])
	assert.Equal(t, "", labels["label3"])
}

// TestHashLabelValues tests the HashLabelValues function
func TestHashLabelValues(t *testing.T) {
	// Test that same label values produce the same hash
	values1 := []string{"value1", "value2"}
	values2 := []string{"value1", "value2"}
	hash1 := collectors.HashLabelValues(values1)
	hash2 := collectors.HashLabelValues(values2)
	assert.Equal(t, hash1, hash2)

	// Test that different label values produce different hashes
	values3 := []string{"value3", "value4"}
	hash3 := collectors.HashLabelValues(values3)
	assert.NotEqual(t, hash1, hash3)

	// Test that order matters
	values4 := []string{"value2", "value1"}
	hash4 := collectors.HashLabelValues(values4)
	assert.NotEqual(t, hash1, hash4)

	// Test empty values
	values5 := []string{"", ""}
	hash5 := collectors.HashLabelValues(values5)
	assert.NotEqual(t, hash1, hash5)

	// Test with many values to ensure unique hashing
	uniqueHashes := make(map[uint64]struct{})
	for i := 0; i < 1000; i++ {
		values := []string{strconv.Itoa(i), "constant"}
		hash := collectors.HashLabelValues(values)
		uniqueHashes[hash] = struct{}{}
	}
	assert.Equal(t, 1000, len(uniqueHashes))
}

// TestEdgeCases tests various edge cases
func TestEdgeCases(t *testing.T) {
	// Test with empty label names
	counterCollector := collectors.NewConstCounterCollector("test_counter", []string{})
	gaugeCollector := collectors.NewConstGaugeCollector("test_gauge", []string{})

	// Add metrics with empty labels
	counterCollector.Add("group", 10, map[string]string{})
	gaugeCollector.Set("group", 20, map[string]string{})

	// Verify the metrics
	counterMetrics := collectMetrics(counterCollector)
	gaugeMetrics := collectMetrics(gaugeCollector)

	require.Len(t, counterMetrics, 1)
	require.Len(t, gaugeMetrics, 1)
	verifyCounterValue(t, counterMetrics[0], 10)
	verifyGaugeValue(t, gaugeMetrics[0], 20)

	// Test with nil label values map
	counterCollector.Add("group", 5, nil)
	gaugeCollector.Set("group", 25, nil)

	// Verify the metrics
	counterMetrics = collectMetrics(counterCollector)
	gaugeMetrics = collectMetrics(gaugeCollector)

	require.Len(t, counterMetrics, 1)
	require.Len(t, gaugeMetrics, 1)
	verifyCounterValue(t, counterMetrics[0], 15)
	verifyGaugeValue(t, gaugeMetrics[0], 25)

	// Test with empty groups
	counterCollector.Add("", 10, map[string]string{})
	gaugeCollector.Set("", 20, map[string]string{})

	// Verify the metrics
	counterMetrics = collectMetrics(counterCollector)
	gaugeMetrics = collectMetrics(gaugeCollector)

	require.Len(t, counterMetrics, 1)
	require.Len(t, gaugeMetrics, 1)

	// Test expiring empty group
	counterCollector.ExpireGroupMetrics("")
	gaugeCollector.ExpireGroupMetrics("")

	// Verify the metrics
	counterMetrics = collectMetrics(counterCollector)
	gaugeMetrics = collectMetrics(gaugeCollector)

	require.Len(t, counterMetrics, 1)
	require.Len(t, gaugeMetrics, 1)
}

// TestUpdateLabelsNoChange tests that UpdateLabels doesn't change anything when no new labels are added
func TestUpdateLabelsNoChange(t *testing.T) {
	// Create collectors
	labelNames := []string{"label1", "label2"}
	counterCollector := collectors.NewConstCounterCollector("test_counter", labelNames)
	gaugeCollector := collectors.NewConstGaugeCollector("test_gauge", labelNames)

	// Add metrics
	counterCollector.Add("group", 10, map[string]string{"label1": "value1", "label2": "value2"})
	gaugeCollector.Set("group", 20, map[string]string{"label1": "value1", "label2": "value2"})

	// Verify initial state
	counterMetrics := collectMetrics(counterCollector)
	gaugeMetrics := collectMetrics(gaugeCollector)
	require.Len(t, counterMetrics, 1)
	require.Len(t, gaugeMetrics, 1)

	// Update labels with the same labels (just in different order)
	counterCollector.UpdateLabels([]string{"label2", "label1"})
	gaugeCollector.UpdateLabels([]string{"label2", "label1"})

	// Verify nothing changed
	assert.ElementsMatch(t, labelNames, counterCollector.LabelNames())
	assert.ElementsMatch(t, labelNames, gaugeCollector.LabelNames())

	counterMetrics = collectMetrics(counterCollector)
	gaugeMetrics = collectMetrics(gaugeCollector)
	require.Len(t, counterMetrics, 1)
	require.Len(t, gaugeMetrics, 1)
	verifyCounterValue(t, counterMetrics[0], 10)
	verifyGaugeValue(t, gaugeMetrics[0], 20)
}

// TestDescribeMethod tests the Describe method of collectors
func TestDescribeMethod(t *testing.T) {
	// Create collectors
	counterCollector := collectors.NewConstCounterCollector("test_counter", []string{"label"})
	gaugeCollector := collectors.NewConstGaugeCollector("test_gauge", []string{"label"})

	// Create channels for descriptions
	counterCh := make(chan *prometheus.Desc, 1)
	gaugeCh := make(chan *prometheus.Desc, 1)

	// Call Describe
	counterCollector.Describe(counterCh)
	gaugeCollector.Describe(gaugeCh)

	// Verify we got descriptions
	assert.Len(t, counterCh, 1)
	assert.Len(t, gaugeCh, 1)

	counterDesc := <-counterCh
	gaugeDesc := <-gaugeCh

	assert.NotNil(t, counterDesc)
	assert.NotNil(t, gaugeDesc)
}

// Helper functions

// collectMetrics collects all metrics from a collector into a slice
func collectMetrics(collector collectors.ConstCollector) []prometheus.Metric {
	var metrics []prometheus.Metric
	ch := make(chan prometheus.Metric)

	go func() {
		collector.Collect(ch)
		close(ch)
	}()

	for metric := range ch {
		metrics = append(metrics, metric)
	}

	return metrics
}

// getMetricValue extracts the value from a metric
func getMetricValue(t *testing.T, metric prometheus.Metric) float64 {
	pb := &dto.Metric{}
	err := metric.Write(pb)
	require.NoError(t, err)

	switch {
	case pb.Gauge != nil:
		return pb.Gauge.GetValue()
	case pb.Counter != nil:
		return pb.Counter.GetValue()
	case pb.Untyped != nil:
		return pb.Untyped.GetValue()
	default:
		t.Fatalf("Unknown metric type")
		return 0
	}
}

// verifyCounterValue verifies that a metric has the expected counter value
func verifyCounterValue(t *testing.T, metric prometheus.Metric, expected float64) {
	pb := &dto.Metric{}
	err := metric.Write(pb)
	require.NoError(t, err)
	require.NotNil(t, pb.Counter, "Metric is not a counter")
	assert.Equal(t, expected, pb.Counter.GetValue())
}

// verifyGaugeValue verifies that a metric has the expected gauge value
func verifyGaugeValue(t *testing.T, metric prometheus.Metric, expected float64) {
	pb := &dto.Metric{}
	err := metric.Write(pb)
	require.NoError(t, err)
	require.NotNil(t, pb.Gauge, "Metric is not a gauge")
	assert.Equal(t, expected, pb.Gauge.GetValue())
}

// verifyLabels verifies that a metric has the expected labels
func verifyLabels(t *testing.T, metric prometheus.Metric, expected map[string]string) {
	labels := extractLabels(t, metric)
	assert.Equal(t, expected, labels)
}

// extractLabels extracts labels from a metric as a map
func extractLabels(t *testing.T, metric prometheus.Metric) map[string]string {
	pb := &dto.Metric{}
	err := metric.Write(pb)
	require.NoError(t, err)

	result := make(map[string]string)
	for _, labelPair := range pb.Label {
		result[labelPair.GetName()] = labelPair.GetValue()
	}
	return result
}
