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

package external

import (
	"bytes"
	"strings"
	"sync"
)

const maxLineSize = 4 * 1024 * 1024

type lineHandler func(line string)

// lineWriter hands every complete line to its handler the moment it arrives. os/exec's
// copying goroutine may outlive Wait when WaitDelay expires, so the state is locked.
type lineWriter struct {
	handler lineHandler

	mu      sync.Mutex
	pending []byte
}

func newLineWriter(handler lineHandler) *lineWriter {
	return &lineWriter{handler: handler}
}

func (w *lineWriter) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()

	w.pending = append(w.pending, p...)
	for {
		end := bytes.IndexByte(w.pending, '\n')
		if end < 0 {
			// A process that never writes a newline must not grow the buffer forever.
			if len(w.pending) >= maxLineSize {
				w.handler(string(w.pending))
				w.pending = w.pending[:0]
			}

			return len(p), nil
		}

		w.handler(strings.TrimSuffix(string(w.pending[:end+1]), "\n"))
		w.pending = w.pending[end+1:]
	}
}

// Flush logs a last line the process left unterminated.
func (w *lineWriter) Flush() {
	w.mu.Lock()
	defer w.mu.Unlock()

	if len(w.pending) > 0 {
		w.handler(string(w.pending))
		w.pending = w.pending[:0]
	}
}

func mergeLineHandlers(handlers ...lineHandler) lineHandler {
	return func(line string) {
		for _, handler := range handlers {
			if handler == nil {
				continue
			}
			handler(line)
		}
	}
}
