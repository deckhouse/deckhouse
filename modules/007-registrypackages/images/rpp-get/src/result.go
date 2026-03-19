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

package main

import (
	"bufio"
	"fmt"
	"os"
	"sync"
)

const (
	resultInstalled = "installed"
	resultSkipped   = "skipped"
	resultRemoved   = "removed"
)

type resultRecorder struct {
	file *os.File
	w    *bufio.Writer
	mu   sync.Mutex
}

func newResultRecorder(path string) *resultRecorder {
	if path == "" {
		return nil
	}

	file, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		// Return a recorder that will surface the error on the first record() call.
		// This keeps the nil-safe contract without silently dropping results.
		return &resultRecorder{}
	}

	return &resultRecorder{
		file: file,
		w:    bufio.NewWriter(file),
	}
}

func (r *resultRecorder) record(action, packageName string) error {
	if r == nil {
		return nil
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	if r.file == nil {
		return fmt.Errorf("result file is not open")
	}

	if _, err := fmt.Fprintf(r.w, "%s %s\n", action, packageName); err != nil {
		return fmt.Errorf("write result file: %w", err)
	}

	if err := r.w.Flush(); err != nil {
		return fmt.Errorf("flush result file: %w", err)
	}

	return nil
}

func (r *resultRecorder) close() error {
	if r == nil || r.file == nil {
		return nil
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	if err := r.w.Flush(); err != nil {
		_ = r.file.Close()
		return fmt.Errorf("flush result file: %w", err)
	}

	return r.file.Close()
}
