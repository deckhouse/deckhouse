package logger

import (
	"bytes"
	"strings"

	"github.com/deckhouse/deckhouse/pkg/log"
)

// using for memberlist logging

type LogWriter struct {
	logger *log.Logger
}

func NewLogWriter(logger *log.Logger) *LogWriter {
	return &LogWriter{logger: logger}
}

func (lw *LogWriter) Write(p []byte) (n int, err error) {
	msg := strings.TrimSpace(string(bytes.TrimRight(p, "\n")))
	if msg == "" {
		return len(p), nil
	}

	switch {
	case strings.HasPrefix(msg, "[ERR]") || strings.HasPrefix(msg, "[ERROR]"):
		lw.logger.Error(strings.TrimPrefix(strings.TrimPrefix(msg, "[ERR]"), "[ERROR]"))
	case strings.HasPrefix(msg, "[WARN]"):
		lw.logger.Warn(strings.TrimPrefix(msg, "[WARN]"))
	case strings.HasPrefix(msg, "[DEBUG]"):
		lw.logger.Debug(strings.TrimPrefix(msg, "[DEBUG]"))
	default:
		lw.logger.Info(strings.TrimPrefix(msg, "[INFO]"))
	}
	return len(p), nil
}
