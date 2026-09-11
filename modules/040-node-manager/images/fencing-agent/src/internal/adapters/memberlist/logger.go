/*
Copyright 2026 Flant JSC

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

package memberlist

import (
	"bytes"
	"io"
	"log/slog"
	"strings"

	"github.com/deckhouse/deckhouse/pkg/log"
)

var debugPrefix = []byte("[DEBUG]")

// logWriter maps memberlist's "[LEVEL] ..." log lines onto the agent logger.
type logWriter struct {
	logger *log.Logger
	debug  bool
}

func newLogWriter(logger *log.Logger) io.Writer {
	return &logWriter{
		logger: logger,
		debug:  logger.GetLevel().Level() <= slog.LevelDebug,
	}
}

func (w *logWriter) Write(p []byte) (int, error) {
	// Drop suppressed debug lines before parsing: memberlist is chatty and this
	// runs on its goroutines.
	if !w.debug && bytes.HasPrefix(p, debugPrefix) {
		return len(p), nil
	}

	level, msg := splitLevel(string(bytes.TrimSpace(p)))

	switch level {
	case "ERR", "ERROR":
		w.logger.Error(msg)
	case "WARN":
		w.logger.Warn(msg)
	case "DEBUG":
		w.logger.Debug(msg)
	default:
		w.logger.Info(msg)
	}

	return len(p), nil
}

func splitLevel(line string) (string, string) {
	if !strings.HasPrefix(line, "[") {
		return "", line
	}

	end := strings.Index(line, "]")
	if end < 0 {
		return "", line
	}

	return line[1:end], strings.TrimSpace(line[end+1:])
}
