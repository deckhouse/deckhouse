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

package logger

import (
	"context"
	"log/slog"
	"sync"
)

// LineWriter is an io.Writer that turns each newline-terminated line into an Info record.
// Used to pipe external command (exec) output into the logger. Replaces logger-as-io.Writer.
// When fileOnly is set, each line is tagged FileOnly so it lands in the debug file but never the
// compact terminal.
type LineWriter struct {
	l        *slog.Logger
	fileOnly bool
	mu       sync.Mutex
	prev     []byte
}

// NewLineWriter returns a LineWriter emitting each line as a plain Info record.
func NewLineWriter(l *slog.Logger) *LineWriter { return &LineWriter{l: l} }

// newFileOnlyLineWriter returns a LineWriter whose lines are tagged FileOnly: they enrich the debug
// file but never reach the terminal. Used for shell-operator/klog capture (enabled by DHCTL_DEBUG),
// which must not change the terminal output.
func newFileOnlyLineWriter(l *slog.Logger) *LineWriter { return &LineWriter{l: l, fileOnly: true} }

func (w *LineWriter) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()

	for _, b := range p {
		switch b {
		case '\n', '\r':
			if line := string(w.prev); line != "" {
				w.emit(line)
			}
			w.prev = w.prev[:0]
		default:
			w.prev = append(w.prev, b)
		}
	}
	return len(p), nil
}

func (w *LineWriter) emit(line string) {
	if w.fileOnly {
		w.l.LogAttrs(context.Background(), slog.LevelInfo, line, FileOnly())
		return
	}
	w.l.Info(line)
}
