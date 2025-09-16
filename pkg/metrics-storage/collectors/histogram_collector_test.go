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
	t.Run("basic constructor", func(t *testing.T) {
		name := "test_histogram"
		labelNames := []string{"method", "status"}
		buckets := []float64{0.1, 0.5, 1.0, 5.0}

		collector := collectors.NewConstHistogramCollector(&collectors.MetricDescription{
			Name:       name,
			LabelNames: labelNames,
		}, buckets)

		assert.Equal(t, name, collector.Name())
		assert.Equal(t, labelNames, collector.LabelNames())
		assert.Equal(t, "histogram", collector.Type())
		assert.Equal(t, buckets, collector.Buckets())
	})

	t.Run("with empty buckets uses default", func(t *testing.T) {
		collector := collectors.NewConstHistogramCollector(&collectors.MetricDescription{
			Name:       "test_histogram",
			LabelNames: []string{},
		}, []float64{})

		assert.Equal(t, prometheus.DefBuckets, collector.Buckets())
	})

	t.Run("with nil buckets uses default", func(t *testing.T) {
		collector := collectors.NewConstHistogramCollector(&collectors.MetricDescription{
			Name:       "test_histogram",
			LabelNames: []string{},
		}, nil)

		assert.Equal(t, prometheus.DefBuckets, collector.Buckets())
	})

	t.Run("buckets are sorted", func(t *testing.T) {
		unsortedBuckets := []float64{5.0, 0.1, 1.0, 0.5}
		expectedBuckets := []float64{0.1, 0.5, 1.0, 5.0}

		collector := collectors.NewConstHistogramCollector(&collectors.MetricDescription{
			Name:       "test_histogram",
			LabelNames: []string{},
		}, unsortedBuckets)

		assert.Equal(t, expectedBuckets, collector.Buckets())
	})

	t.Run("with single bucket", func(t *testing.T) {
		buckets := []float64{1.0}
		collector := collectors.NewConstHistogramCollector(&collectors.MetricDescription{
			Name:       "test_histogram",
			LabelNames: []string{},
		}, buckets)

		assert.Equal(t, buckets, collector.Buckets())
	})

	t.Run("with duplicate buckets", func(t *testing.T) {
		buckets := []float64{1.0, 1.0, 2.0, 1.0}
		expectedBuckets := []float64{1.0, 1.0, 1.0, 2.0} // sorted but duplicates preserved

		collector := collectors.NewConstHistogramCollector(&collectors.MetricDescription{
			Name:       "test_histogram",
			LabelNames: []string{},
		}, buckets)

		assert.Equal(t, expectedBuckets, collector.Buckets())
	})

	t.Run("with help text", func(t *testing.T) {
		collector := collectors.NewConstHistogramCollector(&collectors.MetricDescription{
			Name:       "test_histogram",
			Help:       "Test histogram help text",
			LabelNames: []string{"method"},
		}, []float64{1.0})

		assert.Equal(t, "test_histogram", collector.Name())
		assert.Equal(t, []string{"method"}, collector.LabelNames())
	})

	t.Run("with const labels", func(t *testing.T) {
		collector := collectors.NewConstHistogramCollector(&collectors.MetricDescription{
			Name:        "test_histogram",
			LabelNames:  []string{"method"},
			ConstLabels: map[string]string{"service": "api"},
		}, []float64{1.0})

		assert.Equal(t, "test_histogram", collector.Name())
		assert.Equal(t, []string{"method"}, collector.LabelNames())
	})

	t.Run("empty metric description", func(t *testing.T) {
		collector := collectors.NewConstHistogramCollector(&collectors.MetricDescription{}, []float64{1.0})

		assert.Equal(t, "", collector.Name())
		assert.Nil(t, collector.LabelNames())
		assert.Equal(t, "histogram", collector.Type())
	})
}

func TestConstHistogramCollector_Observe(t *testing.T) {
	t.Run("basic observe operation", func(t *testing.T) {
		buckets := []float64{0.5, 1.0, 2.0}
		collector := collectors.NewConstHistogramCollector(&collectors.MetricDescription{
			Name:       "test_histogram",
			LabelNames: []string{"method"},
		}, buckets)

		collector.Observe(0.8, map[string]string{"method": "GET"}, collectors.WithGroup("group1"))

		metrics := collectHistogramMetrics(t, collector)
		require.Len(t, metrics, 1)

		histData := extractHistogramData(t, metrics[0])
		assert.Equal(t, uint64(1), histData.Count)
		assert.Equal(t, 0.8, histData.Sum)

		// Value 0.8 should be in buckets 1.0 and 2.0 (not in 0.5)
		expectedBuckets := map[float64]uint64{0.5: 0, 1.0: 1, 2.0: 1}
		assert.Equal(t, expectedBuckets, histData.Buckets)
	})

	t.Run("observe multiple values same metric", func(t *testing.T) {
		buckets := []float64{1.0, 2.0, 5.0}
		collector := collectors.NewConstHistogramCollector(&collectors.MetricDescription{
			Name:       "test_histogram",
			LabelNames: []string{"method"},
		}, buckets)

		collector.Observe(0.5, map[string]string{"method": "GET"}, collectors.WithGroup("group1"))
		collector.Observe(1.5, map[string]string{"method": "GET"}, collectors.WithGroup("group1"))
		collector.Observe(3.0, map[string]string{"method": "GET"}, collectors.WithGroup("group1"))

		metrics := collectHistogramMetrics(t, collector)
		require.Len(t, metrics, 1)

		histData := extractHistogramData(t, metrics[0])
		assert.Equal(t, uint64(3), histData.Count)
		assert.Equal(t, 5.0, histData.Sum) // 0.5 + 1.5 + 3.0

		expectedBuckets := map[float64]uint64{
			1.0: 1, // 0.5
			2.0: 2, // 0.5, 1.5
			5.0: 3, // 0.5, 1.5, 3.0
		}
		assert.Equal(t, expectedBuckets, histData.Buckets)
	})

	t.Run("observe different metrics", func(t *testing.T) {
		buckets := []float64{1.0, 2.0}
		collector := collectors.NewConstHistogramCollector(&collectors.MetricDescription{
			Name:       "test_histogram",
			LabelNames: []string{"method"},
		}, buckets)

		collector.Observe(0.5, map[string]string{"method": "GET"}, collectors.WithGroup("group1"))
		collector.Observe(1.5, map[string]string{"method": "POST"}, collectors.WithGroup("group1"))

		metrics := collectHistogramMetrics(t, collector)
		require.Len(t, metrics, 2)

		// Sort metrics by sum for deterministic testing
		sort.Slice(metrics, func(i, j int) bool {
			return extractHistogramData(t, metrics[i]).Sum < extractHistogramData(t, metrics[j]).Sum
		})

		// First metric (GET)
		histData1 := extractHistogramData(t, metrics[0])
		assert.Equal(t, uint64(1), histData1.Count)
		assert.Equal(t, 0.5, histData1.Sum)

		// Second metric (POST)
		histData2 := extractHistogramData(t, metrics[1])
		assert.Equal(t, uint64(1), histData2.Count)
		assert.Equal(t, 1.5, histData2.Sum)
	})

	t.Run("observe value exactly on bucket boundary", func(t *testing.T) {
		buckets := []float64{1.0, 2.0}
		collector := collectors.NewConstHistogramCollector(&collectors.MetricDescription{
			Name:       "test_histogram",
			LabelNames: []string{},
		}, buckets)

		collector.Observe(1.0, nil, collectors.WithGroup("group1"))

		metrics := collectHistogramMetrics(t, collector)
		require.Len(t, metrics, 1)

		histData := extractHistogramData(t, metrics[0])
		expectedBuckets := map[float64]uint64{1.0: 1, 2.0: 1} // Should be in both buckets
		assert.Equal(t, expectedBuckets, histData.Buckets)
	})

	t.Run("observe value larger than all buckets", func(t *testing.T) {
		buckets := []float64{1.0, 2.0}
		collector := collectors.NewConstHistogramCollector(&collectors.MetricDescription{
			Name:       "test_histogram",
			LabelNames: []string{},
		}, buckets)

		collector.Observe(5.0, nil, collectors.WithGroup("group1"))

		metrics := collectHistogramMetrics(t, collector)
		require.Len(t, metrics, 1)

		histData := extractHistogramData(t, metrics[0])
		expectedBuckets := map[float64]uint64{1.0: 0, 2.0: 0} // Should be in no buckets
		assert.Equal(t, expectedBuckets, histData.Buckets)
		assert.Equal(t, uint64(1), histData.Count)
		assert.Equal(t, 5.0, histData.Sum)
	})

	t.Run("observe negative value", func(t *testing.T) {
		buckets := []float64{0.0, 1.0}
		collector := collectors.NewConstHistogramCollector(&collectors.MetricDescription{
			Name:       "test_histogram",
			LabelNames: []string{},
		}, buckets)

		collector.Observe(-1.0, nil, collectors.WithGroup("group1"))

		metrics := collectHistogramMetrics(t, collector)
		require.Len(t, metrics, 1)

		histData := extractHistogramData(t, metrics[0])
		expectedBuckets := map[float64]uint64{0.0: 0, 1.0: 0} // Negative value in no buckets
		assert.Equal(t, expectedBuckets, histData.Buckets)
		assert.Equal(t, uint64(1), histData.Count)
		assert.Equal(t, -1.0, histData.Sum)
	})

	t.Run("observe zero value", func(t *testing.T) {
		buckets := []float64{0.0, 1.0}
		collector := collectors.NewConstHistogramCollector(&collectors.MetricDescription{
			Name:       "test_histogram",
			LabelNames: []string{},
		}, buckets)

		collector.Observe(0.0, nil, collectors.WithGroup("group1"))

		metrics := collectHistogramMetrics(t, collector)
		require.Len(t, metrics, 1)

		histData := extractHistogramData(t, metrics[0])
		expectedBuckets := map[float64]uint64{0.0: 1, 1.0: 1} // Zero should be in both buckets
		assert.Equal(t, expectedBuckets, histData.Buckets)
	})

	t.Run("observe with missing label values", func(t *testing.T) {
		collector := collectors.NewConstHistogramCollector(&collectors.MetricDescription{
			Name:       "test_histogram",
			LabelNames: []string{"method", "status"},
		}, []float64{1.0})

		// Only provide one label value
		collector.Observe(0.5, map[string]string{"method": "GET"}, collectors.WithGroup("group1"))

		metrics := collectHistogramMetrics(t, collector)
		require.Len(t, metrics, 1)

		histData := extractHistogramData(t, metrics[0])
		assert.Equal(t, "GET", histData.Labels["method"])
		assert.Equal(t, "", histData.Labels["status"]) // Missing label should be empty string
	})

	t.Run("observe with extra label values", func(t *testing.T) {
		collector := collectors.NewConstHistogramCollector(&collectors.MetricDescription{
			Name:       "test_histogram",
			LabelNames: []string{"method"},
		}, []float64{1.0})

		// Provide extra labels that aren't in labelNames
		labels := map[string]string{
			"method": "GET",
			"extra":  "ignored",
		}

		collector.Observe(0.5, labels, collectors.WithGroup("group1"))

		metrics := collectHistogramMetrics(t, collector)
		require.Len(t, metrics, 1)

		histData := extractHistogramData(t, metrics[0])
		assert.Equal(t, "GET", histData.Labels["method"])
		assert.NotContains(t, histData.Labels, "extra")
	})
}

func TestConstHistogramCollector_EdgeCases(t *testing.T) {
	t.Run("observe infinity", func(t *testing.T) {
		buckets := []float64{1.0, math.Inf(1)}
		collector := collectors.NewConstHistogramCollector(&collectors.MetricDescription{
			Name:       "test_histogram",
			LabelNames: []string{},
		}, buckets)

		collector.Observe(math.Inf(1), nil, collectors.WithGroup("group1"))

		metrics := collectHistogramMetrics(t, collector)
		require.Len(t, metrics, 1)

		histData := extractHistogramData(t, metrics[0])
		assert.Equal(t, uint64(1), histData.Count)
		assert.True(t, math.IsInf(histData.Sum, 1))
	})

	t.Run("observe NaN", func(t *testing.T) {
		buckets := []float64{1.0, 2.0}
		collector := collectors.NewConstHistogramCollector(&collectors.MetricDescription{
			Name:       "test_histogram",
			LabelNames: []string{},
		}, buckets)

		collector.Observe(math.NaN(), nil, collectors.WithGroup("group1"))

		metrics := collectHistogramMetrics(t, collector)
		require.Len(t, metrics, 1)

		histData := extractHistogramData(t, metrics[0])
		assert.Equal(t, uint64(1), histData.Count)
		assert.True(t, math.IsNaN(histData.Sum))
	})

	t.Run("observe very large value", func(t *testing.T) {
		buckets := []float64{1.0, math.MaxFloat64}
		collector := collectors.NewConstHistogramCollector(&collectors.MetricDescription{
			Name:       "test_histogram",
			LabelNames: []string{},
		}, buckets)

		collector.Observe(math.MaxFloat64/2, nil, collectors.WithGroup("group1"))

		metrics := collectHistogramMetrics(t, collector)
		require.Len(t, metrics, 1)

		histData := extractHistogramData(t, metrics[0])
		assert.Equal(t, uint64(1), histData.Count)
		assert.Equal(t, math.MaxFloat64/2, histData.Sum)
	})

	t.Run("observe very small value", func(t *testing.T) {
		buckets := []float64{math.SmallestNonzeroFloat64, 1.0}
		collector := collectors.NewConstHistogramCollector(&collectors.MetricDescription{
			Name:       "test_histogram",
			LabelNames: []string{},
		}, buckets)

		collector.Observe(math.SmallestNonzeroFloat64/2, nil, collectors.WithGroup("group1"))

		metrics := collectHistogramMetrics(t, collector)
		require.Len(t, metrics, 1)

		histData := extractHistogramData(t, metrics[0])
		assert.Equal(t, uint64(1), histData.Count)
		assert.Equal(t, math.SmallestNonzeroFloat64/2, histData.Sum)
	})
}

func TestConstHistogramCollector_Concurrency(t *testing.T) {
	t.Run("concurrent observations on different metrics", func(t *testing.T) {
		buckets := []float64{1.0, 2.0}
		collector := collectors.NewConstHistogramCollector(&collectors.MetricDescription{
			Name:       "test_histogram",
			LabelNames: []string{"worker"},
		}, buckets)

		numWorkers := 10
		var wg sync.WaitGroup

		for i := 0; i < numWorkers; i++ {
			wg.Add(1)
			go func(workerID int) {
				defer wg.Done()
				value := float64(workerID) * 0.1
				collector.Observe(value, map[string]string{"worker": fmt.Sprintf("worker_%d", workerID)}, collectors.WithGroup("group1"))
			}(i)
		}

		wg.Wait()

		metrics := collectHistogramMetrics(t, collector)
		assert.Len(t, metrics, numWorkers)
	})

	t.Run("concurrent observations on same metric", func(t *testing.T) {
		buckets := []float64{1.0, 2.0}
		collector := collectors.NewConstHistogramCollector(&collectors.MetricDescription{
			Name:       "test_histogram",
			LabelNames: []string{"shared"},
		}, buckets)

		numWorkers := 10
		observationsPerWorker := 100
		var wg sync.WaitGroup

		for i := 0; i < numWorkers; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				for j := 0; j < observationsPerWorker; j++ {
					collector.Observe(0.5, map[string]string{"shared": "metric"}, collectors.WithGroup("group1"))
				}
			}()
		}

		wg.Wait()

		metrics := collectHistogramMetrics(t, collector)
		require.Len(t, metrics, 1)

		histData := extractHistogramData(t, metrics[0])
		expectedCount := uint64(numWorkers * observationsPerWorker)
		expectedSum := float64(numWorkers*observationsPerWorker) * 0.5

		assert.Equal(t, expectedCount, histData.Count)
		assert.InDelta(t, expectedSum, histData.Sum, 0.001)
	})

	t.Run("concurrent mixed operations", func(t *testing.T) {
		buckets := []float64{1.0, 2.0}
		collector := collectors.NewConstHistogramCollector(&collectors.MetricDescription{
			Name:       "test_histogram",
			LabelNames: []string{"operation"},
		}, buckets)

		numWorkers := 5
		var wg sync.WaitGroup

		// Workers doing observations
		for i := 0; i < numWorkers; i++ {
			wg.Add(1)
			go func(workerID int) {
				defer wg.Done()
				for j := 0; j < 10; j++ {
					value := float64(workerID)*0.1 + float64(j)*0.01
					collector.Observe(value, map[string]string{"operation": fmt.Sprintf("observe_%d", workerID)}, collectors.WithGroup("group1"))
				}
			}(i)
		}

		// Workers updating buckets
		for i := 0; i < 2; i++ {
			wg.Add(1)
			go func(_ int) {
				defer wg.Done()
				newBuckets := []float64{0.5, 1.5, 2.5}
				collector.UpdateBuckets(newBuckets)
			}(i)
		}

		wg.Wait()

		metrics := collectHistogramMetrics(t, collector)
		assert.Len(t, metrics, numWorkers)
	})
}

func TestConstHistogramCollector_UpdateBuckets(t *testing.T) {
	t.Run("update to new buckets", func(t *testing.T) {
		initialBuckets := []float64{1.0, 2.0}
		collector := collectors.NewConstHistogramCollector(&collectors.MetricDescription{
			Name:       "test_histogram",
			LabelNames: []string{"method"},
		}, initialBuckets)

		// Add some observations
		collector.Observe(0.5, map[string]string{"method": "GET"}, collectors.WithGroup("group1"))
		collector.Observe(1.5, map[string]string{"method": "GET"}, collectors.WithGroup("group1"))

		// Update buckets
		newBuckets := []float64{0.5, 1.0, 3.0}
		collector.UpdateBuckets(newBuckets)

		assert.Equal(t, newBuckets, collector.Buckets())

		// Verify metrics still exist but bucket distribution may change
		metrics := collectHistogramMetrics(t, collector)
		require.Len(t, metrics, 1)

		histData := extractHistogramData(t, metrics[0])
		assert.Equal(t, uint64(2), histData.Count) // Count should be preserved
		assert.Equal(t, 2.0, histData.Sum)         // Sum should be preserved
	})

	t.Run("update with empty buckets does nothing", func(t *testing.T) {
		initialBuckets := []float64{1.0, 2.0}
		collector := collectors.NewConstHistogramCollector(&collectors.MetricDescription{
			Name:       "test_histogram",
			LabelNames: []string{},
		}, initialBuckets)

		collector.UpdateBuckets([]float64{})

		assert.Equal(t, initialBuckets, collector.Buckets())
	})

	t.Run("update with same buckets does nothing", func(t *testing.T) {
		buckets := []float64{1.0, 2.0, 3.0}
		collector := collectors.NewConstHistogramCollector(&collectors.MetricDescription{
			Name:       "test_histogram",
			LabelNames: []string{},
		}, buckets)

		collector.Observe(1.5, nil, collectors.WithGroup("group1"))

		// Update with same buckets
		collector.UpdateBuckets([]float64{1.0, 2.0, 3.0})

		assert.Equal(t, buckets, collector.Buckets())

		// Verify metrics unchanged
		metrics := collectHistogramMetrics(t, collector)
		require.Len(t, metrics, 1)
	})

	t.Run("update buckets sorts them", func(t *testing.T) {
		collector := collectors.NewConstHistogramCollector(&collectors.MetricDescription{
			Name:       "test_histogram",
			LabelNames: []string{},
		}, []float64{1.0})

		unsortedBuckets := []float64{5.0, 1.0, 3.0}
		expectedBuckets := []float64{1.0, 3.0, 5.0}

		collector.UpdateBuckets(unsortedBuckets)

		assert.Equal(t, expectedBuckets, collector.Buckets())
	})
}

func TestConstHistogramCollector_Reset(t *testing.T) {
	t.Run("reset existing metric", func(t *testing.T) {
		buckets := []float64{1.0, 2.0}
		collector := collectors.NewConstHistogramCollector(&collectors.MetricDescription{
			Name:       "test_histogram",
			LabelNames: []string{"method"},
		}, buckets)

		// Add observations
		collector.Observe(0.5, map[string]string{"method": "GET"}, collectors.WithGroup("group1"))
		collector.Observe(1.5, map[string]string{"method": "GET"}, collectors.WithGroup("group1"))

		// Verify data exists
		metrics := collectHistogramMetrics(t, collector)
		require.Len(t, metrics, 1)
		histData := extractHistogramData(t, metrics[0])
		assert.Equal(t, uint64(2), histData.Count)

		// Reset
		collector.Reset(map[string]string{"method": "GET"}, collectors.WithGroup("group1"))

		// Verify reset
		metrics = collectHistogramMetrics(t, collector)
		require.Len(t, metrics, 1)
		histData = extractHistogramData(t, metrics[0])
		assert.Equal(t, uint64(0), histData.Count)
		assert.Equal(t, 0.0, histData.Sum)

		expectedBuckets := map[float64]uint64{1.0: 0, 2.0: 0}
		assert.Equal(t, expectedBuckets, histData.Buckets)
	})

	t.Run("reset non-existent metric does nothing", func(t *testing.T) {
		collector := collectors.NewConstHistogramCollector(&collectors.MetricDescription{
			Name:       "test_histogram",
			LabelNames: []string{"method"},
		}, []float64{1.0})

		collector.Reset(map[string]string{"method": "GET"}, collectors.WithGroup("group1"))

		metrics := collectHistogramMetrics(t, collector)
		assert.Len(t, metrics, 0)
	})

	t.Run("reset wrong group does nothing", func(t *testing.T) {
		buckets := []float64{1.0}
		collector := collectors.NewConstHistogramCollector(&collectors.MetricDescription{
			Name:       "test_histogram",
			LabelNames: []string{"method"},
		}, buckets)

		collector.Observe(0.5, map[string]string{"method": "GET"}, collectors.WithGroup("group1"))

		// Reset with different group
		collector.Reset(map[string]string{"method": "GET"}, collectors.WithGroup("group2"))

		// Should still have original data
		metrics := collectHistogramMetrics(t, collector)
		require.Len(t, metrics, 1)
		histData := extractHistogramData(t, metrics[0])
		assert.Equal(t, uint64(1), histData.Count)
	})
}

func TestConstHistogramCollector_GetObservationCount(t *testing.T) {
	t.Run("get count for existing metric", func(t *testing.T) {
		collector := collectors.NewConstHistogramCollector(&collectors.MetricDescription{
			Name:       "test_histogram",
			LabelNames: []string{"method"},
		}, []float64{1.0})

		collector.Observe(0.5, map[string]string{"method": "GET"}, collectors.WithGroup("group1"))
		collector.Observe(0.7, map[string]string{"method": "GET"}, collectors.WithGroup("group1"))

		count := collector.GetObservationCount(map[string]string{"method": "GET"}, collectors.WithGroup("group1"))
		assert.Equal(t, uint64(2), count)
	})

	t.Run("get count for non-existent metric", func(t *testing.T) {
		collector := collectors.NewConstHistogramCollector(&collectors.MetricDescription{
			Name:       "test_histogram",
			LabelNames: []string{"method"},
		}, []float64{1.0})

		count := collector.GetObservationCount(map[string]string{"method": "GET"}, collectors.WithGroup("group1"))
		assert.Equal(t, uint64(0), count)
	})

	t.Run("get count with wrong group", func(t *testing.T) {
		collector := collectors.NewConstHistogramCollector(&collectors.MetricDescription{
			Name:       "test_histogram",
			LabelNames: []string{"method"},
		}, []float64{1.0})

		collector.Observe(0.5, map[string]string{"method": "GET"}, collectors.WithGroup("group1"))

		count := collector.GetObservationCount(map[string]string{"method": "GET"}, collectors.WithGroup("group2"))
		assert.Equal(t, uint64(0), count)
	})
}

func TestConstHistogramCollector_GetSum(t *testing.T) {
	t.Run("get sum for existing metric", func(t *testing.T) {
		collector := collectors.NewConstHistogramCollector(&collectors.MetricDescription{
			Name:       "test_histogram",
			LabelNames: []string{"method"},
		}, []float64{1.0})

		collector.Observe(0.5, map[string]string{"method": "GET"}, collectors.WithGroup("group1"))
		collector.Observe(1.5, map[string]string{"method": "GET"}, collectors.WithGroup("group1"))

		sum := collector.GetSum(map[string]string{"method": "GET"}, collectors.WithGroup("group1"))
		assert.Equal(t, 2.0, sum)
	})

	t.Run("get sum for non-existent metric", func(t *testing.T) {
		collector := collectors.NewConstHistogramCollector(&collectors.MetricDescription{
			Name:       "test_histogram",
			LabelNames: []string{"method"},
		}, []float64{1.0})

		sum := collector.GetSum(map[string]string{"method": "GET"}, collectors.WithGroup("group1"))
		assert.Equal(t, 0.0, sum)
	})

	t.Run("get sum with wrong group", func(t *testing.T) {
		collector := collectors.NewConstHistogramCollector(&collectors.MetricDescription{
			Name:       "test_histogram",
			LabelNames: []string{"method"},
		}, []float64{1.0})

		collector.Observe(0.5, map[string]string{"method": "GET"}, collectors.WithGroup("group1"))

		sum := collector.GetSum(map[string]string{"method": "GET"}, collectors.WithGroup("group2"))
		assert.Equal(t, 0.0, sum)
	})
}

func TestConstHistogramCollector_ExpireGroupMetrics(t *testing.T) {
	t.Run("expire existing group", func(t *testing.T) {
		collector := collectors.NewConstHistogramCollector(&collectors.MetricDescription{
			Name:       "test_histogram",
			LabelNames: []string{"method"},
		}, []float64{1.0})

		// Add metrics for different groups
		collector.Observe(0.5, map[string]string{"method": "GET"}, collectors.WithGroup("group1"))
		collector.Observe(1.5, map[string]string{"method": "POST"}, collectors.WithGroup("group1"))
		collector.Observe(2.5, map[string]string{"method": "DELETE"}, collectors.WithGroup("group2"))

		// Verify initial state
		metrics := collectHistogramMetrics(t, collector)
		require.Len(t, metrics, 3)

		// Expire group1
		collector.ExpireGroupMetrics("group1")

		// Verify only group2 remains
		metrics = collectHistogramMetrics(t, collector)
		require.Len(t, metrics, 1)
	})

	t.Run("expire non-existent group", func(t *testing.T) {
		collector := collectors.NewConstHistogramCollector(&collectors.MetricDescription{
			Name:       "test_histogram",
			LabelNames: []string{"method"},
		}, []float64{1.0})

		collector.Observe(0.5, map[string]string{"method": "GET"}, collectors.WithGroup("group1"))

		collector.ExpireGroupMetrics("non_existent")

		metrics := collectHistogramMetrics(t, collector)
		require.Len(t, metrics, 1)
	})

	t.Run("expire from empty collection", func(t *testing.T) {
		collector := collectors.NewConstHistogramCollector(&collectors.MetricDescription{
			Name:       "test_histogram",
			LabelNames: []string{"method"},
		}, []float64{1.0})

		collector.ExpireGroupMetrics("any_group")

		metrics := collectHistogramMetrics(t, collector)
		require.Len(t, metrics, 0)
	})
}

func TestConstHistogramCollector_UpdateLabels(t *testing.T) {
	t.Run("add new labels", func(t *testing.T) {
		collector := collectors.NewConstHistogramCollector(&collectors.MetricDescription{
			Name:       "test_histogram",
			LabelNames: []string{"method"},
		}, []float64{1.0})

		collector.Observe(0.5, map[string]string{"method": "GET"}, collectors.WithGroup("group1"))

		collector.UpdateLabels([]string{"status", "endpoint"})

		expectedLabels := []string{"endpoint", "method", "status"} // sorted
		actualLabels := collector.LabelNames()
		sort.Strings(actualLabels)
		assert.Equal(t, expectedLabels, actualLabels)

		// Verify existing metrics still work
		metrics := collectHistogramMetrics(t, collector)
		require.Len(t, metrics, 1)
	})

	t.Run("add duplicate labels", func(t *testing.T) {
		collector := collectors.NewConstHistogramCollector(&collectors.MetricDescription{
			Name:       "test_histogram",
			LabelNames: []string{"method"},
		}, []float64{1.0})

		collector.UpdateLabels([]string{"method"})

		assert.Equal(t, []string{"method"}, collector.LabelNames())
	})

	t.Run("update with empty labels", func(t *testing.T) {
		collector := collectors.NewConstHistogramCollector(&collectors.MetricDescription{
			Name:       "test_histogram",
			LabelNames: []string{"method"},
		}, []float64{1.0})

		collector.UpdateLabels([]string{})

		assert.Equal(t, []string{"method"}, collector.LabelNames())
	})

	t.Run("update preserves metric values", func(t *testing.T) {
		collector := collectors.NewConstHistogramCollector(&collectors.MetricDescription{
			Name:       "test_histogram",
			LabelNames: []string{"method"},
		}, []float64{1.0})

		collector.Observe(0.5, map[string]string{"method": "GET"}, collectors.WithGroup("group1"))
		collector.Observe(1.5, map[string]string{"method": "POST"}, collectors.WithGroup("group1"))

		collector.UpdateLabels([]string{"status"})

		metrics := collectHistogramMetrics(t, collector)
		require.Len(t, metrics, 2)

		totalSum := 0.0
		for _, metric := range metrics {
			histData := extractHistogramData(t, metric)
			totalSum += histData.Sum
		}
		assert.InDelta(t, 2.0, totalSum, 0.001) // 0.5 + 1.5
	})

	t.Run("update with complex label reordering", func(t *testing.T) {
		collector := collectors.NewConstHistogramCollector(&collectors.MetricDescription{
			Name:       "test_histogram",
			LabelNames: []string{"a", "b", "c"},
		}, []float64{1.0})

		labels := map[string]string{"a": "val_a", "b": "val_b", "c": "val_c"}
		collector.Observe(0.5, labels, collectors.WithGroup("group1"))

		// Add new labels that will change the sort order
		collector.UpdateLabels([]string{"z", "x"})

		// Verify values preserved despite reordering
		metrics := collectHistogramMetrics(t, collector)
		require.Len(t, metrics, 1)

		histData := extractHistogramData(t, metrics[0])
		assert.Equal(t, 0.5, histData.Sum)

		// Verify new label structure
		expectedLabels := []string{"a", "b", "c", "x", "z"}
		actualLabels := collector.LabelNames()
		sort.Strings(actualLabels)
		assert.Equal(t, expectedLabels, actualLabels)
	})
}

func TestConstHistogramCollector_Describe(t *testing.T) {
	collector := collectors.NewConstHistogramCollector(&collectors.MetricDescription{
		Name:       "test_histogram",
		LabelNames: []string{"method"},
	}, []float64{1.0})

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

func TestConstHistogramCollector_Collect(t *testing.T) {
	t.Run("collect empty", func(t *testing.T) {
		collector := collectors.NewConstHistogramCollector(&collectors.MetricDescription{
			Name:       "test_histogram",
			LabelNames: []string{},
		}, []float64{1.0})

		metrics := collectHistogramMetrics(t, collector)
		assert.Len(t, metrics, 0)
	})

	t.Run("collect single metric", func(t *testing.T) {
		collector := collectors.NewConstHistogramCollector(&collectors.MetricDescription{
			Name:       "test_histogram",
			LabelNames: []string{"method"},
		}, []float64{1.0})

		collector.Observe(0.5, map[string]string{"method": "GET"}, collectors.WithGroup("group1"))

		metrics := collectHistogramMetrics(t, collector)
		require.Len(t, metrics, 1)

		histData := extractHistogramData(t, metrics[0])
		assert.Equal(t, uint64(1), histData.Count)
		assert.Equal(t, 0.5, histData.Sum)
	})

	t.Run("collect multiple metrics", func(t *testing.T) {
		collector := collectors.NewConstHistogramCollector(&collectors.MetricDescription{
			Name:       "test_histogram",
			LabelNames: []string{"method"},
		}, []float64{1.0})

		collector.Observe(0.5, map[string]string{"method": "GET"}, collectors.WithGroup("group1"))
		collector.Observe(1.5, map[string]string{"method": "POST"}, collectors.WithGroup("group1"))

		metrics := collectHistogramMetrics(t, collector)
		require.Len(t, metrics, 2)

		totalSum := 0.0
		for _, metric := range metrics {
			histData := extractHistogramData(t, metric)
			totalSum += histData.Sum
		}
		assert.InDelta(t, 2.0, totalSum, 0.001)
	})
}

func TestConstHistogramCollector_InterfaceCompliance(t *testing.T) {
	// Test that ConstHistogramCollector implements ConstCollector interface
	var _ collectors.ConstCollector = (*collectors.ConstHistogramCollector)(nil)

	collector := collectors.NewConstHistogramCollector(&collectors.MetricDescription{
		Name:       "test_histogram",
		LabelNames: []string{},
	}, []float64{1.0})

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

func TestConstHistogramCollector_BucketAccess(t *testing.T) {
	t.Run("buckets method returns copy", func(t *testing.T) {
		originalBuckets := []float64{1.0, 2.0, 3.0}
		collector := collectors.NewConstHistogramCollector(&collectors.MetricDescription{
			Name:       "test_histogram",
			LabelNames: []string{},
		}, originalBuckets)

		returnedBuckets := collector.Buckets()
		returnedBuckets[0] = 999.0 // Modify returned slice

		// Original should be unchanged
		assert.Equal(t, originalBuckets, collector.Buckets())
	})
}

func TestConstHistogramCollector_GroupOperations(t *testing.T) {
	t.Run("operations with explicit groups", func(t *testing.T) {
		collector := collectors.NewConstHistogramCollector(&collectors.MetricDescription{
			Name:       "test_histogram",
			LabelNames: []string{"method"},
		}, []float64{1.0})

		collector.Observe(0.5, map[string]string{"method": "GET"}, collectors.WithGroup("custom_group"))
		collector.Observe(1.5, map[string]string{"method": "GET"}, collectors.WithGroup("custom_group"))

		metrics := collectHistogramMetrics(t, collector)
		require.Len(t, metrics, 1)

		histData := extractHistogramData(t, metrics[0])
		assert.Equal(t, uint64(2), histData.Count)
		assert.Equal(t, 2.0, histData.Sum)

		// Expire by the custom group
		collector.ExpireGroupMetrics("custom_group")

		metrics = collectHistogramMetrics(t, collector)
		require.Len(t, metrics, 0)
	})

	t.Run("different groups same labels", func(t *testing.T) {
		collector := collectors.NewConstHistogramCollector(&collectors.MetricDescription{
			Name:       "test_histogram",
			LabelNames: []string{"method"},
		}, []float64{1.0})

		collector.Observe(0.5, map[string]string{"method": "GET"}, collectors.WithGroup("group_a"))
		collector.Observe(1.5, map[string]string{"method": "GET"}, collectors.WithGroup("group_b"))

		metrics := collectHistogramMetrics(t, collector)
		require.Len(t, metrics, 2)

		// Expire one group
		collector.ExpireGroupMetrics("group_a")

		metrics = collectHistogramMetrics(t, collector)
		require.Len(t, metrics, 1)

		histData := extractHistogramData(t, metrics[0])
		assert.Equal(t, 1.5, histData.Sum)
	})
}

func TestConstHistogramCollector_MetricDescription(t *testing.T) {
	t.Run("empty metric description", func(t *testing.T) {
		collector := collectors.NewConstHistogramCollector(&collectors.MetricDescription{}, []float64{1.0})

		assert.Equal(t, "", collector.Name())
		assert.Nil(t, collector.LabelNames())
		assert.Equal(t, "histogram", collector.Type())
	})

	t.Run("metric description with all fields", func(t *testing.T) {
		collector := collectors.NewConstHistogramCollector(&collectors.MetricDescription{
			Name:        "test_histogram",
			Help:        "Test histogram help",
			LabelNames:  []string{"method"},
			ConstLabels: map[string]string{"service": "api", "version": "v1"},
		}, []float64{1.0})

		assert.Equal(t, "test_histogram", collector.Name())
		assert.Equal(t, []string{"method"}, collector.LabelNames())
	})
}

// Helper functions for blackbox testing

func collectHistogramMetrics(t *testing.T, collector *collectors.ConstHistogramCollector) []prometheus.Metric {
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

type HistogramData struct {
	Count   uint64
	Sum     float64
	Buckets map[float64]uint64
	Labels  map[string]string
}

func extractHistogramData(t *testing.T, metric prometheus.Metric) HistogramData {
	t.Helper()

	var dtoMetric dto.Metric
	err := metric.Write(&dtoMetric)
	require.NoError(t, err)

	histogram := dtoMetric.GetHistogram()
	require.NotNil(t, histogram)

	buckets := make(map[float64]uint64)
	for _, bucket := range histogram.GetBucket() {
		buckets[bucket.GetUpperBound()] = bucket.GetCumulativeCount()
	}

	labels := make(map[string]string)
	for _, labelPair := range dtoMetric.GetLabel() {
		labels[labelPair.GetName()] = labelPair.GetValue()
	}

	return HistogramData{
		Count:   histogram.GetSampleCount(),
		Sum:     histogram.GetSampleSum(),
		Buckets: buckets,
		Labels:  labels,
	}
}
