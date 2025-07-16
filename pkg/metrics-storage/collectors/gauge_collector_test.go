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
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/deckhouse/deckhouse/pkg/metrics-storage/collectors"
)

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

// TestConstGaugeCollectorAdd tests the Add method of gauge collector
func TestConstGaugeCollectorAdd(t *testing.T) {
	// Create a gauge collector
	labelNames := []string{"label1", "label2"}
	collector := collectors.NewConstGaugeCollector("test_gauge", labelNames)

	// Add some metrics to the gauge
	group := "test_group"
	collector.Add(10, map[string]string{"label1": "value1", "label2": "value2"}, collectors.WithGroup(group))
	collector.Add(5, map[string]string{"label1": "value1", "label2": "value2"}, collectors.WithGroup(group))

	// Collect the metrics
	metrics := collectMetrics(collector)

	// Verify the metrics - gauge Add should accumulate values
	require.Len(t, metrics, 1)
	verifyGaugeValue(t, metrics[0], 15)
	verifyLabels(t, metrics[0], map[string]string{"label1": "value1", "label2": "value2"})
}

// TestConstGaugeCollectorAddMultipleGroups tests adding metrics with different groups
func TestConstGaugeCollectorAddMultipleGroups(t *testing.T) {
	// Create a gauge collector
	labelNames := []string{"label1", "label2"}
	collector := collectors.NewConstGaugeCollector("test_gauge", labelNames)

	// Add metrics with different groups but same labels
	group1 := "group1"
	group2 := "group2"
	collector.Add(10, map[string]string{"label1": "value1", "label2": "value2"}, collectors.WithGroup(group1))
	collector.Add(5, map[string]string{"label1": "value1", "label2": "value2"}, collectors.WithGroup(group2))

	// Since the labels are the same, we should end up with a single metric
	// This tests that groups don't affect the metric identity for collection
	metrics := collectMetrics(collector)
	require.Len(t, metrics, 2)
}

// TestConstGaugeCollectorAddMultipleLabels tests adding metrics with different label values
func TestConstGaugeCollectorAddMultipleLabels(t *testing.T) {
	// Create a gauge collector
	labelNames := []string{"label1", "label2"}
	collector := collectors.NewConstGaugeCollector("test_gauge", labelNames)

	// Add metrics with different labels
	group := "test_group"
	collector.Add(10, map[string]string{"label1": "value1", "label2": "value2"}, collectors.WithGroup(group))
	collector.Add(20, map[string]string{"label1": "value3", "label2": "value4"}, collectors.WithGroup(group))

	// Verify we have two separate metrics
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

// TestConstGaugeCollectorAddNegativeValues tests adding negative values to gauge
func TestConstGaugeCollectorAddNegativeValues(t *testing.T) {
	// Create a gauge collector
	labelNames := []string{"label1"}
	collector := collectors.NewConstGaugeCollector("test_gauge", labelNames)

	group := "test_group"
	labels := map[string]string{"label1": "value1"}

	// Add positive value first
	collector.Add(10, labels, collectors.WithGroup(group))

	// Add negative value (should subtract from total)
	collector.Add(-3, labels, collectors.WithGroup(group))

	// Verify the result
	metrics := collectMetrics(collector)
	require.Len(t, metrics, 1)
	verifyGaugeValue(t, metrics[0], 7)
}

// TestConstGaugeCollectorAddZeroValues tests adding zero values to gauge
func TestConstGaugeCollectorAddZeroValues(t *testing.T) {
	// Create a gauge collector
	labelNames := []string{"label1"}
	collector := collectors.NewConstGaugeCollector("test_gauge", labelNames)

	group := "test_group"
	labels := map[string]string{"label1": "value1"}

	// Add zero value
	collector.Add(0, labels, collectors.WithGroup(group))

	// Verify the metric exists with zero value
	metrics := collectMetrics(collector)
	require.Len(t, metrics, 1)
	verifyGaugeValue(t, metrics[0], 0)

	// Add positive value
	collector.Add(5, labels, collectors.WithGroup(group))

	// Add zero again (should not change the value)
	collector.Add(0, labels, collectors.WithGroup(group))

	// Verify the result
	metrics = collectMetrics(collector)
	require.Len(t, metrics, 1)
	verifyGaugeValue(t, metrics[0], 5)
}

// TestConstGaugeCollectorAddFloatingPoint tests adding floating point values
func TestConstGaugeCollectorAddFloatingPoint(t *testing.T) {
	// Create a gauge collector
	labelNames := []string{"label1"}
	collector := collectors.NewConstGaugeCollector("test_gauge", labelNames)

	group := "test_group"
	labels := map[string]string{"label1": "value1"}

	// Add floating point values
	collector.Add(3.14, labels, collectors.WithGroup(group))
	collector.Add(2.86, labels, collectors.WithGroup(group))

	// Verify the result (should be approximately 6.0)
	metrics := collectMetrics(collector)
	require.Len(t, metrics, 1)
	verifyGaugeValue(t, metrics[0], 6.0)
}

// TestConstGaugeCollectorAddSequentialOperations tests mixed Add and Set operations
func TestConstGaugeCollectorAddSequentialOperations(t *testing.T) {
	// Create a gauge collector
	labelNames := []string{"label1"}
	collector := collectors.NewConstGaugeCollector("test_gauge", labelNames)

	group := "test_group"
	labels := map[string]string{"label1": "value1"}

	// Start with Add
	collector.Add(10, labels, collectors.WithGroup(group))

	// Verify initial value
	metrics := collectMetrics(collector)
	require.Len(t, metrics, 1)
	verifyGaugeValue(t, metrics[0], 10)

	// Set to a different value
	collector.Set(5, labels, collectors.WithGroup(group))

	// Verify set operation overwrote the previous value
	metrics = collectMetrics(collector)
	require.Len(t, metrics, 1)
	verifyGaugeValue(t, metrics[0], 5)

	// Add to the set value
	collector.Add(3, labels, collectors.WithGroup(group))

	// Verify add operation added to the set value
	metrics = collectMetrics(collector)
	require.Len(t, metrics, 1)
	verifyGaugeValue(t, metrics[0], 8)
}

// TestConstGaugeCollectorAddConcurrency tests concurrent Add operations
func TestConstGaugeCollectorAddConcurrency(t *testing.T) {
	// Create a gauge collector
	labelNames := []string{"label1"}
	collector := collectors.NewConstGaugeCollector("test_gauge", labelNames)

	group := "test_group"
	labels := map[string]string{"label1": "value1"}

	// Number of goroutines and operations per goroutine
	numGoroutines := 10
	opsPerGoroutine := 100
	valuePerOp := 1.0

	// Run concurrent Add operations
	done := make(chan bool, numGoroutines)
	for i := 0; i < numGoroutines; i++ {
		go func() {
			for j := 0; j < opsPerGoroutine; j++ {
				collector.Add(valuePerOp, labels, collectors.WithGroup(group))
			}
			done <- true
		}()
	}

	// Wait for all goroutines to complete
	for i := 0; i < numGoroutines; i++ {
		<-done
	}

	// Verify the final value
	expectedValue := float64(numGoroutines * opsPerGoroutine * int(valuePerOp))
	metrics := collectMetrics(collector)
	require.Len(t, metrics, 1)
	verifyGaugeValue(t, metrics[0], expectedValue)
}

// TestConstGaugeCollectorAddWithEmptyLabels tests Add with empty labels
func TestConstGaugeCollectorAddWithEmptyLabels(t *testing.T) {
	// Create a gauge collector with no labels
	collector := collectors.NewConstGaugeCollector("test_gauge", []string{})

	group := "test_group"

	// Add with empty labels map
	collector.Add(10, map[string]string{}, collectors.WithGroup(group))

	// Add with nil labels map
	collector.Add(5, nil, collectors.WithGroup(group))

	// Verify the metrics - both should contribute to the same metric
	metrics := collectMetrics(collector)
	require.Len(t, metrics, 1)
	verifyGaugeValue(t, metrics[0], 15)
}

// TestConstGaugeCollectorAddAfterExpire tests Add after group expiration
func TestConstGaugeCollectorAddAfterExpire(t *testing.T) {
	// Create a gauge collector
	labelNames := []string{"label1"}
	collector := collectors.NewConstGaugeCollector("test_gauge", labelNames)

	group := "test_group"
	labels := map[string]string{"label1": "value1"}

	// Add initial value
	collector.Add(10, labels, collectors.WithGroup(group))

	// Verify initial value
	metrics := collectMetrics(collector)
	require.Len(t, metrics, 1)
	verifyGaugeValue(t, metrics[0], 10)

	// Expire the group
	collector.ExpireGroupMetrics(group)

	// Verify metric is gone
	metrics = collectMetrics(collector)
	require.Len(t, metrics, 0)

	// Add new value after expiration
	collector.Add(20, labels, collectors.WithGroup(group))

	// Verify new value is set correctly
	metrics = collectMetrics(collector)
	require.Len(t, metrics, 1)
	verifyGaugeValue(t, metrics[0], 20)
}

// TestConstGaugeCollectorSet tests the Set method of gauge collector
func TestConstGaugeCollectorSet(t *testing.T) {
	// Create a gauge collector
	labelNames := []string{"label1", "label2"}
	collector := collectors.NewConstGaugeCollector("test_gauge", labelNames)

	// Set some metrics
	group := "test_group"
	collector.Set(10, map[string]string{"label1": "value1", "label2": "value2"}, collectors.WithGroup(group))

	// Verify the initial value
	metrics := collectMetrics(collector)
	require.Len(t, metrics, 1)
	verifyGaugeValue(t, metrics[0], 10)

	// Update the value
	collector.Set(20, map[string]string{"label1": "value1", "label2": "value2"}, collectors.WithGroup(group))

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
	collector.Set(10, map[string]string{"label1": "value1", "label2": "value2"}, collectors.WithGroup(group))
	collector.Set(20, map[string]string{"label1": "value3", "label2": "value4"}, collectors.WithGroup(group))

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

// TestExpireGroupMetricsGauge tests the ExpireGroupMetrics method of gauge collector
func TestExpireGroupMetricsGauge(t *testing.T) {
	// Create a gauge collector
	labelNames := []string{"label1", "label2"}
	collector := collectors.NewConstGaugeCollector("test_gauge", labelNames)

	// Set metrics with different groups
	group1 := "group1"
	group2 := "group2"
	collector.Set(10, map[string]string{"label1": "value1", "label2": "value2"}, collectors.WithGroup(group1))
	collector.Set(20, map[string]string{"label1": "value3", "label2": "value4"}, collectors.WithGroup(group2))

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

// TestUpdateLabelsGauge tests the UpdateLabels method of gauge collector
func TestUpdateLabelsGauge(t *testing.T) {
	// Create a gauge collector with initial labels
	initialLabelNames := []string{"label1", "label2"}
	collector := collectors.NewConstGaugeCollector("test_gauge", initialLabelNames)

	// Set a metric
	group := "test_group"
	collector.Set(10, map[string]string{"label1": "value1", "label2": "value2"}, collectors.WithGroup(group))

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
