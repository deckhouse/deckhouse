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
	"log/slog"
	"sync"
	"time"

	"github.com/cenkalti/backoff/v4"
	"github.com/gammazero/deque"
	"github.com/google/uuid"

	"github.com/deckhouse/deckhouse/pkg/log"
)

// queue manages a FIFO task queue with exponential backoff retries and serial execution.
// It processes tasks one at a time, retrying failed tasks based on their backoff policy.
// Tasks can be enqueued with optional wait groups for completion tracking.
// Uses event-driven processing for immediate task execution (no polling delay).
type queue struct {
	wg   *sync.WaitGroup // Used for graceful shutdown
	name string          // Unique name of the queue

	ctx    context.Context    // Context for cancellation
	cancel context.CancelFunc // Cancel function for stopping the queue

	once sync.Once // Ensures Start is called only once

	mu     sync.Mutex                // Protects deque access
	deque  deque.Deque[*taskWrapper] // FIFO queue of tasks
	signal chan struct{}             // Signals when tasks are available (event-driven)

	logger *log.Logger
}

// Task defines the interface for executable tasks.
type Task interface {
	Name() string
	Execute(ctx context.Context) error // Executes the task, returning an error if it fails
}

// taskWrapper encapsulates a task with parent context, id and retry.
type taskWrapper struct {
	ctx context.Context // Task-specific context
	wg  *sync.WaitGroup

	id   string // Unique task identifier
	task Task   // The task to execute

	err error // last task error

	backoff   backoff.BackOff // Exponential backoff policy for retries
	nextRetry time.Time       // Time when the task is eligible for retry
}

// newQueue creates a new queue with the specified name.
// Signal channel enables event-driven task processing without polling.
func newQueue(name string, logger *log.Logger) *queue {
	return &queue{
		wg:   new(sync.WaitGroup),
		name: name,

		deque:  deque.Deque[*taskWrapper]{},
		signal: make(chan struct{}, 1), // Buffered to prevent blocking

		logger: logger.Named("queue." + name),
	}
}

// EnqueueOptions configures task enqueuing behavior.
type EnqueueOptions struct {
	wg     *sync.WaitGroup // Optional WaitGroup to track task completion
	unique bool
}

// EnqueueOption is a functional option for configuring Enqueue.
type EnqueueOption func(*EnqueueOptions)

// WithWait specifies a WaitGroup to track task completion.
func WithWait(wg *sync.WaitGroup) EnqueueOption {
	return func(o *EnqueueOptions) {
		o.wg = wg
	}
}

// WithUnique ensures that the task will not be enqueued if there are more than 2 same tasks(by name) in the queue.
func WithUnique() EnqueueOption {
	return func(o *EnqueueOptions) {
		o.unique = true
	}
}

// Enqueue adds a task to the queue's tail.
// If a WaitGroup is provided via WithWait, WaitGroup sticks with task, add Done will be called after task success
func (q *queue) Enqueue(ctx context.Context, task Task, opts ...EnqueueOption) {
	opt := new(EnqueueOptions)
	for _, o := range opts {
		o(opt)
	}

	if opt.wg != nil {
		opt.wg.Add(1)
	}

	wrapper := &taskWrapper{
		ctx:       ctx,
		wg:        opt.wg,
		id:        uuid.New().String(),
		task:      task,
		backoff:   backoff.NewExponentialBackOff(),
		nextRetry: time.Now(),
	}

	q.logger.Debug("enqueue task", slog.String("id", wrapper.id), slog.String("name", wrapper.task.Name()))

	if opt.unique && q.hasSeveral(task.Name()) {
		if opt.wg != nil {
			opt.wg.Done()
		}

		return
	}

	// Enqueue task under deque lock
	q.mu.Lock()
	q.deque.PushBack(wrapper)
	q.mu.Unlock()

	// Signal processor that task is available (non-blocking)
	select {
	case q.signal <- struct{}{}:
	default: // Channel already has signal pending, no need to add another
	}
}

// Start begins the queue's processing loop in a separate goroutine.
// It processes tasks sequentially, respecting their retry schedules.
// The loop runs until the queue's context is canceled.
// It ensures Start is idempotent using sync.Once.
// Event-driven: processes tasks immediately on enqueue, no polling delay.
func (q *queue) Start(ctx context.Context) *queue {
	q.once.Do(func() {
		q.logger.Info("start queue")

		q.ctx, q.cancel = context.WithCancel(ctx)

		q.wg.Add(1)
		go func() {
			defer q.wg.Done()

			for {
				select {
				case <-q.ctx.Done():
					return
				case <-q.signal:
					// Process all ready tasks
					q.processAvailable()
				}
			}
		}()
	})

	return q
}

// processAvailable processes all tasks that are ready to execute.
// Continues processing until no more ready tasks are available.
// This enables batch processing of multiple ready tasks without delay.
func (q *queue) processAvailable() {
	for {
		// Try to process one task
		if !q.processOne() {
			// No more ready tasks
			return
		}
	}
}

// processOne executes the next ready task from the queue.
// Returns true if a task was processed, false if no ready tasks available.
// It skips tasks not yet ready for retry and handles context cancellation.
// Tasks that fail are re-queued with exponential backoff unless retries are exhausted.
// Context cancellation enables cascade cancellation of parent-child task hierarchies.
func (q *queue) processOne() bool {
	q.mu.Lock()
	if q.deque.Len() == 0 {
		q.mu.Unlock()
		return false
	}

	t := q.deque.Front()
	if t == nil {
		q.mu.Unlock()
		return false
	}

	if time.Now().Before(t.nextRetry) {
		q.mu.Unlock()
		return false // Task not ready for retry
	}

	q.mu.Unlock()

	// Check for parent context cancellation
	select {
	case <-t.ctx.Done():
		q.logger.Debug("context canceled", slog.String("id", t.id), slog.String("name", t.task.Name()))
		if t.wg != nil {
			t.wg.Done()
		}

		// Remove completed task from queue
		q.mu.Lock()
		q.deque.PopFront()
		q.mu.Unlock()

		return true // Task was processed (canceled)
	default:
	}

	q.logger.Debug("process task", slog.String("id", t.id), slog.String("name", t.task.Name()))

	// Execute the task
	if err := t.task.Execute(t.ctx); err != nil {
		q.logger.Warn("task failed", slog.String("id", t.id), slog.String("name", t.task.Name()))

		// Check context again before retrying
		select {
		case <-t.ctx.Done():
			q.logger.Debug("context canceled", slog.String("id", t.id), slog.String("name", t.task.Name()))
			if t.wg != nil {
				t.wg.Done()
			}

			// Remove completed task from queue
			q.mu.Lock()
			q.deque.PopFront()
			q.mu.Unlock()

			return true // Task was processed (canceled)
		default:
		}

		// Retry if backoff allows
		if delay := t.backoff.NextBackOff(); delay != backoff.Stop {
			// record last error and reset running state
			t.err = err
			t.nextRetry = time.Now().Add(delay)

			// Schedule retry signal after delay
			time.AfterFunc(delay, func() {
				select {
				case q.signal <- struct{}{}:
				default:
				}
			})

			return true // Task was processed (will retry later)
		}
	}

	// Task succeeded, remove from queue
	q.mu.Lock()
	q.deque.PopFront()
	q.mu.Unlock()

	if t.wg != nil {
		t.wg.Done()
	}

	return true // Task was processed successfully
}

// Stop cancels the queue's context and waits for the processing loop to finish.
// It ensures all tasks stop gracefully.
func (q *queue) Stop() {
	q.cancel()
	q.wg.Wait()
}

// hasSeveral checks if there are more than 1 task in the queue
func (q *queue) hasSeveral(name string) bool {
	q.mu.Lock()
	defer q.mu.Unlock()

	if q.deque.Len() == 0 {
		return false
	}

	firstFound := false
	for wrapper := range q.deque.Iter() {
		if wrapper.task.Name() == name {
			if firstFound {
				return true
			}

			firstFound = true
		}
	}

	return false
}
