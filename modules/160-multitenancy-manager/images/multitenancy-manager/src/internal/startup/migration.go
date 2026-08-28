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

package startup

import (
	"context"
	"sync"
)

// Migration is a one-shot barrier: the namespace manager marks it done after the
// leftover-project migration finishes, and the project manager waits on it so
// ensureTemplateName cannot persist "simple" before Migrate has a chance to infer
// the real template and stamp Helm ownership.
type Migration struct {
	done chan struct{}
	once sync.Once
}

func NewMigration() *Migration {
	return &Migration{done: make(chan struct{})}
}

// MarkDone closes the barrier. Safe to call more than once and on a nil receiver.
func (m *Migration) MarkDone() {
	if m == nil {
		return
	}
	m.once.Do(func() { close(m.done) })
}

// Wait blocks until MarkDone or ctx is cancelled. A nil receiver is a no-op so
// tests that do not wire the barrier keep working.
func (m *Migration) Wait(ctx context.Context) error {
	if m == nil {
		return nil
	}
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-m.done:
		return nil
	}
}
