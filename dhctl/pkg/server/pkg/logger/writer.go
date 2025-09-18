// Copyright 2024 Flant JSC
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
	"log/slog"
	"sync"
)

type LogWriter[T any] struct {
	l      *slog.Logger
	sendCh chan T
	f      func([]string) T

	m    sync.Mutex
	prev []byte
}

func NewLogWriter[T any](l *slog.Logger, sendCh chan T, f func(lines []string) T) *LogWriter[T] {
	return &LogWriter[T]{
		l:      l,
		sendCh: sendCh,
		f:      f,
	}
}

func (w *LogWriter[T]) Write(p []byte) (n int, err error) {
	w.m.Lock()
	defer w.m.Unlock()

	var lines []string

	for _, b := range p {
		switch b {
		case '\n', '\r':
			s := string(w.prev)
			if s != "" {
				lines = append(lines, s)
			}
			w.prev = []byte{}
		default:
			w.prev = append(w.prev, b)
		}
	}

	if len(lines) > 0 {
		for _, line := range lines {
			w.l.Info(line)
		}
		w.sendCh <- w.f(lines)
	}

	return len(p), nil
}

type DebugLogWriter struct {
	l *slog.Logger
}

func NewDebugLogWriter(l *slog.Logger) *DebugLogWriter {
	return &DebugLogWriter{
		l: l,
	}
}

func (w *DebugLogWriter) Write(p []byte) (n int, err error) {
	// slog is thread safe
	w.l.Debug(string(p))

	return len(p), nil
}
