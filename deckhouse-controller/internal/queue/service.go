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
	"strings"
	"sync"

	"github.com/deckhouse/deckhouse/pkg/log"
)

// Service manages a set of named task queues.
// It provides methods to enqueue tasks, stop individual queues, or stop all queues.
type Service struct {
	ctx context.Context // Parent context for all queues

	mtx    sync.Mutex        // Protects queues map
	queues map[string]*queue // Named queues

	logger *log.Logger
}

// NewService creates a new Service with the given context.
// The context is used for all queues created by the Service.
func NewService(logger *log.Logger) *Service {
	return &Service{
		ctx:    context.Background(),
		queues: make(map[string]*queue),
		logger: logger.Named("queue-service"),
	}
}

// Enqueue adds a task to the specified queue, creating and starting the queue if it doesn't exist.
// It ensures thread-safety using a mutex and propagates the Service's context to the queue.
//
// Context Hierarchy:
// - ctx: Task-specific context for cascade cancellation (e.g., parent task context)
// - w.ctx: Service-level context for lifecycle management (queue shutdown)
// The queue uses w.ctx for its processing loop, but individual tasks use their own ctx.
func (s *Service) Enqueue(ctx context.Context, queueName string, task Task, opts ...EnqueueOption) {
	s.mtx.Lock()
	defer s.mtx.Unlock()

	queueName = strings.TrimSpace(queueName)

	if len(queueName) == 0 || task == nil {
		s.logger.Warn("task or queue is empty")
		return
	}

	if s.queues[queueName] == nil {
		s.logger.Debug("spawn queue", slog.String("name", queueName))
		s.queues[queueName] = newQueue(queueName, s.logger).Start(s.ctx)
	}

	s.queues[queueName].Enqueue(ctx, task, opts...)
}

// Remove stops and removes the named queue.
// If the queue doesn’t exist, it’s a no-op.
func (s *Service) Remove(name string) {
	s.mtx.Lock()
	defer s.mtx.Unlock()

	s.logger.Debug("remove queue", slog.String("name", name))

	if q := s.queues[name]; q != nil {
		q.Stop()
		delete(s.queues, name)
	}
}

// Stop stops and removes all queues.
// It ensures all queues are gracefully shut down.
func (s *Service) Stop() {
	s.mtx.Lock()
	defer s.mtx.Unlock()

	s.logger.Debug("stop queues")

	for name, q := range s.queues {
		q.Stop()
		delete(s.queues, name)
	}
}
