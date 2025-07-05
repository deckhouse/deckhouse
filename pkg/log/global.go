// Copyright 2024 Flant JSC
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

// global logger is deprecated
package log

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"sync/atomic"

	logContext "github.com/deckhouse/deckhouse/pkg/log/context"
)

var defaultLogger atomic.Pointer[Logger]

func init() {
	defaultLogger.Store(NewLogger(WithLevel(slog.LevelInfo)))
}

func SetDefault(l *Logger) {
	defaultLogger.Store(l)
}

func SetDefaultLevel(l Level) {
	defaultLogger.Load().SetLevel(l)
}

func Default() *Logger { return defaultLogger.Load() }

func Log(ctx context.Context, level Level, msg string, args ...any) {
	ctx = logContext.SetCustomKeyContext(ctx)
	Default().Log(ctx, level.Level(), msg, args...)
}

// Deprecated: use Log instead
func Logf(ctx context.Context, level Level, format string, args ...any) {
	ctx = logContext.SetCustomKeyContext(ctx)
	Default().Log(ctx, level.Level(), fmt.Sprintf(format, args...))
}

func LogAttrs(ctx context.Context, level Level, msg string, attrs ...slog.Attr) {
	ctx = logContext.SetCustomKeyContext(ctx)
	Default().LogAttrs(ctx, level.Level(), msg, attrs...)
}

func Debug(msg string, args ...any) {
	ctx := logContext.SetCustomKeyContext(context.Background())
	Default().Log(ctx, LevelDebug.Level(), msg, args...)
}

// Deprecated: use Debug instead
func Debugf(format string, args ...any) {
	ctx := logContext.SetCustomKeyContext(context.Background())
	Default().Log(ctx, LevelDebug.Level(), fmt.Sprintf(format, args...))
}

func DebugContext(ctx context.Context, msg string, args ...any) {
	ctx = logContext.SetCustomKeyContext(ctx)
	Default().Log(ctx, LevelDebug.Level(), msg, args...)
}

func Info(msg string, args ...any) {
	ctx := logContext.SetCustomKeyContext(context.Background())
	Default().Log(ctx, LevelInfo.Level(), msg, args...)
}

// Deprecated: use Info instead
func Infof(format string, args ...any) {
	ctx := logContext.SetCustomKeyContext(context.Background())
	Default().Log(ctx, LevelInfo.Level(), fmt.Sprintf(format, args...))
}

func InfoContext(ctx context.Context, msg string, args ...any) {
	ctx = logContext.SetCustomKeyContext(ctx)
	Default().Log(ctx, LevelInfo.Level(), msg, args...)
}

func Warn(msg string, args ...any) {
	ctx := logContext.SetCustomKeyContext(context.Background())
	Default().Log(ctx, LevelWarn.Level(), msg, args...)
}

// Deprecated: use Warn instead
func Warnf(format string, args ...any) {
	ctx := logContext.SetCustomKeyContext(context.Background())
	Default().Log(ctx, LevelWarn.Level(), fmt.Sprintf(format, args...))
}

func WarnContext(ctx context.Context, msg string, args ...any) {
	ctx = logContext.SetCustomKeyContext(ctx)
	Default().Log(ctx, LevelWarn.Level(), msg, args...)
}

func Error(msg string, args ...any) {
	ctx := logContext.SetCustomKeyContext(context.Background())
	ctx = logContext.SetStackTraceContext(ctx, getStack())
	Default().Log(ctx, LevelError.Level(), msg, args...)
}

// Deprecated: use Error instead
func Errorf(format string, args ...any) {
	ctx := logContext.SetCustomKeyContext(context.Background())
	ctx = logContext.SetStackTraceContext(ctx, getStack())
	Default().Log(ctx, LevelError.Level(), fmt.Sprintf(format, args...))
}

func ErrorContext(ctx context.Context, msg string, args ...any) {
	ctx = logContext.SetCustomKeyContext(ctx)
	Default().Log(ctx, LevelError.Level(), msg, args...)
}

func Fatal(msg string, args ...any) {
	ctx := logContext.SetCustomKeyContext(context.Background())
	ctx = logContext.SetStackTraceContext(ctx, getStack())

	Default().Log(ctx, LevelFatal.Level(), msg, args...)

	os.Exit(1)
}

// Deprecated: use Fatal instead
func Fatalf(format string, args ...any) {
	ctx := logContext.SetCustomKeyContext(context.Background())
	ctx = logContext.SetStackTraceContext(ctx, getStack())

	Default().Log(ctx, LevelFatal.Level(), fmt.Sprintf(format, args...))

	os.Exit(1)
}

func FatalContext(ctx context.Context, msg string, args ...any) {
	ctx = logContext.SetCustomKeyContext(ctx)
	ctx = logContext.SetStackTraceContext(ctx, getStack())

	Default().Log(ctx, LevelFatal.Level(), msg, args...)
	os.Exit(1)
}
