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
	"fmt"
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/deckhouse/deckhouse/pkg/metrics-storage/collectors"
)

// TestHashLabelValues tests the HashLabelValues function
func TestHashLabelValues(t *testing.T) {
	t.Run("empty inputs", func(t *testing.T) {
		hash1 := collectors.HashMetric("", nil)
		hash2 := collectors.HashMetric("", []string{})
		assert.Equal(t, hash1, hash2)
	})

	t.Run("same inputs produce same hash", func(t *testing.T) {
		group := "test_group"
		labelValues := []string{"value1", "value2", "value3"}

		hash1 := collectors.HashMetric(group, labelValues)
		hash2 := collectors.HashMetric(group, labelValues)
		assert.Equal(t, hash1, hash2)
	})

	t.Run("different groups produce different hashes", func(t *testing.T) {
		labelValues := []string{"value1", "value2"}

		hash1 := collectors.HashMetric("group1", labelValues)
		hash2 := collectors.HashMetric("group2", labelValues)
		assert.NotEqual(t, hash1, hash2)
	})

	t.Run("different label values produce different hashes", func(t *testing.T) {
		group := "test_group"

		hash1 := collectors.HashMetric(group, []string{"value1", "value2"})
		hash2 := collectors.HashMetric(group, []string{"value1", "value3"})
		assert.NotEqual(t, hash1, hash2)
	})

	t.Run("order matters for label values", func(t *testing.T) {
		group := "test_group"

		hash1 := collectors.HashMetric(group, []string{"value1", "value2"})
		hash2 := collectors.HashMetric(group, []string{"value2", "value1"})
		assert.NotEqual(t, hash1, hash2)
	})

	t.Run("empty group vs non-empty group", func(t *testing.T) {
		labelValues := []string{"value1", "value2"}

		hash1 := collectors.HashMetric("", labelValues)
		hash2 := collectors.HashMetric("group", labelValues)
		assert.NotEqual(t, hash1, hash2)
	})

	t.Run("empty label values", func(t *testing.T) {
		group := "test_group"

		hash1 := collectors.HashMetric(group, []string{""})
		hash2 := collectors.HashMetric(group, []string{"", ""})
		assert.NotEqual(t, hash1, hash2)
	})

	t.Run("special characters in values", func(t *testing.T) {
		group := "test_group"

		hash1 := collectors.HashMetric(group, []string{"value with spaces", "value\nwith\nnewlines"})
		hash2 := collectors.HashMetric(group, []string{"value with spaces", "value\twith\ttabs"})
		assert.NotEqual(t, hash1, hash2)
	})

	t.Run("large number of label values", func(t *testing.T) {
		group := "test_group"
		labelValues := make([]string, 100)
		for i := range labelValues {
			labelValues[i] = fmt.Sprintf("value_%d", i)
		}

		hash1 := collectors.HashMetric(group, labelValues)

		// Modify one value
		labelValues[50] = "modified_value"
		hash2 := collectors.HashMetric(group, labelValues)

		assert.NotEqual(t, hash1, hash2)
	})
}

// TestEdgeCases tests various edge cases
func TestEdgeCases(t *testing.T) {
	// Test with empty label names
	counterCollector := collectors.NewConstCounterCollector(collectors.MetricDescription{Name: "test_counter", LabelNames: []string{}})
	gaugeCollector := collectors.NewConstGaugeCollector(&collectors.MetricDescription{Name: "test_gauge", LabelNames: []string{}})

	// Add metrics with empty labels
	counterCollector.Add(10, map[string]string{}, collectors.WithGroup("group"))
	gaugeCollector.Set(20, map[string]string{}, collectors.WithGroup("group"))

	// Verify the metrics
	counterMetrics := collectMetrics(counterCollector)
	gaugeMetrics := collectMetrics(gaugeCollector)

	require.Len(t, counterMetrics, 1)
	require.Len(t, gaugeMetrics, 1)
	verifyCounterValue(t, counterMetrics[0], 10)
	verifyGaugeValue(t, gaugeMetrics[0], 20)

	// Test with nil label values map
	counterCollector.Add(5, nil, collectors.WithGroup("group"))
	gaugeCollector.Set(25, nil, collectors.WithGroup("group"))

	// Verify the metrics
	counterMetrics = collectMetrics(counterCollector)
	gaugeMetrics = collectMetrics(gaugeCollector)

	require.Len(t, counterMetrics, 1)
	require.Len(t, gaugeMetrics, 1)
	verifyCounterValue(t, counterMetrics[0], 15)
	verifyGaugeValue(t, gaugeMetrics[0], 25)

	// Test with empty groups
	counterCollector.Add(10, map[string]string{}, collectors.WithGroup(""))
	gaugeCollector.Set(20, map[string]string{}, collectors.WithGroup(""))

	// Verify the metrics
	counterMetrics = collectMetrics(counterCollector)
	gaugeMetrics = collectMetrics(gaugeCollector)

	require.Len(t, counterMetrics, 2)
	require.Len(t, gaugeMetrics, 2)

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
	counterCollector := collectors.NewConstCounterCollector(collectors.MetricDescription{Name: "test_counter", LabelNames: labelNames})
	gaugeCollector := collectors.NewConstGaugeCollector(&collectors.MetricDescription{Name: "test_gauge", LabelNames: labelNames})

	// Add metrics
	counterCollector.Add(10, map[string]string{"label1": "value1", "label2": "value2"}, collectors.WithGroup("group"))
	gaugeCollector.Set(20, map[string]string{"label1": "value1", "label2": "value2"}, collectors.WithGroup("group"))

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
	counterCollector := collectors.NewConstCounterCollector(collectors.MetricDescription{Name: "test_counter", LabelNames: []string{"label"}})
	gaugeCollector := collectors.NewConstGaugeCollector(&collectors.MetricDescription{Name: "test_gauge", LabelNames: []string{"label"}})

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
	ch := make(chan prometheus.Metric)

	go func() {
		collector.Collect(ch)
		close(ch)
	}()

	metrics := make([]prometheus.Metric, 0, 1)
	for metric := range ch {
		metrics = append(metrics, metric)
	}

	return metrics
}
