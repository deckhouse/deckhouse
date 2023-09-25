package ssh

import (
	"bufio"
	"github.com/go-logr/logr"
	"github.com/pkg/errors"
)

// Logger is a wrapper around logr.Logger that implements io.Writer.
type Logger struct {
	logger logr.Logger
	buffer []byte
	line   int
}

// NewLogger creates a new Logger.
func NewLogger(logger logr.Logger) *Logger {
	return &Logger{
		logger: logger,
	}
}

// Write implements io.Writer.
func (l *Logger) Write(p []byte) (n int, err error) {
	n = len(p)

	for {
		advance, output, err := bufio.ScanLines(p, false)
		if err != nil {
			return 0, errors.Wrap(err, "failed to scan lines")
		}

		if advance == 0 {
			l.buffer = append(l.buffer, p...)

			break
		}

		output = append(l.buffer, output...)

		l.buffer = nil

		l.line++

		l.logger.Info("ssh output", "line", l.line, "output", string(output))

		p = p[advance:]
	}

	return n, nil
}
