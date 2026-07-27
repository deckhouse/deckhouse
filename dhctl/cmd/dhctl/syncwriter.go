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

package main

import (
	"io"
	"sync"
)

// syncWriter serializes writes to an underlying WriteCloser so the slog root and the legacy
// tee logger can share the same debug log file without interleaving lines. Removed once the
// legacy logger is gone (final block).
type syncWriter struct {
	mu sync.Mutex
	w  io.WriteCloser
}

func newSyncWriter(w io.WriteCloser) *syncWriter { return &syncWriter{w: w} }

func (s *syncWriter) Write(p []byte) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.w.Write(p)
}

func (s *syncWriter) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.w.Close()
}
