/*
Copyright 2023 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package logr

import (
	"log/slog"

	"github.com/deckhouse/deckhouse/pkg/log"
	"github.com/go-logr/logr"
)

type LogrAdapter struct {
	logger *log.Logger
}

func NewLogrAdapter(logger *log.Logger) logr.Logger {
	return logr.New(&LogrAdapter{
		logger: logger,
	})
}

func (l *LogrAdapter) Enabled(_ int) bool {
	return true
}

func (l *LogrAdapter) Info(level int, msg string, args ...any) {
	switch {
	case level <= 0:
		l.logger.With("severity", level).Info(msg, args...)
	case level >= 3:
		l.logger.With("severity", level).Debug(msg, args...)
	}
}

func (l *LogrAdapter) Error(err error, msg string, _ ...any) {
	l.logger.Error(msg, slog.String("error", err.Error()))
}

func (l *LogrAdapter) WithValues(args ...any) logr.LogSink {
	return &LogrAdapter{
		logger: l.logger.With(args...),
	}
}

func (l *LogrAdapter) WithName(name string) logr.LogSink {
	return &LogrAdapter{
		logger: l.logger.Named(name),
	}
}

func (l *LogrAdapter) Init(_ logr.RuntimeInfo) {}
