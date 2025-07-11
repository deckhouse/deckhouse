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
	"testing"

	"github.com/hashicorp/go-multierror"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/deckhouse/deckhouse/pkg/metrics-storage/operation"
)

// TestString tests the String() method of MetricOperation with different field combinations
func TestString(t *testing.T) {
	floatPtr := func(f float64) *float64 { return &f }

	tests := []struct {
		name     string
		op       operation.MetricOperation
		expected string
	}{
		{
			name:     "Empty operation",
			op:       operation.MetricOperation{},
			expected: "[]",
		},
		{
			name: "Operation with Name only",
			op: operation.MetricOperation{
				Name: "test_metric",
			},
			expected: "[name=test_metric]",
		},
		{
			name: "Operation with Group and Name",
			op: operation.MetricOperation{
				Name:  "test_metric",
				Group: "test_group",
			},
			expected: "[group=test_group, name=test_metric]",
		},
		{
			name: "Operation with Action",
			op: operation.MetricOperation{
				Name:   "test_metric",
				Action: "set",
			},
			expected: "[name=test_metric, action=set]",
		},
		{
			name: "Operation with Value",
			op: operation.MetricOperation{
				Name:  "test_metric",
				Value: floatPtr(42.5),
			},
			expected: "[name=test_metric, value=42.500000]",
		},
		{
			name: "Operation with Buckets",
			op: operation.MetricOperation{
				Name:    "test_metric",
				Buckets: []float64{1, 5, 10},
			},
			expected: "[name=test_metric, buckets=[1 5 10]]",
		},
		{
			name: "Operation with Labels",
			op: operation.MetricOperation{
				Name:   "test_metric",
				Labels: map[string]string{"label1": "value1", "label2": "value2"},
			},
			expected: "[name=test_metric, labels=map[label1:value1 label2:value2]]",
		},
		{
			name: "Operation with all fields",
			op: operation.MetricOperation{
				Name:    "test_metric",
				Group:   "test_group",
				Action:  "observe",
				Value:   floatPtr(42.5),
				Buckets: []float64{1, 5, 10},
				Labels:  map[string]string{"label1": "value1", "label2": "value2"},
			},
			expected: "[group=test_group, name=test_metric, action=observe, value=42.500000, buckets=[1 5 10], labels=map[label1:value1 label2:value2]]",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.op.String()
			assert.Equal(t, tt.expected, result)
		})
	}
}

// TestValidateOperations tests the ValidateOperations function
func TestValidateOperations(t *testing.T) {
	floatPtr := func(f float64) *float64 { return &f }

	validOp := operation.MetricOperation{
		Name:   "valid_metric",
		Action: "set",
		Value:  floatPtr(10),
	}

	invalidActionOp := operation.MetricOperation{
		Name:   "invalid_action_metric",
		Action: "invalid_action",
	}

	invalidValueOp := operation.MetricOperation{
		Name:   "missing_value_metric",
		Action: "set", // set requires value
	}

	tests := []struct {
		name        string
		operations  []operation.MetricOperation
		expectError bool
		errorCount  int
	}{
		{
			name:        "Empty operations",
			operations:  []operation.MetricOperation{},
			expectError: false,
		},
		{
			name:        "Single valid operation",
			operations:  []operation.MetricOperation{validOp},
			expectError: false,
		},
		{
			name:        "Multiple valid operations",
			operations:  []operation.MetricOperation{validOp, validOp},
			expectError: false,
		},
		{
			name:        "Single invalid operation - invalid action",
			operations:  []operation.MetricOperation{invalidActionOp},
			expectError: true,
			errorCount:  2,
		},
		{
			name:        "Single invalid operation - missing value",
			operations:  []operation.MetricOperation{invalidValueOp},
			expectError: true,
			errorCount:  1,
		},
		{
			name:        "Mix of valid and invalid operations",
			operations:  []operation.MetricOperation{validOp, invalidActionOp, invalidValueOp},
			expectError: true,
			errorCount:  3,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := operation.ValidateOperations(tt.operations)

			if tt.expectError {
				assert.Error(t, err)
				if tt.errorCount > 0 {
					merr, ok := err.(*multierror.Error)
					require.True(t, ok, "Expected multierror.Error")
					assert.Equal(t, tt.errorCount, len(merr.Errors), "Expected specific number of errors")
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

// TestValidateMetricOperation tests the ValidateMetricOperation function
func TestValidateMetricOperation(t *testing.T) {
	floatPtr := func(f float64) *float64 { return &f }

	tests := []struct {
		name        string
		op          operation.MetricOperation
		expectError bool
		errorMsg    string
	}{
		// Test action validity
		{
			name: "Invalid action",
			op: operation.MetricOperation{
				Name:   "metric",
				Action: "invalid_action",
			},
			expectError: true,
			errorMsg:    "one of: 'action', 'set' or 'add' is required",
		},

		// Test action validation with no group
		{
			name: "Valid action 'set' with no group",
			op: operation.MetricOperation{
				Name:   "metric",
				Action: "set",
				Value:  floatPtr(10),
			},
			expectError: false,
		},
		{
			name: "Valid action 'add' with no group",
			op: operation.MetricOperation{
				Name:   "metric",
				Action: "add",
				Value:  floatPtr(10),
			},
			expectError: false,
		},
		{
			name: "Valid action 'observe' with no group",
			op: operation.MetricOperation{
				Name:    "metric",
				Action:  "observe",
				Value:   floatPtr(10),
				Buckets: []float64{1, 5, 10},
			},
			expectError: false,
		},
		{
			name: "Invalid action 'expire' with no group",
			op: operation.MetricOperation{
				Name:   "metric",
				Action: "expire",
			},
			expectError: true,
			errorMsg:    "unsupported action",
		},

		// Test action validation with group
		{
			name: "Valid action 'set' with group",
			op: operation.MetricOperation{
				Name:   "metric",
				Group:  "group",
				Action: "set",
				Value:  floatPtr(10),
			},
			expectError: false,
		},
		{
			name: "Valid action 'add' with group",
			op: operation.MetricOperation{
				Name:   "metric",
				Group:  "group",
				Action: "add",
				Value:  floatPtr(10),
			},
			expectError: false,
		},
		{
			name: "Valid action 'expire' with group",
			op: operation.MetricOperation{
				Group:  "group",
				Action: "expire",
			},
			expectError: false,
		},
		{
			name: "Invalid action 'observe' with group",
			op: operation.MetricOperation{
				Name:    "metric",
				Group:   "group",
				Action:  "observe",
				Value:   floatPtr(10),
				Buckets: []float64{1, 5, 10},
			},
			expectError: true,
			errorMsg:    "unsupported action",
		},

		// Test name validation
		{
			name: "Missing name and group",
			op: operation.MetricOperation{
				Action: "set",
				Value:  floatPtr(10),
			},
			expectError: true,
			errorMsg:    "'name' is required",
		},
		{
			name: "With group but missing name for non-expire action",
			op: operation.MetricOperation{
				Group:  "group",
				Action: "set",
				Value:  floatPtr(10),
			},
			expectError: true,
			errorMsg:    "'name' is required when action is not 'expire'",
		},
		{
			name: "With group and expire action but no name - valid",
			op: operation.MetricOperation{
				Group:  "group",
				Action: "expire",
			},
			expectError: false,
		},

		// Test value validation for different actions
		{
			name: "Action 'set' with missing value",
			op: operation.MetricOperation{
				Name:   "metric",
				Action: "set",
			},
			expectError: true,
			errorMsg:    "'value' is required for action 'set'",
		},
		{
			name: "Action 'add' with missing value",
			op: operation.MetricOperation{
				Name:   "metric",
				Action: "add",
			},
			expectError: true,
			errorMsg:    "'value' is required for action 'add'",
		},
		{
			name: "Action 'observe' with missing value",
			op: operation.MetricOperation{
				Name:    "metric",
				Action:  "observe",
				Buckets: []float64{1, 5, 10},
			},
			expectError: true,
			errorMsg:    "'value' is required for action 'observe'",
		},
		{
			name: "Action 'observe' with missing buckets",
			op: operation.MetricOperation{
				Name:   "metric",
				Action: "observe",
				Value:  floatPtr(10),
			},
			expectError: true,
			errorMsg:    "'buckets' is required for action 'observe'",
		},

		// Complex valid cases
		{
			name: "Complete valid 'set' operation",
			op: operation.MetricOperation{
				Name:   "metric",
				Action: "set",
				Value:  floatPtr(42.5),
				Labels: map[string]string{"label": "value"},
			},
			expectError: false,
		},
		{
			name: "Complete valid 'observe' operation",
			op: operation.MetricOperation{
				Name:    "metric",
				Action:  "observe",
				Value:   floatPtr(42.5),
				Buckets: []float64{1, 5, 10},
				Labels:  map[string]string{"label": "value"},
			},
			expectError: false,
		},
		{
			name: "Complete valid grouped 'expire' operation",
			op: operation.MetricOperation{
				Group:  "group",
				Action: "expire",
				Labels: map[string]string{"label": "value"},
			},
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := operation.ValidateMetricOperation(tt.op)

			if tt.expectError {
				assert.Error(t, err)
				if tt.errorMsg != "" {
					assert.Contains(t, err.Error(), tt.errorMsg)
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
