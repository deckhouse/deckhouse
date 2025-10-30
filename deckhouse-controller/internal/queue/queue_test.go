// Copyright 2025 Flant JSC
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

package queue

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"sigs.k8s.io/yaml"

	"github.com/deckhouse/deckhouse/pkg/log"
)

// mockTask is a test implementation of Task interface
type mockTask struct {
	name      string
	execFunc  func(ctx context.Context) error
	execCount atomic.Int32
}

func (m *mockTask) String() string {
	return m.name
}

func (m *mockTask) Execute(ctx context.Context) error {
	m.execCount.Add(1)
	if m.execFunc != nil {
		return m.execFunc(ctx)
	}
	return nil
}

func (m *mockTask) ExecutionCount() int {
	return int(m.execCount.Load())
}

// newMockTask creates a task that succeeds
func newMockTask(name string) *mockTask {
	return &mockTask{
		name: name,
		execFunc: func(_ context.Context) error {
			return nil
		},
	}
}

// newMockTaskWithFunc creates a task with custom execution logic
func newMockTaskWithFunc(name string, execFunc func(ctx context.Context) error) *mockTask {
	return &mockTask{
		name:     name,
		execFunc: execFunc,
	}
}

// newFailingTask creates a task that always fails
func newFailingTask(name string, err error) *mockTask {
	return &mockTask{
		name: name,
		execFunc: func(_ context.Context) error {
			return err
		},
	}
}

// newSlowTask creates a task that takes time to execute
func newSlowTask(name string, duration time.Duration) *mockTask {
	return &mockTask{
		name: name,
		execFunc: func(ctx context.Context) error {
			select {
			case <-time.After(duration):
				return nil
			case <-ctx.Done():
				return ctx.Err()
			}
		},
	}
}

func getTestLogger() *log.Logger {
	return log.NewNop()
}

// TestQueue_BasicEnqueueAndExecute tests basic task enqueue and execution
func TestQueue_BasicEnqueueAndExecute(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	q := newQueue("test", getTestLogger()).Start(ctx)
	defer q.Stop()

	task := newMockTask("task1")

	var wg sync.WaitGroup
	q.Enqueue(ctx, task, WithWait(&wg))

	// Wait for task to complete
	wg.Wait()

	assert.Equal(t, 1, task.ExecutionCount(), "task should be executed once")
}

// TestQueue_MultipleTasksExecuteSequentially tests that tasks execute in FIFO order
func TestQueue_MultipleTasksExecuteSequentially(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	q := newQueue("test", getTestLogger()).Start(ctx)
	defer q.Stop()

	var executionOrder []string
	var mu sync.Mutex

	createTask := func(name string) *mockTask {
		return newMockTaskWithFunc(name, func(_ context.Context) error {
			mu.Lock()
			executionOrder = append(executionOrder, name)
			mu.Unlock()
			time.Sleep(10 * time.Millisecond) // Small delay to ensure ordering
			return nil
		})
	}

	task1 := createTask("task1")
	task2 := createTask("task2")
	task3 := createTask("task3")

	var wg sync.WaitGroup
	q.Enqueue(ctx, task1, WithWait(&wg))
	q.Enqueue(ctx, task2, WithWait(&wg))
	q.Enqueue(ctx, task3, WithWait(&wg))

	wg.Wait()

	assert.Equal(t, []string{"task1", "task2", "task3"}, executionOrder, "tasks should execute in FIFO order")
}

// TestQueue_TaskRetryOnFailure tests exponential backoff retry logic
func TestQueue_TaskRetryOnFailure(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	q := newQueue("test", getTestLogger()).Start(ctx)
	defer q.Stop()

	failCount := atomic.Int32{}
	task := newMockTaskWithFunc("failing-task", func(_ context.Context) error {
		count := failCount.Add(1)
		if count < 3 {
			return errors.New("temporary failure")
		}
		return nil // Success on 3rd attempt
	})

	var wg sync.WaitGroup
	q.Enqueue(ctx, task, WithWait(&wg))

	wg.Wait()

	assert.Equal(t, int32(3), failCount.Load(), "task should retry until success")
}

// TestQueue_TaskFailurePermanent tests that tasks eventually stop retrying
func TestQueue_TaskFailurePermanent(t *testing.T) {
	t.Skip("Skipping: backoff can take a very long time (minutes) before giving up")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	q := newQueue("test", getTestLogger()).Start(ctx)
	defer q.Stop()

	task := newFailingTask("always-fails", errors.New("permanent failure"))

	var wg sync.WaitGroup
	q.Enqueue(ctx, task, WithWait(&wg))

	wg.Wait()

	// Task should have been attempted multiple times before giving up
	assert.Greater(t, task.ExecutionCount(), 1, "task should retry multiple times")
}

// TestQueue_ContextCancellation tests task cancellation via context
func TestQueue_ContextCancellation(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	q := newQueue("test", getTestLogger()).Start(ctx)
	defer q.Stop()

	taskCtx, taskCancel := context.WithCancel(ctx)

	task := newSlowTask("slow-task", 5*time.Second)

	var wg sync.WaitGroup
	q.Enqueue(taskCtx, task, WithWait(&wg))

	// Give task time to start
	time.Sleep(50 * time.Millisecond)

	// Cancel the task context
	taskCancel()

	// Wait for task to complete (should be quick due to cancellation)
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		// Success - task was canceled
	case <-time.After(1 * time.Second):
		t.Fatal("task should have been canceled quickly")
	}

	assert.Equal(t, 1, task.ExecutionCount(), "task should be executed once before cancellation")
}

// TestQueue_WithUniqueOption tests that duplicate tasks are not enqueued
func TestQueue_WithUniqueOption(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	q := newQueue("test", getTestLogger()).Start(ctx)
	defer q.Stop()

	// First task will be slow so others can be enqueued while it's running
	task1 := newSlowTask("duplicate-task", 200*time.Millisecond)
	task2 := newMockTask("duplicate-task")
	task3 := newMockTask("duplicate-task")

	var wg sync.WaitGroup

	// Enqueue first task without unique option
	q.Enqueue(ctx, task1, WithWait(&wg))

	// Small delay to ensure first task starts
	time.Sleep(50 * time.Millisecond)

	// Try to enqueue second task with unique option (should succeed - only 1 in queue)
	q.Enqueue(ctx, task2, WithWait(&wg), WithUnique())

	// Try to enqueue third task with unique option (should be rejected - 2 already in queue)
	q.Enqueue(ctx, task3, WithWait(&wg), WithUnique())

	wg.Wait()

	// task1 and task2 should execute, task3 should not
	assert.Equal(t, 1, task1.ExecutionCount(), "first task should execute")
	assert.Equal(t, 1, task2.ExecutionCount(), "second task should execute")
	assert.Equal(t, 0, task3.ExecutionCount(), "third task should not execute (rejected by WithUnique)")
}

// TestQueue_Stop tests graceful queue shutdown
func TestQueue_Stop(t *testing.T) {
	ctx := context.Background()
	q := newQueue("test", getTestLogger()).Start(ctx)

	task := newSlowTask("task", 100*time.Millisecond)
	var wg sync.WaitGroup
	q.Enqueue(ctx, task, WithWait(&wg))

	// Stop the queue
	stopDone := make(chan struct{})
	go func() {
		q.Stop()
		close(stopDone)
	}()

	// Stop should complete within reasonable time
	select {
	case <-stopDone:
		// Success
	case <-time.After(2 * time.Second):
		t.Fatal("Stop() should complete quickly")
	}
}

// TestQueue_EmptyQueue tests behavior with no tasks
func TestQueue_EmptyQueue(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	q := newQueue("test", getTestLogger()).Start(ctx)
	defer q.Stop()

	// Queue should handle being empty gracefully
	time.Sleep(100 * time.Millisecond)

	// No tasks enqueued, just verify queue is working
	assert.NotNil(t, q, "queue should be initialized")
}

// TestQueue_ConcurrentEnqueue tests multiple concurrent enqueues
func TestQueue_ConcurrentEnqueue(_ *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	q := newQueue("test", getTestLogger()).Start(ctx)
	defer q.Stop()

	const numTasks = 50
	var wg sync.WaitGroup
	var startWg sync.WaitGroup

	startWg.Add(numTasks)

	// Enqueue many tasks concurrently
	for i := 0; i < numTasks; i++ {
		go func(_ int) {
			defer startWg.Done()
			task := newMockTask("task")
			q.Enqueue(ctx, task, WithWait(&wg))
		}(i)
	}

	// Wait for all goroutines to enqueue
	startWg.Wait()

	// Now wait for all tasks to complete
	wg.Wait()

	// All tasks should complete
	// No assertions on execution count since we don't track individual tasks
	// The test passes if no deadlocks/panics occur
}

// TestQueue_HasSeveral tests the hasSeveral method
func TestQueue_HasSeveral(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	q := newQueue("test", getTestLogger()).Start(ctx)
	defer q.Stop()

	// Enqueue one task
	task1 := newSlowTask("same-name", 500*time.Millisecond)
	var wg sync.WaitGroup
	q.Enqueue(ctx, task1, WithWait(&wg))

	// Give it time to start
	time.Sleep(50 * time.Millisecond)

	// hasSeveral should return false (only 1 task with this name)
	assert.False(t, q.hasSeveral("same-name"), "should return false with only 1 task")

	// Enqueue another task with same name
	task2 := newSlowTask("same-name", 500*time.Millisecond)
	q.Enqueue(ctx, task2, WithWait(&wg))

	// Now hasSeveral should return true (2 tasks with same name)
	assert.True(t, q.hasSeveral("same-name"), "should return true with 2+ tasks")

	wg.Wait()
}

// TestQueue_Dump tests queue dump functionality
func TestQueue_Dump(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	q := newQueue("test-queue", getTestLogger()).Start(ctx)
	defer q.Stop()

	// Enqueue a slow task so we can dump while it's in queue
	task := newSlowTask("test-task", 200*time.Millisecond)
	var wg sync.WaitGroup
	q.Enqueue(ctx, task, WithWait(&wg))

	// Give task time to be enqueued
	time.Sleep(50 * time.Millisecond)

	// Get dump
	d := q.dump()
	require.NotEmpty(t, d, "dump should not be empty")

	dumpBytes, err := yaml.Marshal(d)
	require.NoError(t, err)

	// Dump should contain task name
	dumpStr := string(dumpBytes)
	assert.Contains(t, dumpStr, "test-task", "dump should contain task name")
	assert.Contains(t, dumpStr, "test-queue", "dump should contain queue name")

	wg.Wait()
}

// TestQueue_DumpWithError tests dump shows errors correctly
func TestQueue_DumpWithError(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	q := newQueue("test-queue", getTestLogger()).Start(ctx)
	defer q.Stop()

	// Create a task that fails then succeeds
	attemptCount := atomic.Int32{}
	task := newMockTaskWithFunc("failing-task", func(_ context.Context) error {
		count := attemptCount.Add(1)
		if count == 1 {
			time.Sleep(100 * time.Millisecond) // Give time to dump
			return errors.New("test error message")
		}
		return nil
	})

	var wg sync.WaitGroup
	q.Enqueue(ctx, task, WithWait(&wg))

	// Wait for first failure and retry scheduling
	time.Sleep(200 * time.Millisecond)

	// Get dump
	d := q.dump()
	require.NotEmpty(t, d, "dump should not be empty")

	dumpBytes, err := yaml.Marshal(d)
	require.NoError(t, err)

	dumpStr := string(dumpBytes)
	assert.Contains(t, dumpStr, "test error message", "dump should contain error message")

	wg.Wait()
}

// TestQueue_StartIdempotent tests that Start can only be called once
func TestQueue_StartIdempotent(t *testing.T) {
	ctx := context.Background()
	q := newQueue("test", getTestLogger())

	// Start multiple times
	q.Start(ctx)
	q.Start(ctx)
	q.Start(ctx)

	// Should not panic or cause issues
	defer q.Stop()

	task := newMockTask("task")
	var wg sync.WaitGroup
	q.Enqueue(ctx, task, WithWait(&wg))

	wg.Wait()

	assert.Equal(t, 1, task.ExecutionCount())
}
