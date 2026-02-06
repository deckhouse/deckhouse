package logger

import (
	"bytes"
	"regexp"
	"strings"

	"github.com/deckhouse/deckhouse/pkg/log"
)

// using for memberlist logging

// memberlist log format: "2026/02/06 13:28:27 [LEVEL] memberlist: message"
var memberlistLogRegex = regexp.MustCompile(`^\d{4}/\d{2}/\d{2} \d{2}:\d{2}:\d{2} \[(\w+)\] (.*)$`)

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

	level := "INFO"
	text := msg

	if matches := memberlistLogRegex.FindStringSubmatch(msg); len(matches) == 3 {
		level = matches[1]
		text = matches[2]
	}

	switch level {
	case "ERR", "ERROR":
		lw.logger.Error(text)
	case "WARN":
		lw.logger.Warn(text)
	case "DEBUG":
		lw.logger.Debug(text)
	default:
		lw.logger.Info(text)
	}

	return len(p), nil
}
