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

package logger

import (
	"context"
	"log/slog"
)

// RunProcess wraps fn with process start/end (or start/fail) marker records, so the handler
// can render a process block. The returned error is fn's error, unchanged.
func RunProcess(ctx context.Context, l *slog.Logger, name string, fn func(context.Context) error) error {
	emit(ctx, l, slog.LevelInfo, "Starting: "+name, processAttr(processStart, name))
	if err := fn(ctx); err != nil {
		emit(ctx, l, slog.LevelError, "Failed: "+name, processAttr(processFail, name))
		return err
	}
	emit(ctx, l, slog.LevelInfo, "Finished: "+name, processAttr(processEnd, name))
	return nil
}

func emit(ctx context.Context, l *slog.Logger, level slog.Level, msg string, attrs []slog.Attr) {
	// Marker records carry only their process attrs; the handler routes them to the renderer via
	// isRendererMarker (they are renderer control, not compact-view text).
	l.LogAttrs(ctx, level, msg, attrs...)
}

// Success logs a success line. It is NOT tagged for the compact view — only successful PHASE
// transitions appear there (emitted by the phases progress consumer). Generic successes are
// verbose-only.
func Success(ctx context.Context, l *slog.Logger, msg string) {
	l.InfoContext(ctx, "✅ "+msg)
}

// Fail logs a failure line at Error level, so it is always visible (compact and verbose).
func Fail(ctx context.Context, l *slog.Logger, msg string) {
	l.ErrorContext(ctx, "❌ "+msg)
}

// FailRetry logs a retryable failure at Warn level, so it is always visible.
func FailRetry(ctx context.Context, l *slog.Logger, msg string) {
	l.WarnContext(ctx, "🔄 "+msg)
}

// JSON writes a raw JSON payload as a file record (not TTY-tagged).
func JSON(ctx context.Context, l *slog.Logger, data []byte) {
	l.InfoContext(ctx, string(data))
}
