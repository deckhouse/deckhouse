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

package task

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// notifyWait only expires when a notification is genuinely lost; the tasks here
// end at once.
const notifyWait = 10 * time.Second

// The tests synchronise on channels rather than on sleeps: a task waits to be
// released, and a notify callback reports completion. A manager whose whole job
// is goroutine bookkeeping cannot be tested by a stopwatch without going flaky
// on a loaded machine.

// held is the size of the registry, which is what every leak would show up in.
func held(m *Manager) int {
	m.mu.Lock()
	defer m.mu.Unlock()

	return len(m.tasks)
}

// TestManager_RegistryIsEmptiedAgain walks a task through every way it can end
// and checks what the registry is left holding. A task that outlives its own
// completion is a node that never gets a second eviction.
func TestManager_RegistryIsEmptiedAgain(t *testing.T) {
	for _, tc := range []struct {
		name     string
		finish   bool // let the task return
		collect  bool // read the result
		cancel   bool // stop it instead
		wantHeld int
	}{
		{name: "a running task is held", wantHeld: 1},
		{name: "a finished task is held until its result is read", finish: true, wantHeld: 1},
		{name: "reading the result forgets the task", finish: true, collect: true, wantHeld: 0},
		{name: "cancelling forgets the task", cancel: true, wantHeld: 0},
		{name: "cancelling a finished task forgets it too", finish: true, cancel: true, wantHeld: 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			release := make(chan struct{})
			done := make(chan struct{}, 1)
			m := NewManager()

			err := m.Start(t.Context(), "node-1",
				func(ctx context.Context) error {
					select {
					case <-release:
						return nil
					case <-ctx.Done():
						return ctx.Err()
					}
				},
				func(context.Context) { done <- struct{}{} })
			if err != nil {
				t.Fatalf("start: %v", err)
			}

			if tc.finish {
				close(release)
				<-done
			} else {
				t.Cleanup(func() { close(release) })
			}

			if tc.collect {
				if finished, err := m.Result("node-1"); !finished || err != nil {
					t.Fatalf("result = (%v, %v), want (true, nil)", finished, err)
				}
			}
			if tc.cancel {
				if _, err := m.Cancel(t.Context(), "node-1"); err != nil {
					t.Fatalf("cancel: %v", err)
				}
			}

			if got := held(m); got != tc.wantHeld {
				t.Fatalf("registry holds %d tasks, want %d", got, tc.wantHeld)
			}
		})
	}
}

// TestManager_ResultReportsTheOutcomeOnce is the other half of forgetting a
// task: the result is handed over exactly once, and the id is free afterwards.
func TestManager_ResultReportsTheOutcomeOnce(t *testing.T) {
	boom := errors.New("boom")
	done := make(chan struct{}, 1)
	m := NewManager()

	if err := m.Start(t.Context(), "node-1",
		func(context.Context) error { return boom },
		func(context.Context) { done <- struct{}{} }); err != nil {
		t.Fatalf("start: %v", err)
	}
	<-done

	if finished, err := m.Result("node-1"); !finished || !errors.Is(err, boom) {
		t.Fatalf("first result = (%v, %v), want (true, boom)", finished, err)
	}
	if finished, err := m.Result("node-1"); finished || err != nil {
		t.Fatalf("second result = (%v, %v), want (false, nil)", finished, err)
	}

	release := make(chan struct{})
	t.Cleanup(func() { close(release) })
	if err := m.Start(t.Context(), "node-1", func(context.Context) error { <-release; return nil }, nil); err != nil {
		t.Fatalf("start after the result was read: %v", err)
	}
}

// TestManager_NotifiesHoweverTheTaskEnded pins the notification to the task
// ending, not to how it ended. It runs on the parent context rather than the
// task's own, so moving a timeout onto the task cannot silently swallow it — a
// caller waiting for the notification would otherwise wait for ever.
func TestManager_NotifiesHoweverTheTaskEnded(t *testing.T) {
	for _, tc := range []struct {
		name   string
		cancel bool
	}{
		{name: "the task returns on its own"},
		{name: "the task is cancelled", cancel: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			release := make(chan struct{})
			notified := make(chan struct{}, 1)
			m := NewManager()

			err := m.Start(context.Background(), "node-1",
				func(ctx context.Context) error {
					select {
					case <-release:
						return nil
					case <-ctx.Done():
						return ctx.Err()
					}
				},
				func(context.Context) { notified <- struct{}{} })
			if err != nil {
				t.Fatalf("start: %v", err)
			}

			if tc.cancel {
				if _, err := m.Cancel(t.Context(), "node-1"); err != nil {
					t.Fatalf("cancel: %v", err)
				}
			} else {
				close(release)
			}

			select {
			case <-notified:
			case <-time.After(notifyWait):
				t.Fatal("the task ended without notifying")
			}
		})
	}
}

// TestManager_StartsOneTaskPerID is the invariant the whole manager exists for.
func TestManager_StartsOneTaskPerID(t *testing.T) {
	release := make(chan struct{})
	t.Cleanup(func() { close(release) })
	m := NewManager()

	var runs atomic.Int32
	fn := func(context.Context) error {
		runs.Add(1)
		<-release
		return nil
	}

	var started atomic.Int32
	var wg sync.WaitGroup
	for range 10 {
		wg.Go(func() {
			switch err := m.Start(t.Context(), "node-1", fn, nil); {
			case err == nil:
				started.Add(1)
			case errors.Is(err, ErrExists):
			default:
				t.Errorf("start: %v", err)
			}
		})
	}
	wg.Wait()

	if got := started.Load(); got != 1 {
		t.Fatalf("%d calls started a task, want 1", got)
	}
	if got := held(m); got != 1 {
		t.Fatalf("registry holds %d tasks, want 1", got)
	}
}

func TestManager_DifferentIDsRunAtTheSameTime(t *testing.T) {
	release := make(chan struct{})
	t.Cleanup(func() { close(release) })
	started := make(chan TaskID, 2)
	m := NewManager()

	for _, id := range []TaskID{"node-1", "node-2"} {
		if err := m.Start(t.Context(), id, func(context.Context) error {
			started <- id
			<-release
			return nil
		}, nil); err != nil {
			t.Fatalf("start %s: %v", id, err)
		}
	}

	seen := map[TaskID]bool{<-started: true}
	seen[<-started] = true
	if !seen["node-1"] || !seen["node-2"] {
		t.Fatalf("both tasks should be running, saw %v", seen)
	}
}

// TestManager_CancelWaitsForTheGoroutine is the contract a caller undoing a
// task's side effects depends on: by the time Cancel returns, nothing is left
// running to race with.
func TestManager_CancelWaitsForTheGoroutine(t *testing.T) {
	started := make(chan struct{}, 1)
	stopped := make(chan struct{})
	m := NewManager()

	err := m.Start(context.Background(), "node-1", func(ctx context.Context) error {
		started <- struct{}{}
		<-ctx.Done()
		close(stopped)
		return ctx.Err()
	}, nil)
	if err != nil {
		t.Fatalf("start: %v", err)
	}
	<-started

	cancelled, err := m.Cancel(t.Context(), "node-1")
	if err != nil || !cancelled {
		t.Fatalf("cancel = (%v, %v), want (true, nil)", cancelled, err)
	}

	select {
	case <-stopped:
	default:
		t.Fatal("Cancel returned while the task was still running")
	}
}

// TestManager_CancelBoundaries covers the two edges of cancelling: nothing to
// cancel, and a task that will not stop before the caller's context does.
func TestManager_CancelBoundaries(t *testing.T) {
	t.Run("nothing is running", func(t *testing.T) {
		m := NewManager()

		cancelled, err := m.Cancel(t.Context(), "node-1")
		if cancelled || err != nil {
			t.Fatalf("cancel = (%v, %v), want (false, nil)", cancelled, err)
		}
	})

	t.Run("the caller's context expires first", func(t *testing.T) {
		release := make(chan struct{})
		t.Cleanup(func() { close(release) })
		m := NewManager()

		// A task that ignores its own context is the one Cancel cannot wait out.
		if err := m.Start(context.Background(), "node-1",
			func(context.Context) error { <-release; return nil }, nil); err != nil {
			t.Fatalf("start: %v", err)
		}

		ctx, cancel := context.WithCancel(context.Background())
		cancel()

		cancelled, err := m.Cancel(ctx, "node-1")
		if !errors.Is(err, context.Canceled) || cancelled {
			t.Fatalf("cancel = (%v, %v), want (false, context.Canceled)", cancelled, err)
		}
		if got := held(m); got != 0 {
			t.Fatalf("a task Cancel gave up on is still held: %d", got)
		}
	})
}
