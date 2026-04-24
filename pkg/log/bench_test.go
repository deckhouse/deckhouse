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
	"context"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/deckhouse/deckhouse/pkg/log"
)

var fixedTime = time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)

func fixedTimeFunc(_ time.Time) time.Time { return fixedTime }

func newBenchLogger(opts ...log.Option) *log.Logger {
	defaults := []log.Option{
		log.WithOutput(io.Discard),
		log.WithLevel(slog.LevelDebug),
		log.WithTimeFunc(fixedTimeFunc),
	}
	return log.NewLogger(append(defaults, opts...)...)
}

func newStdlibLogger() *slog.Logger {
	return slog.New(slog.NewJSONHandler(io.Discard, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	}))
}

// ── Simple message, no attributes ──────────────────────────────────

func BenchmarkDeckhouse_JSON_NoAttrs(b *testing.B) {
	logger := newBenchLogger(log.WithHandlerType(log.JSONHandlerType))
	b.ResetTimer()
	b.ReportAllocs()
	for b.Loop() {
		logger.Info("simple message")
	}
}

func BenchmarkDeckhouse_Text_NoAttrs(b *testing.B) {
	logger := newBenchLogger(log.WithHandlerType(log.TextHandlerType))
	b.ResetTimer()
	b.ReportAllocs()
	for b.Loop() {
		logger.Info("simple message")
	}
}

func BenchmarkStdlib_JSON_NoAttrs(b *testing.B) {
	logger := newStdlibLogger()
	b.ResetTimer()
	b.ReportAllocs()
	for b.Loop() {
		logger.Info("simple message")
	}
}

// ── 3 string attributes ───────────────────────────────────────────

func BenchmarkDeckhouse_JSON_3Attrs(b *testing.B) {
	logger := newBenchLogger(log.WithHandlerType(log.JSONHandlerType))
	b.ResetTimer()
	b.ReportAllocs()
	for b.Loop() {
		logger.Info("request handled",
			slog.String("method", "GET"),
			slog.String("path", "/api/v1/users"),
			slog.String("status", "200"),
		)
	}
}

func BenchmarkDeckhouse_Text_3Attrs(b *testing.B) {
	logger := newBenchLogger(log.WithHandlerType(log.TextHandlerType))
	b.ResetTimer()
	b.ReportAllocs()
	for b.Loop() {
		logger.Info("request handled",
			slog.String("method", "GET"),
			slog.String("path", "/api/v1/users"),
			slog.String("status", "200"),
		)
	}
}

func BenchmarkStdlib_JSON_3Attrs(b *testing.B) {
	logger := newStdlibLogger()
	b.ResetTimer()
	b.ReportAllocs()
	for b.Loop() {
		logger.Info("request handled",
			slog.String("method", "GET"),
			slog.String("path", "/api/v1/users"),
			slog.String("status", "200"),
		)
	}
}

// ── 10 mixed-type attributes ──────────────────────────────────────

func BenchmarkDeckhouse_JSON_10Attrs(b *testing.B) {
	logger := newBenchLogger(log.WithHandlerType(log.JSONHandlerType))
	b.ResetTimer()
	b.ReportAllocs()
	for b.Loop() {
		logger.Info("detailed event",
			slog.String("service", "user-api"),
			slog.String("method", "POST"),
			slog.String("path", "/api/v1/users"),
			slog.Int("status", 201),
			slog.Duration("latency", 42*time.Millisecond),
			slog.String("user_id", "usr_abc123"),
			slog.String("request_id", "req_xyz789"),
			slog.Int64("bytes_in", 1024),
			slog.Int64("bytes_out", 4096),
			slog.Bool("cached", false),
		)
	}
}

func BenchmarkStdlib_JSON_10Attrs(b *testing.B) {
	logger := newStdlibLogger()
	b.ResetTimer()
	b.ReportAllocs()
	for b.Loop() {
		logger.Info("detailed event",
			slog.String("service", "user-api"),
			slog.String("method", "POST"),
			slog.String("path", "/api/v1/users"),
			slog.Int("status", 201),
			slog.Duration("latency", 42*time.Millisecond),
			slog.String("user_id", "usr_abc123"),
			slog.String("request_id", "req_xyz789"),
			slog.Int64("bytes_in", 1024),
			slog.Int64("bytes_out", 4096),
			slog.Bool("cached", false),
		)
	}
}

// ── Pre-bound context (With) ──────────────────────────────────────

func BenchmarkDeckhouse_JSON_WithContext(b *testing.B) {
	logger := newBenchLogger(log.WithHandlerType(log.JSONHandlerType)).
		With(slog.String("service", "user-api"), slog.String("version", "1.2.3"))
	b.ResetTimer()
	b.ReportAllocs()
	for b.Loop() {
		logger.Info("request handled",
			slog.String("method", "GET"),
			slog.String("path", "/api/v1/users"),
		)
	}
}

func BenchmarkStdlib_JSON_WithContext(b *testing.B) {
	logger := newStdlibLogger().
		With(slog.String("service", "user-api"), slog.String("version", "1.2.3"))
	b.ResetTimer()
	b.ReportAllocs()
	for b.Loop() {
		logger.Info("request handled",
			slog.String("method", "GET"),
			slog.String("path", "/api/v1/users"),
		)
	}
}

// ── Groups ────────────────────────────────────────────────────────

func BenchmarkDeckhouse_JSON_WithGroup(b *testing.B) {
	logger := newBenchLogger(log.WithHandlerType(log.JSONHandlerType)).
		WithGroup("http")
	b.ResetTimer()
	b.ReportAllocs()
	for b.Loop() {
		logger.Info("request",
			slog.String("method", "GET"),
			slog.String("path", "/api/users"),
			slog.Int("status", 200),
		)
	}
}

func BenchmarkStdlib_JSON_WithGroup(b *testing.B) {
	logger := newStdlibLogger().WithGroup("http")
	b.ResetTimer()
	b.ReportAllocs()
	for b.Loop() {
		logger.Info("request",
			slog.String("method", "GET"),
			slog.String("path", "/api/users"),
			slog.Int("status", 200),
		)
	}
}

// ── Error level (with stack trace) ────────────────────────────────

func BenchmarkDeckhouse_JSON_Error(b *testing.B) {
	logger := newBenchLogger(log.WithHandlerType(log.JSONHandlerType))
	b.ResetTimer()
	b.ReportAllocs()
	for b.Loop() {
		logger.Error("operation failed", slog.String("error", "connection refused"))
	}
}

// ── Disabled level (short-circuit) ────────────────────────────────

func BenchmarkDeckhouse_JSON_Disabled(b *testing.B) {
	logger := newBenchLogger(
		log.WithHandlerType(log.JSONHandlerType),
		log.WithLevel(slog.LevelWarn),
	)
	b.ResetTimer()
	b.ReportAllocs()
	for b.Loop() {
		logger.Debug("should be skipped", slog.String("key", "value"))
	}
}

func BenchmarkStdlib_JSON_Disabled(b *testing.B) {
	logger := slog.New(slog.NewJSONHandler(io.Discard, &slog.HandlerOptions{
		Level: slog.LevelWarn,
	}))
	b.ResetTimer()
	b.ReportAllocs()
	for b.Loop() {
		logger.Debug("should be skipped", slog.String("key", "value"))
	}
}

// ── Parallel (contention) ─────────────────────────────────────────

func BenchmarkDeckhouse_JSON_Parallel(b *testing.B) {
	logger := newBenchLogger(log.WithHandlerType(log.JSONHandlerType))
	b.ResetTimer()
	b.ReportAllocs()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			logger.Info("parallel message",
				slog.String("key", "value"),
				slog.Int("count", 42),
			)
		}
	})
}

func BenchmarkStdlib_JSON_Parallel(b *testing.B) {
	logger := newStdlibLogger()
	b.ResetTimer()
	b.ReportAllocs()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			logger.Info("parallel message",
				slog.String("key", "value"),
				slog.Int("count", 42),
			)
		}
	})
}

// ── LogAttrs (avoid any/interface boxing) ─────────────────────────

func BenchmarkDeckhouse_JSON_LogAttrs_3(b *testing.B) {
	logger := newBenchLogger(log.WithHandlerType(log.JSONHandlerType))
	ctx := context.Background()
	b.ResetTimer()
	b.ReportAllocs()
	for b.Loop() {
		logger.LogAttrs(ctx, slog.LevelInfo, "request handled",
			slog.String("method", "GET"),
			slog.String("path", "/api/v1/users"),
			slog.String("status", "200"),
		)
	}
}

func BenchmarkStdlib_JSON_LogAttrs_3(b *testing.B) {
	logger := newStdlibLogger()
	ctx := context.Background()
	b.ResetTimer()
	b.ReportAllocs()
	for b.Loop() {
		logger.LogAttrs(ctx, slog.LevelInfo, "request handled",
			slog.String("method", "GET"),
			slog.String("path", "/api/v1/users"),
			slog.String("status", "200"),
		)
	}
}

