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

package rpp

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
)

type loggerWrapper struct {
	logger *slog.Logger
}

func newLogger(logger *slog.Logger) *loggerWrapper {
	return &loggerWrapper{
		logger: logger,
	}
}

func (w *loggerWrapper) Errorf(format string, args ...any) {
	w.logger.ErrorContext(context.Background(), fmt.Sprintf(format, args...))
}

func (w *loggerWrapper) Infof(format string, args ...any) {
	// suppress shutdown message it need for server, not for dhctl
	if strings.HasPrefix(format, "graceful shutdown") {
		return
	}

	w.logger.InfoContext(context.Background(), fmt.Sprintf(format, args...))
}

func (w *loggerWrapper) Warnf(format string, args ...any) {
	w.logger.WarnContext(context.Background(), fmt.Sprintf(format, args...))
}

func (w *loggerWrapper) Debugf(format string, args ...any) {
	w.logger.DebugContext(context.Background(), fmt.Sprintf(format, args...))
}

func (w *loggerWrapper) Error(msg string, args ...any) {
	w.Errorf(msg, args...)
}

type interactiveLoggerWrapper struct {
	logger *slog.Logger
}

func newInteractiveLogger(logger *slog.Logger) *interactiveLoggerWrapper {
	return &interactiveLoggerWrapper{
		logger: logger,
	}
}

func (w *interactiveLoggerWrapper) Errorf(format string, args ...any) {
	w.logger.DebugContext(context.Background(), fmt.Sprintf(format, args...))
}

func (w *interactiveLoggerWrapper) Infof(format string, args ...any) {
	// suppress shutdown message it need for server, not for dhctl
	if strings.HasPrefix(format, "graceful shutdown") {
		return
	}

	w.logger.DebugContext(context.Background(), fmt.Sprintf(format, args...))
}

func (w *interactiveLoggerWrapper) Warnf(format string, args ...any) {
	w.logger.DebugContext(context.Background(), fmt.Sprintf(format, args...))
}

func (w *interactiveLoggerWrapper) Debugf(format string, args ...any) {
	w.logger.DebugContext(context.Background(), fmt.Sprintf(format, args...))
}

func (w *interactiveLoggerWrapper) Error(msg string, args ...any) {
	w.logger.DebugContext(context.Background(), msg)
}
