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
	"bytes"
	"context"
	"fmt"
	"log/slog"
	"strings"

	libdhctl_log "github.com/deckhouse/lib-dhctl/pkg/log"
)

// Adapter wraps slog.Logger and implements libdhctl_log.LoggerProvider
// and its internal interfaces.
type Adapter struct {
	logger *slog.Logger
	ctx    context.Context
}

var _ libdhctl_log.Logger = (*Adapter)(nil)

// NewAdapter creates a new logger adapter.
func NewAdapter(logger *slog.Logger) *Adapter {
	return &Adapter{
		logger: logger,
		ctx:    context.Background(),
	}
}

// NewLibdhctlAdapter is the canonical constructor used at the lib-connection boundary.
// It binds ctx so every record the adapter emits carries the caller's context
// (cancellation, trace span, context values) instead of a detached background one.
func NewLibdhctlAdapter(ctx context.Context) *Adapter {
	return NewAdapter(FromContext(ctx)).WithContext(ctx)
}

// WithContext returns a new Adapter with the given context.
func (a *Adapter) WithContext(ctx context.Context) *Adapter {
	return &Adapter{
		logger: a.logger,
		ctx:    ctx,
	}
}

// FlushAndClose implements libdhctl_log.Logger.
func (a *Adapter) FlushAndClose() error {
	return nil
}

// Process implements libdhctl_log.Logger.
func (a *Adapter) Process(p libdhctl_log.Process, t string, run func() error) error {
	return RunProcess(a.ctx, a.logger, t, func(context.Context) error { return run() })
}

// The Info/Warn/Error/Debug methods below carry lib-connection's streamed per-line output
// (bashible/ssh/exec, including remote `set -x` stderr). They are tagged FileOnly(): the debug
// file keeps everything, but the compact terminal never shows them — even at Error level they would
// flood the screen, so the user is pointed to the debug-log file. -v reveals them. The curated,
// always-visible output comes from the process boxes (ProcessLogger) and Fail (red badge).

func (a *Adapter) stream(level slog.Level, msg string) {
	a.logger.LogAttrs(a.ctx, level, msg, FileOnly())
}

func (a *Adapter) InfoFWithoutLn(format string, args ...interface{}) {
	a.stream(slog.LevelInfo, fmt.Sprintf(format, args...))
}

func (a *Adapter) InfoLn(args ...interface{}) {
	a.stream(slog.LevelInfo, fmt.Sprint(args...))
}

func (a *Adapter) ErrorFWithoutLn(format string, args ...interface{}) {
	a.stream(slog.LevelError, fmt.Sprintf(format, args...))
}

func (a *Adapter) ErrorLn(args ...interface{}) {
	a.stream(slog.LevelError, fmt.Sprint(args...))
}

func (a *Adapter) DebugFWithoutLn(format string, args ...interface{}) {
	a.stream(slog.LevelDebug, fmt.Sprintf(format, args...))
}

func (a *Adapter) DebugLn(args ...interface{}) {
	a.stream(slog.LevelDebug, fmt.Sprint(args...))
}

func (a *Adapter) WarnFWithoutLn(format string, args ...interface{}) {
	a.stream(slog.LevelWarn, fmt.Sprintf(format, args...))
}

func (a *Adapter) WarnLn(args ...interface{}) {
	a.stream(slog.LevelWarn, fmt.Sprint(args...))
}

func (a *Adapter) InfoF(format string, args ...interface{}) {
	a.stream(slog.LevelInfo, strings.TrimSuffix(fmt.Sprintf(format, args...), "\n"))
}

func (a *Adapter) ErrorF(format string, args ...interface{}) {
	a.stream(slog.LevelError, strings.TrimSuffix(fmt.Sprintf(format, args...), "\n"))
}

func (a *Adapter) DebugF(format string, args ...interface{}) {
	a.stream(slog.LevelDebug, strings.TrimSuffix(fmt.Sprintf(format, args...), "\n"))
}

func (a *Adapter) WarnF(format string, args ...interface{}) {
	a.stream(slog.LevelWarn, strings.TrimSuffix(fmt.Sprintf(format, args...), "\n"))
}

// Success and FailRetry are per-process / per-attempt lib-connection notices, emitted in bulk
// (each dependency check, each retry). They are NOT compact-tagged: they stay in the debug file
// and only surface with -v. Only successful PHASE transitions appear in the compact view (see
// pkg/operations/phases). FailRetry is retry noise — logged at Debug so a recovered retry stays
// quiet. Fail is a real failure: it surfaces by its Error level and renders the red FAILED badge.
func (a *Adapter) Success(msg string)   { a.logger.InfoContext(a.ctx, msg) }
func (a *Adapter) FailRetry(msg string) { a.logger.DebugContext(a.ctx, msg) }

func (a *Adapter) Fail(msg string) {
	a.logger.LogAttrs(a.ctx, slog.LevelError, msg, BadgeFailed())
}

func (a *Adapter) Warning(msg string) {
	a.stream(slog.LevelWarn, msg)
}

func (a *Adapter) SilentLogger() *libdhctl_log.SilentLogger {
	return libdhctl_log.NewSilentLogger()
}

func (a *Adapter) JSON(data []byte) {
	a.stream(slog.LevelInfo, string(data))
}

func (a *Adapter) Write(p []byte) (int, error) {
	a.stream(slog.LevelInfo, strings.TrimSuffix(string(p), "\n"))
	return len(p), nil
}

// BufferLogger returns an Adapter writing every record to buffer as sanitized JSON. The buffer is
// never a terminal, so this is a plain file-style sink (no progress UI).
func (a *Adapter) BufferLogger(buffer *bytes.Buffer) libdhctl_log.Logger {
	lv := new(slog.LevelVar)
	lv.Set(slog.LevelDebug)
	return NewAdapter(slog.New(slog.NewJSONHandler(buffer, handlerOptions(lv))))
}
