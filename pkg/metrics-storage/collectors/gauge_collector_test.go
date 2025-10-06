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

func TestNewConstGaugeCollector(t *testing.T) {
	t.Run("basic constructor", func(t *testing.T) {
		name := "test_gauge"
		labelNames := []string{"method", "status"}

		collector := collectors.NewConstGaugeCollector(&collectors.MetricDescription{
			Name:       name,
			LabelNames: labelNames,
		})

		assert.Equal(t, name, collector.Name())
		assert.Equal(t, labelNames, collector.LabelNames())
		assert.Equal(t, "gauge", collector.Type())
	})

	t.Run("with empty label names", func(t *testing.T) {
		collector := collectors.NewConstGaugeCollector(&collectors.MetricDescription{
			Name:       "test_gauge",
			LabelNames: []string{},
		})

		assert.Equal(t, "test_gauge", collector.Name())
		assert.Equal(t, []string{}, collector.LabelNames())
		assert.Equal(t, "gauge", collector.Type())
	})

	t.Run("with nil label names", func(t *testing.T) {
		collector := collectors.NewConstGaugeCollector(&collectors.MetricDescription{
			Name:       "test_gauge",
			LabelNames: nil,
		})

		assert.Equal(t, "test_gauge", collector.Name())
		assert.Nil(t, collector.LabelNames())
		assert.Equal(t, "gauge", collector.Type())
	})

	t.Run("with single label", func(t *testing.T) {
		collector := collectors.NewConstGaugeCollector(&collectors.MetricDescription{
			Name:       "test_gauge",
			LabelNames: []string{"method"},
		})

		assert.Equal(t, []string{"method"}, collector.LabelNames())
	})

	t.Run("with help text", func(t *testing.T) {
		collector := collectors.NewConstGaugeCollector(&collectors.MetricDescription{
			Name:       "test_gauge",
			Help:       "Test gauge help text",
			LabelNames: []string{"method"},
		})

		assert.Equal(t, "test_gauge", collector.Name())
		assert.Equal(t, []string{"method"}, collector.LabelNames())
	})

	t.Run("with const labels", func(t *testing.T) {
		collector := collectors.NewConstGaugeCollector(&collectors.MetricDescription{
			Name:        "test_gauge",
			LabelNames:  []string{"method"},
			ConstLabels: map[string]string{"service": "api"},
		})

		assert.Equal(t, "test_gauge", collector.Name())
		assert.Equal(t, []string{"method"}, collector.LabelNames())
	})
}

func TestConstGaugeCollector_Set(t *testing.T) {
	t.Run("basic set operation", func(t *testing.T) {
		collector := collectors.NewConstGaugeCollector(&collectors.MetricDescription{
			Name:       "test_gauge",
			LabelNames: []string{"method"},
		})

		collector.Set(42.5, map[string]string{"method": "GET"}, collectors.WithGroup("group1"))

		metrics := collectGaugeMetrics(t, collector)
		require.Len(t, metrics, 1)
		verifyGaugeValue(t, metrics[0], 42.5)
		verifyGaugeLabels(t, metrics[0], map[string]string{"method": "GET"})
	})

	t.Run("set overwrites previous value", func(t *testing.T) {
		collector := collectors.NewConstGaugeCollector(&collectors.MetricDescription{
			Name:       "test_gauge",
			LabelNames: []string{"method"},
		})

		collector.Set(10.0, map[string]string{"method": "GET"}, collectors.WithGroup("group1"))
		collector.Set(20.0, map[string]string{"method": "GET"}, collectors.WithGroup("group1"))

		metrics := collectGaugeMetrics(t, collector)
		require.Len(t, metrics, 1)
		verifyGaugeValue(t, metrics[0], 20.0)
	})

	t.Run("set multiple different metrics", func(t *testing.T) {
		collector := collectors.NewConstGaugeCollector(&collectors.MetricDescription{
			Name:       "test_gauge",
			LabelNames: []string{"method"},
		})

		collector.Set(10.0, map[string]string{"method": "GET"}, collectors.WithGroup("group1"))
		collector.Set(20.0, map[string]string{"method": "POST"}, collectors.WithGroup("group1"))
		collector.Set(30.0, map[string]string{"method": "PUT"}, collectors.WithGroup("group1"))

		metrics := collectGaugeMetrics(t, collector)
		require.Len(t, metrics, 3)

		// Sort by value for deterministic testing
		sort.Slice(metrics, func(i, j int) bool {
			return getGaugeValue(t, metrics[i]) < getGaugeValue(t, metrics[j])
		})

		verifyGaugeValue(t, metrics[0], 10.0) // GET
		verifyGaugeValue(t, metrics[1], 20.0) // POST
		verifyGaugeValue(t, metrics[2], 30.0) // PUT
	})

	t.Run("set with multiple labels", func(t *testing.T) {
		collector := collectors.NewConstGaugeCollector(&collectors.MetricDescription{
			Name:       "test_gauge",
			LabelNames: []string{"method", "status", "endpoint"},
		})

		labels := map[string]string{
			"method":   "GET",
			"status":   "200",
			"endpoint": "/api/users",
		}

		collector.Set(123.45, labels, collectors.WithGroup("group1"))

		metrics := collectGaugeMetrics(t, collector)
		require.Len(t, metrics, 1)
		verifyGaugeValue(t, metrics[0], 123.45)
		verifyGaugeLabels(t, metrics[0], labels)
	})

	t.Run("set with empty labels map", func(t *testing.T) {
		collector := collectors.NewConstGaugeCollector(&collectors.MetricDescription{
			Name:       "test_gauge",
			LabelNames: []string{},
		})

		collector.Set(42.0, map[string]string{}, collectors.WithGroup("group1"))
		collector.Set(24.0, nil, collectors.WithGroup("group1")) // Should overwrite

		metrics := collectGaugeMetrics(t, collector)
		require.Len(t, metrics, 1)
		verifyGaugeValue(t, metrics[0], 24.0)
	})

	t.Run("set with missing label values", func(t *testing.T) {
		collector := collectors.NewConstGaugeCollector(&collectors.MetricDescription{
			Name:       "test_gauge",
			LabelNames: []string{"method", "status"},
		})

		// Only provide one label value
		collector.Set(50.0, map[string]string{"method": "GET"}, collectors.WithGroup("group1"))

		metrics := collectGaugeMetrics(t, collector)
		require.Len(t, metrics, 1)

		labels := extractGaugeLabels(t, metrics[0])
		assert.Equal(t, "GET", labels["method"])
		assert.Equal(t, "", labels["status"]) // Missing label should be empty string
	})

	t.Run("set with extra label values", func(t *testing.T) {
		collector := collectors.NewConstGaugeCollector(&collectors.MetricDescription{
			Name:       "test_gauge",
			LabelNames: []string{"method"},
		})

		// Provide extra labels that aren't in labelNames
		labels := map[string]string{
			"method": "GET",
			"extra":  "ignored",
		}

		collector.Set(75.0, labels, collectors.WithGroup("group1"))

		metrics := collectGaugeMetrics(t, collector)
		require.Len(t, metrics, 1)

		actualLabels := extractGaugeLabels(t, metrics[0])
		assert.Equal(t, "GET", actualLabels["method"])
		assert.NotContains(t, actualLabels, "extra")
	})
}

func TestConstGaugeCollector_Add(t *testing.T) {
	t.Run("basic add operation", func(t *testing.T) {
		collector := collectors.NewConstGaugeCollector(&collectors.MetricDescription{
			Name:       "test_gauge",
			LabelNames: []string{"method"},
		})

		collector.Add(10.0, map[string]string{"method": "GET"}, collectors.WithGroup("group1"))

		metrics := collectGaugeMetrics(t, collector)
		require.Len(t, metrics, 1)
		verifyGaugeValue(t, metrics[0], 10.0)
	})

	t.Run("add accumulates values", func(t *testing.T) {
		collector := collectors.NewConstGaugeCollector(&collectors.MetricDescription{
			Name:       "test_gauge",
			LabelNames: []string{"method"},
		})

		collector.Add(10.0, map[string]string{"method": "GET"}, collectors.WithGroup("group1"))
		collector.Add(5.0, map[string]string{"method": "GET"}, collectors.WithGroup("group1"))
		collector.Add(-2.0, map[string]string{"method": "GET"}, collectors.WithGroup("group1"))

		metrics := collectGaugeMetrics(t, collector)
		require.Len(t, metrics, 1)
		verifyGaugeValue(t, metrics[0], 13.0)
	})

	t.Run("add to different metrics", func(t *testing.T) {
		collector := collectors.NewConstGaugeCollector(&collectors.MetricDescription{
			Name:       "test_gauge",
			LabelNames: []string{"method"},
		})

		collector.Add(10.0, map[string]string{"method": "GET"}, collectors.WithGroup("group1"))
		collector.Add(20.0, map[string]string{"method": "POST"}, collectors.WithGroup("group1"))
		collector.Add(5.0, map[string]string{"method": "GET"}, collectors.WithGroup("group1"))

		metrics := collectGaugeMetrics(t, collector)
		require.Len(t, metrics, 2)

		// Sort by value for deterministic testing
		sort.Slice(metrics, func(i, j int) bool {
			return getGaugeValue(t, metrics[i]) < getGaugeValue(t, metrics[j])
		})

		verifyGaugeValue(t, metrics[0], 15.0) // GET: 10 + 5
		verifyGaugeValue(t, metrics[1], 20.0) // POST: 20
	})

	t.Run("add with zero value", func(t *testing.T) {
		collector := collectors.NewConstGaugeCollector(&collectors.MetricDescription{
			Name:       "test_gauge",
			LabelNames: []string{},
		})

		collector.Add(0.0, nil, collectors.WithGroup("group1"))

		metrics := collectGaugeMetrics(t, collector)
		require.Len(t, metrics, 1)
		verifyGaugeValue(t, metrics[0], 0.0)
	})

	t.Run("add negative values", func(t *testing.T) {
		collector := collectors.NewConstGaugeCollector(&collectors.MetricDescription{
			Name:       "test_gauge",
			LabelNames: []string{},
		})

		collector.Add(10.0, nil, collectors.WithGroup("group1"))
		collector.Add(-15.0, nil, collectors.WithGroup("group1"))

		metrics := collectGaugeMetrics(t, collector)
		require.Len(t, metrics, 1)
		verifyGaugeValue(t, metrics[0], -5.0)
	})
}

func TestConstGaugeCollector_MixedOperations(t *testing.T) {
	t.Run("set then add", func(t *testing.T) {
		collector := collectors.NewConstGaugeCollector(&collectors.MetricDescription{
			Name:       "test_gauge",
			LabelNames: []string{"method"},
		})

		collector.Set(100.0, map[string]string{"method": "GET"}, collectors.WithGroup("group1"))
		collector.Add(50.0, map[string]string{"method": "GET"}, collectors.WithGroup("group1"))

		metrics := collectGaugeMetrics(t, collector)
		require.Len(t, metrics, 1)
		verifyGaugeValue(t, metrics[0], 150.0)
	})

	t.Run("add then set", func(t *testing.T) {
		collector := collectors.NewConstGaugeCollector(&collectors.MetricDescription{
			Name:       "test_gauge",
			LabelNames: []string{"method"},
		})

		collector.Add(100.0, map[string]string{"method": "GET"}, collectors.WithGroup("group1"))
		collector.Set(50.0, map[string]string{"method": "GET"}, collectors.WithGroup("group1"))

		metrics := collectGaugeMetrics(t, collector)
		require.Len(t, metrics, 1)
		verifyGaugeValue(t, metrics[0], 50.0) // Set overwrites previous value
	})

	t.Run("complex sequence", func(t *testing.T) {
		collector := collectors.NewConstGaugeCollector(&collectors.MetricDescription{
			Name:       "test_gauge",
			LabelNames: []string{"method"},
		})

		collector.Set(10.0, map[string]string{"method": "GET"}, collectors.WithGroup("group1"))
		collector.Add(5.0, map[string]string{"method": "GET"}, collectors.WithGroup("group1"))
		collector.Add(-3.0, map[string]string{"method": "GET"}, collectors.WithGroup("group1"))
		collector.Set(20.0, map[string]string{"method": "GET"}, collectors.WithGroup("group1"))
		collector.Add(2.5, map[string]string{"method": "GET"}, collectors.WithGroup("group1"))

		metrics := collectGaugeMetrics(t, collector)
		require.Len(t, metrics, 1)
		verifyGaugeValue(t, metrics[0], 22.5) // Set to 20, then add 2.5
	})
}

func TestConstGaugeCollector_EdgeCases(t *testing.T) {
	t.Run("very large values", func(t *testing.T) {
		collector := collectors.NewConstGaugeCollector(&collectors.MetricDescription{
			Name:       "test_gauge",
			LabelNames: []string{},
		})

		largeValue := math.MaxFloat64
		collector.Set(largeValue, nil, collectors.WithGroup("group1"))

		metrics := collectGaugeMetrics(t, collector)
		require.Len(t, metrics, 1)
		verifyGaugeValue(t, metrics[0], largeValue)
	})

	t.Run("very small values", func(t *testing.T) {
		collector := collectors.NewConstGaugeCollector(&collectors.MetricDescription{
			Name:       "test_gauge",
			LabelNames: []string{},
		})

		smallValue := math.SmallestNonzeroFloat64
		collector.Set(smallValue, nil, collectors.WithGroup("group1"))

		metrics := collectGaugeMetrics(t, collector)
		require.Len(t, metrics, 1)
		verifyGaugeValue(t, metrics[0], smallValue)
	})

	t.Run("precision preservation", func(t *testing.T) {
		collector := collectors.NewConstGaugeCollector(&collectors.MetricDescription{
			Name:       "test_gauge",
			LabelNames: []string{},
		})

		preciseValue := 3.141592653589793238462643383279502884197
		collector.Set(preciseValue, nil, collectors.WithGroup("group1"))

		metrics := collectGaugeMetrics(t, collector)
		require.Len(t, metrics, 1)
		verifyGaugeValue(t, metrics[0], preciseValue)
	})
}

func TestConstGaugeCollector_Concurrency(t *testing.T) {
	t.Run("concurrent sets on different metrics", func(t *testing.T) {
		collector := collectors.NewConstGaugeCollector(&collectors.MetricDescription{
			Name:       "test_gauge",
			LabelNames: []string{"worker"},
		})

		numWorkers := 10
		var wg sync.WaitGroup

		for i := 0; i < numWorkers; i++ {
			wg.Add(1)
			go func(workerID int) {
				defer wg.Done()
				value := float64(workerID * 10)
				collector.Set(value, map[string]string{"worker": fmt.Sprintf("worker_%d", workerID)}, collectors.WithGroup("group1"))
			}(i)
		}

		wg.Wait()

		metrics := collectGaugeMetrics(t, collector)
		require.Len(t, metrics, numWorkers)

		// Verify each worker has correct value
		valueMap := make(map[string]float64)
		for _, metric := range metrics {
			labels := extractGaugeLabels(t, metric)
			value := getGaugeValue(t, metric)
			valueMap[labels["worker"]] = value
		}

		for i := 0; i < numWorkers; i++ {
			expectedValue := float64(i * 10)
			workerKey := fmt.Sprintf("worker_%d", i)
			assert.Equal(t, expectedValue, valueMap[workerKey])
		}
	})

	t.Run("concurrent adds on same metric", func(t *testing.T) {
		collector := collectors.NewConstGaugeCollector(&collectors.MetricDescription{
			Name:       "test_gauge",
			LabelNames: []string{"shared"},
		})

		numWorkers := 10
		incrementsPerWorker := 100
		var wg sync.WaitGroup

		for i := 0; i < numWorkers; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				for j := 0; j < incrementsPerWorker; j++ {
					collector.Add(1.0, map[string]string{"shared": "metric"}, collectors.WithGroup("group1"))
				}
			}()
		}

		wg.Wait()

		metrics := collectGaugeMetrics(t, collector)
		require.Len(t, metrics, 1)
		verifyGaugeValue(t, metrics[0], float64(numWorkers*incrementsPerWorker))
	})

	t.Run("concurrent mixed operations", func(t *testing.T) {
		collector := collectors.NewConstGaugeCollector(&collectors.MetricDescription{
			Name:       "test_gauge",
			LabelNames: []string{"operation"},
		})

		numWorkers := 5
		var wg sync.WaitGroup

		// Workers doing sets
		for i := 0; i < numWorkers; i++ {
			wg.Add(1)
			go func(workerID int) {
				defer wg.Done()
				for j := 0; j < 10; j++ {
					value := float64(workerID*10 + j)
					collector.Set(value, map[string]string{"operation": fmt.Sprintf("set_%d", workerID)}, collectors.WithGroup("group1"))
				}
			}(i)
		}

		// Workers doing adds
		for i := 0; i < numWorkers; i++ {
			wg.Add(1)
			go func(workerID int) {
				defer wg.Done()
				for j := 0; j < 10; j++ {
					collector.Add(1.0, map[string]string{"operation": fmt.Sprintf("add_%d", workerID)}, collectors.WithGroup("group1"))
				}
			}(i)
		}

		wg.Wait()

		metrics := collectGaugeMetrics(t, collector)
		require.Len(t, metrics, numWorkers*2) // 5 set + 5 add operations

		// Verify add operations accumulated correctly
		for _, metric := range metrics {
			labels := extractGaugeLabels(t, metric)
			operation := labels["operation"]
			value := getGaugeValue(t, metric)

			if operation[:3] == "add" {
				assert.Equal(t, 10.0, value) // Each add worker adds 1.0 ten times
			}
		}
	})
}

func TestConstGaugeCollector_ExpireGroupMetrics(t *testing.T) {
	collector := collectors.NewConstGaugeCollector(&collectors.MetricDescription{
		Name:       "test_gauge",
		LabelNames: []string{"method"},
	})

	// Add metrics for different groups
	collector.Set(10.0, map[string]string{"method": "GET"}, collectors.WithGroup("group1"))
	collector.Set(20.0, map[string]string{"method": "POST"}, collectors.WithGroup("group1"))
	collector.Set(30.0, map[string]string{"method": "DELETE"}, collectors.WithGroup("group2"))
	collector.Set(40.0, map[string]string{"method": "PUT"}, collectors.WithGroup("group2"))

	// Verify initial state
	metrics := collectGaugeMetrics(t, collector)
	require.Len(t, metrics, 4)

	// Expire group1
	collector.ExpireGroupMetrics("group1")

	// Verify only group2 metrics remain
	metrics = collectGaugeMetrics(t, collector)
	require.Len(t, metrics, 2)

	// Verify remaining metrics are from group2
	for _, metric := range metrics {
		labels := extractGaugeLabels(t, metric)
		method := labels["method"]
		assert.True(t, method == "DELETE" || method == "PUT")
	}
}

func TestConstGaugeCollector_ExpireNonExistentGroup(t *testing.T) {
	collector := collectors.NewConstGaugeCollector(&collectors.MetricDescription{
		Name:       "test_gauge",
		LabelNames: []string{"method"},
	})

	collector.Set(10.0, map[string]string{"method": "GET"}, collectors.WithGroup("group1"))

	// Expire non-existent group
	collector.ExpireGroupMetrics("non_existent")

	// Verify metric still exists
	metrics := collectGaugeMetrics(t, collector)
	require.Len(t, metrics, 1)
}

func TestConstGaugeCollector_ExpireEmptyCollection(t *testing.T) {
	collector := collectors.NewConstGaugeCollector(&collectors.MetricDescription{
		Name:       "test_gauge",
		LabelNames: []string{"method"},
	})

	// Expire from empty collection
	collector.ExpireGroupMetrics("any_group")

	// Should not panic and collection should remain empty
	metrics := collectGaugeMetrics(t, collector)
	require.Len(t, metrics, 0)
}

func TestConstGaugeCollector_UpdateLabels(t *testing.T) {
	t.Run("add new labels", func(t *testing.T) {
		collector := collectors.NewConstGaugeCollector(&collectors.MetricDescription{
			Name:       "test_gauge",
			LabelNames: []string{"method"},
		})

		// Add initial metric
		collector.Set(42.0, map[string]string{"method": "GET"}, collectors.WithGroup("group1"))

		// Update labels
		collector.UpdateLabels([]string{"status", "endpoint"})

		// Verify label names updated
		expectedLabels := []string{"endpoint", "method", "status"} // sorted
		actualLabels := collector.LabelNames()
		sort.Strings(actualLabels)
		assert.Equal(t, expectedLabels, actualLabels)

		// Verify existing metrics still work
		metrics := collectGaugeMetrics(t, collector)
		require.Len(t, metrics, 1)
		verifyGaugeValue(t, metrics[0], 42.0)
	})

	t.Run("add duplicate labels", func(t *testing.T) {
		collector := collectors.NewConstGaugeCollector(&collectors.MetricDescription{
			Name:       "test_gauge",
			LabelNames: []string{"method"},
		})

		collector.Set(42.0, map[string]string{"method": "GET"}, collectors.WithGroup("group1"))

		// Try to add existing label
		collector.UpdateLabels([]string{"method"})

		// Should not change
		assert.Equal(t, []string{"method"}, collector.LabelNames())

		metrics := collectGaugeMetrics(t, collector)
		require.Len(t, metrics, 1)
	})

	t.Run("update with empty labels", func(t *testing.T) {
		collector := collectors.NewConstGaugeCollector(&collectors.MetricDescription{
			Name:       "test_gauge",
			LabelNames: []string{"method"},
		})

		collector.UpdateLabels([]string{})

		// Should not change
		assert.Equal(t, []string{"method"}, collector.LabelNames())
	})

	t.Run("update preserves metric values", func(t *testing.T) {
		collector := collectors.NewConstGaugeCollector(&collectors.MetricDescription{
			Name:       "test_gauge",
			LabelNames: []string{"method"},
		})

		collector.Set(100.5, map[string]string{"method": "GET"}, collectors.WithGroup("group1"))
		collector.Set(200.7, map[string]string{"method": "POST"}, collectors.WithGroup("group1"))

		// Update labels
		collector.UpdateLabels([]string{"status"})

		// Verify metrics preserve their values
		metrics := collectGaugeMetrics(t, collector)
		require.Len(t, metrics, 2)

		values := make([]float64, len(metrics))
		for i, metric := range metrics {
			values[i] = getGaugeValue(t, metric)
		}
		sort.Float64s(values)

		assert.InDelta(t, 100.5, values[0], 0.001)
		assert.InDelta(t, 200.7, values[1], 0.001)
	})

	t.Run("update with complex label reordering", func(t *testing.T) {
		collector := collectors.NewConstGaugeCollector(&collectors.MetricDescription{
			Name:       "test_gauge",
			LabelNames: []string{"a", "b", "c"},
		})

		labels := map[string]string{"a": "val_a", "b": "val_b", "c": "val_c"}
		collector.Set(987.65, labels, collectors.WithGroup("group1"))

		// Add new labels that will change the sort order
		collector.UpdateLabels([]string{"z", "x"})

		// Verify values preserved despite reordering
		metrics := collectGaugeMetrics(t, collector)
		require.Len(t, metrics, 1)
		verifyGaugeValue(t, metrics[0], 987.65)

		// Verify new label structure
		expectedLabels := []string{"a", "b", "c", "x", "z"}
		actualLabels := collector.LabelNames()
		sort.Strings(actualLabels)
		assert.Equal(t, expectedLabels, actualLabels)
	})
}

func TestConstGaugeCollector_Describe(t *testing.T) {
	collector := collectors.NewConstGaugeCollector(&collectors.MetricDescription{
		Name:       "test_gauge",
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
	assert.Contains(t, desc.String(), "test_gauge")
}

func TestConstGaugeCollector_Collect(t *testing.T) {
	t.Run("collect empty", func(t *testing.T) {
		collector := collectors.NewConstGaugeCollector(&collectors.MetricDescription{
			Name:       "test_gauge",
			LabelNames: []string{},
		})

		metrics := collectGaugeMetrics(t, collector)
		assert.Len(t, metrics, 0)
	})

	t.Run("collect single metric", func(t *testing.T) {
		collector := collectors.NewConstGaugeCollector(&collectors.MetricDescription{
			Name:       "test_gauge",
			LabelNames: []string{"method"},
		})

		collector.Set(123.456, map[string]string{"method": "GET"}, collectors.WithGroup("group1"))

		metrics := collectGaugeMetrics(t, collector)
		require.Len(t, metrics, 1)
		verifyGaugeValue(t, metrics[0], 123.456)
	})

	t.Run("collect multiple metrics", func(t *testing.T) {
		collector := collectors.NewConstGaugeCollector(&collectors.MetricDescription{
			Name:       "test_gauge",
			LabelNames: []string{"method"},
		})

		collector.Set(10.1, map[string]string{"method": "GET"}, collectors.WithGroup("group1"))
		collector.Set(20.2, map[string]string{"method": "POST"}, collectors.WithGroup("group1"))
		collector.Set(30.3, map[string]string{"method": "PUT"}, collectors.WithGroup("group1"))

		metrics := collectGaugeMetrics(t, collector)
		require.Len(t, metrics, 3)

		totalValue := float64(0)
		for _, metric := range metrics {
			totalValue += getGaugeValue(t, metric)
		}
		assert.InDelta(t, 60.6, totalValue, 0.001)
	})
}

func TestConstGaugeCollector_InterfaceCompliance(t *testing.T) {
	// Test that ConstGaugeCollector implements ConstCollector interface
	var _ collectors.ConstCollector = (*collectors.ConstGaugeCollector)(nil)

	collector := collectors.NewConstGaugeCollector(&collectors.MetricDescription{
		Name:       "test_gauge",
		LabelNames: []string{},
	})

	// Test all interface methods
	assert.Equal(t, "test_gauge", collector.Name())
	assert.Equal(t, "gauge", collector.Type())
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

func TestConstGaugeCollector_LabelOrdering(t *testing.T) {
	// Test that label ordering is consistent
	collector := collectors.NewConstGaugeCollector(&collectors.MetricDescription{
		Name:       "test_gauge",
		LabelNames: []string{"z", "a", "m"},
	})

	collector.Set(42.0, map[string]string{"z": "val_z", "a": "val_a", "m": "val_m"}, collectors.WithGroup("group1"))

	metrics := collectGaugeMetrics(t, collector)
	require.Len(t, metrics, 1)

	// Verify labels are in the expected order (should match labelNames order)
	labels := extractGaugeLabels(t, metrics[0])
	assert.Equal(t, "val_z", labels["z"])
	assert.Equal(t, "val_a", labels["a"])
	assert.Equal(t, "val_m", labels["m"])
}

func TestConstGaugeCollector_GroupOperations(t *testing.T) {
	t.Run("operations with explicit groups", func(t *testing.T) {
		collector := collectors.NewConstGaugeCollector(&collectors.MetricDescription{
			Name:       "test_gauge",
			LabelNames: []string{"method"},
		})

		// Use WithGroup option
		collector.Set(10.0, map[string]string{"method": "GET"}, collectors.WithGroup("custom_group"))
		collector.Add(5.0, map[string]string{"method": "GET"}, collectors.WithGroup("custom_group"))

		metrics := collectGaugeMetrics(t, collector)
		require.Len(t, metrics, 1)
		verifyGaugeValue(t, metrics[0], 15.0)

		// Expire by the custom group
		collector.ExpireGroupMetrics("custom_group")

		metrics = collectGaugeMetrics(t, collector)
		require.Len(t, metrics, 0)
	})

	t.Run("different groups same labels", func(t *testing.T) {
		collector := collectors.NewConstGaugeCollector(&collectors.MetricDescription{
			Name:       "test_gauge",
			LabelNames: []string{"method"},
		})

		collector.Set(10.0, map[string]string{"method": "GET"}, collectors.WithGroup("group_a"))
		collector.Set(20.0, map[string]string{"method": "GET"}, collectors.WithGroup("group_b"))

		metrics := collectGaugeMetrics(t, collector)
		require.Len(t, metrics, 2)

		// Expire one group
		collector.ExpireGroupMetrics("group_a")

		metrics = collectGaugeMetrics(t, collector)
		require.Len(t, metrics, 1)
		verifyGaugeValue(t, metrics[0], 20.0)
	})
}

func TestConstGaugeCollector_MetricDescription(t *testing.T) {
	t.Run("empty metric description", func(t *testing.T) {
		collector := collectors.NewConstGaugeCollector(&collectors.MetricDescription{})

		assert.Equal(t, "", collector.Name())
		assert.Nil(t, collector.LabelNames())
		assert.Equal(t, "gauge", collector.Type())
	})

	t.Run("metric description with all fields", func(t *testing.T) {
		collector := collectors.NewConstGaugeCollector(&collectors.MetricDescription{
			Name:        "test_gauge",
			Help:        "Test gauge help",
			LabelNames:  []string{"method"},
			ConstLabels: map[string]string{"service": "api", "version": "v1"},
		})

		assert.Equal(t, "test_gauge", collector.Name())
		assert.Equal(t, []string{"method"}, collector.LabelNames())
	})
}

// Helper functions for blackbox testing

func collectGaugeMetrics(t *testing.T, collector *collectors.ConstGaugeCollector) []prometheus.Metric {
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

func verifyGaugeValue(t *testing.T, metric prometheus.Metric, expectedValue float64) {
	t.Helper()

	var dtoMetric dto.Metric
	err := metric.Write(&dtoMetric)
	require.NoError(t, err)

	gauge := dtoMetric.GetGauge()
	require.NotNil(t, gauge)

	if math.IsNaN(expectedValue) {
		assert.True(t, math.IsNaN(gauge.GetValue()))
	} else {
		assert.Equal(t, expectedValue, gauge.GetValue())
	}
}

func verifyGaugeLabels(t *testing.T, metric prometheus.Metric, expectedLabels map[string]string) {
	t.Helper()

	actualLabels := extractGaugeLabels(t, metric)
	assert.Equal(t, expectedLabels, actualLabels)
}

func getGaugeValue(t *testing.T, metric prometheus.Metric) float64 {
	t.Helper()

	var dtoMetric dto.Metric
	err := metric.Write(&dtoMetric)
	require.NoError(t, err)
	return dtoMetric.GetGauge().GetValue()
}

func extractGaugeLabels(t *testing.T, metric prometheus.Metric) map[string]string {
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
