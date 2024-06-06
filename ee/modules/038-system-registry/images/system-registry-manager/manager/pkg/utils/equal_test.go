package utils

import (
	"testing"
)

func TestEqualYaml(t *testing.T) {
	tests := []struct {
		name         string
		lYamlContent string
		rYamlContent string
		expected     bool
		expectError  bool
	}{
		{
			name:         "Equal YAMLs",
			lYamlContent: "key: value\n",
			rYamlContent: "key: value\n",
			expected:     true,
			expectError:  false,
		},
		{
			name:         "Different YAMLs",
			lYamlContent: "key: value\n",
			rYamlContent: "key: different\n",
			expected:     false,
			expectError:  false,
		},
		{
			name:         "Invalid YAML",
			lYamlContent: "qwewerwer",
			rYamlContent: "key: value\n",
			expected:     false,
			expectError:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			actual, err := EqualYaml(tt.lYamlContent, tt.rYamlContent)

			if tt.expectError && err == nil {
				t.Errorf("Expected error, but got nil")
			}

			if !tt.expectError && err != nil {
				t.Errorf("Unexpected error: %v", err)
			}

			if actual != tt.expected {
				t.Errorf("Expected %v, but got %v", tt.expected, actual)
			}
		})
	}
}

func TestEqualJson(t *testing.T) {
	tests := []struct {
		name         string
		lJsonContent string
		rJsonContent string
		expected     bool
	}{
		{
			name:         "Equal JSONs",
			lJsonContent: `{"key": "value"}`,
			rJsonContent: `{"key": "value"}`,
			expected:     true,
		},
		{
			name:         "Different JSONs",
			lJsonContent: `{"key": "value"}`,
			rJsonContent: `{"key": "different"}`,
			expected:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			actual := EqualJson(tt.lJsonContent, tt.rJsonContent)
			if actual != tt.expected {
				t.Errorf("Expected %v, but got %v", tt.expected, actual)
			}
		})
	}
}
