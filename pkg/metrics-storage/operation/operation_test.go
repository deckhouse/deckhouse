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

package operation_test

import (
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/deckhouse/deckhouse/pkg/metrics-storage/operation"
)

func TestMetricAction_IsValid(t *testing.T) {
	tests := []struct {
		name     string
		action   operation.MetricAction
		expected bool
	}{
		{
			name:     "valid counter add action",
			action:   operation.ActionCounterAdd,
			expected: true,
		},
		{
			name:     "valid gauge add action",
			action:   operation.ActionGaugeAdd,
			expected: true,
		},
		{
			name:     "valid gauge set action",
			action:   operation.ActionGaugeSet,
			expected: true,
		},
		{
			name:     "valid histogram observe action",
			action:   operation.ActionHistogramObserve,
			expected: true,
		},
		{
			name:     "valid expire metrics action",
			action:   operation.ActionExpireMetrics,
			expected: true,
		},
		{
			name:     "invalid action - negative value",
			action:   operation.MetricAction(-1),
			expected: false,
		},
		{
			name:     "invalid action - large value",
			action:   operation.MetricAction(999),
			expected: false,
		},
		{
			name:     "invalid action - zero beyond range",
			action:   operation.MetricAction(100),
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.action.IsValid()
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestMetricAction_String(t *testing.T) {
	tests := []struct {
		name     string
		action   operation.MetricAction
		expected string
	}{
		{
			name:     "counter add action string",
			action:   operation.ActionCounterAdd,
			expected: "CounterAdd",
		},
		{
			name:     "gauge add action string",
			action:   operation.ActionGaugeAdd,
			expected: "GaugeAdd",
		},
		{
			name:     "gauge set action string",
			action:   operation.ActionGaugeSet,
			expected: "GaugeSet",
		},
		{
			name:     "histogram observe action string",
			action:   operation.ActionHistogramObserve,
			expected: "HistogramObserve",
		},
		{
			name:     "expire metrics action string",
			action:   operation.ActionExpireMetrics,
			expected: "ExpireMetrics",
		},
		{
			name:     "unknown action string - negative",
			action:   operation.MetricAction(-1),
			expected: "Unknown",
		},
		{
			name:     "unknown action string - large value",
			action:   operation.MetricAction(999),
			expected: "Unknown",
		},
		{
			name:     "unknown action string - out of range",
			action:   operation.MetricAction(10),
			expected: "Unknown",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.action.String()
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestValidateMetricOperation_ValidOperations(t *testing.T) {
	tests := []struct {
		name      string
		operation operation.MetricOperation
	}{
		{
			name: "valid counter add operation",
			operation: operation.MetricOperation{
				Name:   "test_counter",
				Value:  floatPtr(1.0),
				Action: operation.ActionCounterAdd,
				Labels: map[string]string{"key": "value"},
			},
		},
		{
			name: "valid gauge add operation",
			operation: operation.MetricOperation{
				Name:   "test_gauge",
				Value:  floatPtr(5.5),
				Action: operation.ActionGaugeAdd,
				Labels: map[string]string{"env": "test"},
			},
		},
		{
			name: "valid gauge set operation",
			operation: operation.MetricOperation{
				Name:   "test_gauge_set",
				Value:  floatPtr(0.0),
				Action: operation.ActionGaugeSet,
			},
		},
		{
			name: "valid histogram observe operation",
			operation: operation.MetricOperation{
				Name:    "test_histogram",
				Value:   floatPtr(2.5),
				Action:  operation.ActionHistogramObserve,
				Buckets: []float64{1.0, 5.0, 10.0},
				Labels:  map[string]string{"method": "GET"},
			},
		},
		{
			name: "valid expire metrics operation with group",
			operation: operation.MetricOperation{
				Group:  "test_group",
				Action: operation.ActionExpireMetrics,
			},
		},
		{
			name: "valid operation with negative value",
			operation: operation.MetricOperation{
				Name:   "test_negative",
				Value:  floatPtr(-10.5),
				Action: operation.ActionGaugeSet,
			},
		},
		{
			name: "valid operation with empty labels",
			operation: operation.MetricOperation{
				Name:   "test_empty_labels",
				Value:  floatPtr(1.0),
				Action: operation.ActionCounterAdd,
				Labels: map[string]string{},
			},
		},
		{
			name: "valid operation with nil labels",
			operation: operation.MetricOperation{
				Name:   "test_nil_labels",
				Value:  floatPtr(1.0),
				Action: operation.ActionCounterAdd,
				Labels: nil,
			},
		},
		{
			name: "valid operation with group and name",
			operation: operation.MetricOperation{
				Name:   "test_grouped",
				Value:  floatPtr(1.0),
				Action: operation.ActionCounterAdd,
				Group:  "test_group",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := operation.ValidateMetricOperation(tt.operation)
			assert.NoError(t, err, "Expected valid operation to pass validation")
		})
	}
}

func TestValidateMetricOperation_InvalidOperations(t *testing.T) {
	tests := []struct {
		name          string
		operation     operation.MetricOperation
		expectedError string
	}{
		{
			name: "invalid action",
			operation: operation.MetricOperation{
				Name:   "test_metric",
				Value:  floatPtr(1.0),
				Action: operation.MetricAction(999),
			},
			expectedError: "one of: 'action', 'set' or 'add' is required",
		},
		{
			name: "missing name and group",
			operation: operation.MetricOperation{
				Value:  floatPtr(1.0),
				Action: operation.ActionCounterAdd,
			},
			expectedError: "'name' is required",
		},
		{
			name: "missing value for counter add",
			operation: operation.MetricOperation{
				Name:   "test_counter",
				Action: operation.ActionCounterAdd,
			},
			expectedError: "'value' is required for action 'CounterAdd'",
		},
		{
			name: "missing value for gauge add",
			operation: operation.MetricOperation{
				Name:   "test_gauge",
				Action: operation.ActionGaugeAdd,
			},
			expectedError: "'value' is required for action 'GaugeAdd'",
		},
		{
			name: "missing value for gauge set",
			operation: operation.MetricOperation{
				Name:   "test_gauge",
				Action: operation.ActionGaugeSet,
			},
			expectedError: "'value' is required for action 'GaugeSet'",
		},
		{
			name: "missing value for histogram observe",
			operation: operation.MetricOperation{
				Name:    "test_histogram",
				Action:  operation.ActionHistogramObserve,
				Buckets: []float64{1.0, 5.0},
			},
			expectedError: "'value' is required for action 'HistogramObserve'",
		},
		{
			name: "missing buckets for histogram observe",
			operation: operation.MetricOperation{
				Name:   "test_histogram",
				Value:  floatPtr(2.5),
				Action: operation.ActionHistogramObserve,
			},
			expectedError: "'buckets' is required for action 'HistogramObserve'",
		},
		{
			name: "expire metrics without group",
			operation: operation.MetricOperation{
				Name:   "test_metric",
				Action: operation.ActionExpireMetrics,
			},
			expectedError: "unsupported action 'ExpireMetrics'",
		},
		{
			name: "missing name with group for non-expire action",
			operation: operation.MetricOperation{
				Group:  "test_group",
				Value:  floatPtr(1.0),
				Action: operation.ActionCounterAdd,
			},
			expectedError: "'name' is required when action is not 'expire'",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := operation.ValidateMetricOperation(tt.operation)
			require.Error(t, err, "Expected invalid operation to fail validation")
			assert.Contains(t, err.Error(), tt.expectedError, "Error message should contain expected text")
		})
	}
}

func TestValidateMetricOperation_MultipleErrors(t *testing.T) {
	tests := []struct {
		name           string
		operation      operation.MetricOperation
		expectedErrors []string
	}{
		{
			name: "invalid action and missing value",
			operation: operation.MetricOperation{
				Name:   "test_metric",
				Action: operation.MetricAction(-1),
			},
			expectedErrors: []string{
				"one of: 'action', 'set' or 'add' is required",
			},
		},
		{
			name: "missing name and value for counter",
			operation: operation.MetricOperation{
				Action: operation.ActionCounterAdd,
			},
			expectedErrors: []string{
				"'name' is required",
				"'value' is required for action 'CounterAdd'",
			},
		},
		{
			name: "histogram with missing value and buckets",
			operation: operation.MetricOperation{
				Name:   "test_histogram",
				Action: operation.ActionHistogramObserve,
			},
			expectedErrors: []string{
				"'value' is required for action 'HistogramObserve'",
				"'buckets' is required for action 'HistogramObserve'",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := operation.ValidateMetricOperation(tt.operation)
			require.Error(t, err, "Expected invalid operation to fail validation")

			errStr := err.Error()
			for _, expectedErr := range tt.expectedErrors {
				assert.Contains(t, errStr, expectedErr, "Error should contain: %s", expectedErr)
			}
		})
	}
}

func TestValidateOperations_ValidBatch(t *testing.T) {
	operations := []operation.MetricOperation{
		{
			Name:   "counter1",
			Value:  floatPtr(1.0),
			Action: operation.ActionCounterAdd,
		},
		{
			Name:   "gauge1",
			Value:  floatPtr(5.0),
			Action: operation.ActionGaugeSet,
		},
		{
			Name:    "histogram1",
			Value:   floatPtr(2.5),
			Action:  operation.ActionHistogramObserve,
			Buckets: []float64{1.0, 5.0, 10.0},
		},
		{
			Group:  "test_group",
			Action: operation.ActionExpireMetrics,
		},
	}

	err := operation.ValidateOperations(operations...)
	assert.NoError(t, err, "Expected all valid operations to pass validation")
}

func TestValidateOperations_InvalidBatch(t *testing.T) {
	operations := []operation.MetricOperation{
		{
			Name:   "valid_counter",
			Value:  floatPtr(1.0),
			Action: operation.ActionCounterAdd,
		},
		{
			Name:   "invalid_counter",
			Action: operation.ActionCounterAdd, // missing value
		},
		{
			Action: operation.MetricAction(-1), // invalid action and missing name
		},
		{
			Name:   "invalid_histogram",
			Value:  floatPtr(1.0),
			Action: operation.ActionHistogramObserve, // missing buckets
		},
	}

	err := operation.ValidateOperations(operations...)
	require.Error(t, err, "Expected batch with invalid operations to fail validation")

	errStr := err.Error()
	// Check that multiple errors are reported
	assert.Contains(t, errStr, "'value' is required for action 'CounterAdd'")
	assert.Contains(t, errStr, "one of: 'action', 'set' or 'add' is required")
	assert.Contains(t, errStr, "'buckets' is required for action 'HistogramObserve'")
}

func TestValidateOperations_EmptyBatch(t *testing.T) {
	var operations []operation.MetricOperation

	err := operation.ValidateOperations(operations...)
	assert.NoError(t, err, "Expected empty operations batch to be valid")
}

func TestValidateOperations_NilBatch(t *testing.T) {
	err := operation.ValidateOperations()
	assert.NoError(t, err, "Expected nil operations batch to be valid")
}

// Edge cases and boundary conditions
func TestValidateMetricOperation_EdgeCases(t *testing.T) {
	tests := []struct {
		name      string
		operation operation.MetricOperation
		wantError bool
	}{
		{
			name: "zero value is valid",
			operation: operation.MetricOperation{
				Name:   "test_zero",
				Value:  floatPtr(0.0),
				Action: operation.ActionGaugeSet,
			},
			wantError: false,
		},
		{
			name: "very large positive value",
			operation: operation.MetricOperation{
				Name:   "test_large",
				Value:  floatPtr(1e308),
				Action: operation.ActionGaugeSet,
			},
			wantError: false,
		},
		{
			name: "very large negative value",
			operation: operation.MetricOperation{
				Name:   "test_large_negative",
				Value:  floatPtr(-1e308),
				Action: operation.ActionGaugeSet,
			},
			wantError: false,
		},
		{
			name: "empty string name",
			operation: operation.MetricOperation{
				Name:   "",
				Value:  floatPtr(1.0),
				Action: operation.ActionCounterAdd,
			},
			wantError: true,
		},
		{
			name: "empty group string",
			operation: operation.MetricOperation{
				Group:  "",
				Action: operation.ActionExpireMetrics,
			},
			wantError: true,
		},
		{
			name: "whitespace only name",
			operation: operation.MetricOperation{
				Name:   "   ",
				Value:  floatPtr(1.0),
				Action: operation.ActionCounterAdd,
			},
			wantError: false, // validation doesn't check for whitespace-only names
		},
		{
			name: "empty buckets array",
			operation: operation.MetricOperation{
				Name:    "test_histogram",
				Value:   floatPtr(1.0),
				Action:  operation.ActionHistogramObserve,
				Buckets: []float64{},
			},
			wantError: false, // empty buckets array is technically provided
		},
		{
			name: "buckets with zero values",
			operation: operation.MetricOperation{
				Name:    "test_histogram_zeros",
				Value:   floatPtr(1.0),
				Action:  operation.ActionHistogramObserve,
				Buckets: []float64{0.0, 0.0, 0.0},
			},
			wantError: false,
		},
		{
			name: "buckets with negative values",
			operation: operation.MetricOperation{
				Name:    "test_histogram_negative",
				Value:   floatPtr(1.0),
				Action:  operation.ActionHistogramObserve,
				Buckets: []float64{-1.0, -0.5, 0.0, 1.0},
			},
			wantError: false,
		},
		{
			name: "single bucket",
			operation: operation.MetricOperation{
				Name:    "test_histogram_single",
				Value:   floatPtr(1.0),
				Action:  operation.ActionHistogramObserve,
				Buckets: []float64{5.0},
			},
			wantError: false,
		},
		{
			name: "labels with empty values",
			operation: operation.MetricOperation{
				Name:   "test_empty_label_values",
				Value:  floatPtr(1.0),
				Action: operation.ActionCounterAdd,
				Labels: map[string]string{
					"key1": "",
					"key2": "value2",
					"key3": "",
				},
			},
			wantError: false,
		},
		{
			name: "labels with special characters",
			operation: operation.MetricOperation{
				Name:   "test_special_chars",
				Value:  floatPtr(1.0),
				Action: operation.ActionCounterAdd,
				Labels: map[string]string{
					"key-with-dashes":  "value",
					"key_with_under":   "value",
					"key.with.dots":    "value",
					"key/with/slashes": "value",
					"key with spaces":  "value with spaces",
				},
			},
			wantError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := operation.ValidateMetricOperation(tt.operation)
			if tt.wantError {
				assert.Error(t, err, "Expected operation to be invalid")
			} else {
				assert.NoError(t, err, "Expected operation to be valid")
			}
		})
	}
}

// Stress tests
func TestValidateOperations_LargeBatch(t *testing.T) {
	const batchSize = 1000
	operations := make([]operation.MetricOperation, batchSize)

	// Fill with valid operations
	for i := 0; i < batchSize; i++ {
		operations[i] = operation.MetricOperation{
			Name:   fmt.Sprintf("metric_%d", i),
			Value:  floatPtr(float64(i)),
			Action: operation.ActionCounterAdd,
			Labels: map[string]string{
				"index": fmt.Sprintf("%d", i),
				"batch": "large",
			},
		}
	}

	err := operation.ValidateOperations(operations...)
	assert.NoError(t, err, "Expected large batch of valid operations to pass validation")
}

func TestValidateOperations_LargeBatchWithErrors(t *testing.T) {
	const batchSize = 100
	operations := make([]operation.MetricOperation, batchSize)

	// Fill with mix of valid and invalid operations
	for i := 0; i < batchSize; i++ {
		if i%2 == 0 {
			// Valid operation
			operations[i] = operation.MetricOperation{
				Name:   fmt.Sprintf("metric_%d", i),
				Value:  floatPtr(float64(i)),
				Action: operation.ActionCounterAdd,
			}
		} else {
			// Invalid operation (missing value)
			operations[i] = operation.MetricOperation{
				Name:   fmt.Sprintf("metric_%d", i),
				Action: operation.ActionCounterAdd,
			}
		}
	}

	err := operation.ValidateOperations(operations...)
	require.Error(t, err, "Expected batch with invalid operations to fail validation")

	// Count the number of errors (should be batchSize/2)
	errorCount := strings.Count(err.Error(), "'value' is required for action 'CounterAdd'")
	assert.Equal(t, batchSize/2, errorCount, "Expected %d errors for invalid operations", batchSize/2)
}

// Test all action combinations
func TestValidateMetricOperation_AllActions(t *testing.T) {
	actions := []operation.MetricAction{
		operation.ActionCounterAdd,
		operation.ActionGaugeAdd,
		operation.ActionGaugeSet,
		operation.ActionHistogramObserve,
		operation.ActionExpireMetrics,
	}

	for _, action := range actions {
		t.Run(fmt.Sprintf("action_%s", action.String()), func(t *testing.T) {
			baseOp := operation.MetricOperation{
				Name:   "test_metric",
				Value:  floatPtr(1.0),
				Action: action,
				Labels: map[string]string{"test": "label"},
			}

			switch action {
			case operation.ActionHistogramObserve:
				baseOp.Buckets = []float64{0.1, 1.0, 10.0}
			case operation.ActionExpireMetrics:
				baseOp.Group = "test_group"
				baseOp.Name = ""   // Name not required for expire
				baseOp.Value = nil // Value not required for expire
			}

			err := operation.ValidateMetricOperation(baseOp)
			assert.NoError(t, err, "Expected valid operation for action %s", action.String())
		})
	}
}

// Helper function to create float64 pointer
func floatPtr(f float64) *float64 {
	return &f
}

// Benchmark tests for performance validation
func BenchmarkValidateMetricOperation_Valid(b *testing.B) {
	op := operation.MetricOperation{
		Name:   "benchmark_metric",
		Value:  floatPtr(1.0),
		Action: operation.ActionCounterAdd,
		Labels: map[string]string{"benchmark": "true"},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = operation.ValidateMetricOperation(op)
	}
}

func BenchmarkValidateMetricOperation_Invalid(b *testing.B) {
	op := operation.MetricOperation{
		Name:   "benchmark_metric",
		Action: operation.ActionCounterAdd, // missing value
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = operation.ValidateMetricOperation(op)
	}
}

func BenchmarkValidateOperations_LargeBatch(b *testing.B) {
	const batchSize = 1000
	operations := make([]operation.MetricOperation, batchSize)

	for i := 0; i < batchSize; i++ {
		operations[i] = operation.MetricOperation{
			Name:   fmt.Sprintf("metric_%d", i),
			Value:  floatPtr(float64(i)),
			Action: operation.ActionCounterAdd,
		}
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = operation.ValidateOperations(operations...)
	}
}
