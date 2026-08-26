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

// Package task runs long operations in the background: one per subject at a
// time, kept until its result is collected, with a notification when it ends.
// A reconciler starts the work, returns, and is woken to read the outcome.
package task

import (
	"context"
	"errors"
	"sync"
)

var ErrExists = errors.New("task already exists")

type (
	TaskID   string
	TaskFn   func(ctx context.Context) error
	NotifyFn func(ctx context.Context)
)

type task struct {
	cancel context.CancelFunc
	done   chan struct{}
	err    error
}

type Manager struct {
	mu    sync.Mutex
	tasks map[TaskID]*task
}

func NewManager() *Manager {
	return &Manager{
		tasks: make(map[TaskID]*task),
	}
}

// Start launches a background task.
// Returns ErrExists if the id already has a task, running or finished.
//
// ctx is the parent of the task's own context — pass the long-lived one, not a
// reconcile's, or the task dies with the call that started it.
func (m *Manager) Start(ctx context.Context, id TaskID, taskFn TaskFn, notifyFn NotifyFn) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.tasks[id]; exists {
		return ErrExists
	}

	ctx, cancel := context.WithCancel(ctx)
	t := &task{
		cancel: cancel,
		done:   make(chan struct{}),
	}
	m.tasks[id] = t

	go func() {
		defer cancel()

		var err error
		if taskFn != nil {
			err = taskFn(ctx)
		}

		// Published before anybody is told to come and read it.
		m.mu.Lock()
		t.err = err
		close(t.done)
		m.mu.Unlock()

		if notifyFn != nil {
			notifyFn(ctx)
		}
	}()

	return nil
}

// Result reports whether the task for id has finished and, if so, its error,
// forgetting it so the next Start runs afresh. Still running or absent is not
// finished.
func (m *Manager) Result(id TaskID) (bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	t, exists := m.tasks[id]
	if !exists {
		return false, nil
	}

	select {
	case <-t.done:
		delete(m.tasks, id)
		return true, t.err
	default:
		return false, nil
	}
}

// Cancel stops the task for id and waits for its goroutine to return, so the
// caller may undo its side effects without racing it, and reports whether there
// was one. A ctx that expires first returns its error; the task is forgotten
// either way.
//
// ponytail: the wait is bounded only by ctx. A drain stops within ~6s (a budget
// retry sleeps a flat 5s); give Cancel its own deadline if a future task cannot
// promise that.
func (m *Manager) Cancel(ctx context.Context, id TaskID) (bool, error) {
	m.mu.Lock()
	t, exists := m.tasks[id]
	if !exists {
		m.mu.Unlock()
		return false, nil
	}
	delete(m.tasks, id)
	m.mu.Unlock()

	t.cancel()

	select {
	case <-t.done:
		return true, nil
	case <-ctx.Done():
		return false, ctx.Err()
	}
}
