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
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestService_BasicEnqueue tests basic service enqueue functionality
func TestService_BasicEnqueue(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	svc := NewService(ctx, getTestLogger())
	defer svc.Stop()

	task := newMockTask("task1")
	var wg sync.WaitGroup

	svc.Enqueue(ctx, "test-queue", task, WithWait(&wg))

	wg.Wait()

	assert.Equal(t, 1, task.ExecutionCount(), "task should be executed")
}

// TestService_MultipleQueues tests that service can manage multiple queues
func TestService_MultipleQueues(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	svc := NewService(ctx, getTestLogger())
	defer svc.Stop()

	task1 := newMockTask("task1")
	task2 := newMockTask("task2")
	task3 := newMockTask("task3")

	var wg sync.WaitGroup

	// Enqueue tasks to different queues
	svc.Enqueue(ctx, "queue1", task1, WithWait(&wg))
	svc.Enqueue(ctx, "queue2", task2, WithWait(&wg))
	svc.Enqueue(ctx, "queue1", task3, WithWait(&wg))

	wg.Wait()

	assert.Equal(t, 1, task1.ExecutionCount())
	assert.Equal(t, 1, task2.ExecutionCount())
	assert.Equal(t, 1, task3.ExecutionCount())
}

// TestService_QueueCreatedOnDemand tests that queues are created automatically
func TestService_QueueCreatedOnDemand(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	svc := NewService(ctx, getTestLogger())
	defer svc.Stop()

	// Initially no queues
	assert.Len(t, svc.queues, 0, "should start with no queues")

	task := newMockTask("task")
	var wg sync.WaitGroup
	svc.Enqueue(ctx, "new-queue", task, WithWait(&wg))

	// Queue should be created
	svc.mtx.Lock()
	queueCount := len(svc.queues)
	svc.mtx.Unlock()

	assert.Equal(t, 1, queueCount, "queue should be created on first enqueue")

	wg.Wait()
}

// TestService_RemoveQueue tests queue removal
func TestService_RemoveQueue(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	svc := NewService(ctx, getTestLogger())
	defer svc.Stop()

	task := newMockTask("task")
	var wg sync.WaitGroup
	svc.Enqueue(ctx, "removable-queue", task, WithWait(&wg))

	wg.Wait()

	// Remove the queue
	svc.Remove("removable-queue")

	svc.mtx.Lock()
	_, exists := svc.queues["removable-queue"]
	svc.mtx.Unlock()

	assert.False(t, exists, "queue should be removed")
}

// TestService_RemoveNonExistentQueue tests removing a queue that doesn't exist
func TestService_RemoveNonExistentQueue(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	svc := NewService(ctx, getTestLogger())
	defer svc.Stop()

	// Should not panic
	svc.Remove("non-existent-queue")
}

// TestService_StopAllQueues tests stopping all queues
func TestService_StopAllQueues(t *testing.T) {
	ctx := context.Background()

	svc := NewService(ctx, getTestLogger())

	// Create multiple queues
	task1 := newMockTask("task1")
	task2 := newMockTask("task2")

	var wg sync.WaitGroup
	svc.Enqueue(ctx, "queue1", task1, WithWait(&wg))
	svc.Enqueue(ctx, "queue2", task2, WithWait(&wg))

	wg.Wait()

	// Stop all
	svc.Stop()

	svc.mtx.Lock()
	queueCount := len(svc.queues)
	svc.mtx.Unlock()

	assert.Equal(t, 0, queueCount, "all queues should be removed")
}

// TestService_EnqueueWithEmptyName tests enqueueing with empty queue name
func TestService_EnqueueWithEmptyName(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	svc := NewService(ctx, getTestLogger())
	defer svc.Stop()

	task := newMockTask("task")
	var wg sync.WaitGroup
	wg.Add(1) // Add manually since task won't be enqueued

	svc.Enqueue(ctx, "", task, WithWait(&wg))
	wg.Done() // Done manually

	// Task should not be enqueued
	assert.Equal(t, 0, task.ExecutionCount(), "task should not be executed with empty queue name")

	svc.mtx.Lock()
	queueCount := len(svc.queues)
	svc.mtx.Unlock()

	assert.Equal(t, 0, queueCount, "no queue should be created")
}

// TestService_EnqueueWithWhitespaceName tests enqueueing with whitespace queue name
func TestService_EnqueueWithWhitespaceName(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	svc := NewService(ctx, getTestLogger())
	defer svc.Stop()

	task := newMockTask("task")
	var wg sync.WaitGroup
	wg.Add(1) // Add manually

	svc.Enqueue(ctx, "   ", task, WithWait(&wg))
	wg.Done() // Done manually

	// Task should not be enqueued
	assert.Equal(t, 0, task.ExecutionCount(), "task should not be executed with whitespace queue name")
}

// TestService_EnqueueNilTask tests enqueueing nil task
func TestService_EnqueueNilTask(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	svc := NewService(ctx, getTestLogger())
	defer svc.Stop()

	// Should not panic
	svc.Enqueue(ctx, "test-queue", nil)

	svc.mtx.Lock()
	queueCount := len(svc.queues)
	svc.mtx.Unlock()

	assert.Equal(t, 0, queueCount, "no queue should be created for nil task")
}

// TestService_Dump tests service dump functionality
func TestService_Dump(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	svc := NewService(ctx, getTestLogger())
	defer svc.Stop()

	// Enqueue a slow task
	task := newSlowTask("test-task", 200*time.Millisecond)
	var wg sync.WaitGroup
	svc.Enqueue(ctx, "test-queue", task, WithWait(&wg))

	// Give task time to be enqueued
	time.Sleep(50 * time.Millisecond)

	// Get dump
	dumpBytes := svc.Dump("test-queue")
	require.NotEmpty(t, dumpBytes, "dump should not be empty")

	dumpStr := string(dumpBytes)
	assert.Contains(t, dumpStr, "test-task", "dump should contain task name")

	wg.Wait()
}

// TestService_DumpEmptyQueueName tests dump with empty queue name
func TestService_DumpEmptyQueueName(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	svc := NewService(ctx, getTestLogger())
	defer svc.Stop()

	dumpBytes := svc.Dump("")
	assert.Empty(t, dumpBytes, "dump should be empty for empty queue name")
}

// TestService_DumpNonExistentQueue tests dump of non-existent queue
func TestService_DumpNonExistentQueue(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	svc := NewService(ctx, getTestLogger())
	defer svc.Stop()

	dumpBytes := svc.Dump("non-existent")
	assert.Empty(t, dumpBytes, "dump should be empty for non-existent queue")
}

// TestService_ConcurrentEnqueueToSameQueue tests concurrent enqueues to same queue
func TestService_ConcurrentEnqueueToSameQueue(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	svc := NewService(ctx, getTestLogger())
	defer svc.Stop()

	const numTasks = 50
	var wg sync.WaitGroup
	var startWg sync.WaitGroup

	startWg.Add(numTasks)

	// Enqueue many tasks concurrently to the same queue
	for i := 0; i < numTasks; i++ {
		go func() {
			defer startWg.Done()
			task := newMockTask("task")
			svc.Enqueue(ctx, "shared-queue", task, WithWait(&wg))
		}()
	}

	// Wait for all goroutines to enqueue
	startWg.Wait()

	// Now wait for all tasks to complete
	wg.Wait()

	// All tasks should complete without deadlock or panic
	svc.mtx.Lock()
	queueCount := len(svc.queues)
	svc.mtx.Unlock()

	assert.Equal(t, 1, queueCount, "should have exactly one queue")
}

// TestService_ConcurrentEnqueueToDifferentQueues tests concurrent enqueues to different queues
func TestService_ConcurrentEnqueueToDifferentQueues(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	svc := NewService(ctx, getTestLogger())
	defer svc.Stop()

	const numQueues = 10
	var wg sync.WaitGroup
	var startWg sync.WaitGroup

	startWg.Add(numQueues)

	// Enqueue tasks to different queues concurrently
	for i := 0; i < numQueues; i++ {
		go func(n int) {
			defer startWg.Done()
			task := newMockTask("task")
			queueName := string(rune('a' + n))
			svc.Enqueue(ctx, queueName, task, WithWait(&wg))
		}(i)
	}

	// Wait for all goroutines to enqueue
	startWg.Wait()

	// Now wait for all tasks to complete
	wg.Wait()

	svc.mtx.Lock()
	queueCount := len(svc.queues)
	svc.mtx.Unlock()

	assert.Equal(t, numQueues, queueCount, "should have created all queues")
}

// TestService_ContextCancellationStopsQueues tests that canceling service context stops queues
func TestService_ContextCancellationStopsQueues(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())

	svc := NewService(ctx, getTestLogger())

	// Enqueue a long-running task
	task := newSlowTask("long-task", 10*time.Second)
	var wg sync.WaitGroup
	svc.Enqueue(ctx, "test-queue", task, WithWait(&wg))

	// Give task time to start
	time.Sleep(50 * time.Millisecond)

	// Cancel service context
	cancel()

	// Wait for task completion (should be quick due to cancellation)
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		// Success
	case <-time.After(2 * time.Second):
		t.Fatal("task should complete quickly after context cancellation")
	}

	// Clean up
	svc.Stop()
}

// TestService_QueueReuse tests that enqueueing to the same queue name reuses the queue
func TestService_QueueReuse(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	svc := NewService(ctx, getTestLogger())
	defer svc.Stop()

	task1 := newMockTask("task1")
	task2 := newMockTask("task2")

	var wg sync.WaitGroup

	// Enqueue first task
	svc.Enqueue(ctx, "reusable-queue", task1, WithWait(&wg))

	wg.Wait()

	// Enqueue second task to same queue
	svc.Enqueue(ctx, "reusable-queue", task2, WithWait(&wg))

	wg.Wait()

	svc.mtx.Lock()
	queueCount := len(svc.queues)
	svc.mtx.Unlock()

	assert.Equal(t, 1, queueCount, "should reuse existing queue")
	assert.Equal(t, 1, task1.ExecutionCount())
	assert.Equal(t, 1, task2.ExecutionCount())
}
