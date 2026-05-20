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

package client

import (
	"bytes"
	"log/slog"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestLogger_SlogAdapter_RoutesAllLevels verifies the adapter is a true
// passthrough: every level lands on the underlying *slog.Logger and the
// attributes from With propagate to the final record.
func TestLogger_SlogAdapter_RoutesAllLevels(t *testing.T) {
	var buf bytes.Buffer
	handler := slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug})
	logger := NewSlogLogger(slog.New(handler)).With("component", "registry-test")

	logger.Debug("dbg msg", slog.String("kind", "dbg"))
	logger.Info("inf msg")
	logger.Warn("wrn msg")

	out := buf.String()
	assert.Contains(t, out, "dbg msg")
	assert.Contains(t, out, "inf msg")
	assert.Contains(t, out, "wrn msg")
	assert.Contains(t, out, "component=registry-test")
	assert.Contains(t, out, "kind=dbg")
}

// TestLogger_NilFallsBackToDefault locks in the documented contract: a nil
// *slog.Logger turns into slog.Default() so callers never hit a nil deref.
func TestLogger_NilFallsBackToDefault(t *testing.T) {
	logger := NewSlogLogger(nil)
	// Smoke: any of the four methods must not panic. We don't capture
	// slog.Default()'s output because it goes to stderr and asserting on
	// that would be flaky.
	logger.Debug("debug")
	logger.Info("info")
	logger.Warn("warn")
	_ = logger.With("k", "v")
}

// TestLogger_Discard drops everything.
func TestLogger_Discard(t *testing.T) {
	logger := DiscardLogger()
	logger.Debug("should not appear")
	logger.Info("nor this")
	logger.Warn("or this")
	// Just smoke: nothing to assert on, the point is "no panic, no output".
}

// TestLogger_WithReturnsLogger_Chainable proves successive With calls keep
// piling attributes onto the slog handler rather than starting from scratch.
func TestLogger_WithReturnsLogger_Chainable(t *testing.T) {
	var buf bytes.Buffer
	handler := slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug})
	logger := NewSlogLogger(slog.New(handler)).
		With("a", "1").
		With("b", "2").
		With("c", "3")

	logger.Info("hi")
	out := buf.String()

	for _, want := range []string{"a=1", "b=2", "c=3", "hi"} {
		if !strings.Contains(out, want) {
			t.Fatalf("missing %q in %q", want, out)
		}
	}
}
