/*
Copyright 2024 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package utils

import (
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestEqualYaml(t *testing.T) {
	tests := []struct {
		name     string
		input1   []byte
		input2   []byte
		expected bool
	}{
		{
			name:     "Equal YAML",
			input1:   []byte("key: value"),
			input2:   []byte("key: value"),
			expected: true,
		},
		{
			name:     "Different YAML",
			input1:   []byte("key1: value1"),
			input2:   []byte("key2: value2"),
			expected: false,
		},
		{
			name:     "Invalid YAML",
			input1:   []byte("invalid yaml"),
			input2:   []byte("key: value"),
			expected: false,
		},
		{
			name:     "YAML with different order",
			input1:   []byte("a: \"a\"\nb: \"b\""),
			input2:   []byte("b: \"b\"\na: \"a\""),
			expected: true,
		},
		{
			name:     "YAML with nested structures",
			input1:   []byte("key:\n  subkey: value"),
			input2:   []byte("key:\n  subkey: value"),
			expected: true,
		},
		// Add more test cases as needed
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := EqualYaml(tc.input1, tc.input2)
			assert.Equal(t, tc.expected, result)
		})
	}
}

func TestEqualJson(t *testing.T) {
	tests := []struct {
		name     string
		input1   []byte
		input2   []byte
		expected bool
	}{
		{
			name:     "Equal JSON",
			input1:   []byte(`{"key": "value"}`),
			input2:   []byte(`{"key": "value"}`),
			expected: true,
		},
		{
			name:     "Different JSON",
			input1:   []byte(`{"key1": "value1"}`),
			input2:   []byte(`{"key2": "value2"}`),
			expected: false,
		},
		{
			name:     "Invalid JSON",
			input1:   []byte("invalid json"),
			input2:   []byte(`{"key": "value"}`),
			expected: false,
		},
		{
			name:     "JSON with different order",
			input1:   []byte(`{"a": "a", "b": "b"}`),
			input2:   []byte(`{"b": "b", "a": "a"}`),
			expected: true,
		},
		{
			name:     "JSON with nested structures",
			input1:   []byte(`{"key": {"subkey": "value"}}`),
			input2:   []byte(`{"key": {"subkey": "value"}}`),
			expected: true,
		},
		// Add more test cases as needed
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := EqualJson(tc.input1, tc.input2)
			assert.Equal(t, tc.expected, result)
		})
	}
}
