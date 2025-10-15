//nolint:goprintffuncname
package nelm

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/deckhouse/deckhouse/pkg/log"
	nelmlog "github.com/werf/nelm/pkg/log"
)

// Ensure nelmLogger implements the nelmlog.Logger interface
var _ nelmlog.Logger = (*nelmLogger)(nil)

// newNelmLogger creates a new logger adapter that wraps the Deckhouse logger
// to implement the nelm logger interface
func newNelmLogger(logger *log.Logger) *nelmLogger {
	return &nelmLogger{
		logger: logger,
	}
}

// nelmLogger is an adapter that bridges the nelm logger interface with Deckhouse's logging system
type nelmLogger struct {
	logger *log.Logger
}

// Trace logs a trace-level message (mapped to debug with trace flag)
func (n *nelmLogger) Trace(ctx context.Context, format string, a ...interface{}) {
	n.logger.With(slog.Bool("trace", true)).Log(ctx, log.LevelDebug.Level(), fmt.Sprintf(format, a...))
}

// TraceStruct logs a trace-level message with a structured object
func (n *nelmLogger) TraceStruct(ctx context.Context, obj interface{}, format string, a ...interface{}) {
	n.logger.With(slog.Bool("trace", true)).Log(ctx, log.LevelDebug.Level(), fmt.Sprintf(format, a...), slog.Any("obj", obj))
}

// TracePush starts a new trace-level log context (nelm uses this for indentation)
func (n *nelmLogger) TracePush(ctx context.Context, _, format string, a ...interface{}) {
	n.logger.With(slog.Bool("trace", true)).Log(ctx, log.LevelDebug.Level(), fmt.Sprintf(format, a...))
}

// TracePop ends a trace-level log context (no-op in this implementation)
func (n *nelmLogger) TracePop(_ context.Context, _ string) {
	// No-op: we don't maintain a stack for indentation
}

// Debug logs a debug-level message
func (n *nelmLogger) Debug(ctx context.Context, format string, a ...interface{}) {
	n.logger.DebugContext(ctx, fmt.Sprintf(format, a...))
}

// DebugPush starts a new debug-level log context
func (n *nelmLogger) DebugPush(ctx context.Context, _, format string, a ...interface{}) {
	n.logger.DebugContext(ctx, fmt.Sprintf(format, a...))
}

// DebugPop ends a debug-level log context (no-op)
func (n *nelmLogger) DebugPop(_ context.Context, _ string) {
}

// Info logs an info-level message
func (n *nelmLogger) Info(ctx context.Context, format string, a ...interface{}) {
	n.logger.InfoContext(ctx, fmt.Sprintf(format, a...))
}

// InfoPush starts a new info-level log context
func (n *nelmLogger) InfoPush(ctx context.Context, _, format string, a ...interface{}) {
	n.logger.InfoContext(ctx, fmt.Sprintf(format, a...))
}

// InfoPop ends an info-level log context (no-op)
func (n *nelmLogger) InfoPop(_ context.Context, _ string) {
}

// Warn logs a warning-level message
func (n *nelmLogger) Warn(ctx context.Context, format string, a ...interface{}) {
	n.logger.WarnContext(ctx, fmt.Sprintf(format, a...))
}

// WarnPush starts a new warning-level log context
func (n *nelmLogger) WarnPush(ctx context.Context, _, format string, a ...interface{}) {
	n.logger.WarnContext(ctx, fmt.Sprintf(format, a...))
}

// WarnPop ends a warning-level log context (no-op)
func (n *nelmLogger) WarnPop(_ context.Context, _ string) {
}

// Error logs an error-level message
func (n *nelmLogger) Error(ctx context.Context, format string, a ...interface{}) {
	n.logger.ErrorContext(ctx, fmt.Sprintf(format, a...))
}

// ErrorPush starts a new error-level log context
func (n *nelmLogger) ErrorPush(ctx context.Context, _, format string, a ...interface{}) {
	n.logger.ErrorContext(ctx, fmt.Sprintf(format, a...))
}

// ErrorPop ends an error-level log context (no-op)
func (n *nelmLogger) ErrorPop(_ context.Context, _ string) {
}

// InfoBlock logs a block title and executes a function within that context
// Nelm uses this for grouping related log messages
func (n *nelmLogger) InfoBlock(ctx context.Context, opts nelmlog.BlockOptions, fn func()) {
	n.logger.InfoContext(ctx, opts.BlockTitle)

	fn()
}

// InfoBlockErr logs a block title and executes a function that may return an error
func (n *nelmLogger) InfoBlockErr(ctx context.Context, opts nelmlog.BlockOptions, fn func() error) error {
	n.logger.InfoContext(ctx, opts.BlockTitle)

	return fmt.Errorf("inner func err: %w", fn())
}

// BlockContentWidth returns the preferred width for block content formatting
func (n *nelmLogger) BlockContentWidth(_ context.Context) int {
	return 120
}

// SetLevel converts a nelm log level to Deckhouse log level and sets it
func (n *nelmLogger) SetLevel(_ context.Context, lvl nelmlog.Level) {
	newLvl := log.LevelInfo

	// Map nelm log levels to Deckhouse log levels
	switch lvl {
	case nelmlog.TraceLevel:
		newLvl = log.LevelTrace
	case nelmlog.DebugLevel:
		newLvl = log.LevelDebug
	case nelmlog.InfoLevel:
		newLvl = log.LevelInfo
	case nelmlog.WarningLevel:
		newLvl = log.LevelWarn
	case nelmlog.ErrorLevel:
		newLvl = log.LevelError
	}

	n.logger.SetLevel(newLvl)
}

// Level returns the current log level in nelm's format
func (n *nelmLogger) Level(_ context.Context) nelmlog.Level {
	// Map Deckhouse log levels to nelm log levels
	switch n.logger.GetLevel() {
	case log.LevelTrace:
		return nelmlog.TraceLevel
	case log.LevelDebug:
		return nelmlog.DebugLevel
	case log.LevelInfo:
		return nelmlog.InfoLevel
	case log.LevelWarn:
		return nelmlog.WarningLevel
	case log.LevelError:
		return nelmlog.ErrorLevel
	case log.LevelFatal:
		return nelmlog.ErrorLevel
	default:
		return nelmlog.InfoLevel
	}
}

// AcceptLevel determines if a given log level should be logged
// Always returns true as level filtering is handled by the underlying logger
func (n *nelmLogger) AcceptLevel(_ context.Context, _ nelmlog.Level) bool {
	return true
}

// With creates a new logger with additional structured logging fields
func (n *nelmLogger) With(args ...any) *nelmLogger {
	return &nelmLogger{
		logger: n.logger.With(args...),
	}
}

// EnrichWithLabels adds label key-value pairs to the logger as structured fields
// This is used by nelm to add contextual information like release names, namespaces, etc.
func (n *nelmLogger) EnrichWithLabels(labelsMaps ...map[string]string) *nelmLogger {
	for _, labels := range labelsMaps {
		for k, v := range labels {
			n.logger = n.logger.With(slog.String(k, v))
		}
	}

	return n
}
