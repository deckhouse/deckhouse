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
	"math"
	"sort"
	"sync"
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/deckhouse/deckhouse/pkg/metrics-storage/collectors"
)

func TestNewConstCounterCollector(t *testing.T) {
	t.Run("basic constructor", func(t *testing.T) {
		name := "test_counter"
		labelNames := []string{"method", "status"}

		collector := collectors.NewConstCounterCollector(collectors.MetricDescription{
			Name:       name,
			LabelNames: labelNames,
		})

		assert.Equal(t, name, collector.Name())
		assert.Equal(t, labelNames, collector.LabelNames())
		assert.Equal(t, "counter", collector.Type())
	})

	t.Run("with empty label names", func(t *testing.T) {
		collector := collectors.NewConstCounterCollector(collectors.MetricDescription{
			Name:       "test_counter",
			LabelNames: []string{},
		})

		assert.Equal(t, "test_counter", collector.Name())
		assert.Equal(t, []string{}, collector.LabelNames())
		assert.Equal(t, "counter", collector.Type())
	})

	t.Run("with nil label names", func(t *testing.T) {
		collector := collectors.NewConstCounterCollector(collectors.MetricDescription{
			Name:       "test_counter",
			LabelNames: nil,
		})

		assert.Equal(t, "test_counter", collector.Name())
		assert.Nil(t, collector.LabelNames())
		assert.Equal(t, "counter", collector.Type())
	})

	t.Run("with single label", func(t *testing.T) {
		collector := collectors.NewConstCounterCollector(collectors.MetricDescription{
			Name:       "test_counter",
			LabelNames: []string{"method"},
		})

		assert.Equal(t, []string{"method"}, collector.LabelNames())
	})
}

func TestConstCounterCollector_Add(t *testing.T) {
	t.Run("basic add operation", func(t *testing.T) {
		collector := collectors.NewConstCounterCollector(collectors.MetricDescription{
			Name:       "test_counter",
			LabelNames: []string{"method"},
		})

		collector.Add(1, map[string]string{"method": "GET"})

		metrics := collectCounterMetrics(t, collector)
		require.Len(t, metrics, 1)
		verifyCounterValue(t, metrics[0], 1)
		verifyCounterLabels(t, metrics[0], map[string]string{"method": "GET"})
	})

	t.Run("multiple adds same metric", func(t *testing.T) {
		collector := collectors.NewConstCounterCollector(collectors.MetricDescription{
			Name:       "test_counter",
			LabelNames: []string{"method"},
		})

		collector.Add(1, map[string]string{"method": "GET"})
		collector.Add(2, map[string]string{"method": "GET"})
		collector.Add(3, map[string]string{"method": "GET"})

		metrics := collectCounterMetrics(t, collector)
		require.Len(t, metrics, 1)
		verifyCounterValue(t, metrics[0], 6)
	})

	t.Run("multiple adds different metrics", func(t *testing.T) {
		collector := collectors.NewConstCounterCollector(collectors.MetricDescription{
			Name:       "test_counter",
			LabelNames: []string{"method"},
		})

		collector.Add(1, map[string]string{"method": "GET"})
		collector.Add(2, map[string]string{"method": "POST"})
		collector.Add(3, map[string]string{"method": "GET"})

		metrics := collectCounterMetrics(t, collector)
		require.Len(t, metrics, 2)

		// Sort by value for deterministic testing
		sort.Slice(metrics, func(i, j int) bool {
			return getCounterValue(t, metrics[i]) < getCounterValue(t, metrics[j])
		})

		verifyCounterValue(t, metrics[0], 2) // POST
		verifyCounterValue(t, metrics[1], 4) // GET
	})

	t.Run("add with multiple labels", func(t *testing.T) {
		collector := collectors.NewConstCounterCollector(collectors.MetricDescription{
			Name:       "test_counter",
			LabelNames: []string{"method", "status", "endpoint"},
		})

		labels := map[string]string{
			"method":   "GET",
			"status":   "200",
			"endpoint": "/api/users",
		}

		collector.Add(5, labels)

		metrics := collectCounterMetrics(t, collector)
		require.Len(t, metrics, 1)
		verifyCounterValue(t, metrics[0], 5)
		verifyCounterLabels(t, metrics[0], labels)
	})

	t.Run("add with empty labels map", func(t *testing.T) {
		collector := collectors.NewConstCounterCollector(collectors.MetricDescription{
			Name:       "test_counter",
			LabelNames: []string{},
		})

		collector.Add(10, map[string]string{})
		collector.Add(5, nil)

		metrics := collectCounterMetrics(t, collector)
		require.Len(t, metrics, 1)
		verifyCounterValue(t, metrics[0], 15)
	})

	t.Run("add with missing label values", func(t *testing.T) {
		collector := collectors.NewConstCounterCollector(collectors.MetricDescription{
			Name:       "test_counter",
			LabelNames: []string{"method", "status"},
		})

		// Only provide one label value
		collector.Add(1, map[string]string{"method": "GET"})

		metrics := collectCounterMetrics(t, collector)
		require.Len(t, metrics, 1)

		labels := extractCounterLabels(t, metrics[0])
		assert.Equal(t, "GET", labels["method"])
		assert.Equal(t, "", labels["status"]) // Missing label should be empty string
	})

	t.Run("add with extra label values", func(t *testing.T) {
		collector := collectors.NewConstCounterCollector(collectors.MetricDescription{
			Name:       "test_counter",
			LabelNames: []string{"method"},
		})

		// Provide extra labels that aren't in labelNames
		labels := map[string]string{
			"method": "GET",
			"extra":  "ignored",
		}

		collector.Add(1, labels)

		metrics := collectCounterMetrics(t, collector)
		require.Len(t, metrics, 1)

		actualLabels := extractCounterLabels(t, metrics[0])
		assert.Equal(t, "GET", actualLabels["method"])
		assert.NotContains(t, actualLabels, "extra")
	})
}

func TestConstCounterCollector_AddEdgeCases(t *testing.T) {
	t.Run("add zero value", func(t *testing.T) {
		collector := collectors.NewConstCounterCollector(collectors.MetricDescription{
			Name:       "test_counter",
			LabelNames: []string{},
		})

		collector.Add(0, nil)

		metrics := collectCounterMetrics(t, collector)
		require.Len(t, metrics, 1)
		verifyCounterValue(t, metrics[0], 0)
	})

	t.Run("add fractional value", func(t *testing.T) {
		collector := collectors.NewConstCounterCollector(collectors.MetricDescription{
			Name:       "test_counter",
			LabelNames: []string{},
		})

		// Counter should handle fractional values correctly
		collector.Add(3.7, nil)
		collector.Add(2.9, nil)

		metrics := collectCounterMetrics(t, collector)
		require.Len(t, metrics, 1)
		verifyCounterValue(t, metrics[0], 5)
	})

	t.Run("add negative value", func(t *testing.T) {
		collector := collectors.NewConstCounterCollector(collectors.MetricDescription{
			Name:       "test_counter",
			LabelNames: []string{},
		})

		// This should not panic, but behavior depends on implementation
		collector.Add(-1, nil)

		metrics := collectCounterMetrics(t, collector)
		require.Len(t, metrics, 1)
		// The exact value depends on how negative values are handled
	})

	t.Run("add special float values", func(t *testing.T) {
		collector := collectors.NewConstCounterCollector(collectors.MetricDescription{
			Name:       "test_counter",
			LabelNames: []string{"type"},
		})

		testCases := []struct {
			name  string
			value float64
		}{
			{"positive infinity", math.Inf(1)},
			{"negative infinity", math.Inf(-1)},
			{"NaN", math.NaN()},
			{"very small", 1e-10},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				// Should not panic
				collector.Add(tc.value, map[string]string{"type": tc.name})

				metrics := collectCounterMetrics(t, collector)
				assert.NotEmpty(t, metrics)
			})
		}
	})
}

func TestConstCounterCollector_Concurrency(t *testing.T) {
	collector := collectors.NewConstCounterCollector(collectors.MetricDescription{
		Name:       "test_counter",
		LabelNames: []string{"worker"},
	})

	numWorkers := 10
	incrementsPerWorker := 100
	var wg sync.WaitGroup

	// Start multiple goroutines adding to different metrics
	for i := 0; i < numWorkers; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			for j := 0; j < incrementsPerWorker; j++ {
				collector.Add(1, map[string]string{"worker": fmt.Sprintf("worker_%d", workerID)})
			}
		}(i)
	}

	wg.Wait()

	metrics := collectCounterMetrics(t, collector)
	require.Len(t, metrics, numWorkers)

	totalValue := float64(0)
	for _, metric := range metrics {
		value := getCounterValue(t, metric)
		assert.Equal(t, float64(incrementsPerWorker), value)
		totalValue += value
	}

	assert.Equal(t, float64(numWorkers*incrementsPerWorker), totalValue)
}

func TestConstCounterCollector_ConcurrentSameMetric(t *testing.T) {
	collector := collectors.NewConstCounterCollector(collectors.MetricDescription{
		Name:       "test_counter",
		LabelNames: []string{"shared"},
	})

	numWorkers := 10
	incrementsPerWorker := 100
	var wg sync.WaitGroup

	// Start multiple goroutines adding to the same metric
	for i := 0; i < numWorkers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < incrementsPerWorker; j++ {
				collector.Add(1, map[string]string{"shared": "metric"})
			}
		}()
	}

	wg.Wait()

	metrics := collectCounterMetrics(t, collector)
	require.Len(t, metrics, 1)
	verifyCounterValue(t, metrics[0], float64(numWorkers*incrementsPerWorker))
}

func TestConstCounterCollector_ExpireGroupMetrics(t *testing.T) {
	collector := collectors.NewConstCounterCollector(collectors.MetricDescription{
		Name:       "test_counter",
		LabelNames: []string{"method"},
	})

	// Add metrics for different groups
	collector.Add(1, map[string]string{"method": "GET"}, collectors.WithGroup("group1"))
	collector.Add(2, map[string]string{"method": "POST"}, collectors.WithGroup("group1"))
	collector.Add(3, map[string]string{"method": "DELETE"}, collectors.WithGroup("group2"))
	collector.Add(4, map[string]string{"method": "PUT"}, collectors.WithGroup("group2"))

	// Verify initial state
	metrics := collectCounterMetrics(t, collector)
	require.Len(t, metrics, 4)

	// Expire group1
	collector.ExpireGroupMetrics("group1")

	// Verify only group2 metrics remain
	metrics = collectCounterMetrics(t, collector)
	require.Len(t, metrics, 2)

	// Verify remaining metrics are from group2
	for _, metric := range metrics {
		labels := extractCounterLabels(t, metric)
		method := labels["method"]
		assert.True(t, method == "DELETE" || method == "PUT")
	}
}

func TestConstCounterCollector_ExpireNonExistentGroup(t *testing.T) {
	collector := collectors.NewConstCounterCollector(collectors.MetricDescription{
		Name:       "test_counter",
		LabelNames: []string{"method"},
	})

	collector.Add(1, map[string]string{"method": "GET"})

	// Expire non-existent group
	collector.ExpireGroupMetrics("non_existent")

	// Verify metric still exists
	metrics := collectCounterMetrics(t, collector)
	require.Len(t, metrics, 1)
}

func TestConstCounterCollector_ExpireEmptyCollection(t *testing.T) {
	collector := collectors.NewConstCounterCollector(collectors.MetricDescription{
		Name:       "test_counter",
		LabelNames: []string{"method"},
	})

	// Expire from empty collection
	collector.ExpireGroupMetrics("any_group")

	// Should not panic and collection should remain empty
	metrics := collectCounterMetrics(t, collector)
	require.Len(t, metrics, 0)
}

func TestConstCounterCollector_UpdateLabels(t *testing.T) {
	t.Run("add new labels", func(t *testing.T) {
		collector := collectors.NewConstCounterCollector(collectors.MetricDescription{
			Name:       "test_counter",
			LabelNames: []string{"method"},
		})

		// Add initial metric
		collector.Add(1, map[string]string{"method": "GET"})

		// Update labels
		collector.UpdateLabels([]string{"status", "endpoint"})

		// Verify label names updated
		expectedLabels := []string{"endpoint", "method", "status"} // sorted
		actualLabels := collector.LabelNames()
		sort.Strings(actualLabels)
		assert.Equal(t, expectedLabels, actualLabels)

		// Verify existing metrics still work
		metrics := collectCounterMetrics(t, collector)
		require.Len(t, metrics, 1)
		verifyCounterValue(t, metrics[0], 1)
	})

	t.Run("add duplicate labels", func(t *testing.T) {
		collector := collectors.NewConstCounterCollector(collectors.MetricDescription{
			Name:       "test_counter",
			LabelNames: []string{"method"},
		})

		collector.Add(1, map[string]string{"method": "GET"})

		// Try to add existing label
		collector.UpdateLabels([]string{"method"})

		// Should not change
		assert.Equal(t, []string{"method"}, collector.LabelNames())

		metrics := collectCounterMetrics(t, collector)
		require.Len(t, metrics, 1)
	})

	t.Run("update with empty labels", func(t *testing.T) {
		collector := collectors.NewConstCounterCollector(collectors.MetricDescription{
			Name:       "test_counter",
			LabelNames: []string{"method"},
		})

		collector.UpdateLabels([]string{})

		// Should not change
		assert.Equal(t, []string{"method"}, collector.LabelNames())
	})

	t.Run("update preserves metric values", func(t *testing.T) {
		collector := collectors.NewConstCounterCollector(collectors.MetricDescription{
			Name:       "test_counter",
			LabelNames: []string{"method"},
		})

		collector.Add(5, map[string]string{"method": "GET"})
		collector.Add(3, map[string]string{"method": "POST"})

		// Update labels
		collector.UpdateLabels([]string{"status"})

		// Verify metrics preserve their values
		metrics := collectCounterMetrics(t, collector)
		require.Len(t, metrics, 2)

		totalValue := float64(0)
		for _, metric := range metrics {
			totalValue += getCounterValue(t, metric)
		}
		assert.Equal(t, float64(8), totalValue)
	})

	t.Run("update with complex label reordering", func(t *testing.T) {
		collector := collectors.NewConstCounterCollector(collectors.MetricDescription{
			Name:       "test_counter",
			LabelNames: []string{"a", "b", "c"},
		})

		labels := map[string]string{"a": "val_a", "b": "val_b", "c": "val_c"}
		collector.Add(10, labels)

		// Add new labels that will change the sort order
		collector.UpdateLabels([]string{"z", "x"})

		// Verify values preserved despite reordering
		metrics := collectCounterMetrics(t, collector)
		require.Len(t, metrics, 1)
		verifyCounterValue(t, metrics[0], 10)

		// Verify new label structure
		expectedLabels := []string{"a", "b", "c", "x", "z"}
		actualLabels := collector.LabelNames()
		sort.Strings(actualLabels)
		assert.Equal(t, expectedLabels, actualLabels)
	})
}

func TestConstCounterCollector_Describe(t *testing.T) {
	collector := collectors.NewConstCounterCollector(collectors.MetricDescription{
		Name:       "test_counter",
		LabelNames: []string{"method"},
	})

	ch := make(chan *prometheus.Desc, 1)
	collector.Describe(ch)
	close(ch)

	var desc *prometheus.Desc
	for d := range ch {
		desc = d
	}

	require.NotNil(t, desc)
	assert.Contains(t, desc.String(), "test_counter")
}

func TestConstCounterCollector_Collect(t *testing.T) {
	t.Run("collect empty", func(t *testing.T) {
		collector := collectors.NewConstCounterCollector(collectors.MetricDescription{
			Name:       "test_counter",
			LabelNames: []string{},
		})

		metrics := collectCounterMetrics(t, collector)
		assert.Len(t, metrics, 0)
	})

	t.Run("collect single metric", func(t *testing.T) {
		collector := collectors.NewConstCounterCollector(collectors.MetricDescription{
			Name:       "test_counter",
			LabelNames: []string{"method"},
		})

		collector.Add(42, map[string]string{"method": "GET"})

		metrics := collectCounterMetrics(t, collector)
		require.Len(t, metrics, 1)
		verifyCounterValue(t, metrics[0], 42)
	})

	t.Run("collect multiple metrics", func(t *testing.T) {
		collector := collectors.NewConstCounterCollector(collectors.MetricDescription{
			Name:       "test_counter",
			LabelNames: []string{"method"},
		})

		collector.Add(1, map[string]string{"method": "GET"})
		collector.Add(2, map[string]string{"method": "POST"})
		collector.Add(3, map[string]string{"method": "PUT"})

		metrics := collectCounterMetrics(t, collector)
		require.Len(t, metrics, 3)

		totalValue := float64(0)
		for _, metric := range metrics {
			totalValue += getCounterValue(t, metric)
		}
		assert.Equal(t, float64(6), totalValue)
	})
}

func TestConstCounterCollector_InterfaceCompliance(t *testing.T) {
	// Test that ConstCounterCollector implements ConstCollector interface
	var _ collectors.ConstCollector = (*collectors.ConstCounterCollector)(nil)

	collector := collectors.NewConstCounterCollector(collectors.MetricDescription{
		Name:       "test_counter",
		LabelNames: []string{},
	})

	// Test all interface methods
	assert.Equal(t, "test_counter", collector.Name())
	assert.Equal(t, "counter", collector.Type())
	assert.NotNil(t, collector.LabelNames())

	// Test Describe doesn't panic
	ch := make(chan *prometheus.Desc, 1)
	collector.Describe(ch)
	close(ch)

	// Test Collect doesn't panic
	metricCh := make(chan prometheus.Metric, 10)
	collector.Collect(metricCh)
	close(metricCh)
}

func TestConstCounterCollector_LabelOrdering(t *testing.T) {
	// Test that label ordering is consistent
	collector := collectors.NewConstCounterCollector(collectors.MetricDescription{
		Name:       "test_counter",
		LabelNames: []string{"z", "a", "m"},
	})

	collector.Add(1, map[string]string{"z": "val_z", "a": "val_a", "m": "val_m"})

	metrics := collectCounterMetrics(t, collector)
	require.Len(t, metrics, 1)

	// Verify labels are in the expected order (should match labelNames order)
	labels := extractCounterLabels(t, metrics[0])
	assert.Equal(t, "val_z", labels["z"])
	assert.Equal(t, "val_a", labels["a"])
	assert.Equal(t, "val_m", labels["m"])
}

func TestConstCounterCollector_MetricDescription(t *testing.T) {
	t.Run("empty metric description", func(t *testing.T) {
		collector := collectors.NewConstCounterCollector(collectors.MetricDescription{})

		assert.Equal(t, "", collector.Name())
		assert.Nil(t, collector.LabelNames())
		assert.Equal(t, "counter", collector.Type())
	})

	t.Run("metric description with help", func(t *testing.T) {
		collector := collectors.NewConstCounterCollector(collectors.MetricDescription{
			Name:       "test_counter",
			Help:       "Test counter help",
			LabelNames: []string{"method"},
		})

		assert.Equal(t, "test_counter", collector.Name())
		assert.Equal(t, []string{"method"}, collector.LabelNames())
	})

	t.Run("metric description with const labels", func(t *testing.T) {
		collector := collectors.NewConstCounterCollector(collectors.MetricDescription{
			Name:        "test_counter",
			LabelNames:  []string{"method"},
			ConstLabels: map[string]string{"service": "api"},
		})

		assert.Equal(t, "test_counter", collector.Name())
		assert.Equal(t, []string{"method"}, collector.LabelNames())
	})
}

// Helper functions for blackbox testing

func collectCounterMetrics(t *testing.T, collector *collectors.ConstCounterCollector) []prometheus.Metric {
	t.Helper()

	ch := make(chan prometheus.Metric, 100)
	collector.Collect(ch)
	close(ch)

	metrics := make([]prometheus.Metric, 0, 1)
	for metric := range ch {
		metrics = append(metrics, metric)
	}
	return metrics
}

func verifyCounterValue(t *testing.T, metric prometheus.Metric, expectedValue float64) {
	t.Helper()

	var dtoMetric dto.Metric
	err := metric.Write(&dtoMetric)
	require.NoError(t, err)

	counter := dtoMetric.GetCounter()
	require.NotNil(t, counter)
	assert.Equal(t, expectedValue, counter.GetValue())
}

func verifyCounterLabels(t *testing.T, metric prometheus.Metric, expectedLabels map[string]string) {
	t.Helper()

	actualLabels := extractCounterLabels(t, metric)
	assert.Equal(t, expectedLabels, actualLabels)
}

func getCounterValue(t *testing.T, metric prometheus.Metric) float64 {
	t.Helper()

	var dtoMetric dto.Metric
	err := metric.Write(&dtoMetric)
	require.NoError(t, err)
	return dtoMetric.GetCounter().GetValue()
}

func extractCounterLabels(t *testing.T, metric prometheus.Metric) map[string]string {
	t.Helper()

	var dtoMetric dto.Metric
	err := metric.Write(&dtoMetric)
	require.NoError(t, err)

	labels := make(map[string]string)
	for _, labelPair := range dtoMetric.GetLabel() {
		labels[labelPair.GetName()] = labelPair.GetValue()
	}
	return labels
}
