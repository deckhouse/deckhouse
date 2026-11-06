// Copyright 2026 Flant JSC
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

package log_test

import (
	"errors"
	"io"
	"log/slog"
	"testing"

	"github.com/deckhouse/deckhouse/pkg/log"
	"github.com/stretchr/testify/assert"
)

func TestType(t *testing.T) {
	t.Parallel()

	attr := log.Type("key", "string")
	assert.Equal(t, "key", attr.Key)
	assert.Equal(t, "string", attr.Value.String())

	attr = log.Type("type", 42)
	assert.Equal(t, "type", attr.Key)
	assert.Equal(t, "int", attr.Value.String())
}

func TestErr(t *testing.T) {
	t.Parallel()

	// Test with nil error
	attr := log.Err(nil)
	assert.Equal(t, "error", attr.Key)
	assert.Equal(t, "nil", attr.Value.String())

	// Test with actual error
	err := errors.New("test error")
	attr = log.Err(err)
	assert.Equal(t, "error", attr.Key)
	assert.Equal(t, "test error", attr.Value.String())

	// Test with custom error type
	customErr := &customError{message: "custom error message"}
	attr = log.Err(customErr)
	assert.Equal(t, "error", attr.Key)
	assert.Equal(t, "custom error message", attr.Value.String())

	// Test with nil custom error pointer
	var nilCustomErr *customError
	attr = log.Err(nilCustomErr)
	assert.Equal(t, "error", attr.Key)
	assert.Equal(t, "nil", attr.Value.String())

	// Test with custom error value type
	valueErr := valueError{message: "value error message"}
	attr = log.Err(valueErr)
	assert.Equal(t, "error", attr.Key)
	assert.Equal(t, "value error message", attr.Value.String())
}

// customError is a simple custom error type for testing (pointer receiver)
type customError struct {
	message string
}

func (e *customError) Error() string {
	return e.message
}

// valueError is a custom error type with value receiver
type valueError struct {
	message string
}

func (e valueError) Error() string {
	return e.message
}

func TestRawJSON(t *testing.T) {
	t.Parallel()

	// Test with valid JSON
	validJSON := `{"key": "value"}`
	attr := log.RawJSON("data", validJSON)
	assert.Equal(t, "data", attr.Key)
	// Not parsed until the record is written
	assert.Equal(t, slog.KindLogValuer, attr.Value.Kind())
	// The resolved value should be slog.AnyValue with the map
	val := attr.Value.Resolve()
	assert.Equal(t, slog.KindAny, val.Kind())
	m := val.Any().(map[string]any)
	assert.Equal(t, "value", m["key"])

	// Test with invalid JSON
	invalidJSON := `{"key": "value"`
	attr = log.RawJSON("data", invalidJSON)
	assert.Equal(t, "data", attr.Key)
	assert.Equal(t, slog.KindString, attr.Value.Resolve().Kind())
	assert.Equal(t, invalidJSON, attr.Value.Resolve().String())
}

func TestRawYAML(t *testing.T) {
	t.Parallel()

	// Test with valid YAML
	validYAML := `key: value`
	attr := log.RawYAML("data", validYAML)
	assert.Equal(t, "data", attr.Key)
	// Not parsed until the record is written
	assert.Equal(t, slog.KindLogValuer, attr.Value.Kind())
	val := attr.Value.Resolve()
	assert.Equal(t, slog.KindAny, val.Kind())
	m := val.Any().(map[string]any)
	assert.Equal(t, "value", m["key"])

	// Test with invalid YAML
	invalidYAML := `key: value:`
	attr = log.RawYAML("data", invalidYAML)
	assert.Equal(t, "data", attr.Key)
	assert.Equal(t, slog.KindString, attr.Value.Resolve().Kind())
	assert.Equal(t, invalidYAML, attr.Value.Resolve().String())
}

// Values for Debug used to be parsed even when Debug was off.
func TestRawIsNotParsedForDisabledLevel(t *testing.T) {
	l := log.NewLogger(log.WithLevel(slog.LevelInfo), log.WithOutput(io.Discard))

	allocs := testing.AllocsPerRun(100, func() {
		l.Debug("values", log.RawJSON("json", `{"a": [1, 2, 3], "b": {"c": "d"}}`), log.RawYAML("yaml", "a:\n  b: [1, 2, 3]\n"))
	})

	// parsing takes about a hundred allocations; creating and passing the two attributes takes a few
	assert.Less(t, allocs, float64(10))
}

func TestNewJSONRaw(t *testing.T) {
	t.Parallel()

	raw := log.NewJSONRaw(`{"test": "data"}`)
	assert.NotNil(t, raw)
	// Test via LogValue
	val := raw.LogValue()
	assert.Equal(t, slog.KindAny, val.Kind())
	m := val.Any().(map[string]any)
	assert.Equal(t, "data", m["test"])
}

func TestNewYAMLRaw(t *testing.T) {
	t.Parallel()

	raw := log.NewYAMLRaw(`test: data`)
	assert.NotNil(t, raw)
	val := raw.LogValue()
	assert.Equal(t, slog.KindAny, val.Kind())
	m := val.Any().(map[string]any)
	assert.Equal(t, "data", m["test"])
}

func TestRaw_LogValue(t *testing.T) {
	t.Parallel()

	// Test valid JSON
	raw := log.NewJSONRaw(`{"key": "value"}`)
	val := raw.LogValue()
	assert.Equal(t, slog.KindAny, val.Kind())
	m := val.Any().(map[string]any)
	assert.Equal(t, "value", m["key"])

	// Test invalid JSON
	raw = log.NewJSONRaw(`invalid json`)
	val = raw.LogValue()
	assert.Equal(t, slog.KindString, val.Kind())
	assert.Equal(t, "invalid json", val.String())

	// Test valid YAML
	raw = log.NewYAMLRaw(`key: value`)
	val = raw.LogValue()
	assert.Equal(t, slog.KindAny, val.Kind())
	m = val.Any().(map[string]any)
	assert.Equal(t, "value", m["key"])

	// Test invalid YAML
	raw = log.NewYAMLRaw(`invalid: yaml:`)
	val = raw.LogValue()
	assert.Equal(t, slog.KindString, val.Kind())
	assert.Equal(t, "invalid: yaml:", val.String())
}
