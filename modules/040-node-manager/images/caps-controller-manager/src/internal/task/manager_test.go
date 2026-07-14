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

	if atomic.LoadInt32(&counter) != 1 {
		t.Fatalf("expected task to run once, got %d", counter)
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
	_, exists := mgr.tasks["id"]
	mgr.mu.Unlock()

	if exists {
		t.Fatalf("task should be removed after completion")
	}
}

func TestSpawn_DifferentTaskTypesDoNotConflict(t *testing.T) {
	mgr := &Manager{
		tasks: make(map[string]*taskEntry),
	}

	ctx := ctrl.LoggerInto(context.Background(), ctrl.Log.WithName("test"))

	var test1Count int32
	var test2Count int32

	test1 := func(ctx context.Context, data any) error {
		atomic.AddInt32(&test1Count, 1)
		return nil
	}

	test2 := func(ctx context.Context, data any) error {
		atomic.AddInt32(&test2Count, 1)
		return nil
	}

	// NOTE: this test will FAIL if you key only by taskID
	mgr.Spawn(ctx, "same", "test1", nil, test1)
	mgr.Spawn(ctx, "same", "test2", nil, test2)

	time.Sleep(50 * time.Millisecond)

	if test1Count != 1 || test2Count != 1 {
		t.Fatalf("task types are conflicting: test1Count=%d test2Count=%d", test1Count, test2Count)
	}
}
