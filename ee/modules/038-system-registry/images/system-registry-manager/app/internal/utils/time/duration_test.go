/*
Copyright 2024 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package time

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"sigs.k8s.io/yaml"
)

// WithPointer struct - Event with a pointer to Duration
type WithPointer struct {
	Duration *Duration `json:"duration" yaml:"duration"`
}

// WithoutPointer struct - Event without a pointer to Duration
type WithoutPointer struct {
	Duration Duration `json:"duration" yaml:"duration"`
}

func TestWithPointer_UnmarshalJSON(t *testing.T) {
	tests := []struct {
		name        string
		inputjson   string
		expected    WithPointer
		expectError bool
	}{
		{
			name:        "Valid string",
			inputjson:   `{"duration": "2h30m"}`,
			expected:    WithPointer{Duration: &Duration{2*time.Hour + 30*time.Minute}},
			expectError: false,
		},
		{
			name:        "Valid string 2",
			inputjson:   `{"duration": "2h6ns"}`,
			expected:    WithPointer{Duration: &Duration{2*time.Hour + 6*time.Nanosecond}},
			expectError: false,
		},
		{
			name:        "Valid float64",
			inputjson:   `{"duration": -10.0001}`,
			expected:    WithPointer{Duration: &Duration{-10 * time.Nanosecond}},
			expectError: false,
		},
		{
			name:        "Valid float64 2",
			inputjson:   `{"duration": 10.0001}`,
			expected:    WithPointer{Duration: &Duration{10 * time.Nanosecond}},
			expectError: false,
		},
		{
			name:        "Empty duration",
			inputjson:   `{"duration": null}`,
			expected:    WithPointer{Duration: nil}, // Invalid input should result in nil
			expectError: false,
		},
		{
			name:        "Invalid duration",
			inputjson:   `{"duration": "invalid"}`,
			expected:    WithPointer{Duration: nil}, // Invalid input should result in nil
			expectError: true,
		},
		{
			name:        "Invalid duration 2",
			inputjson:   `{"duration": ""}`,
			expected:    WithPointer{Duration: nil}, // Invalid input should result in nil
			expectError: true,
		},
		{
			name:        "Empty JSON object",
			inputjson:   `{}`,
			expected:    WithPointer{Duration: nil}, // Empty object should leave pointer nil
			expectError: false,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var e WithPointer
			err := json.Unmarshal([]byte(test.inputjson), &e)

			if test.expectError {
				assert.Error(t, err, test.name)
				return
			}

			assert.NoError(t, err, test.name)

			// Check for DurationWithPointer
			if test.expected.Duration == nil {
				assert.Nil(t, e.Duration, test.name)
			} else {
				assert.NotNil(t, e.Duration, test.name)
				assert.Equal(t, test.expected.Duration.Duration, e.Duration.Duration, test.name)
			}
		})
	}
}

func TestWithoutPointer_UnmarshalJSON(t *testing.T) {
	tests := []struct {
		name        string
		inputjson   string
		expected    WithoutPointer
		expectError bool
	}{
		{
			name:        "Valid string",
			inputjson:   `{"duration": "2h30m"}`,
			expected:    WithoutPointer{Duration: Duration{2*time.Hour + 30*time.Minute}},
			expectError: false,
		},
		{
			name:        "Valid string 2",
			inputjson:   `{"duration": "2h6ns"}`,
			expected:    WithoutPointer{Duration: Duration{2*time.Hour + 6*time.Nanosecond}},
			expectError: false,
		},
		{
			name:        "Valid float64",
			inputjson:   `{"duration": -10.0001}`,
			expected:    WithoutPointer{Duration: Duration{-10 * time.Nanosecond}},
			expectError: false,
		},
		{
			name:        "Valid float64 2",
			inputjson:   `{"duration": 10.0001}`,
			expected:    WithoutPointer{Duration: Duration{10 * time.Nanosecond}},
			expectError: false,
		},
		{
			name:        "Empty duration",
			inputjson:   `{"duration": null}`,
			expected:    WithoutPointer{Duration: Duration{0}}, // Invalid input should result in 0 duration
			expectError: true,
		},
		{
			name:        "Invalid duration",
			inputjson:   `{"duration": "invalid"}`,
			expected:    WithoutPointer{Duration: Duration{0}}, // Invalid input should result in 0 duration
			expectError: true,
		},
		{
			name:        "Invalid duration 2",
			inputjson:   `{"duration": ""}`,
			expected:    WithoutPointer{Duration: Duration{0}}, // Invalid input should result in 0 duration
			expectError: true,
		},
		{
			name:        "Empty JSON object",
			inputjson:   `{}`,
			expected:    WithoutPointer{Duration: Duration{0}}, // Empty object should default to 0s
			expectError: false,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var e WithoutPointer
			err := json.Unmarshal([]byte(test.inputjson), &e)

			if test.expectError {
				assert.Error(t, err, test.name)
				return
			}

			assert.NoError(t, err, test.name)

			// Check for DurationWithoutPointer
			assert.Equal(t, test.expected.Duration.Duration, e.Duration.Duration, test.name)
		})
	}
}

func TestWithPointer_UnmarshalYAML(t *testing.T) {
	tests := []struct {
		name        string
		inputyaml   string
		expected    WithPointer
		expectError bool
	}{
		{
			name:        "Valid string",
			inputyaml:   `duration: "2h30m"`,
			expected:    WithPointer{Duration: &Duration{2*time.Hour + 30*time.Minute}},
			expectError: false,
		},
		{
			name:        "Valid string 2",
			inputyaml:   `duration: "2h6ns"`,
			expected:    WithPointer{Duration: &Duration{2*time.Hour + 6*time.Nanosecond}},
			expectError: false,
		},
		{
			name:        "Valid float64",
			inputyaml:   `duration: -10.0001`,
			expected:    WithPointer{Duration: &Duration{-10 * time.Nanosecond}},
			expectError: false,
		},
		{
			name:        "Valid float64 2",
			inputyaml:   `duration: 10.0001`,
			expected:    WithPointer{Duration: &Duration{10 * time.Nanosecond}},
			expectError: false,
		},
		{
			name:        "Empty duration",
			inputyaml:   `duration: null`,
			expected:    WithPointer{Duration: nil}, // Invalid input should result in nil
			expectError: false,
		},
		{
			name:        "Invalid duration",
			inputyaml:   `duration: "invalid"`,
			expected:    WithPointer{Duration: nil}, // Invalid input should result in nil
			expectError: true,
		},
		{
			name:        "Invalid duration 2",
			inputyaml:   `duration: ""`,
			expected:    WithPointer{Duration: nil}, // Invalid input should result in nil
			expectError: true,
		},
		{
			name:        "Empty YAML object",
			inputyaml:   `{}`,
			expected:    WithPointer{Duration: nil}, // Empty object should leave pointer nil
			expectError: false,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var e WithPointer
			err := yaml.Unmarshal([]byte(test.inputyaml), &e)

			if test.expectError {
				assert.Error(t, err, test.name)
				return
			}

			assert.NoError(t, err, test.name)

			// Check for DurationWithPointer
			if test.expected.Duration == nil {
				assert.Nil(t, e.Duration, test.name)
			} else {
				assert.NotNil(t, e.Duration, test.name)
				assert.Equal(t, test.expected.Duration.Duration, e.Duration.Duration, test.name)
			}
		})
	}
}

func TestWithoutPointer_UnmarshalYAML(t *testing.T) {
	tests := []struct {
		name        string
		inputyaml   string
		expected    WithoutPointer
		expectError bool
	}{
		{
			name:        "Valid string",
			inputyaml:   `duration: "2h30m"`,
			expected:    WithoutPointer{Duration: Duration{2*time.Hour + 30*time.Minute}},
			expectError: false,
		},
		{
			name:        "Valid string 2",
			inputyaml:   `duration: "2h6ns"`,
			expected:    WithoutPointer{Duration: Duration{2*time.Hour + 6*time.Nanosecond}},
			expectError: false,
		},
		{
			name:        "Valid float64",
			inputyaml:   `duration: -10.0001`,
			expected:    WithoutPointer{Duration: Duration{-10 * time.Nanosecond}},
			expectError: false,
		},
		{
			name:        "Valid float64 2",
			inputyaml:   `duration: 10.0001`,
			expected:    WithoutPointer{Duration: Duration{10 * time.Nanosecond}},
			expectError: false,
		},
		{
			name:        "Empty duration",
			inputyaml:   `duration: null`,
			expected:    WithoutPointer{Duration: Duration{0}}, // Invalid input should result in 0 duration
			expectError: true,
		},
		{
			name:        "Invalid duration",
			inputyaml:   `duration: "invalid"`,
			expected:    WithoutPointer{Duration: Duration{0}}, // Invalid input should result in 0 duration
			expectError: true,
		},
		{
			name:        "Invalid duration 2",
			inputyaml:   `duration: ""`,
			expected:    WithoutPointer{Duration: Duration{0}}, // Invalid input should result in 0 duration
			expectError: true,
		},
		{
			name:        "Empty YAML object",
			inputyaml:   `{}`,
			expected:    WithoutPointer{Duration: Duration{0}}, // Empty object should default to 0s
			expectError: false,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var e WithoutPointer
			err := yaml.Unmarshal([]byte(test.inputyaml), &e)

			if test.expectError {
				assert.Error(t, err, test.name)
				return
			}

			assert.NoError(t, err, test.name)

			// Check for DurationWithoutPointer
			assert.Equal(t, test.expected.Duration.Duration, e.Duration.Duration, test.name)
		})
	}
}
