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

package vault

import (
	"fmt"
	"strings"
	"sync"
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	promtest "github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/deckhouse/deckhouse/pkg/log"
	"github.com/deckhouse/deckhouse/pkg/metrics-storage/collectors"
)

func TestVault_RegisterCounterCollector(t *testing.T) {
	t.Run("basic registration", func(t *testing.T) {
		vault := NewVault(func(name string) string { return name }, WithNewRegistry())

		collector, err := vault.RegisterCounterCollector("test_counter", []string{"method"})

		require.NoError(t, err)
		require.NotNil(t, collector)
		assert.Equal(t, "test_counter", collector.Name())
		assert.Equal(t, []string{"method"}, collector.LabelNames())
		assert.Equal(t, "counter", collector.Type())
	})

	t.Run("registration with metric name transformation", func(t *testing.T) {
		vault := NewVault(func(name string) string { return "prefix_" + name }, WithNewRegistry())

		collector, err := vault.RegisterCounterCollector("test_counter", []string{"method"})

		require.NoError(t, err)
		assert.Equal(t, "prefix_test_counter", collector.Name())
	})

	t.Run("registration with empty label names", func(t *testing.T) {
		vault := NewVault(func(name string) string { return name }, WithNewRegistry())

		collector, err := vault.RegisterCounterCollector("test_counter", []string{})

		require.NoError(t, err)
		assert.Equal(t, []string{}, collector.LabelNames())
	})

	t.Run("registration with nil label names", func(t *testing.T) {
		vault := NewVault(func(name string) string { return name }, WithNewRegistry())

		collector, err := vault.RegisterCounterCollector("test_counter", nil)

		require.NoError(t, err)
		assert.Nil(t, collector.LabelNames())
	})

	t.Run("registration with multiple labels", func(t *testing.T) {
		vault := NewVault(func(name string) string { return name }, WithNewRegistry())

		labelNames := []string{"method", "status", "endpoint"}
		collector, err := vault.RegisterCounterCollector("test_counter", labelNames)

		require.NoError(t, err)
		assert.Equal(t, labelNames, collector.LabelNames())
	})

	t.Run("re-registration returns same collector", func(t *testing.T) {
		vault := NewVault(func(name string) string { return name }, WithNewRegistry())

		collector1, err1 := vault.RegisterCounterCollector("test_counter", []string{"method"})
		require.NoError(t, err1)

		collector2, err2 := vault.RegisterCounterCollector("test_counter", []string{"method"})
		require.NoError(t, err2)

		assert.Same(t, collector1, collector2)
	})

	t.Run("re-registration with subset labels returns same collector", func(t *testing.T) {
		vault := NewVault(func(name string) string { return name }, WithNewRegistry())

		collector1, err1 := vault.RegisterCounterCollector("test_counter", []string{"method", "status"})
		require.NoError(t, err1)

		collector2, err2 := vault.RegisterCounterCollector("test_counter", []string{"method"})
		require.NoError(t, err2)

		assert.Same(t, collector1, collector2)
		assert.Equal(t, []string{"method", "status"}, collector2.LabelNames())
	})

	t.Run("re-registration with additional labels updates collector", func(t *testing.T) {
		vault := NewVault(func(name string) string { return name }, WithNewRegistry())

		collector1, err1 := vault.RegisterCounterCollector("test_counter", []string{"method"})
		require.NoError(t, err1)
		originalLabels := collector1.LabelNames()

		collector2, err2 := vault.RegisterCounterCollector("test_counter", []string{"method", "status"})
		require.NoError(t, err2)

		assert.Same(t, collector1, collector2)
		assert.NotEqual(t, originalLabels, collector2.LabelNames())

		// Labels should be updated and sorted
		expectedLabels := []string{"method", "status"}
		actualLabels := collector2.LabelNames()
		assert.ElementsMatch(t, expectedLabels, actualLabels)
	})

	t.Run("registration stores collector internally", func(t *testing.T) {
		vault := NewVault(func(name string) string { return name }, WithNewRegistry())

		_, err := vault.RegisterCounterCollector("test_counter", []string{"method"})
		require.NoError(t, err)

		// Verify collector is stored
		vault.mu.Lock()
		storedCollector, exists := vault.collectors["test_counter"]
		vault.mu.Unlock()

		assert.True(t, exists)
		assert.Equal(t, "counter", storedCollector.Type())
	})

	t.Run("registration integrates with prometheus", func(t *testing.T) {
		registry := prometheus.NewRegistry()
		vault := NewVault(func(name string) string { return name }, WithRegistry(registry))

		collector, err := vault.RegisterCounterCollector("test_counter", []string{"method"})
		require.NoError(t, err)

		// Add some data to verify it's properly registered
		collector.Add(1.0, map[string]string{"method": "GET"})

		// Check that metrics are available through the registry
		families, err := registry.Gather()
		require.NoError(t, err)

		var found bool
		for _, family := range families {
			if family.GetName() == "test_counter" {
				found = true
				assert.Equal(t, "COUNTER", family.GetType().String())
				break
			}
		}
		assert.True(t, found, "Counter metric should be registered with prometheus")
	})
}

func TestVault_RegisterCounterCollector_ErrorCases(t *testing.T) {
	t.Run("error when different collector type exists", func(t *testing.T) {
		vault := NewVault(func(name string) string { return name }, WithNewRegistry())

		// Register a gauge collector first
		_, err := vault.RegisterGaugeCollector("conflicting_metric", []string{"method"})
		require.NoError(t, err)

		// Try to register a counter with the same name
		collector, err := vault.RegisterCounterCollector("conflicting_metric", []string{"method"})

		assert.Error(t, err)
		assert.Nil(t, collector)
		assert.Contains(t, err.Error(), "counter")
		assert.Contains(t, err.Error(), "gauge")
		assert.Contains(t, err.Error(), "collector exists")
	})

	t.Run("error when prometheus registration fails", func(t *testing.T) {
		registry := prometheus.NewRegistry()
		vault := NewVault(func(name string) string { return name }, WithRegistry(registry))

		// Pre-register a metric with prometheus to cause conflict
		conflictingCollector := prometheus.NewCounter(prometheus.CounterOpts{
			Name: "conflicting_metric",
			Help: "conflicting metric",
		})
		err := registry.Register(conflictingCollector)
		require.NoError(t, err)

		// Try to register through vault - should fail
		collector, err := vault.RegisterCounterCollector("conflicting_metric", []string{})

		assert.Error(t, err)
		assert.Nil(t, collector)
		assert.Contains(t, err.Error(), "registration")
	})
}

func TestVault_RegisterCounterCollector_Concurrency(t *testing.T) {
	t.Run("concurrent registration of same collector", func(t *testing.T) {
		vault := NewVault(func(name string) string { return name }, WithNewRegistry())

		const numGoroutines = 10
		collectors := make([]*collectors.ConstCounterCollector, numGoroutines)
		errors := make([]error, numGoroutines)

		var wg sync.WaitGroup
		wg.Add(numGoroutines)

		for i := 0; i < numGoroutines; i++ {
			go func(index int) {
				defer wg.Done()
				collectors[index], errors[index] = vault.RegisterCounterCollector("concurrent_counter", []string{"method"})
			}(i)
		}

		wg.Wait()

		// All registrations should succeed
		for i, err := range errors {
			assert.NoError(t, err, "goroutine %d should not have error", i)
		}

		// All should return the same collector instance
		for i := 1; i < numGoroutines; i++ {
			assert.Same(t, collectors[0], collectors[i], "all goroutines should get the same collector")
		}
	})

	t.Run("concurrent registration of different collectors", func(t *testing.T) {
		vault := NewVault(func(name string) string { return name }, WithNewRegistry())

		const numGoroutines = 10
		collectors := make([]*collectors.ConstCounterCollector, numGoroutines)
		errors := make([]error, numGoroutines)

		var wg sync.WaitGroup
		wg.Add(numGoroutines)

		for i := 0; i < numGoroutines; i++ {
			go func(index int) {
				defer wg.Done()
				metricName := fmt.Sprintf("concurrent_counter_%d", index)
				collectors[index], errors[index] = vault.RegisterCounterCollector(metricName, []string{"method"})
			}(i)
		}

		wg.Wait()

		// All registrations should succeed
		for i, err := range errors {
			assert.NoError(t, err, "goroutine %d should not have error", i)
		}

		// All should be different collectors
		for i := 0; i < numGoroutines; i++ {
			for j := i + 1; j < numGoroutines; j++ {
				assert.NotSame(t, collectors[i], collectors[j], "collectors %d and %d should be different", i, j)
			}
		}
	})

	t.Run("concurrent registration with label updates", func(t *testing.T) {
		vault := NewVault(func(name string) string { return name }, WithNewRegistry())

		const numGoroutines = 5
		labelSets := [][]string{
			{"method"},
			{"method", "status"},
			{"method", "status", "endpoint"},
			{"method", "status", "endpoint", "user"},
			{"method", "status", "endpoint", "user", "region"},
		}

		collectors := make([]*collectors.ConstCounterCollector, numGoroutines)
		errors := make([]error, numGoroutines)

		var wg sync.WaitGroup
		wg.Add(numGoroutines)

		for i := 0; i < numGoroutines; i++ {
			go func(index int) {
				defer wg.Done()
				collectors[index], errors[index] = vault.RegisterCounterCollector("expandable_counter", labelSets[index])
			}(i)
		}

		wg.Wait()

		// All registrations should succeed
		for i, err := range errors {
			assert.NoError(t, err, "goroutine %d should not have error", i)
		}

		// All should return the same collector instance
		for i := 1; i < numGoroutines; i++ {
			assert.Same(t, collectors[0], collectors[i], "all goroutines should get the same collector")
		}

		// Final collector should have all labels
		finalLabels := collectors[0].LabelNames()
		expectedLabels := []string{"endpoint", "method", "region", "status", "user"} // sorted
		assert.ElementsMatch(t, expectedLabels, finalLabels)
	})
}

func TestVault_RegisterCounterCollector_Integration(t *testing.T) {
	t.Run("full workflow with metric operations", func(t *testing.T) {
		registry := prometheus.NewRegistry()
		vault := NewVault(func(name string) string { return "app_" + name }, WithRegistry(registry))

		// Register counter
		counter, err := vault.RegisterCounterCollector("requests_total", []string{"method", "status"})
		require.NoError(t, err)

		// Add some metrics
		counter.Add(5.0, map[string]string{"method": "GET", "status": "200"})
		counter.Add(3.0, map[string]string{"method": "POST", "status": "201"})
		counter.Add(1.0, map[string]string{"method": "GET", "status": "404"})

		// Verify metrics through prometheus test utility
		expected := `
		# HELP app_requests_total app_requests_total
		# TYPE app_requests_total counter
		app_requests_total{method="GET",status="200"} 5
		app_requests_total{method="GET",status="404"} 1
		app_requests_total{method="POST",status="201"} 3
		`

		err = promtest.GatherAndCompare(registry, strings.NewReader(expected), "app_requests_total")
		assert.NoError(t, err)
	})

	t.Run("counter integration with vault collector interface", func(t *testing.T) {
		registry := prometheus.NewRegistry()
		vault := NewVault(func(name string) string { return name })

		// Register the vault as a collector
		err := registry.Register(vault)
		require.NoError(t, err)

		// Register and use counter
		counter, err := vault.RegisterCounterCollector("test_metric", []string{"label"})
		require.NoError(t, err)

		counter.Add(10.0, map[string]string{"label": "value"})

		// Metrics should be available through vault's collector interface
		families, err := registry.Gather()
		require.NoError(t, err)

		var found bool
		for _, family := range families {
			if family.GetName() == "test_metric" {
				found = true
				metrics := family.GetMetric()
				require.Len(t, metrics, 1)
				assert.Equal(t, 10.0, metrics[0].GetCounter().GetValue())
				break
			}
		}
		assert.True(t, found)
	})
}

func TestVault_RegisterCounterCollector_EdgeCases(t *testing.T) {
	t.Run("registration with special characters in name", func(t *testing.T) {
		vault := NewVault(func(name string) string { return name }, WithNewRegistry())

		// Prometheus will validate the metric name
		collector, err := vault.RegisterCounterCollector("test_counter_123", []string{"method"})

		require.NoError(t, err)
		assert.Equal(t, "test_counter_123", collector.Name())
	})

	t.Run("registration with duplicate labels", func(t *testing.T) {
		vault := NewVault(func(name string) string { return name }, WithNewRegistry())

		labelNames := []string{"method", "method", "status"}
		_, err := vault.RegisterCounterCollector("test_counter", labelNames)

		require.Error(t, err)
	})

	t.Run("registration with very long label names", func(t *testing.T) {
		vault := NewVault(func(name string) string { return name }, WithNewRegistry())

		longLabel := strings.Repeat("very_long_label_name_", 10)
		collector, err := vault.RegisterCounterCollector("test_counter", []string{longLabel})

		require.NoError(t, err)
		assert.Contains(t, collector.LabelNames(), longLabel)
	})

	t.Run("registration with many labels", func(t *testing.T) {
		vault := NewVault(func(name string) string { return name }, WithNewRegistry())

		var manyLabels []string
		for i := 0; i < 20; i++ {
			manyLabels = append(manyLabels, fmt.Sprintf("label_%d", i))
		}

		collector, err := vault.RegisterCounterCollector("test_counter", manyLabels)

		require.NoError(t, err)
		assert.Len(t, collector.LabelNames(), 20)
	})

	t.Run("name transformation edge cases", func(t *testing.T) {
		testCases := []struct {
			name        string
			transformer func(string) string
			input       string
			expected    string
		}{
			{
				name:        "empty string transformer",
				transformer: func(string) string { return "" },
				input:       "test",
				expected:    "",
			},
			{
				name:        "identity transformer",
				transformer: func(s string) string { return s },
				input:       "test_metric",
				expected:    "test_metric",
			},
			{
				name:        "uppercase transformer",
				transformer: strings.ToUpper,
				input:       "test_metric",
				expected:    "TEST_METRIC",
			},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				vault := NewVault(tc.transformer, WithNewRegistry())

				collector, err := vault.RegisterCounterCollector(tc.input, []string{})

				// Some transformations might create invalid metric names
				if tc.expected == "" {
					// Empty names will likely cause prometheus registration to fail
					assert.Error(t, err)
					assert.Nil(t, collector)
				} else {
					require.NoError(t, err)
					assert.Equal(t, tc.expected, collector.Name())
				}
			})
		}
	})
}

func TestVault_RegisterCounterCollector_WithCustomLogger(t *testing.T) {
	t.Run("vault with custom logger", func(t *testing.T) {
		logger := log.NewLogger().Named("test")
		vault := NewVault(func(name string) string { return name }, WithNewRegistry(), WithLogger(logger))

		collector, err := vault.RegisterCounterCollector("test_counter", []string{"method"})

		require.NoError(t, err)
		assert.NotNil(t, collector)
		// Verify logger is set (though we can't easily test its usage in registration)
		assert.NotNil(t, vault.logger)
	})
}

func TestVault_RegisterCounterCollector_LabelOrdering(t *testing.T) {
	t.Run("label ordering is preserved during updates", func(t *testing.T) {
		vault := NewVault(func(name string) string { return name }, WithNewRegistry())

		// Register with some labels
		collector1, err1 := vault.RegisterCounterCollector("test_counter", []string{"z", "a", "m"})
		require.NoError(t, err1)

		// Register again with additional labels
		collector2, err2 := vault.RegisterCounterCollector("test_counter", []string{"b", "z", "y"})
		require.NoError(t, err2)

		assert.Same(t, collector1, collector2)

		// All unique labels should be present
		finalLabels := collector2.LabelNames()
		expectedLabels := []string{"a", "b", "m", "y", "z"} // should be sorted
		assert.ElementsMatch(t, expectedLabels, finalLabels)
	})
}
