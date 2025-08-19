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
	"fmt"
	"log/slog"
	"os"
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

	m     sync.Mutex
	prev  []byte
	lines []string
}

func NewDebugLogWriter(l *slog.Logger) *DebugLogWriter {
	return &DebugLogWriter{
		l: l,
	}
}

func (w *DebugLogWriter) Write(p []byte) (n int, err error) {
	fmt.Fprintln(os.Stderr, "Try to lock debug log writer")
	w.m.Lock()

	fmt.Fprintln(os.Stderr, "Locked debug log writer. Closed channel to finish monitor. Defer to unlock")

	defer func() {
		fmt.Fprintln(os.Stderr, "Try to unlock debug log writer")
		w.m.Unlock()
		fmt.Fprintln(os.Stderr, "Debug log writer unlocked")
	}()

	fmt.Fprintf(os.Stderr, "Split log %s by line\n", string(p))

	for _, b := range p {
		switch b {
		case '\n', '\r':
			s := string(w.prev)
			if s != "" {
				w.lines = append(w.lines, s)
			}
			w.prev = []byte{}
		default:
			w.prev = append(w.prev, b)
		}
	}

	fmt.Fprintf(os.Stderr, "Splited log %s by line; lines %d\n", string(p), len(w.lines))

	if len(w.lines) > 0 {
		for _, line := range w.lines {
			fmt.Fprintf(os.Stderr, "debudlogwriter: write to sterr: %s\n", line)
			//w.l.Debug(line)
			//fmt.Fprintf(os.Stderr, "debudlogwriter: sent to logger: %s\n", line)
		}
	}

	fmt.Fprintf(os.Stderr, "debudlogwriter: starting getting len of bufffer\n")

	llen := len(p)

	fmt.Fprintf(os.Stderr, "debudlogwriter: got len of buffer. Set lines to nil %d\n", llen)
	w.lines = nil

	return llen, nil
}
