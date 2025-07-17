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
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/deckhouse/deckhouse/pkg/metrics-storage/operation"
)

// TestMetricActionConstants verifies the constant values
func TestMetricActionConstants(t *testing.T) {
	// Test that the constants have the expected string values
	assert.Equal(t, "set", string(operation.ActionOldGaugeSet))
	assert.Equal(t, "add", string(operation.ActionCounterAdd))
	assert.Equal(t, "observe", string(operation.ActionHistogramObserve))
	assert.Equal(t, "expire", string(operation.ActionExpireMetrics))
}

// TestIsValid tests the validity checking for actions
func TestAction_IsValid(t *testing.T) {
	tests := []struct {
		name   string
		action operation.MetricAction
		valid  bool
	}{
		// Valid actions
		{name: "ActionSet", action: operation.ActionOldGaugeSet, valid: true},
		{name: "ActionAdd", action: operation.ActionCounterAdd, valid: true},
		{name: "ActionObserve", action: operation.ActionHistogramObserve, valid: true},
		{name: "ActionExpire", action: operation.ActionExpireMetrics, valid: true},

		// Invalid actions
		{name: "Empty string", action: "", valid: false},
		{name: "Undefined action", action: "undefined", valid: false},
		{name: "Case sensitivity", action: "SET", valid: false},
		{name: "Misspelled action", action: "sett", valid: false},
		{name: "Action with whitespace", action: " set ", valid: false},
		{name: "Numeric action", action: "123", valid: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			isValid := tt.action.IsValid()
			assert.Equal(t, tt.valid, isValid, "Expected IsValid() to return %v for '%s'", tt.valid, tt.action)
		})
	}
}

// TestString tests the string representation of actions
func TestAction_String(t *testing.T) {
	tests := []struct {
		name     string
		action   operation.MetricAction
		expected string
	}{
		{name: "ActionSet", action: operation.ActionOldGaugeSet, expected: "set"},
		{name: "ActionAdd", action: operation.ActionCounterAdd, expected: "add"},
		{name: "ActionObserve", action: operation.ActionHistogramObserve, expected: "observe"},
		{name: "ActionExpire", action: operation.ActionExpireMetrics, expected: "expire"},
		{name: "Empty action", action: "", expected: ""},
		{name: "Custom action", action: "custom", expected: "custom"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.action.String()
			assert.Equal(t, tt.expected, result)
		})
	}
}

// TestMetricActionInSwitch tests using actions in switch statements
func TestMetricActionInSwitch(t *testing.T) {
	tests := []struct {
		name     string
		action   operation.MetricAction
		expected string
	}{
		{name: "ActionSet", action: operation.ActionOldGaugeSet, expected: "set case"},
		{name: "ActionAdd", action: operation.ActionCounterAdd, expected: "add case"},
		{name: "ActionObserve", action: operation.ActionHistogramObserve, expected: "observe case"},
		{name: "ActionExpire", action: operation.ActionExpireMetrics, expected: "expire case"},
		{name: "Unknown action", action: "unknown", expected: "default case"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var result string

			// This tests how the enum would be used in typical code
			switch tt.action {
			case operation.ActionOldGaugeSet:
				result = "set case"
			case operation.ActionCounterAdd:
				result = "add case"
			case operation.ActionHistogramObserve:
				result = "observe case"
			case operation.ActionExpireMetrics:
				result = "expire case"
			default:
				result = "default case"
			}

			assert.Equal(t, tt.expected, result)
		})
	}
}

// TestJSONSerialization tests serializing actions to/from JSON
func TestJSONSerialization(t *testing.T) {
	// Define test structure
	type TestStruct struct {
		Action operation.MetricAction `json:"action"`
	}

	tests := []struct {
		name     string
		action   operation.MetricAction
		expected string
	}{
		{name: "ActionSet", action: operation.ActionOldGaugeSet, expected: `{"action":"set"}`},
		{name: "ActionAdd", action: operation.ActionCounterAdd, expected: `{"action":"add"}`},
		{name: "ActionObserve", action: operation.ActionHistogramObserve, expected: `{"action":"observe"}`},
		{name: "ActionExpire", action: operation.ActionExpireMetrics, expected: `{"action":"expire"}`},
		{name: "Empty action", action: "", expected: `{"action":""}`},
		{name: "Custom action", action: "custom", expected: `{"action":"custom"}`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Test marshaling
			testStruct := TestStruct{Action: tt.action}
			data, err := json.Marshal(testStruct)
			require.NoError(t, err)
			assert.JSONEq(t, tt.expected, string(data))

			// Test unmarshaling
			var unmarshaled TestStruct
			err = json.Unmarshal(data, &unmarshaled)
			require.NoError(t, err)
			assert.Equal(t, tt.action, unmarshaled.Action)
		})
	}
}

// TestActionComparison tests comparing action values
func TestActionComparison(t *testing.T) {
	// Test with variables
	action1 := operation.ActionOldGaugeSet
	action2 := operation.ActionOldGaugeSet
	action3 := operation.ActionCounterAdd

	assert.True(t, action1 == action2)
	assert.False(t, action1 == action3)

	// Test with string conversion
	assert.True(t, operation.ActionOldGaugeSet == "set")
	assert.True(t, operation.ActionCounterAdd == "add")
}
