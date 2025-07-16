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

func TestNewConstHistogramCollector(t *testing.T) {
	t.Run("with custom buckets", func(t *testing.T) {
		buckets := []float64{0.1, 1.0, 10.0}
		labelNames := []string{"label1", "label2"}
		collector := collectors.NewConstHistogramCollector("test_histogram", labelNames, buckets)

		assert.Equal(t, "test_histogram", collector.Name())
		assert.Equal(t, labelNames, collector.LabelNames())
		assert.Equal(t, buckets, collector.Buckets())
		assert.Equal(t, "histogram", collector.Type())
	})

	t.Run("with unsorted buckets", func(t *testing.T) {
		buckets := []float64{10.0, 0.1, 1.0}
		expected := []float64{0.1, 1.0, 10.0}
		collector := collectors.NewConstHistogramCollector("test_histogram", []string{}, buckets)

		assert.Equal(t, expected, collector.Buckets())
	})

	t.Run("with empty buckets", func(t *testing.T) {
		collector := collectors.NewConstHistogramCollector("test_histogram", []string{}, []float64{})

		assert.Equal(t, prometheus.DefBuckets, collector.Buckets())
	})

	t.Run("with nil buckets", func(t *testing.T) {
		collector := collectors.NewConstHistogramCollector("test_histogram", []string{}, nil)

		assert.Equal(t, prometheus.DefBuckets, collector.Buckets())
	})
}

func TestConstHistogramCollector_Observe(t *testing.T) {
	t.Run("basic observation", func(t *testing.T) {
		buckets := []float64{0.1, 1.0, 10.0}
		collector := collectors.NewConstHistogramCollector("test_histogram", []string{"method"}, buckets)

		collector.Observe(0.5, map[string]string{"method": "GET"}, collectors.WithGroup("group1"))

		metrics := collectHistogramMetrics(t, collector)
		require.Len(t, metrics, 1)

		verifyHistogramMetric(t, metrics[0], 1, 0.5, map[float64]uint64{
			0.1:  0, // 0.5 > 0.1
			1.0:  1, // 0.5 <= 1.0
			10.0: 1, // 0.5 <= 10.0
		})
	})

	t.Run("multiple observations same metric", func(t *testing.T) {
		buckets := []float64{0.1, 1.0, 10.0}
		collector := collectors.NewConstHistogramCollector("test_histogram", []string{"method"}, buckets)

		collector.Observe(0.05, map[string]string{"method": "GET"}, collectors.WithGroup("group1"))
		collector.Observe(0.5, map[string]string{"method": "GET"}, collectors.WithGroup("group1"))
		collector.Observe(5.0, map[string]string{"method": "GET"}, collectors.WithGroup("group1"))

		metrics := collectHistogramMetrics(t, collector)
		require.Len(t, metrics, 1)

		verifyHistogramMetric(t, metrics[0], 3, 5.55, map[float64]uint64{
			0.1:  1, // 0.05 <= 0.1
			1.0:  2, // 0.05, 0.5 <= 1.0
			10.0: 3, // all values <= 10.0
		})
	})

	t.Run("multiple observations different metrics", func(t *testing.T) {
		buckets := []float64{1.0, 10.0}
		collector := collectors.NewConstHistogramCollector("test_histogram", []string{"method"}, buckets)

		collector.Observe(0.5, map[string]string{"method": "GET"}, collectors.WithGroup("group1"))
		collector.Observe(5.0, map[string]string{"method": "POST"}, collectors.WithGroup("group1"))

		metrics := collectHistogramMetrics(t, collector)
		require.Len(t, metrics, 2)

		// Sort metrics by sum for deterministic testing
		sort.Slice(metrics, func(i, j int) bool {
			return getHistogramSum(t, metrics[i]) < getHistogramSum(t, metrics[j])
		})

		verifyHistogramMetric(t, metrics[0], 1, 0.5, map[float64]uint64{1.0: 1, 10.0: 1})
		verifyHistogramMetric(t, metrics[1], 1, 5.0, map[float64]uint64{1.0: 0, 10.0: 1})
	})

	t.Run("edge case values", func(t *testing.T) {
		buckets := []float64{1.0, 10.0}
		collector := collectors.NewConstHistogramCollector("test_histogram", []string{}, buckets)

		// Test exact bucket boundary
		collector.Observe(1.0, nil, collectors.WithGroup("group1"))
		// Test zero
		collector.Observe(0.0, nil, collectors.WithGroup("group1"))
		// Test negative
		collector.Observe(-1.0, nil, collectors.WithGroup("group1"))
		// Test very large value
		collector.Observe(100.0, nil, collectors.WithGroup("group1"))

		metrics := collectHistogramMetrics(t, collector)
		require.Len(t, metrics, 1)

		verifyHistogramMetric(t, metrics[0], 4, 100.0, map[float64]uint64{
			1.0:  3, // -1.0, 0.0, 1.0 <= 1.0
			10.0: 3, // 100.0 > 10.0, so only the first 3
		})
	})

	t.Run("with empty labels", func(t *testing.T) {
		collector := collectors.NewConstHistogramCollector("test_histogram", []string{}, []float64{1.0})

		collector.Observe(0.5, map[string]string{}, collectors.WithGroup("group1"))
		collector.Observe(0.3, nil, collectors.WithGroup("group1"))

		metrics := collectHistogramMetrics(t, collector)
		require.Len(t, metrics, 1)
		verifyHistogramMetric(t, metrics[0], 2, 0.8, map[float64]uint64{1.0: 2})
	})
}

func TestConstHistogramCollector_Concurrency(t *testing.T) {
	collector := collectors.NewConstHistogramCollector("test_histogram", []string{"worker"}, []float64{1.0, 10.0})

	numWorkers := 10
	observationsPerWorker := 100
	var wg sync.WaitGroup

	for i := 0; i < numWorkers; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			for j := 0; j < observationsPerWorker; j++ {
				value := float64(j % 5)
				collector.Observe(value, map[string]string{
					"worker": fmt.Sprintf("worker_%d", workerID),
				}, collectors.WithGroup("group1"))
			}
		}(i)
	}

	wg.Wait()

	metrics := collectHistogramMetrics(t, collector)
	require.Len(t, metrics, numWorkers)

	totalCount := uint64(0)
	for _, metric := range metrics {
		count := getHistogramCount(t, metric)
		assert.Equal(t, uint64(observationsPerWorker), count)
		totalCount += count
	}

	assert.Equal(t, uint64(numWorkers*observationsPerWorker), totalCount)
}

func TestConstHistogramCollector_ExpireGroupMetrics(t *testing.T) {
	collector := collectors.NewConstHistogramCollector("test_histogram", []string{"method"}, []float64{1.0})

	collector.Observe(0.5, map[string]string{"method": "GET"}, collectors.WithGroup("group1"))
	collector.Observe(1.5, map[string]string{"method": "POST"}, collectors.WithGroup("group2"))
	collector.Observe(0.3, map[string]string{"method": "PUT"}, collectors.WithGroup("group1"))

	// Verify initial state
	metrics := collectHistogramMetrics(t, collector)
	require.Len(t, metrics, 3)

	// Expire group1
	collector.ExpireGroupMetrics("group1")

	// Verify group1 metrics are gone
	metrics = collectHistogramMetrics(t, collector)
	require.Len(t, metrics, 1)

	// Verify remaining metric is from group2
	labels := extractHistogramLabels(t, metrics[0])
	assert.Equal(t, "POST", labels["method"])
}

func TestConstHistogramCollector_UpdateLabels(t *testing.T) {
	t.Run("add new labels", func(t *testing.T) {
		collector := collectors.NewConstHistogramCollector("test_histogram", []string{"method"}, []float64{1.0})

		collector.Observe(0.5, map[string]string{"method": "GET"}, collectors.WithGroup("group1"))

		// Add new label
		collector.UpdateLabels([]string{"status"})

		expectedLabels := []string{"method", "status"}
		sort.Strings(expectedLabels)
		actualLabels := collector.LabelNames()
		sort.Strings(actualLabels)

		assert.Equal(t, expectedLabels, actualLabels)

		// Verify existing metrics still work
		metrics := collectHistogramMetrics(t, collector)
		require.Len(t, metrics, 1)
	})

	t.Run("no change when same labels", func(t *testing.T) {
		collector := collectors.NewConstHistogramCollector("test_histogram", []string{"method"}, []float64{1.0})

		collector.Observe(0.5, map[string]string{"method": "GET"}, collectors.WithGroup("group1"))

		// Try to add existing label
		collector.UpdateLabels([]string{"method"})

		assert.Equal(t, []string{"method"}, collector.LabelNames())

		metrics := collectHistogramMetrics(t, collector)
		require.Len(t, metrics, 1)
	})

	t.Run("update with empty labels", func(t *testing.T) {
		collector := collectors.NewConstHistogramCollector("test_histogram", []string{"method"}, []float64{1.0})

		collector.UpdateLabels([]string{})

		assert.Equal(t, []string{"method"}, collector.LabelNames())
	})
}

func TestConstHistogramCollector_UpdateBuckets(t *testing.T) {
	t.Run("update to new buckets", func(t *testing.T) {
		collector := collectors.NewConstHistogramCollector("test_histogram", []string{}, []float64{1.0, 10.0})

		collector.Observe(0.5, nil, collectors.WithGroup("group1"))
		collector.Observe(5.0, nil, collectors.WithGroup("group1"))

		// Update buckets
		newBuckets := []float64{0.1, 1.0, 5.0, 50.0}
		collector.UpdateBuckets(newBuckets)

		assert.Equal(t, newBuckets, collector.Buckets())

		// Verify metrics still exist (though bucket counts may be approximated)
		metrics := collectHistogramMetrics(t, collector)
		require.Len(t, metrics, 1)
		assert.Equal(t, uint64(2), getHistogramCount(t, metrics[0]))
		assert.Equal(t, 5.5, getHistogramSum(t, metrics[0]))
	})

	t.Run("update with unsorted buckets", func(t *testing.T) {
		collector := collectors.NewConstHistogramCollector("test_histogram", []string{}, []float64{1.0})

		unsortedBuckets := []float64{10.0, 1.0, 5.0}
		expectedBuckets := []float64{1.0, 5.0, 10.0}

		collector.UpdateBuckets(unsortedBuckets)

		assert.Equal(t, expectedBuckets, collector.Buckets())
	})

	t.Run("no change when same buckets", func(t *testing.T) {
		buckets := []float64{1.0, 10.0}
		collector := collectors.NewConstHistogramCollector("test_histogram", []string{}, buckets)

		collector.Observe(0.5, nil, collectors.WithGroup("group1"))

		// Update with same buckets
		collector.UpdateBuckets(buckets)

		assert.Equal(t, buckets, collector.Buckets())

		metrics := collectHistogramMetrics(t, collector)
		require.Len(t, metrics, 1)
	})

	t.Run("update with empty buckets", func(t *testing.T) {
		collector := collectors.NewConstHistogramCollector("test_histogram", []string{}, []float64{1.0})

		collector.UpdateBuckets([]float64{})

		// Should not change
		assert.Equal(t, []float64{1.0}, collector.Buckets())
	})
}
func TestConstHistogramCollector_Reset(t *testing.T) {
	collector := collectors.NewConstHistogramCollector("test_histogram", []string{"method"}, []float64{1.0, 10.0})

	collector.Observe(0.5, map[string]string{"method": "GET"}, collectors.WithGroup("group1"))
	collector.Observe(5.0, map[string]string{"method": "GET"}, collectors.WithGroup("group1"))
	collector.Observe(1.5, map[string]string{"method": "POST"}, collectors.WithGroup("group1"))

	// Reset GET metric
	collector.Reset(map[string]string{"method": "GET"}, collectors.WithGroup("group1"))

	metrics := collectHistogramMetrics(t, collector)
	require.Len(t, metrics, 2)

	// Find the reset metric
	var resetMetric, unchangedMetric prometheus.Metric
	for _, metric := range metrics {
		labels := extractHistogramLabels(t, metric)
		if labels["method"] == "GET" {
			resetMetric = metric
		} else {
			unchangedMetric = metric
		}
	}

	require.NotNil(t, resetMetric)
	require.NotNil(t, unchangedMetric)

	// Verify reset metric
	verifyHistogramMetric(t, resetMetric, 0, 0.0, map[float64]uint64{1.0: 0, 10.0: 0})

	// Verify unchanged metric
	verifyHistogramMetric(t, unchangedMetric, 1, 1.5, map[float64]uint64{1.0: 0, 10.0: 1})
}

func TestConstHistogramCollector_GetObservationCount(t *testing.T) {
	collector := collectors.NewConstHistogramCollector("test_histogram", []string{"method"}, []float64{1.0})

	collector.Observe(0.5, map[string]string{"method": "GET"}, collectors.WithGroup("group1"))
	collector.Observe(1.5, map[string]string{"method": "GET"}, collectors.WithGroup("group1"))

	count := collector.GetObservationCount(map[string]string{"method": "GET"}, collectors.WithGroup("group1"))
	assert.Equal(t, uint64(2), count)

	// Non-existent metric
	count = collector.GetObservationCount(map[string]string{"method": "POST"}, collectors.WithGroup("group1"))
	assert.Equal(t, uint64(0), count)

	// Non-existent group
	count = collector.GetObservationCount(map[string]string{"method": "GET"}, collectors.WithGroup("group2"))
	assert.Equal(t, uint64(0), count)
}

func TestConstHistogramCollector_GetSum(t *testing.T) {
	collector := collectors.NewConstHistogramCollector("test_histogram", []string{"method"}, []float64{1.0})

	collector.Observe(0.5, map[string]string{"method": "GET"}, collectors.WithGroup("group1"))
	collector.Observe(1.5, map[string]string{"method": "GET"}, collectors.WithGroup("group1"))

	sum := collector.GetSum(map[string]string{"method": "GET"}, collectors.WithGroup("group1"))
	assert.Equal(t, 2.0, sum)

	// Non-existent metric
	sum = collector.GetSum(map[string]string{"method": "POST"}, collectors.WithGroup("group1"))
	assert.Equal(t, 0.0, sum)

	// Non-existent group
	sum = collector.GetSum(map[string]string{"method": "GET"}, collectors.WithGroup("group2"))
	assert.Equal(t, 0.0, sum)
}

func TestConstHistogramCollector_Describe(t *testing.T) {
	collector := collectors.NewConstHistogramCollector("test_histogram", []string{"method"}, []float64{1.0})

	ch := make(chan *prometheus.Desc, 1)
	collector.Describe(ch)
	close(ch)

	var desc *prometheus.Desc
	for d := range ch {
		desc = d
	}

	require.NotNil(t, desc)
	assert.Contains(t, desc.String(), "test_histogram")
}

func TestConstHistogramCollector_SpecialValues(t *testing.T) {
	collector := collectors.NewConstHistogramCollector("test_histogram", []string{}, []float64{1.0, 10.0})

	// Test special float values
	testCases := []struct {
		name  string
		value float64
	}{
		{"positive infinity", math.Inf(1)},
		{"negative infinity", math.Inf(-1)},
		{"NaN", math.NaN()},
		{"very large number", 1e308},
		{"very small number", 1e-308},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			collector.Observe(tc.value, nil, collectors.WithGroup("group1"))

			// Should not panic and should collect metrics
			metrics := collectHistogramMetrics(t, collector)
			require.Len(t, metrics, 1)
		})
	}
}

func TestConstHistogramCollector_InterfaceCompliance(t *testing.T) {
	// Test that ConstHistogramCollector implements ConstCollector interface
	var _ collectors.ConstCollector = (*collectors.ConstHistogramCollector)(nil)

	collector := collectors.NewConstHistogramCollector("test_histogram", []string{}, []float64{1.0})

	// Test all interface methods
	assert.Equal(t, "test_histogram", collector.Name())
	assert.Equal(t, "histogram", collector.Type())
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

// Helper functions for blackbox testing

func collectHistogramMetrics(t *testing.T, collector *collectors.ConstHistogramCollector) []prometheus.Metric {
	t.Helper()

	ch := make(chan prometheus.Metric, 100)
	collector.Collect(ch)
	close(ch)

	var metrics []prometheus.Metric
	for metric := range ch {
		metrics = append(metrics, metric)
	}
	return metrics
}

func verifyHistogramMetric(t *testing.T, metric prometheus.Metric, expectedCount uint64, expectedSum float64, expectedBuckets map[float64]uint64) {
	t.Helper()

	var dtoMetric dto.Metric
	err := metric.Write(&dtoMetric)
	require.NoError(t, err)

	histogram := dtoMetric.GetHistogram()
	require.NotNil(t, histogram)

	assert.Equal(t, expectedCount, histogram.GetSampleCount())
	assert.InDelta(t, expectedSum, histogram.GetSampleSum(), 0.0001)

	actualBuckets := make(map[float64]uint64)
	for _, bucket := range histogram.GetBucket() {
		actualBuckets[bucket.GetUpperBound()] = bucket.GetCumulativeCount()
	}

	assert.Equal(t, expectedBuckets, actualBuckets)
}

func getHistogramCount(t *testing.T, metric prometheus.Metric) uint64 {
	t.Helper()

	var dtoMetric dto.Metric
	err := metric.Write(&dtoMetric)
	require.NoError(t, err)
	return dtoMetric.GetHistogram().GetSampleCount()
}

func getHistogramSum(t *testing.T, metric prometheus.Metric) float64 {
	t.Helper()

	var dtoMetric dto.Metric
	err := metric.Write(&dtoMetric)
	require.NoError(t, err)
	return dtoMetric.GetHistogram().GetSampleSum()
}

func extractHistogramLabels(t *testing.T, metric prometheus.Metric) map[string]string {
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
