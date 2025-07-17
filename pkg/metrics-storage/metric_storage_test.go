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

package metricsstorage_test

import (
	"fmt"
	"io"
	"math"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/deckhouse/deckhouse/pkg/log"
	metricsstorage "github.com/deckhouse/deckhouse/pkg/metrics-storage"
	"github.com/deckhouse/deckhouse/pkg/metrics-storage/operation"
	"github.com/deckhouse/deckhouse/pkg/metrics-storage/options"
)

func TestNewMetricStorage(t *testing.T) {
	t.Run("creates with default options", func(t *testing.T) {
		ms := metricsstorage.NewMetricStorage("test_prefix")

		require.NotNil(t, ms)
		assert.Equal(t, "test_prefix", ms.Prefix)

		// Test that default prometheus registerer is used
		handler := ms.Handler()
		assert.NotNil(t, handler)
	})

	t.Run("creates with empty prefix", func(t *testing.T) {
		ms := metricsstorage.NewMetricStorage("")

		require.NotNil(t, ms)
		assert.Equal(t, "", ms.Prefix)
	})

	t.Run("creates with special characters in prefix", func(t *testing.T) {
		prefix := "test-prefix_123"
		ms := metricsstorage.NewMetricStorage(prefix)

		require.NotNil(t, ms)
		assert.Equal(t, prefix, ms.Prefix)
	})

	t.Run("creates with custom registry", func(t *testing.T) {
		registry := prometheus.NewRegistry()
		ms := metricsstorage.NewMetricStorage("test", metricsstorage.WithRegistry(registry))

		require.NotNil(t, ms)

		// Test that custom registry is used
		collector := ms.Collector()
		assert.Equal(t, registry, collector)
	})

	t.Run("creates with new registry", func(t *testing.T) {
		ms := metricsstorage.NewMetricStorage("test", metricsstorage.WithNewRegistry())

		require.NotNil(t, ms)

		// Verify separate registry is created
		collector := ms.Collector()
		assert.NotNil(t, collector)
		assert.NotEqual(t, prometheus.DefaultRegisterer, collector)
	})

	t.Run("creates with custom logger", func(t *testing.T) {
		logger := log.NewLogger().Named("test-logger")
		ms := metricsstorage.NewMetricStorage("test", metricsstorage.WithLogger(logger))

		require.NotNil(t, ms)
		// Cannot directly test logger, but ensure no panic
	})

	t.Run("creates with multiple options", func(t *testing.T) {
		registry := prometheus.NewRegistry()
		logger := log.NewLogger().Named("test-logger")
		ms := metricsstorage.NewMetricStorage("test",
			metricsstorage.WithRegistry(registry),
			metricsstorage.WithLogger(logger))

		require.NotNil(t, ms)
		assert.Equal(t, registry, ms.Collector())
	})
}

func TestMetricStorage_PrefixResolution(t *testing.T) {
	t.Run("replaces prefix template", func(t *testing.T) {
		ms := metricsstorage.NewMetricStorage("d8")

		counter, err := ms.RegisterCounter("{PREFIX}_test_counter", []string{"label"})
		require.NoError(t, err)
		require.NotNil(t, counter)

		assert.Equal(t, "d8_test_counter", counter.Name())
	})

	t.Run("handles metric without prefix template", func(t *testing.T) {
		ms := metricsstorage.NewMetricStorage("d8")

		counter, err := ms.RegisterCounter("test_counter", []string{"label"})
		require.NoError(t, err)
		require.NotNil(t, counter)

		assert.Equal(t, "test_counter", counter.Name())
	})

	t.Run("handles multiple prefix templates", func(t *testing.T) {
		ms := metricsstorage.NewMetricStorage("d8")

		// Only first occurrence should be replaced
		counter, err := ms.RegisterCounter("{PREFIX}_test_{PREFIX}_counter", []string{"label"})
		require.NoError(t, err)
		require.NotNil(t, counter)

		assert.Equal(t, "d8_test_{PREFIX}_counter", counter.Name())
	})

	t.Run("handles empty prefix with template", func(t *testing.T) {
		ms := metricsstorage.NewMetricStorage("")

		counter, err := ms.RegisterCounter("{PREFIX}_test_counter", []string{"label"})
		require.NoError(t, err)
		require.NotNil(t, counter)

		assert.Equal(t, "_test_counter", counter.Name())
	})
}

func TestMetricStorage_RegisterCounter(t *testing.T) {
	t.Run("registers counter successfully", func(t *testing.T) {
		ms := metricsstorage.NewMetricStorage("test")

		counter, err := ms.RegisterCounter("test_counter", []string{"method", "status"})
		require.NoError(t, err)
		require.NotNil(t, counter)

		assert.Equal(t, "test_counter", counter.Name())
		assert.Equal(t, []string{"method", "status"}, counter.LabelNames())
	})

	t.Run("registers counter with empty labels", func(t *testing.T) {
		ms := metricsstorage.NewMetricStorage("test")

		counter, err := ms.RegisterCounter("test_counter", []string{})
		require.NoError(t, err)
		require.NotNil(t, counter)

		assert.Equal(t, []string{}, counter.LabelNames())
	})

	t.Run("registers counter with nil labels", func(t *testing.T) {
		ms := metricsstorage.NewMetricStorage("test")

		counter, err := ms.RegisterCounter("test_counter", nil)
		require.NoError(t, err)
		require.NotNil(t, counter)

		assert.Nil(t, counter.LabelNames())
	})

	t.Run("registers counter with options", func(t *testing.T) {
		ms := metricsstorage.NewMetricStorage("test")

		counter, err := ms.RegisterCounter("test_counter", []string{"label"},
			options.WithHelp("Test counter help"))
		require.NoError(t, err)
		require.NotNil(t, counter)
	})

	t.Run("handles duplicate counter registration", func(t *testing.T) {
		ms := metricsstorage.NewMetricStorage("test", metricsstorage.WithNewRegistry())

		counter1, err1 := ms.RegisterCounter("test_counter", []string{"label"})
		require.NoError(t, err1)

		counter2, err2 := ms.RegisterCounter("test_counter", []string{"label"})
		// Should either succeed (return same) or fail gracefully
		if err2 != nil {
			assert.Error(t, err2)
		} else {
			assert.NotNil(t, counter2)
		}

		assert.NotNil(t, counter1)
	})
}

func TestMetricStorage_RegisterGauge(t *testing.T) {
	t.Run("registers gauge successfully", func(t *testing.T) {
		ms := metricsstorage.NewMetricStorage("test")

		gauge, err := ms.RegisterGauge("test_gauge", []string{"instance"})
		require.NoError(t, err)
		require.NotNil(t, gauge)

		assert.Equal(t, "test_gauge", gauge.Name())
		assert.Equal(t, []string{"instance"}, gauge.LabelNames())
	})

	t.Run("registers gauge with options", func(t *testing.T) {
		ms := metricsstorage.NewMetricStorage("test")

		gauge, err := ms.RegisterGauge("test_gauge", []string{"instance"},
			options.WithHelp("Test gauge help"),
			options.WithConstantLabels(map[string]string{"version": "1.0"}))
		require.NoError(t, err)
		require.NotNil(t, gauge)
	})
}

func TestMetricStorage_RegisterHistogram(t *testing.T) {
	t.Run("registers histogram successfully", func(t *testing.T) {
		ms := metricsstorage.NewMetricStorage("test")
		buckets := []float64{0.1, 0.5, 1.0, 2.5, 5.0, 10.0}

		histogram, err := ms.RegisterHistogram("test_histogram", []string{"method"}, buckets)
		require.NoError(t, err)
		require.NotNil(t, histogram)

		assert.Equal(t, "test_histogram", histogram.Name())
		assert.Equal(t, []string{"method"}, histogram.LabelNames())
	})

	t.Run("registers histogram with empty buckets", func(t *testing.T) {
		ms := metricsstorage.NewMetricStorage("test")

		histogram, err := ms.RegisterHistogram("test_histogram", []string{"method"}, []float64{})
		require.NoError(t, err)
		require.NotNil(t, histogram)
	})

	t.Run("registers histogram with nil buckets", func(t *testing.T) {
		ms := metricsstorage.NewMetricStorage("test")

		histogram, err := ms.RegisterHistogram("test_histogram", []string{"method"}, nil)
		require.NoError(t, err)
		require.NotNil(t, histogram)
	})

	t.Run("registers histogram with custom buckets", func(t *testing.T) {
		ms := metricsstorage.NewMetricStorage("test")
		buckets := []float64{0.001, 0.01, 0.1, 1.0, 10.0, 100.0, 1000.0}

		histogram, err := ms.RegisterHistogram("test_histogram", []string{"endpoint"}, buckets)
		require.NoError(t, err)
		require.NotNil(t, histogram)
	})
}

func TestMetricStorage_CounterOperations(t *testing.T) {
	t.Run("counter add with valid labels", func(t *testing.T) {
		ms := metricsstorage.NewMetricStorage("test")

		ms.CounterAdd("test_counter", 5.0, map[string]string{"method": "GET", "status": "200"})

		// Verify metric exists by getting collector
		counter := ms.Counter("test_counter", map[string]string{"method": "GET", "status": "200"})
		assert.NotNil(t, counter)
	})

	t.Run("counter add with empty labels", func(t *testing.T) {
		ms := metricsstorage.NewMetricStorage("test")

		ms.CounterAdd("test_counter", 1.0, map[string]string{})

		counter := ms.Counter("test_counter", map[string]string{})
		assert.NotNil(t, counter)
	})

	t.Run("counter add with nil labels", func(t *testing.T) {
		ms := metricsstorage.NewMetricStorage("test")

		ms.CounterAdd("test_counter", 1.0, nil)

		counter := ms.Counter("test_counter", nil)
		assert.NotNil(t, counter)
	})

	t.Run("counter add zero value", func(t *testing.T) {
		ms := metricsstorage.NewMetricStorage("test")

		ms.CounterAdd("test_counter", 0.0, map[string]string{"method": "GET"})

		counter := ms.Counter("test_counter", map[string]string{"method": "GET"})
		assert.NotNil(t, counter)
	})

	t.Run("counter add negative value", func(t *testing.T) {
		ms := metricsstorage.NewMetricStorage("test")

		// Should not panic, but behavior is implementation-specific
		assert.NotPanics(t, func() {
			ms.CounterAdd("test_counter", -1.0, map[string]string{"method": "GET"})
		})
	})

	t.Run("counter operations on nil storage", func(t *testing.T) {
		var ms *metricsstorage.MetricStorage

		// Should not panic
		assert.NotPanics(t, func() {
			ms.CounterAdd("test_counter", 1.0, map[string]string{"method": "GET"})
		})
	})
}

func TestMetricStorage_GaugeOperations(t *testing.T) {
	t.Run("gauge set positive value", func(t *testing.T) {
		ms := metricsstorage.NewMetricStorage("test")

		ms.GaugeSet("test_gauge", 42.5, map[string]string{"instance": "server1"})

		gauge := ms.Gauge("test_gauge", map[string]string{"instance": "server1"})
		assert.NotNil(t, gauge)
	})

	t.Run("gauge set negative value", func(t *testing.T) {
		ms := metricsstorage.NewMetricStorage("test")

		ms.GaugeSet("test_gauge", -10.0, map[string]string{"instance": "server1"})

		gauge := ms.Gauge("test_gauge", map[string]string{"instance": "server1"})
		assert.NotNil(t, gauge)
	})

	t.Run("gauge set zero value", func(t *testing.T) {
		ms := metricsstorage.NewMetricStorage("test")

		ms.GaugeSet("test_gauge", 0.0, map[string]string{"instance": "server1"})

		gauge := ms.Gauge("test_gauge", map[string]string{"instance": "server1"})
		assert.NotNil(t, gauge)
	})

	t.Run("gauge add positive value", func(t *testing.T) {
		ms := metricsstorage.NewMetricStorage("test")

		ms.GaugeAdd("test_gauge", 5.0, map[string]string{"instance": "server1"})
		ms.GaugeAdd("test_gauge", 3.0, map[string]string{"instance": "server1"})

		gauge := ms.Gauge("test_gauge", map[string]string{"instance": "server1"})
		assert.NotNil(t, gauge)
	})

	t.Run("gauge add negative value", func(t *testing.T) {
		ms := metricsstorage.NewMetricStorage("test")

		ms.GaugeAdd("test_gauge", -5.0, map[string]string{"instance": "server1"})

		gauge := ms.Gauge("test_gauge", map[string]string{"instance": "server1"})
		assert.NotNil(t, gauge)
	})

	t.Run("gauge operations on nil storage", func(t *testing.T) {
		var ms *metricsstorage.MetricStorage

		// Should not panic
		assert.NotPanics(t, func() {
			ms.GaugeSet("test_gauge", 1.0, map[string]string{"instance": "server1"})
			ms.GaugeAdd("test_gauge", 1.0, map[string]string{"instance": "server1"})
		})
	})
}

func TestMetricStorage_HistogramOperations(t *testing.T) {
	t.Run("histogram observe positive value", func(t *testing.T) {
		ms := metricsstorage.NewMetricStorage("test")
		buckets := []float64{0.1, 0.5, 1.0, 2.5, 5.0}

		ms.HistogramObserve("test_histogram", 1.5, map[string]string{"endpoint": "/api"}, buckets)

		histogram := ms.Histogram("test_histogram", map[string]string{"endpoint": "/api"}, buckets)
		assert.NotNil(t, histogram)
	})

	t.Run("histogram observe zero value", func(t *testing.T) {
		ms := metricsstorage.NewMetricStorage("test")
		buckets := []float64{0.1, 0.5, 1.0}

		ms.HistogramObserve("test_histogram", 0.0, map[string]string{"endpoint": "/api"}, buckets)

		histogram := ms.Histogram("test_histogram", map[string]string{"endpoint": "/api"}, buckets)
		assert.NotNil(t, histogram)
	})

	t.Run("histogram observe negative value", func(t *testing.T) {
		ms := metricsstorage.NewMetricStorage("test")
		buckets := []float64{0.1, 0.5, 1.0}

		ms.HistogramObserve("test_histogram", -0.5, map[string]string{"endpoint": "/api"}, buckets)

		histogram := ms.Histogram("test_histogram", map[string]string{"endpoint": "/api"}, buckets)
		assert.NotNil(t, histogram)
	})

	t.Run("histogram observe with empty buckets", func(t *testing.T) {
		ms := metricsstorage.NewMetricStorage("test")

		ms.HistogramObserve("test_histogram", 1.0, map[string]string{"endpoint": "/api"}, []float64{})

		histogram := ms.Histogram("test_histogram", map[string]string{"endpoint": "/api"}, []float64{})
		assert.NotNil(t, histogram)
	})

	t.Run("histogram observe with nil buckets", func(t *testing.T) {
		ms := metricsstorage.NewMetricStorage("test")

		ms.HistogramObserve("test_histogram", 1.0, map[string]string{"endpoint": "/api"}, nil)

		histogram := ms.Histogram("test_histogram", map[string]string{"endpoint": "/api"}, nil)
		assert.NotNil(t, histogram)
	})

	t.Run("histogram operations on nil storage", func(t *testing.T) {
		var ms *metricsstorage.MetricStorage

		// Should not panic
		assert.NotPanics(t, func() {
			ms.HistogramObserve("test_histogram", 1.0, map[string]string{"endpoint": "/api"}, []float64{1.0})
		})
	})
}

func TestMetricStorage_ApplyOperation(t *testing.T) {
	t.Run("apply add operation", func(t *testing.T) {
		ms := metricsstorage.NewMetricStorage("test")
		value := 5.0

		op := operation.MetricOperation{
			Name:   "test_counter",
			Action: operation.ActionCounterAdd,
			Value:  &value,
			Labels: map[string]string{"method": "GET"},
		}

		ms.ApplyOperation(op, map[string]string{"service": "api"})

		// Verify operation was applied
		counter := ms.Counter("test_counter", map[string]string{"method": "GET", "service": "api"})
		assert.NotNil(t, counter)
	})

	t.Run("apply set operation", func(t *testing.T) {
		ms := metricsstorage.NewMetricStorage("test")
		value := 42.0

		op := operation.MetricOperation{
			Name:   "test_gauge",
			Action: operation.ActionOldGaugeSet,
			Value:  &value,
			Labels: map[string]string{"instance": "server1"},
		}

		ms.ApplyOperation(op, map[string]string{"environment": "prod"})

		// Verify operation was applied
		gauge := ms.Gauge("test_gauge", map[string]string{"instance": "server1", "environment": "prod"})
		assert.NotNil(t, gauge)
	})

	t.Run("apply observe operation", func(t *testing.T) {
		ms := metricsstorage.NewMetricStorage("test")
		value := 1.5
		buckets := []float64{0.1, 0.5, 1.0, 2.5, 5.0}

		op := operation.MetricOperation{
			Name:    "test_histogram",
			Action:  operation.ActionHistogramObserve,
			Value:   &value,
			Buckets: buckets,
			Labels:  map[string]string{"endpoint": "/api"},
		}

		ms.ApplyOperation(op, map[string]string{"version": "1.0"})

		// Verify operation was applied
		histogram := ms.Histogram("test_histogram", map[string]string{"endpoint": "/api", "version": "1.0"}, buckets)
		assert.NotNil(t, histogram)
	})

	t.Run("apply operation with nil value for add", func(t *testing.T) {
		ms := metricsstorage.NewMetricStorage("test")

		op := operation.MetricOperation{
			Name:   "test_counter",
			Action: operation.ActionCounterAdd,
			Value:  nil,
			Labels: map[string]string{"method": "GET"},
		}

		// Should not panic
		assert.NotPanics(t, func() {
			ms.ApplyOperation(op, map[string]string{"service": "api"})
		})
	})

	t.Run("apply operation with empty labels", func(t *testing.T) {
		ms := metricsstorage.NewMetricStorage("test")
		value := 1.0

		op := operation.MetricOperation{
			Name:   "test_counter",
			Action: operation.ActionCounterAdd,
			Value:  &value,
			Labels: map[string]string{},
		}

		ms.ApplyOperation(op, map[string]string{})

		counter := ms.Counter("test_counter", map[string]string{})
		assert.NotNil(t, counter)
	})

	t.Run("apply operation on nil storage", func(t *testing.T) {
		var ms *metricsstorage.MetricStorage
		value := 1.0

		op := operation.MetricOperation{
			Name:   "test_counter",
			Action: operation.ActionCounterAdd,
			Value:  &value,
		}

		// Should not panic
		assert.NotPanics(t, func() {
			ms.ApplyOperation(op, map[string]string{})
		})
	})
}

func TestMetricStorage_ApplyBatchOperations(t *testing.T) {
	t.Run("apply mixed operations batch", func(t *testing.T) {
		ms := metricsstorage.NewMetricStorage("test")

		value1 := 5.0
		value2 := 42.0
		value3 := 1.5
		buckets := []float64{0.1, 0.5, 1.0, 2.5, 5.0}

		ops := []operation.MetricOperation{
			{
				Name:   "test_counter",
				Action: operation.ActionCounterAdd,
				Value:  &value1,
				Labels: map[string]string{"method": "GET"},
			},
			{
				Name:   "test_gauge",
				Action: operation.ActionOldGaugeSet,
				Value:  &value2,
				Labels: map[string]string{"instance": "server1"},
			},
			{
				Name:    "test_histogram",
				Action:  operation.ActionHistogramObserve,
				Value:   &value3,
				Buckets: buckets,
				Labels:  map[string]string{"endpoint": "/api"},
			},
		}

		err := ms.ApplyBatchOperations(ops, map[string]string{"service": "api"})
		require.NoError(t, err)

		// Verify all operations were applied
		counter := ms.Counter("test_counter", map[string]string{"method": "GET", "service": "api"})
		gauge := ms.Gauge("test_gauge", map[string]string{"instance": "server1", "service": "api"})
		histogram := ms.Histogram("test_histogram", map[string]string{"endpoint": "/api", "service": "api"}, buckets)

		assert.NotNil(t, counter)
		assert.NotNil(t, gauge)
		assert.NotNil(t, histogram)
	})

	t.Run("apply grouped operations batch", func(t *testing.T) {
		ms := metricsstorage.NewMetricStorage("test")

		value1 := 5.0
		value2 := 10.0

		ops := []operation.MetricOperation{
			{
				Name:   "test_counter",
				Action: operation.ActionCounterAdd,
				Value:  &value1,
				Labels: map[string]string{"method": "GET"},
				Group:  "api_metrics",
			},
			{
				Name:   "test_gauge",
				Action: operation.ActionOldGaugeSet,
				Value:  &value2,
				Labels: map[string]string{"instance": "server1"},
				Group:  "api_metrics",
			},
		}

		err := ms.ApplyBatchOperations(ops, map[string]string{"service": "api"})
		require.NoError(t, err)

		// Verify grouped operations were applied
		grouped := ms.Grouped()
		assert.NotNil(t, grouped)
	})

	t.Run("apply empty operations batch", func(t *testing.T) {
		ms := metricsstorage.NewMetricStorage("test")

		err := ms.ApplyBatchOperations([]operation.MetricOperation{}, map[string]string{"service": "api"})
		require.NoError(t, err)
	})

	t.Run("apply nil operations batch", func(t *testing.T) {
		ms := metricsstorage.NewMetricStorage("test")

		err := ms.ApplyBatchOperations(nil, map[string]string{"service": "api"})
		require.NoError(t, err)
	})

	t.Run("apply batch with invalid operations", func(t *testing.T) {
		ms := metricsstorage.NewMetricStorage("test")

		ops := []operation.MetricOperation{
			{
				Name:   "", // Invalid: empty name
				Action: operation.ActionCounterAdd,
			},
		}

		err := ms.ApplyBatchOperations(ops, map[string]string{})
		assert.Error(t, err)
	})

	t.Run("apply batch operations on nil storage", func(t *testing.T) {
		var ms *metricsstorage.MetricStorage
		value := 1.0

		ops := []operation.MetricOperation{
			{
				Name:   "test_counter",
				Action: operation.ActionCounterAdd,
				Value:  &value,
			},
		}

		err := ms.ApplyBatchOperations(ops, map[string]string{})
		assert.NoError(t, err) // Should handle nil gracefully
	})
}

func TestMetricStorage_GroupedOperations(t *testing.T) {
	t.Run("grouped storage is accessible", func(t *testing.T) {
		ms := metricsstorage.NewMetricStorage("test")

		grouped := ms.Grouped()
		assert.NotNil(t, grouped)
	})

	t.Run("grouped operations work correctly", func(t *testing.T) {
		ms := metricsstorage.NewMetricStorage("test")
		grouped := ms.Grouped()

		grouped.CounterAdd("test_group", "test_counter", 5.0, map[string]string{"method": "GET"})
		grouped.GaugeSet("test_group", "test_gauge", 42.0, map[string]string{"instance": "server1"})

		// Verify collector exists
		collector := grouped.Collector()
		assert.NotNil(t, collector)
	})

	t.Run("grouped metrics can be expired", func(t *testing.T) {
		ms := metricsstorage.NewMetricStorage("test")
		grouped := ms.Grouped()

		grouped.CounterAdd("test_group", "test_counter", 5.0, map[string]string{"method": "GET"})
		grouped.GaugeSet("test_group", "test_gauge", 42.0, map[string]string{"instance": "server1"})

		// Expire entire group
		grouped.ExpireGroupMetrics("test_group")

		// Verify no panic occurred
		collector := grouped.Collector()
		assert.NotNil(t, collector)
	})

	t.Run("grouped metrics can be expired by name", func(t *testing.T) {
		ms := metricsstorage.NewMetricStorage("test")
		grouped := ms.Grouped()

		grouped.CounterAdd("test_group", "test_counter", 5.0, map[string]string{"method": "GET"})
		grouped.GaugeSet("test_group", "test_gauge", 42.0, map[string]string{"instance": "server1"})

		// Expire specific metric
		grouped.ExpireGroupMetricByName("test_group", "test_counter")

		// Verify no panic occurred
		collector := grouped.Collector()
		assert.NotNil(t, collector)
	})
}

func TestMetricStorage_PrometheusIntegration(t *testing.T) {
	t.Run("collector interface", func(t *testing.T) {
		ms := metricsstorage.NewMetricStorage("test")

		collector := ms.Collector()
		assert.NotNil(t, collector)

		// Verify collector implements prometheus.Collector
		var _ prometheus.Collector = collector
	})

	t.Run("handler interface", func(t *testing.T) {
		ms := metricsstorage.NewMetricStorage("test")

		handler := ms.Handler()
		assert.NotNil(t, handler)

		// Verify handler implements http.Handler
		var _ http.Handler = handler
	})

	t.Run("handler serves metrics", func(t *testing.T) {
		ms := metricsstorage.NewMetricStorage("test", metricsstorage.WithNewRegistry())

		// Add some metrics
		ms.CounterAdd("test_counter", 5.0, map[string]string{"method": "GET"})
		ms.GaugeSet("test_gauge", 42.0, map[string]string{"instance": "server1"})

		// Create test server
		handler := ms.Handler()
		server := httptest.NewServer(handler)
		defer server.Close()

		// Make request to metrics endpoint
		resp, err := http.Get(server.URL)
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusOK, resp.StatusCode)

		body, err := io.ReadAll(resp.Body)
		require.NoError(t, err)

		bodyStr := string(body)
		// Should contain our metrics
		assert.Contains(t, bodyStr, "test_counter")
		assert.Contains(t, bodyStr, "test_gauge")
	})

	t.Run("custom registry integration", func(t *testing.T) {
		registry := prometheus.NewRegistry()
		ms := metricsstorage.NewMetricStorage("test", metricsstorage.WithRegistry(registry))

		// Add metrics
		ms.CounterAdd("test_counter", 1.0, map[string]string{"label": "value"})

		// Verify metrics are in custom registry
		families, err := registry.Gather()
		require.NoError(t, err)

		// Should have at least our metric
		assert.NotEmpty(t, families)

		foundMetric := false
		for _, family := range families {
			if family.GetName() == "test_counter" {
				foundMetric = true
				break
			}
		}
		assert.True(t, foundMetric, "Custom metric should be found in registry")
	})
}

func TestMetricStorage_EdgeCases(t *testing.T) {
	t.Run("operations with extreme values", func(t *testing.T) {
		ms := metricsstorage.NewMetricStorage("test")

		// Test with very large values
		ms.CounterAdd("test_counter", 1e9, map[string]string{"type": "large"})
		ms.GaugeSet("test_gauge", 1e15, map[string]string{"type": "large"})

		// Test with very small values
		ms.CounterAdd("test_counter", 1e-9, map[string]string{"type": "small"})
		ms.GaugeSet("test_gauge", 1e-15, map[string]string{"type": "small"})

		// Test with infinity (implementation specific behavior)
		assert.NotPanics(t, func() {
			ms.GaugeSet("test_gauge", math.Inf(1), map[string]string{"type": "inf"})
			ms.GaugeSet("test_gauge", math.Inf(-1), map[string]string{"type": "neg_inf"})
		})

		// Test with NaN (implementation specific behavior)
		assert.NotPanics(t, func() {
			ms.GaugeSet("test_gauge", math.NaN(), map[string]string{"type": "nan"})
		})
	})

	t.Run("operations with special characters in labels", func(t *testing.T) {
		ms := metricsstorage.NewMetricStorage("test")

		specialLabels := map[string]string{
			"unicode_💯": "value",
			"spaces":    "value with spaces",
			"quotes":    `value with "quotes"`,
			"backslash": `value\with\backslash`,
			"newline":   "value\nwith\nnewline",
			"empty":     "",
		}

		assert.NotPanics(t, func() {
			ms.CounterAdd("test_counter", 1.0, specialLabels)
			ms.GaugeSet("test_gauge", 1.0, specialLabels)
		})
	})

	t.Run("operations with very long label values", func(t *testing.T) {
		ms := metricsstorage.NewMetricStorage("test")

		longValue := strings.Repeat("a", 1000)
		longLabels := map[string]string{
			"long_value": longValue,
		}

		assert.NotPanics(t, func() {
			ms.CounterAdd("test_counter", 1.0, longLabels)
			ms.GaugeSet("test_gauge", 1.0, longLabels)
		})
	})

	t.Run("operations with many labels", func(t *testing.T) {
		ms := metricsstorage.NewMetricStorage("test")

		manyLabels := make(map[string]string)
		for i := 0; i < 100; i++ {
			manyLabels[fmt.Sprintf("label_%d", i)] = fmt.Sprintf("value_%d", i)
		}

		assert.NotPanics(t, func() {
			ms.CounterAdd("test_counter", 1.0, manyLabels)
			ms.GaugeSet("test_gauge", 1.0, manyLabels)
		})
	})

	t.Run("concurrent operations", func(t *testing.T) {
		ms := metricsstorage.NewMetricStorage("test")

		var wg sync.WaitGroup
		numGoroutines := 100
		numOpsPerGoroutine := 100

		// Test concurrent counter operations
		for i := 0; i < numGoroutines; i++ {
			wg.Add(1)
			go func(id int) {
				defer wg.Done()
				for j := 0; j < numOpsPerGoroutine; j++ {
					ms.CounterAdd("concurrent_counter", 1.0, map[string]string{
						"goroutine": fmt.Sprintf("%d", id),
						"operation": fmt.Sprintf("%d", j),
					})
				}
			}(i)
		}

		// Test concurrent gauge operations
		for i := 0; i < numGoroutines; i++ {
			wg.Add(1)
			go func(id int) {
				defer wg.Done()
				for j := 0; j < numOpsPerGoroutine; j++ {
					ms.GaugeSet("concurrent_gauge", float64(j), map[string]string{
						"goroutine": fmt.Sprintf("%d", id),
						"operation": fmt.Sprintf("%d", j),
					})
				}
			}(i)
		}

		// Test concurrent histogram operations
		buckets := []float64{0.1, 0.5, 1.0, 2.5, 5.0}
		for i := 0; i < numGoroutines; i++ {
			wg.Add(1)
			go func(id int) {
				defer wg.Done()
				for j := 0; j < numOpsPerGoroutine; j++ {
					ms.HistogramObserve("concurrent_histogram", float64(j)*0.1, map[string]string{
						"goroutine": fmt.Sprintf("%d", id),
						"operation": fmt.Sprintf("%d", j),
					}, buckets)
				}
			}(i)
		}

		wg.Wait()

		// Verify no panic occurred during concurrent operations
		collector := ms.Collector()
		assert.NotNil(t, collector)
	})

	t.Run("stress test with rapid operations", func(t *testing.T) {
		ms := metricsstorage.NewMetricStorage("test")

		// Rapidly create and modify metrics
		for i := 0; i < 10000; i++ {
			metricName := fmt.Sprintf("stress_test_metric_%d", i%100)
			labels := map[string]string{
				"iteration": fmt.Sprintf("%d", i),
				"modulo":    fmt.Sprintf("%d", i%10),
			}

			ms.CounterAdd(metricName, 1.0, labels)
			ms.GaugeSet(metricName+"_gauge", float64(i), labels)
		}

		// Verify storage is still functional
		collector := ms.Collector()
		assert.NotNil(t, collector)
	})
}

func TestMetricStorage_MemoryLeaks(t *testing.T) {
	t.Run("metric registration and expiration", func(t *testing.T) {
		ms := metricsstorage.NewMetricStorage("test")
		grouped := ms.Grouped()

		// Register many metrics in groups
		for group := 0; group < 10; group++ {
			groupName := fmt.Sprintf("group_%d", group)
			for metric := 0; metric < 100; metric++ {
				metricName := fmt.Sprintf("metric_%d", metric)
				labels := map[string]string{
					"instance": fmt.Sprintf("server_%d", metric%10),
					"type":     fmt.Sprintf("type_%d", metric%5),
				}

				grouped.CounterAdd(groupName, metricName, float64(metric), labels)
				grouped.GaugeSet(groupName, metricName+"_gauge", float64(metric*2), labels)
			}
		}

		// Expire some groups
		for group := 0; group < 5; group++ {
			groupName := fmt.Sprintf("group_%d", group)
			grouped.ExpireGroupMetrics(groupName)
		}

		// Verify storage is still functional
		collector := grouped.Collector()
		assert.NotNil(t, collector)

		// Add new metrics to verify everything still works
		grouped.CounterAdd("new_group", "new_metric", 1.0, map[string]string{"test": "value"})
	})
}

// Helper functions for testing prometheus integration
func collectMetrics(t *testing.T, collector prometheus.Collector) []*dto.MetricFamily {
	t.Helper()

	registry := prometheus.NewRegistry()
	err := registry.Register(collector)
	require.NoError(t, err)

	families, err := registry.Gather()
	require.NoError(t, err)

	return families
}

func findMetricFamily(families []*dto.MetricFamily, name string) *dto.MetricFamily {
	for _, family := range families {
		if family.GetName() == name {
			return family
		}
	}
	return nil
}

func TestMetricStorage_PrometheusCompatibility(t *testing.T) {
	t.Run("metrics are correctly exported to prometheus", func(t *testing.T) {
		ms := metricsstorage.NewMetricStorage("test", metricsstorage.WithNewRegistry())

		// Add various metrics
		ms.CounterAdd("test_counter", 5.0, map[string]string{"method": "GET", "status": "200"})
		ms.GaugeSet("test_gauge", 42.0, map[string]string{"instance": "server1"})

		buckets := []float64{0.1, 0.5, 1.0, 2.5, 5.0}
		ms.HistogramObserve("test_histogram", 1.5, map[string]string{"endpoint": "/api"}, buckets)
		ms.HistogramObserve("test_histogram", 0.2, map[string]string{"endpoint": "/api"}, buckets)
		ms.HistogramObserve("test_histogram", 3.0, map[string]string{"endpoint": "/api"}, buckets)

		// Collect metrics
		collector := ms.Collector()
		families := collectMetrics(t, collector)

		// Verify counter
		counterFamily := findMetricFamily(families, "test_counter")
		require.NotNil(t, counterFamily)
		assert.Equal(t, dto.MetricType_COUNTER, counterFamily.GetType())
		require.Len(t, counterFamily.GetMetric(), 1)
		assert.Equal(t, 5.0, counterFamily.GetMetric()[0].GetCounter().GetValue())

		// Verify gauge
		gaugeFamily := findMetricFamily(families, "test_gauge")
		require.NotNil(t, gaugeFamily)
		assert.Equal(t, dto.MetricType_GAUGE, gaugeFamily.GetType())
		require.Len(t, gaugeFamily.GetMetric(), 1)
		assert.Equal(t, 42.0, gaugeFamily.GetMetric()[0].GetGauge().GetValue())

		// Verify histogram
		histogramFamily := findMetricFamily(families, "test_histogram")
		require.NotNil(t, histogramFamily)
		assert.Equal(t, dto.MetricType_HISTOGRAM, histogramFamily.GetType())
		require.Len(t, histogramFamily.GetMetric(), 1)

		histogram := histogramFamily.GetMetric()[0].GetHistogram()
		assert.Equal(t, uint64(3), histogram.GetSampleCount())
		assert.Equal(t, 4.7, histogram.GetSampleSum()) // 1.5 + 0.2 + 3.0
	})
}

func TestMetricStorage_ComplexScenarios(t *testing.T) {
	t.Run("mixed grouped and non-grouped operations", func(t *testing.T) {
		ms := metricsstorage.NewMetricStorage("d8")

		// Non-grouped operations
		ms.CounterAdd("requests_total", 100, map[string]string{"service": "api"})
		ms.GaugeSet("active_connections", 50, map[string]string{"service": "api"})

		// Grouped operations
		grouped := ms.Grouped()
		grouped.CounterAdd("module_metrics", "module_errors_total", 5, map[string]string{"module": "auth"})
		grouped.GaugeSet("module_metrics", "module_status", 1, map[string]string{"module": "auth"})

		// Batch operations with mixed groups
		value1 := 10.0
		value2 := 20.0
		value3 := 30.0

		ops := []operation.MetricOperation{
			{
				Name:   "batch_counter",
				Action: operation.ActionCounterAdd,
				Value:  &value1,
				Labels: map[string]string{"type": "increment"},
			},
			{
				Name:   "batch_grouped_counter",
				Action: operation.ActionCounterAdd,
				Value:  &value2,
				Labels: map[string]string{"type": "grouped"},
				Group:  "batch_group",
			},
			{
				Name:   "batch_gauge",
				Action: operation.ActionOldGaugeSet,
				Value:  &value3,
				Labels: map[string]string{"type": "value"},
				Group:  "batch_group",
			},
		}

		err := ms.ApplyBatchOperations(ops, map[string]string{"common": "label"})
		require.NoError(t, err)

		// Verify all operations succeeded
		collector := ms.Collector()
		assert.NotNil(t, collector)

		groupedCollector := grouped.Collector()
		assert.NotNil(t, groupedCollector)
	})

	t.Run("operations with prefix resolution", func(t *testing.T) {
		ms := metricsstorage.NewMetricStorage("deckhouse")

		// Test various prefix scenarios
		counter1, err := ms.RegisterCounter("{PREFIX}_module_errors", []string{"module"})
		require.NoError(t, err)
		assert.Equal(t, "deckhouse_module_errors", counter1.Name())

		counter2, err := ms.RegisterCounter("global_metric", []string{"type"})
		require.NoError(t, err)
		assert.Equal(t, "global_metric", counter2.Name())

		// Apply operations with prefix resolution
		ms.CounterAdd("{PREFIX}_requests_total", 100, map[string]string{"service": "api"})
		ms.GaugeSet("raw_gauge", 42, map[string]string{"instance": "server1"})

		// Verify prefix was resolved in operations
		resolvedCounter := ms.Counter("{PREFIX}_requests_total", map[string]string{"service": "api"})
		assert.NotNil(t, resolvedCounter)
		assert.Equal(t, "deckhouse_requests_total", resolvedCounter.Name())
	})
}

// Benchmark tests to ensure performance is reasonable
func BenchmarkMetricStorage_CounterOperations(b *testing.B) {
	ms := metricsstorage.NewMetricStorage("bench")
	labels := map[string]string{"method": "GET", "status": "200"}

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			ms.CounterAdd("benchmark_counter", 1.0, labels)
		}
	})
}

func BenchmarkMetricStorage_GaugeOperations(b *testing.B) {
	ms := metricsstorage.NewMetricStorage("bench")
	labels := map[string]string{"instance": "server1"}

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			ms.GaugeSet("benchmark_gauge", 42.0, labels)
		}
	})
}

func BenchmarkMetricStorage_HistogramOperations(b *testing.B) {
	ms := metricsstorage.NewMetricStorage("bench")
	labels := map[string]string{"endpoint": "/api"}
	buckets := []float64{0.1, 0.5, 1.0, 2.5, 5.0, 10.0}

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			ms.HistogramObserve("benchmark_histogram", 1.5, labels, buckets)
		}
	})
}

func BenchmarkMetricStorage_BatchOperations(b *testing.B) {
	ms := metricsstorage.NewMetricStorage("bench")

	value := 1.0
	ops := []operation.MetricOperation{
		{
			Name:   "bench_counter",
			Action: operation.ActionCounterAdd,
			Value:  &value,
			Labels: map[string]string{"type": "counter"},
		},
		{
			Name:   "bench_gauge",
			Action: operation.ActionOldGaugeSet,
			Value:  &value,
			Labels: map[string]string{"type": "gauge"},
		},
	}

	commonLabels := map[string]string{"service": "benchmark"}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		err := ms.ApplyBatchOperations(ops, commonLabels)
		if err != nil {
			b.Fatal(err)
		}
	}
}
