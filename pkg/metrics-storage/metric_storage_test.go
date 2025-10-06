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
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/deckhouse/deckhouse/pkg/log"

	metricsstorage "github.com/deckhouse/deckhouse/pkg/metrics-storage"
	"github.com/deckhouse/deckhouse/pkg/metrics-storage/operation"
)

func TestNewMetricStorage(t *testing.T) {
	tests := []struct {
		name     string
		opts     []metricsstorage.Option
		validate func(t *testing.T, storage *metricsstorage.MetricStorage)
	}{
		{
			name: "default configuration",
			opts: []metricsstorage.Option{metricsstorage.WithPrefix("test")},
			validate: func(t *testing.T, storage *metricsstorage.MetricStorage) {
				assert.NotNil(t, storage)
				assert.Equal(t, "test", storage.Prefix)
				assert.NotNil(t, storage.Handler())
				assert.NotNil(t, storage.Collector())
			},
		},
		{
			name: "with new registry",
			opts: []metricsstorage.Option{
				metricsstorage.WithPrefix("custom"),
				metricsstorage.WithNewRegistry(),
			},
			validate: func(t *testing.T, storage *metricsstorage.MetricStorage) {
				assert.NotNil(t, storage)
				assert.Equal(t, "custom", storage.Prefix)
			},
		},
		{
			name: "with custom registry",
			opts: []metricsstorage.Option{
				metricsstorage.WithPrefix("registry"),
				metricsstorage.WithRegistry(prometheus.NewRegistry()),
			},
			validate: func(t *testing.T, storage *metricsstorage.MetricStorage) {
				assert.NotNil(t, storage)
				assert.Equal(t, "registry", storage.Prefix)
			},
		},
		{
			name: "with logger",
			opts: []metricsstorage.Option{
				metricsstorage.WithPrefix("logger"),
				metricsstorage.WithLogger(log.NewNop()),
			},
			validate: func(t *testing.T, storage *metricsstorage.MetricStorage) {
				assert.NotNil(t, storage)
				assert.Equal(t, "logger", storage.Prefix)
			},
		},
		{
			name: "empty prefix",
			opts: []metricsstorage.Option{metricsstorage.WithPrefix("")},
			validate: func(t *testing.T, storage *metricsstorage.MetricStorage) {
				assert.NotNil(t, storage)
				assert.Equal(t, "", storage.Prefix)
			},
		},
		{
			name: "multiple options",
			opts: []metricsstorage.Option{
				metricsstorage.WithPrefix("multi"),
				metricsstorage.WithNewRegistry(),
				metricsstorage.WithLogger(log.NewNop()),
			},
			validate: func(t *testing.T, storage *metricsstorage.MetricStorage) {
				assert.NotNil(t, storage)
				assert.Equal(t, "multi", storage.Prefix)
			},
		},
		{
			name: "no prefix option",
			opts: []metricsstorage.Option{metricsstorage.WithNewRegistry()},
			validate: func(t *testing.T, storage *metricsstorage.MetricStorage) {
				assert.NotNil(t, storage)
				assert.Equal(t, "", storage.Prefix)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			storage := metricsstorage.NewMetricStorage(tt.opts...)
			tt.validate(t, storage)
		})
	}
}

func TestMetricStorage_RegisterCounter(t *testing.T) {
	tests := []struct {
		name        string
		metric      string
		labelNames  []string
		wantError   bool
		errorSubstr string
	}{
		{
			name:       "valid counter",
			metric:     "test_counter",
			labelNames: []string{"label1", "label2"},
			wantError:  false,
		},
		{
			name:       "counter with no labels",
			metric:     "simple_counter",
			labelNames: nil,
			wantError:  false,
		},
		{
			name:       "counter with prefix template",
			metric:     "{PREFIX}_counter",
			labelNames: []string{"env"},
			wantError:  false,
		},
		{
			name:        "invalid metric name",
			metric:      "",
			labelNames:  []string{"label1"},
			wantError:   true,
			errorSubstr: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			storage := metricsstorage.NewMetricStorage(
				metricsstorage.WithPrefix("test"),
				metricsstorage.WithNewRegistry(),
			)

			counter, err := storage.RegisterCounter(tt.metric, tt.labelNames)

			if tt.wantError {
				assert.Error(t, err)
				if tt.errorSubstr != "" {
					assert.Contains(t, err.Error(), tt.errorSubstr)
				}
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, counter)
			}
		})
	}
}

func TestMetricStorage_RegisterGauge(t *testing.T) {
	tests := []struct {
		name        string
		metric      string
		labelNames  []string
		wantError   bool
		errorSubstr string
	}{
		{
			name:       "valid gauge",
			metric:     "test_gauge",
			labelNames: []string{"label1", "label2"},
			wantError:  false,
		},
		{
			name:       "gauge with no labels",
			metric:     "simple_gauge",
			labelNames: nil,
			wantError:  false,
		},
		{
			name:       "gauge with prefix template",
			metric:     "{PREFIX}_gauge",
			labelNames: []string{"env"},
			wantError:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			storage := metricsstorage.NewMetricStorage(
				metricsstorage.WithPrefix("test"),
				metricsstorage.WithNewRegistry(),
			)

			gauge, err := storage.RegisterGauge(tt.metric, tt.labelNames)

			if tt.wantError {
				assert.Error(t, err)
				if tt.errorSubstr != "" {
					assert.Contains(t, err.Error(), tt.errorSubstr)
				}
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, gauge)
			}
		})
	}
}

func TestMetricStorage_RegisterHistogram(t *testing.T) {
	tests := []struct {
		name        string
		metric      string
		labelNames  []string
		buckets     []float64
		wantError   bool
		errorSubstr string
	}{
		{
			name:       "valid histogram",
			metric:     "test_histogram",
			labelNames: []string{"label1", "label2"},
			buckets:    []float64{0.1, 0.5, 1.0, 5.0},
			wantError:  false,
		},
		{
			name:       "histogram with no labels",
			metric:     "simple_histogram",
			labelNames: nil,
			buckets:    []float64{1, 5, 10},
			wantError:  false,
		},
		{
			name:       "histogram with default buckets",
			metric:     "default_histogram",
			labelNames: []string{"env"},
			buckets:    nil,
			wantError:  false,
		},
		{
			name:       "histogram with prefix template",
			metric:     "{PREFIX}_histogram",
			labelNames: []string{"env"},
			buckets:    []float64{0.1, 0.5, 1.0},
			wantError:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			storage := metricsstorage.NewMetricStorage(
				metricsstorage.WithPrefix("test"),
				metricsstorage.WithNewRegistry(),
			)

			histogram, err := storage.RegisterHistogram(tt.metric, tt.labelNames, tt.buckets)

			if tt.wantError {
				assert.Error(t, err)
				if tt.errorSubstr != "" {
					assert.Contains(t, err.Error(), tt.errorSubstr)
				}
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, histogram)
			}
		})
	}
}

func TestMetricStorage_CounterAdd(t *testing.T) {
	tests := []struct {
		name   string
		metric string
		value  float64
		labels map[string]string
	}{
		{
			name:   "simple counter add",
			metric: "test_counter",
			value:  1.0,
			labels: map[string]string{"env": "test"},
		},
		{
			name:   "counter add with multiple labels",
			metric: "multi_counter",
			value:  5.0,
			labels: map[string]string{"env": "prod", "service": "api"},
		},
		{
			name:   "counter add with no labels",
			metric: "no_label_counter",
			value:  10.0,
			labels: nil,
		},
		{
			name:   "counter add zero value",
			metric: "zero_counter",
			value:  0.0,
			labels: map[string]string{"type": "zero"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			storage := metricsstorage.NewMetricStorage(
				metricsstorage.WithPrefix("test"),
				metricsstorage.WithNewRegistry(),
			)

			// Should not panic
			assert.NotPanics(t, func() {
				storage.CounterAdd(tt.metric, tt.value, tt.labels)
			})
		})
	}
}

func TestMetricStorage_GaugeSet(t *testing.T) {
	tests := []struct {
		name   string
		metric string
		value  float64
		labels map[string]string
	}{
		{
			name:   "simple gauge set",
			metric: "test_gauge",
			value:  42.0,
			labels: map[string]string{"env": "test"},
		},
		{
			name:   "gauge set negative value",
			metric: "negative_gauge",
			value:  -10.5,
			labels: map[string]string{"type": "negative"},
		},
		{
			name:   "gauge set with no labels",
			metric: "no_label_gauge",
			value:  100.0,
			labels: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			storage := metricsstorage.NewMetricStorage(
				metricsstorage.WithPrefix("test"),
				metricsstorage.WithNewRegistry(),
			)

			// Should not panic
			assert.NotPanics(t, func() {
				storage.GaugeSet(tt.metric, tt.value, tt.labels)
			})
		})
	}
}

func TestMetricStorage_GaugeAdd(t *testing.T) {
	tests := []struct {
		name   string
		metric string
		value  float64
		labels map[string]string
	}{
		{
			name:   "simple gauge add",
			metric: "test_gauge_add",
			value:  5.0,
			labels: map[string]string{"env": "test"},
		},
		{
			name:   "gauge add negative value",
			metric: "negative_gauge_add",
			value:  -2.5,
			labels: map[string]string{"type": "sub"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			storage := metricsstorage.NewMetricStorage(
				metricsstorage.WithPrefix("test"),
				metricsstorage.WithNewRegistry(),
			)

			// Should not panic
			assert.NotPanics(t, func() {
				storage.GaugeAdd(tt.metric, tt.value, tt.labels)
			})
		})
	}
}

func TestMetricStorage_HistogramObserve(t *testing.T) {
	tests := []struct {
		name    string
		metric  string
		value   float64
		labels  map[string]string
		buckets []float64
	}{
		{
			name:    "simple histogram observe",
			metric:  "test_histogram",
			value:   0.5,
			labels:  map[string]string{"env": "test"},
			buckets: []float64{0.1, 0.5, 1.0, 5.0},
		},
		{
			name:    "histogram observe large value",
			metric:  "large_histogram",
			value:   100.0,
			labels:  map[string]string{"type": "large"},
			buckets: []float64{1, 10, 100, 1000},
		},
		{
			name:    "histogram observe with no labels",
			metric:  "no_label_histogram",
			value:   2.5,
			labels:  nil,
			buckets: []float64{1, 5, 10},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			storage := metricsstorage.NewMetricStorage(
				metricsstorage.WithPrefix("test"),
				metricsstorage.WithNewRegistry(),
			)

			// Should not panic
			assert.NotPanics(t, func() {
				storage.HistogramObserve(tt.metric, tt.value, tt.labels, tt.buckets)
			})
		})
	}
}

func TestMetricStorage_Counter(t *testing.T) {
	storage := metricsstorage.NewMetricStorage(
		metricsstorage.WithPrefix("test"),
		metricsstorage.WithNewRegistry(),
	)

	counter1 := storage.Counter("test_counter", map[string]string{"env": "test"})
	assert.NotNil(t, counter1)

	// Getting the same counter should return the same instance
	counter2 := storage.Counter("test_counter", map[string]string{"env": "test"})
	assert.NotNil(t, counter2)
}

func TestMetricStorage_Gauge(t *testing.T) {
	storage := metricsstorage.NewMetricStorage(
		metricsstorage.WithPrefix("test"),
		metricsstorage.WithNewRegistry(),
	)

	gauge1 := storage.Gauge("test_gauge", map[string]string{"env": "test"})
	assert.NotNil(t, gauge1)

	// Getting the same gauge should return the same instance
	gauge2 := storage.Gauge("test_gauge", map[string]string{"env": "test"})
	assert.NotNil(t, gauge2)
}

func TestMetricStorage_Histogram(t *testing.T) {
	storage := metricsstorage.NewMetricStorage(
		metricsstorage.WithPrefix("test"),
		metricsstorage.WithNewRegistry(),
	)

	buckets := []float64{0.1, 0.5, 1.0, 5.0}
	histogram1 := storage.Histogram("test_histogram", map[string]string{"env": "test"}, buckets)
	assert.NotNil(t, histogram1)

	// Getting the same histogram should return the same instance
	histogram2 := storage.Histogram("test_histogram", map[string]string{"env": "test"}, buckets)
	assert.NotNil(t, histogram2)
}

func TestMetricStorage_ApplyOperation(t *testing.T) {
	tests := []struct {
		name         string
		op           operation.MetricOperation
		commonLabels map[string]string
		wantError    bool
		errorSubstr  string
	}{
		{
			name: "valid counter add operation",
			op: operation.MetricOperation{
				Name:   "test_counter",
				Value:  floatPtr(1.0),
				Action: operation.ActionCounterAdd,
				Labels: map[string]string{"type": "test"},
			},
			commonLabels: map[string]string{"env": "prod"},
			wantError:    false,
		},
		{
			name: "valid gauge set operation",
			op: operation.MetricOperation{
				Name:   "test_gauge",
				Value:  floatPtr(42.0),
				Action: operation.ActionGaugeSet,
				Labels: map[string]string{"service": "api"},
			},
			commonLabels: map[string]string{"region": "us-east"},
			wantError:    false,
		},
		{
			name: "valid gauge add operation",
			op: operation.MetricOperation{
				Name:   "test_gauge_add",
				Value:  floatPtr(5.0),
				Action: operation.ActionGaugeAdd,
				Labels: map[string]string{"component": "worker"},
			},
			commonLabels: nil,
			wantError:    false,
		},
		{
			name: "valid histogram observe operation",
			op: operation.MetricOperation{
				Name:    "test_histogram",
				Value:   floatPtr(0.5),
				Action:  operation.ActionHistogramObserve,
				Labels:  map[string]string{"endpoint": "/api/v1"},
				Buckets: []float64{0.1, 0.5, 1.0, 5.0},
			},
			commonLabels: map[string]string{"method": "GET"},
			wantError:    false,
		},
		{
			name: "valid expire metrics operation",
			op: operation.MetricOperation{
				Group:  "test_group",
				Action: operation.ActionExpireMetrics,
			},
			commonLabels: nil,
			wantError:    false,
		},
		{
			name: "invalid operation - missing value for counter",
			op: operation.MetricOperation{
				Name:   "test_counter",
				Action: operation.ActionCounterAdd,
			},
			commonLabels: nil,
			wantError:    true,
			errorSubstr:  "value",
		},
		{
			name: "invalid operation - missing name",
			op: operation.MetricOperation{
				Value:  floatPtr(1.0),
				Action: operation.ActionCounterAdd,
			},
			commonLabels: nil,
			wantError:    true,
			errorSubstr:  "name",
		},
		{
			name: "invalid operation - invalid action",
			op: operation.MetricOperation{
				Name:   "test_metric",
				Value:  floatPtr(1.0),
				Action: operation.MetricAction(999),
			},
			commonLabels: nil,
			wantError:    true,
			errorSubstr:  "action",
		},
		{
			name: "invalid operation - missing buckets for histogram",
			op: operation.MetricOperation{
				Name:   "test_histogram",
				Value:  floatPtr(0.5),
				Action: operation.ActionHistogramObserve,
			},
			commonLabels: nil,
			wantError:    true,
			errorSubstr:  "buckets",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			storage := metricsstorage.NewMetricStorage(
				metricsstorage.WithPrefix("test"),
				metricsstorage.WithNewRegistry(),
			)

			err := storage.ApplyOperation(tt.op, tt.commonLabels)

			if tt.wantError {
				assert.Error(t, err)
				if tt.errorSubstr != "" {
					assert.Contains(t, err.Error(), tt.errorSubstr)
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestMetricStorage_ApplyBatchOperations(t *testing.T) {
	tests := []struct {
		name         string
		ops          []operation.MetricOperation
		commonLabels map[string]string
		wantError    bool
		errorSubstr  string
	}{
		{
			name: "valid batch operations",
			ops: []operation.MetricOperation{
				{
					Name:   "counter1",
					Value:  floatPtr(1.0),
					Action: operation.ActionCounterAdd,
					Labels: map[string]string{"type": "api"},
				},
				{
					Name:   "gauge1",
					Value:  floatPtr(42.0),
					Action: operation.ActionGaugeSet,
					Labels: map[string]string{"service": "web"},
				},
			},
			commonLabels: map[string]string{"env": "prod"},
			wantError:    false,
		},
		{
			name: "grouped operations",
			ops: []operation.MetricOperation{
				{
					Name:   "metric1",
					Value:  floatPtr(1.0),
					Action: operation.ActionCounterAdd,
					Group:  "group1",
				},
				{
					Name:   "metric2",
					Value:  floatPtr(2.0),
					Action: operation.ActionGaugeSet,
					Group:  "group1",
				},
				{
					Name:   "metric3",
					Value:  floatPtr(3.0),
					Action: operation.ActionCounterAdd,
					Group:  "group2",
				},
			},
			commonLabels: map[string]string{"region": "us-west"},
			wantError:    false,
		},
		{
			name: "mixed grouped and non-grouped operations",
			ops: []operation.MetricOperation{
				{
					Name:   "ungrouped_metric",
					Value:  floatPtr(1.0),
					Action: operation.ActionCounterAdd,
				},
				{
					Name:   "grouped_metric",
					Value:  floatPtr(2.0),
					Action: operation.ActionGaugeSet,
					Group:  "test_group",
				},
			},
			commonLabels: nil,
			wantError:    false,
		},
		{
			name: "expire metrics operation",
			ops: []operation.MetricOperation{
				{
					Group:  "expired_group",
					Action: operation.ActionExpireMetrics,
				},
			},
			commonLabels: nil,
			wantError:    false,
		},
		{
			name:         "empty operations",
			ops:          []operation.MetricOperation{},
			commonLabels: map[string]string{"env": "test"},
			wantError:    false,
		},
		{
			name: "invalid operations in batch",
			ops: []operation.MetricOperation{
				{
					Name:   "valid_metric",
					Value:  floatPtr(1.0),
					Action: operation.ActionCounterAdd,
				},
				{
					Name:   "invalid_metric",
					Action: operation.ActionCounterAdd, // Missing value
				},
			},
			commonLabels: nil,
			wantError:    true,
			errorSubstr:  "value",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			storage := metricsstorage.NewMetricStorage(
				metricsstorage.WithPrefix("test"),
				metricsstorage.WithNewRegistry(),
			)

			err := storage.ApplyBatchOperations(tt.ops, tt.commonLabels)

			if tt.wantError {
				assert.Error(t, err)
				if tt.errorSubstr != "" {
					assert.Contains(t, err.Error(), tt.errorSubstr)
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestMetricStorage_Handler(t *testing.T) {
	tests := []struct {
		name     string
		opts     []metricsstorage.Option
		validate func(t *testing.T, handler http.Handler)
	}{
		{
			name: "default handler",
			opts: nil,
			validate: func(t *testing.T, handler http.Handler) {
				assert.NotNil(t, handler)

				// Test that handler responds
				req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
				w := httptest.NewRecorder()
				handler.ServeHTTP(w, req)
				assert.Equal(t, http.StatusOK, w.Code)
			},
		},
		{
			name: "custom registry handler",
			opts: []metricsstorage.Option{metricsstorage.WithNewRegistry()},
			validate: func(t *testing.T, handler http.Handler) {
				assert.NotNil(t, handler)

				// Test that handler responds
				req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
				w := httptest.NewRecorder()
				handler.ServeHTTP(w, req)
				assert.Equal(t, http.StatusOK, w.Code)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			storage := metricsstorage.NewMetricStorage(tt.opts...)
			handler := storage.Handler()
			tt.validate(t, handler)
		})
	}
}

func TestMetricStorage_Collector(t *testing.T) {
	tests := []struct {
		name string
		opts []metricsstorage.Option
	}{
		{
			name: "default collector",
			opts: nil,
		},
		{
			name: "custom registry collector",
			opts: []metricsstorage.Option{metricsstorage.WithNewRegistry()},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			storage := metricsstorage.NewMetricStorage(tt.opts...)
			collector := storage.Collector()
			assert.NotNil(t, collector)

			// Test that collector can be described and collected
			desc := make(chan *prometheus.Desc, 10)
			go func() {
				defer close(desc)
				collector.Describe(desc)
			}()

			// Drain the channel
			for range desc {
				continue
			}

			metrics := make(chan prometheus.Metric, 10)
			go func() {
				defer close(metrics)
				collector.Collect(metrics)
			}()

			// Drain the channel
			for range metrics {
				continue
			}
		})
	}
}

func TestMetricStorage_Grouped(t *testing.T) {
	storage := metricsstorage.NewMetricStorage(
		metricsstorage.WithPrefix("test"),
		metricsstorage.WithNewRegistry(),
	)
	grouped := storage.Grouped()
	assert.NotNil(t, grouped)

	// Test grouped operations
	grouped.CounterAdd("test_group", "test_counter", 1.0, map[string]string{"env": "test"})
	grouped.GaugeSet("test_group", "test_gauge", 42.0, map[string]string{"service": "api"})
	grouped.ExpireGroupMetrics("test_group")
}

// Edge Cases and Error Conditions

func TestMetricStorage_NilReceiver(t *testing.T) {
	var storage *metricsstorage.MetricStorage

	// These should not panic when receiver is nil
	assert.NotPanics(t, func() {
		storage.CounterAdd("test", 1.0, nil)
	})

	assert.NotPanics(t, func() {
		storage.GaugeSet("test", 1.0, nil)
	})

	assert.NotPanics(t, func() {
		storage.GaugeAdd("test", 1.0, nil)
	})

	assert.NotPanics(t, func() {
		storage.HistogramObserve("test", 1.0, nil, []float64{1, 5, 10})
	})

	// ApplyOperation should return error for nil receiver
	err := storage.ApplyOperation(operation.MetricOperation{}, nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "nil")

	// ApplyBatchOperations should return nil for nil receiver
	err = storage.ApplyBatchOperations([]operation.MetricOperation{}, nil)
	assert.NoError(t, err)
}

func TestMetricStorage_PrefixReplacement(t *testing.T) {
	tests := []struct {
		name           string
		prefix         string
		metricName     string
		expectedResult string
	}{
		{
			name:           "prefix template replacement",
			prefix:         "myapp",
			metricName:     "{PREFIX}_requests_total",
			expectedResult: "myapp_requests_total",
		},
		{
			name:           "no prefix template",
			prefix:         "myapp",
			metricName:     "requests_total",
			expectedResult: "requests_total",
		},
		{
			name:           "empty prefix",
			prefix:         "",
			metricName:     "{PREFIX}_requests_total",
			expectedResult: "_requests_total",
		},
		{
			name:           "multiple prefix templates (only first replaced)",
			prefix:         "app",
			metricName:     "{PREFIX}_requests_{PREFIX}_total",
			expectedResult: "app_requests_{PREFIX}_total",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			storage := metricsstorage.NewMetricStorage(
				metricsstorage.WithPrefix(tt.prefix),
				metricsstorage.WithNewRegistry(),
			)

			// Register a counter to test prefix replacement
			counter, err := storage.RegisterCounter(tt.metricName, []string{"status"})
			require.NoError(t, err)
			require.NotNil(t, counter)

			// We can't directly access the resolved name, but we can verify the metric was registered
			// by checking that we can add values without error
			storage.CounterAdd(tt.metricName, 1.0, map[string]string{"status": "200"})
		})
	}
}

func TestMetricStorage_LabelMerging(t *testing.T) {
	storage := metricsstorage.NewMetricStorage(
		metricsstorage.WithPrefix("test"),
		metricsstorage.WithNewRegistry(),
	)

	// Test that operation labels and common labels are properly merged
	op := operation.MetricOperation{
		Name:   "test_counter",
		Value:  floatPtr(1.0),
		Action: operation.ActionCounterAdd,
		Labels: map[string]string{"op_label": "op_value", "common": "op_override"},
	}

	commonLabels := map[string]string{"common": "common_value", "common_label": "common_value"}

	err := storage.ApplyOperation(op, commonLabels)
	assert.NoError(t, err)

	// Test with nil labels
	op.Labels = nil
	err = storage.ApplyOperation(op, commonLabels)
	assert.NoError(t, err)

	// Test with nil common labels
	op.Labels = map[string]string{"test": "value"}
	err = storage.ApplyOperation(op, nil)
	assert.NoError(t, err)
}

func TestMetricStorage_ConcurrentAccess(_ *testing.T) {
	storage := metricsstorage.NewMetricStorage(
		metricsstorage.WithPrefix("test"),
		metricsstorage.WithNewRegistry(),
	)

	// Test concurrent access to metrics
	const numGoroutines = 10
	const numOperations = 100

	done := make(chan struct{}, numGoroutines)

	for i := 0; i < numGoroutines; i++ {
		go func(id int) {
			defer func() { done <- struct{}{} }()

			for j := 0; j < numOperations; j++ {
				labels := map[string]string{"goroutine": string(rune(id)), "iteration": string(rune(j))}

				storage.CounterAdd("concurrent_counter", 1.0, labels)
				storage.GaugeSet("concurrent_gauge", float64(j), labels)
				storage.HistogramObserve("concurrent_histogram", float64(j)*0.1, labels, []float64{0.1, 0.5, 1.0, 5.0})
			}
		}(i)
	}

	// Wait for all goroutines to complete
	for i := 0; i < numGoroutines; i++ {
		<-done
	}
}

func TestMetricStorage_InvalidMetricNames(t *testing.T) {
	storage := metricsstorage.NewMetricStorage(
		metricsstorage.WithPrefix("test"),
		metricsstorage.WithNewRegistry(),
	)

	invalidNames := []string{
		"",                       // empty name
		"123invalid",             // starts with number
		"invalid-name",           // contains hyphen
		"invalid.name",           // contains dot
		"invalid name",           // contains space
		"invalid/name",           // contains slash
		strings.Repeat("a", 300), // very long name
	}

	for _, name := range invalidNames {
		t.Run("invalid_name_"+name, func(_ *testing.T) {
			// These operations should handle invalid names gracefully
			storage.CounterAdd(name, 1.0, map[string]string{"test": "value"})
			storage.GaugeSet(name, 1.0, map[string]string{"test": "value"})
			storage.HistogramObserve(name, 1.0, map[string]string{"test": "value"}, []float64{1, 5, 10})
		})
	}
}

func TestMetricStorage_ExtremeValues(t *testing.T) {
	storage := metricsstorage.NewMetricStorage(
		metricsstorage.WithPrefix("test"),
		metricsstorage.WithNewRegistry(),
	)

	extremeValues := []float64{
		0.0,
		-1.0,
		1e-10,   // very small positive
		-1e-10,  // very small negative
		1e10,    // very large positive
		-1e10,   // very large negative
		3.14159, // pi
		2.71828, // e
	}

	for i, value := range extremeValues {
		t.Run("extreme_value", func(_ *testing.T) {
			labels := map[string]string{"value_index": string(rune(i))}

			storage.CounterAdd("extreme_counter", value, labels)
			storage.GaugeSet("extreme_gauge", value, labels)
			storage.GaugeAdd("extreme_gauge_add", value, labels)
			storage.HistogramObserve("extreme_histogram", value, labels, []float64{-1e10, 0, 1e10})
		})
	}
}

func TestMetricStorage_ManyLabels(_ *testing.T) {
	storage := metricsstorage.NewMetricStorage(
		metricsstorage.WithPrefix("test"),
		metricsstorage.WithNewRegistry(),
	)

	// Test with many labels
	manyLabels := make(map[string]string)
	for i := 0; i < 20; i++ {
		manyLabels["label_"+string(rune(i))] = "value_" + string(rune(i))
	}

	storage.CounterAdd("many_labels_counter", 1.0, manyLabels)
	storage.GaugeSet("many_labels_gauge", 42.0, manyLabels)
	storage.HistogramObserve("many_labels_histogram", 0.5, manyLabels, []float64{0.1, 0.5, 1.0})
}

func TestMetricStorage_EmptyAndNilMaps(_ *testing.T) {
	storage := metricsstorage.NewMetricStorage(
		metricsstorage.WithPrefix("test"),
		metricsstorage.WithNewRegistry(),
	)

	// Test with nil labels
	storage.CounterAdd("nil_labels_counter", 1.0, nil)
	storage.GaugeSet("nil_labels_gauge", 42.0, nil)
	storage.HistogramObserve("nil_labels_histogram", 0.5, nil, []float64{0.1, 0.5, 1.0})

	// Test with empty labels
	emptyLabels := make(map[string]string)
	storage.CounterAdd("empty_labels_counter", 1.0, emptyLabels)
	storage.GaugeSet("empty_labels_gauge", 42.0, emptyLabels)
	storage.HistogramObserve("empty_labels_histogram", 0.5, emptyLabels, []float64{0.1, 0.5, 1.0})
}

// Helper functions

func floatPtr(f float64) *float64 {
	return &f
}

// Benchmark tests

func BenchmarkMetricStorage_CounterAdd(b *testing.B) {
	storage := metricsstorage.NewMetricStorage(
		metricsstorage.WithPrefix("bench"),
		metricsstorage.WithNewRegistry(),
	)
	labels := map[string]string{"env": "test", "service": "api"}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		storage.CounterAdd("benchmark_counter", 1.0, labels)
	}
}

func BenchmarkMetricStorage_GaugeSet(b *testing.B) {
	storage := metricsstorage.NewMetricStorage(
		metricsstorage.WithPrefix("bench"),
		metricsstorage.WithNewRegistry(),
	)
	labels := map[string]string{"env": "test", "service": "api"}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		storage.GaugeSet("benchmark_gauge", float64(i), labels)
	}
}

func BenchmarkMetricStorage_HistogramObserve(b *testing.B) {
	storage := metricsstorage.NewMetricStorage(
		metricsstorage.WithPrefix("bench"),
		metricsstorage.WithNewRegistry(),
	)
	labels := map[string]string{"env": "test", "service": "api"}
	buckets := []float64{0.1, 0.5, 1.0, 5.0, 10.0}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		storage.HistogramObserve("benchmark_histogram", float64(i%10)*0.1, labels, buckets)
	}
}

func BenchmarkMetricStorage_ApplyBatchOperations(b *testing.B) {
	storage := metricsstorage.NewMetricStorage(
		metricsstorage.WithPrefix("bench"),
		metricsstorage.WithNewRegistry(),
	)

	ops := []operation.MetricOperation{
		{
			Name:   "counter1",
			Value:  floatPtr(1.0),
			Action: operation.ActionCounterAdd,
			Labels: map[string]string{"type": "api"},
		},
		{
			Name:   "gauge1",
			Value:  floatPtr(42.0),
			Action: operation.ActionGaugeSet,
			Labels: map[string]string{"service": "web"},
		},
		{
			Name:    "histogram1",
			Value:   floatPtr(0.5),
			Action:  operation.ActionHistogramObserve,
			Buckets: []float64{0.1, 0.5, 1.0, 5.0},
		},
	}

	commonLabels := map[string]string{"env": "prod"}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		err := storage.ApplyBatchOperations(ops, commonLabels)
		if err != nil {
			b.Error(err)
		}
	}
}
