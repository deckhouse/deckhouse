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

func (lw *LogWriter) Write(p []byte) (int, error) {
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
