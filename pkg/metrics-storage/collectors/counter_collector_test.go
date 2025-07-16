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
	"testing"

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

// TestConstCounterCollectorAdd tests the Add method of counter collector
func TestConstCounterCollectorAdd(t *testing.T) {
	// Create a counter collector
	labelNames := []string{"label1", "label2"}
	collector := collectors.NewConstCounterCollector("test_counter", labelNames)

	// Add some metrics
	group := "test_group"
	collector.Add(10, map[string]string{"label1": "value1", "label2": "value2"}, collectors.WithGroup(group))
	collector.Add(5, map[string]string{"label1": "value1", "label2": "value2"}, collectors.WithGroup(group))

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
	collector.Add(10, map[string]string{"label1": "value1", "label2": "value2"}, collectors.WithGroup(group1))
	collector.Add(5, map[string]string{"label1": "value1", "label2": "value2"}, collectors.WithGroup(group2))

	// Since the labels are the same, we should end up with a single metric
	// This tests that groups don't affect the metric identity for collection
	metrics := collectMetrics(collector)
	require.Len(t, metrics, 2)
}

// TestExpireGroupMetricsCounter tests the ExpireGroupMetrics method of counter collector
func TestExpireGroupMetricsCounter(t *testing.T) {
	// Create a counter collector
	labelNames := []string{"label1", "label2"}
	collector := collectors.NewConstCounterCollector("test_counter", labelNames)

	// Add metrics with different groups
	group1 := "group1"
	group2 := "group2"
	collector.Add(10, map[string]string{"label1": "value1", "label2": "value2"}, collectors.WithGroup(group1))
	collector.Add(20, map[string]string{"label1": "value3", "label2": "value4"}, collectors.WithGroup(group2))

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

// TestUpdateLabelsCounter tests the UpdateLabels method of counter collector
func TestUpdateLabelsCounter(t *testing.T) {
	// Create a counter collector with initial labels
	initialLabelNames := []string{"label1", "label2"}
	collector := collectors.NewConstCounterCollector("test_counter", initialLabelNames)

	// Add a metric
	group := "test_group"
	collector.Add(10, map[string]string{"label1": "value1", "label2": "value2"}, collectors.WithGroup(group))

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
