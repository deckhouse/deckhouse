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

// outputHandler logs every complete line the moment it arrives. os/exec's copying
// goroutine may outlive Wait when WaitDelay expires, so the state is locked.
type outputHandler struct {
	logLine func(line string)

	mu      sync.Mutex
	pending []byte
}

func newOutputHandler(logLine func(line string)) *outputHandler {
	return &outputHandler{logLine: logLine}
}

func (o *outputHandler) Write(p []byte) (int, error) {
	o.mu.Lock()
	defer o.mu.Unlock()

	o.pending = append(o.pending, p...)
	for {
		end := bytes.IndexByte(o.pending, '\n')
		if end < 0 {
			// A process that never writes a newline must not grow the buffer forever.
			if len(o.pending) >= maxLineSize {
				o.logLine(string(o.pending))
				o.pending = o.pending[:0]
			}

			return len(p), nil
		}

		o.logLine(strings.TrimSuffix(string(o.pending[:end+1]), "\n"))
		o.pending = o.pending[end+1:]
	}
}

// Flush logs a last line the process left unterminated.
func (o *outputHandler) Flush() {
	o.mu.Lock()
	defer o.mu.Unlock()

	if len(o.pending) > 0 {
		o.logLine(string(o.pending))
		o.pending = o.pending[:0]
	}
}
