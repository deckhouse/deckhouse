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

// Package taskevent provides event handling for Kubernetes and schedule events.
// It converts events from multiple sources into queue tasks for processing.
//
// The Handler orchestrates events from:
//   - Kubernetes resources (via kubeEventsManager)
//   - Scheduled cron jobs (via scheduleManager)
//
// Thread Safety: The Handler uses sync.Once to ensure Start() can only execute once,
// preventing goroutine leaks and race conditions from multiple Start() calls.
package eventhandler

import (
	"context"
	"log/slog"
	"sync"

	hookcontroller "github.com/flant/shell-operator/pkg/hook/controller"
	kubeeventsmanager "github.com/flant/shell-operator/pkg/kube_events_manager"
	kemtypes "github.com/flant/shell-operator/pkg/kube_events_manager/types"
	schedulemanager "github.com/flant/shell-operator/pkg/schedule_manager"

	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/operator/tasks/hookrun"
	packagemanager "github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/manager"
	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/queue"
	"github.com/deckhouse/deckhouse/pkg/log"
)

type Config struct {
	KubeEventsManager kubeeventsmanager.KubeEventsManager
	ScheduleManager   schedulemanager.ScheduleManager
	PackageManager    *packagemanager.Manager
	QueueService      *queue.Service
}

// Handler manages the event processing loop for Kubernetes and schedule events.
// It listens to events from multiple sources and enqueues tasks for processing.
//
// Lifecycle:
//   - Create with New()
//   - Start event processing with Start() (can only be called once due to sync.Once)
//   - Stop event processing with Stop() (waits for goroutine to finish)
//
// Thread Safety:
//   - Start() is protected by sync.Once to prevent multiple goroutines
//   - Stop() uses WaitGroup to ensure graceful shutdown
//   - Context fields are written in once.Do() before goroutine reads them
type Handler struct {
	// ctx is the handler's cancellable context, set once in Start()
	ctx    context.Context
	cancel context.CancelFunc

	// once ensures Start() logic executes only once
	once sync.Once
	// wg tracks the event processing goroutine for graceful shutdown
	wg *sync.WaitGroup

	kubeEventsManager kubeeventsmanager.KubeEventsManager
	scheduleManager   schedulemanager.ScheduleManager
	packageManager    *packagemanager.Manager
	queueService      *queue.Service

	logger *log.Logger
}

// New creates a new Handler with the given configuration.
// The returned Handler is ready to be started with Start().
//
// Initializes:
//   - WaitGroup for tracking the event processing goroutine
//   - sync.Once to ensure Start() executes only once
//
// Note: The handler's context and cancel function are set when Start() is called first time.
func New(conf Config, logger *log.Logger) *Handler {
	return &Handler{
		wg:   new(sync.WaitGroup),
		once: sync.Once{},

		queueService:      conf.QueueService,
		scheduleManager:   conf.ScheduleManager,
		kubeEventsManager: conf.KubeEventsManager,
		packageManager:    conf.PackageManager,

		logger: logger.Named("kube-event-handler"),
	}
}

// Start begins the event processing loop in a new goroutine.
// It creates a cancellable context from the provided parent context and starts
// listening to events from both the Kubernetes event manager and schedule manager.
//
// Events are converted to tasks via kubeTasks and scheduleTasks callbacks,
// then enqueued to the queue service for processing.
//
// Thread Safety:
//   - Protected by sync.Once - subsequent calls to Start() are no-ops
//   - WaitGroup is incremented before spawning goroutine (Add called before Do)
//   - Context is set inside once.Do() before goroutine uses it
//
// The goroutine runs until:
//   - h.ctx is cancelled (via Stop())
//   - One of the event channels closes (abnormal termination)
//
// To stop the handler, call Stop() which waits for goroutine completion.
func (h *Handler) Start(ctx context.Context) {
	h.once.Do(func() {
		h.logger.Info("start loop")

		// Create cancellable context before starting goroutine
		h.ctx, h.cancel = context.WithCancel(ctx)

		// Increment WaitGroup before goroutine starts
		h.wg.Add(1)

		go func() {
			// Ensure WaitGroup is decremented when goroutine exits
			defer h.wg.Done()

			for {
				// res holds tasks to enqueue, populated by event handlers
				var res map[string][]queue.Task

				select {
				case <-h.ctx.Done():
					h.logger.Info("stop event loop")
					return

				case crontab := <-h.scheduleManager.Ch():
					// Convert schedule event to tasks using handler's context
					h.logger.Info("creates schedule tasks", slog.String("crontab", crontab))
					res = h.scheduleTaskBuilder(h.ctx, crontab)

				case kubeEvent := <-h.kubeEventsManager.Ch():
					// Convert Kubernetes event to tasks using handler's context
					h.logger.Info("creates kube events", slog.String("kube_event", kubeEvent.String()))
					res = h.kubeTaskBuilder(h.ctx, kubeEvent)
				}

				// Enqueue all tasks from the event handlers
				// Uses handler's context for task lifecycle management
				for queueName, tasks := range res {
					for _, task := range tasks {
						h.logger.Info("enqueue task", slog.String("task", task.String()), slog.String("queue", queueName))
						h.queueService.Enqueue(h.ctx, queueName, task)
					}
				}
			}
		}()
	})
}

// Stop gracefully shuts down the event handler.
// It cancels the handler's context to signal the event loop to terminate,
// then waits for the goroutine to finish processing.
//
// This method is safe to call even if Start() was never called or called multiple times,
// as sync.Once ensures the goroutine is created at most once.
//
// Blocks until the event processing goroutine exits.
func (h *Handler) Stop() {
	h.logger.Info("stop loop")

	if h.cancel != nil {
		h.cancel()
		h.wg.Wait()
	}
}

// kubeTaskBuilder converts a Kubernetes event into queue tasks.
// It receives a Kubernetes event and returns a map of queue names to tasks.
// The returned map keys are queue names, and values are task lists to enqueue.
func (h *Handler) kubeTaskBuilder(ctx context.Context, kubeEvent kemtypes.KubeEvent) map[string][]queue.Task {
	builder := func(_ context.Context, name, hook string, info hookcontroller.BindingExecutionInfo) (string, queue.Task) {
		h.logger.Debug("create task by kube event",
			slog.String("hook", hook),
			slog.String("name", name),
			slog.String("event", kubeEvent.String()))

		queueName := info.QueueName
		if queueName == "main" {
			queueName = name
		}

		return queueName, hookrun.New(name, hook, info.BindingContext, h, h.logger)
	}

	return h.packageManager.BuildKubeTasks(ctx, kubeEvent, builder)
}

// scheduleTaskBuilder converts a cron schedule trigger into queue tasks.
// It receives the cron schedule string and returns a map of queue names to tasks.
// The returned map keys are queue names, and values are task lists to enqueue.
func (h *Handler) scheduleTaskBuilder(ctx context.Context, crontab string) map[string][]queue.Task {
	builder := func(_ context.Context, name, hook string, info hookcontroller.BindingExecutionInfo) (string, queue.Task) {
		h.logger.Debug("create task by schedule event",
			slog.String("hook", hook),
			slog.String("name", name),
			slog.String("event", crontab))

		queueName := info.QueueName
		if queueName == "main" {
			queueName = name
		}

		return queueName, hookrun.New(name, hook, info.BindingContext, h, h.logger)
	}

	return h.packageManager.BuildScheduleTasks(ctx, crontab, builder)
}

func (h *Handler) QueueService() *queue.Service {
	return h.queueService
}

func (h *Handler) PackageManager() *packagemanager.Manager {
	return h.packageManager
}
