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

	"k8s.io/klog/v2"
	ctrl "sigs.k8s.io/controller-runtime"
)

func init() {
	ctrl.SetLogger(klog.Background())
}

func TestSpawn_SingleExecution(t *testing.T) {
	mgr := &Manager{
		tasks: make(map[string]*taskEntry),
	}

	ctx := ctrl.LoggerInto(context.Background(), ctrl.Log.WithName("test"))

	var counter int32

	task := func(ctx context.Context, data any) error {
		atomic.AddInt32(&counter, 1)
		time.Sleep(50 * time.Millisecond)
		return nil
	}

	// trigger multiple times concurrently
	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			mgr.Spawn(ctx, "same", "test", nil, task)
		}()
	}
	wg.Wait()

	time.Sleep(100 * time.Millisecond)

	if got := atomic.LoadInt32(&counter); got != 1 {
		t.Fatalf("expected task to run once, got %d", got)
	}
}

func TestSpawn_NonBlocking(t *testing.T) {
	mgr := &Manager{
		tasks: make(map[string]*taskEntry),
	}

	ctx := ctrl.LoggerInto(context.Background(), ctrl.Log.WithName("test"))

	task := func(ctx context.Context, data any) error {
		time.Sleep(100 * time.Millisecond)
		return nil
	}

	start := time.Now()
	_, finished := mgr.Spawn(ctx, "id", "test", nil, task)
	elapsed := time.Since(start)

	if finished {
		t.Fatalf("expected not finished immediately")
	}

	if elapsed > 10*time.Millisecond {
		t.Fatalf("spawn is blocking, took %v", elapsed)
	}
}

func TestSpawn_EventualCompletion(t *testing.T) {
	mgr := &Manager{
		tasks: make(map[string]*taskEntry),
	}

	ctx := ctrl.LoggerInto(context.Background(), ctrl.Log.WithName("test"))

	task := func(ctx context.Context, data any) error {
		time.Sleep(50 * time.Millisecond)
		return errors.New("test error")
	}

	// first call starts task
	done, finished := mgr.Spawn(ctx, "id", "test", nil, task)
	if finished {
		t.Fatalf("should not be finished on first call")
	}

	// poll until finished
	var res error
	for i := 0; i < 10; i++ {
		time.Sleep(20 * time.Millisecond)
		done, finished = mgr.Spawn(ctx, "id", "test", nil, task)
		if finished {
			res = done
			break
		}
	}

	if !finished {
		t.Fatalf("task did not finish in time")
	}

	if res.Error() != "test error" {
		t.Fatalf("unexpected result %v", res)
	}
}

func TestSpawn_ConcurrentWaiters(t *testing.T) {
	mgr := &Manager{
		tasks: make(map[string]*taskEntry),
	}

	ctx := ctrl.LoggerInto(context.Background(), ctrl.Log.WithName("test"))

	task := func(ctx context.Context, data any) error {
		time.Sleep(50 * time.Millisecond)
		return errors.New("test error")
	}

	var wg sync.WaitGroup
	results := make([]error, 10)

	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()

			for {
				done, finished := mgr.Spawn(ctx, "id", "test", nil, task)
				if finished {
					results[i] = done
					return
				}
				time.Sleep(10 * time.Millisecond)
			}
		}(i)
	}

	wg.Wait()

	for i, r := range results {
		if r.Error() != "test error" {
			t.Fatalf("goroutine %d got unexpected result %v", i, r)
		}
	}
}

func TestSpawn_TaskRemovedAfterCompletion(t *testing.T) {
	mgr := &Manager{
		tasks: make(map[string]*taskEntry),
	}

	ctx := ctrl.LoggerInto(context.Background(), ctrl.Log.WithName("test"))

	task := func(ctx context.Context, data any) error {
		return nil
	}

	// run once
	for {
		_, finished := mgr.Spawn(ctx, "id", "test", nil, task)
		if finished {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}

	// allow cleanup
	time.Sleep(20 * time.Millisecond)

	mgr.mu.Lock()
	_, exists := mgr.tasks[taskKey("id", "test")]
	mgr.mu.Unlock()

	if exists {
		t.Fatalf("task should be removed after completion")
	}
}

// countingTask returns a task that counts its runs and signals every one of them.
//
// The signal is what the test synchronises on. A fixed sleep would both race with the task
// goroutine and go flaky on a loaded machine, and polling Spawn instead is worse than either:
// once a slot is free, Spawn starts the task itself, so a key collision would be papered over
// by the very call meant to observe it.
func countingTask(counter *int32, ran chan<- struct{}) Task {
	return func(_ context.Context, _ any) error {
		atomic.AddInt32(counter, 1)
		ran <- struct{}{}

		return nil
	}
}

// awaitTaskRun waits for a task to report a run, failing if it never does.
func awaitTaskRun(t *testing.T, ran <-chan struct{}, name string) {
	t.Helper()

	select {
	case <-ran:
	case <-time.After(5 * time.Second):
		t.Fatalf("task %q never ran: its slot is held by another task", name)
	}
}

func TestSpawn_DifferentTaskTypesDoNotConflict(t *testing.T) {
	mgr := &Manager{
		tasks: make(map[string]*taskEntry),
	}

	ctx := ctrl.LoggerInto(context.Background(), ctrl.Log.WithName("test"))

	var test1Count int32
	var test2Count int32

	test1Ran := make(chan struct{}, 1)
	test2Ran := make(chan struct{}, 1)

	// NOTE: this test will FAIL if you key only by taskID.
	// Both are spawned before either is awaited: keyed by id alone, the second Spawn would
	// find the slot occupied and test2 would never run.
	mgr.Spawn(ctx, "same", "test1", nil, countingTask(&test1Count, test1Ran))
	mgr.Spawn(ctx, "same", "test2", nil, countingTask(&test2Count, test2Ran))

	awaitTaskRun(t, test1Ran, "test1")
	awaitTaskRun(t, test2Ran, "test2")

	if got1, got2 := atomic.LoadInt32(&test1Count), atomic.LoadInt32(&test2Count); got1 != 1 || got2 != 1 {
		t.Fatalf("task types are conflicting: test1Count=%d test2Count=%d", got1, got2)
	}
}

// Without a separator in the key, ("x", "yz") and ("xy", "z") land in the same map entry and
// the second task never runs while the first one owns the slot.
func TestSpawn_IDAndTaskTypeBoundaryDoesNotCollide(t *testing.T) {
	mgr := &Manager{
		tasks: make(map[string]*taskEntry),
	}

	ctx := ctrl.LoggerInto(context.Background(), ctrl.Log.WithName("test"))

	var firstCount int32
	var secondCount int32

	firstRan := make(chan struct{}, 1)
	secondRan := make(chan struct{}, 1)

	mgr.Spawn(ctx, "machine", "cleanup", nil, countingTask(&firstCount, firstRan))
	mgr.Spawn(ctx, "machinecl", "eanup", nil, countingTask(&secondCount, secondRan))

	awaitTaskRun(t, firstRan, "machine/cleanup")
	awaitTaskRun(t, secondRan, "machinecl/eanup")

	if got1, got2 := atomic.LoadInt32(&firstCount), atomic.LoadInt32(&secondCount); got1 != 1 || got2 != 1 {
		t.Fatalf("keys are colliding: firstCount=%d secondCount=%d", got1, got2)
	}
}
