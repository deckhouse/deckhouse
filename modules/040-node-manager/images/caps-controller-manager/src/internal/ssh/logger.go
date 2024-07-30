/*
Copyright 2023 Flant JSC

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

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

		l.logger.Info("OpenSSH client output", "line", l.line, "output", string(output))

		p = p[advance:]
	}

	return n, nil
}
