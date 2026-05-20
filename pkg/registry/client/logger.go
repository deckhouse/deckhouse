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
	"io"
	"log/slog"
)

// Logger is the structured logging interface used by the registry client.
//
// It is intentionally a tiny, slog-shaped surface so the package itself
// does not depend on any particular logger implementation. Callers can
// supply:
//
//   - a *log/slog.Logger via [NewSlogLogger] (recommended default),
//   - any drop-in wrapper that satisfies these four methods (e.g. an
//     adapter over github.com/deckhouse/deckhouse/pkg/log, zerolog, zap…).
//
// Argument conventions follow slog: pairs of (key string, value any) or
// pre-built [slog.Attr] values are both accepted by With/Debug/Info/Warn.
// The interface deliberately omits Error/Fatal — the client surfaces
// failures through return values, not through the logger.
type Logger interface {
	Debug(msg string, args ...any)
	Info(msg string, args ...any)
	Warn(msg string, args ...any)

	// With returns a child logger with the given key/value attributes
	// pre-attached. The returned value must be safe to call concurrently
	// with the parent.
	With(args ...any) Logger
}

// NewSlogLogger wraps a *slog.Logger as a [Logger]. nil l is replaced by
// [slog.Default] so the result is always safe to use.
//
// The wrapper is a no-cost passthrough: every method delegates directly,
// and With returns a new wrapper around l.With(args...) so the slog handler
// chain is preserved across nested With calls.
func NewSlogLogger(l *slog.Logger) Logger {
	if l == nil {
		l = slog.Default()
	}
	return slogLogger{l: l}
}

// DiscardLogger returns a Logger that drops every record. Useful for tests
// and for callers that want to silence client chatter without nil-checking
// at every call site (the client never panics on a nil logger because it
// substitutes [slog.Default] internally, but Discard is more explicit).
func DiscardLogger() Logger {
	return NewSlogLogger(slog.New(slog.NewTextHandler(io.Discard, nil)))
}

// slogLogger is a tiny adapter from *slog.Logger to the local Logger
// interface. Kept unexported because callers should construct it via
// NewSlogLogger / DiscardLogger.
type slogLogger struct{ l *slog.Logger }

func (s slogLogger) Debug(msg string, args ...any) { s.l.Debug(msg, args...) }
func (s slogLogger) Info(msg string, args ...any)  { s.l.Info(msg, args...) }
func (s slogLogger) Warn(msg string, args ...any)  { s.l.Warn(msg, args...) }
func (s slogLogger) With(args ...any) Logger {
	return slogLogger{l: s.l.With(args...)}
}
