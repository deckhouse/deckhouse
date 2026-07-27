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

// Package logging provides a logr.Logger adapter over pkg/log for use with controller-runtime.
package logging

import (
	"context"
	"log/slog"

	"github.com/go-logr/logr"

	"github.com/deckhouse/deckhouse/pkg/log"
)

// NewLogrAdapter wraps a *log.Logger as a logr.Logger for use with controller-runtime.
// Usage: ctrl.SetLogger(NewLogrAdapter(log.NewLogger(...)))
func NewLogrAdapter(logger *log.Logger) logr.Logger {
	return logr.New(&logrSink{logger: logger})
}

type logrSink struct {
	logger *log.Logger
}

func (s *logrSink) Init(_ logr.RuntimeInfo) {}

func (s *logrSink) Enabled(level int) bool {
	return s.logger.GetLevel().Level() <= logrLevel(level)
}

func (s *logrSink) Info(level int, msg string, keysAndValues ...interface{}) {
	s.logger.Log(context.Background(), logrLevel(level), msg, keysAndValues...)
}

func (s *logrSink) Error(err error, msg string, keysAndValues ...interface{}) {
	args := make([]interface{}, 0, len(keysAndValues)+2)
	args = append(args, keysAndValues...)
	args = append(args, "error", err)

	s.logger.Error(msg, args...)
}

func (s *logrSink) WithValues(keysAndValues ...interface{}) logr.LogSink {
	return &logrSink{logger: s.logger.With(keysAndValues...)}
}

func (s *logrSink) WithName(name string) logr.LogSink {
	return &logrSink{logger: s.logger.Named(name)}
}

func logrLevel(level int) slog.Level {
	switch {
	case level <= 0:
		return log.LevelInfo.Level()
	case level >= 2:
		return log.LevelTrace.Level()
	default:
		return log.LevelDebug.Level()
	}
}
