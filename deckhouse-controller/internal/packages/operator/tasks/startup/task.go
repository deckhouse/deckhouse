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

package startup

import (
	"context"
	"fmt"
	"log/slog"
	"sync"

	bindingctx "github.com/flant/shell-operator/pkg/hook/binding_context"
	hookcontroller "github.com/flant/shell-operator/pkg/hook/controller"

	taskhooksync "github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/operator/tasks/hooksync"
	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/queue"
	"github.com/deckhouse/deckhouse/pkg/log"
)

const (
	taskTracer = "package-startup"
)

type manager interface {
	StartupPackage(ctx context.Context, name string) error
	RunPackage(ctx context.Context, name string) error
	InitializeHooks(ctx context.Context, name string) (map[string][]hookcontroller.BindingExecutionInfo, error)

	UnlockKubernetesMonitors(name, hook string, monitors ...string)
	RunPackageHook(ctx context.Context, name, hook string, bctx []bindingctx.BindingContext) error
}

type queueService interface {
	Enqueue(ctx context.Context, name string, task queue.Task, opts ...queue.EnqueueOption)
}

type task struct {
	packageName string

	manager manager
	queue   queueService

	logger *log.Logger
}

func NewTask(name string, manager manager, queueService queueService, logger *log.Logger) queue.Task {
	return &task{
		packageName: name,
		manager:     manager,
		queue:       queueService,
		logger:      logger.Named(taskTracer),
	}
}

func (t *task) String() string {
	return "Startup"
}

func (t *task) Execute(ctx context.Context) error {
	t.logger.Debug("startup package", slog.String("name", t.packageName))

	// Step 1: Enable kubernetes/schedule hooks - registers watchers and cron schedules
	infos, err := t.manager.InitializeHooks(ctx, t.packageName)
	if err != nil {
		return fmt.Errorf("initialize hooks: %w", err)
	}

	// Step 2: Enqueue hook synchronization tasks
	// For each hook binding, we need to:
	// - Execute initial synchronization if ExecuteHookOnSynchronization=true
	// - Unlock monitors to allow future events to trigger the hook
	// - If WaitForSynchronization=true, block until sync completes
	t.logger.Debug("wait for sync tasks to finish", slog.String("name", t.packageName))
	wg := new(sync.WaitGroup)
	for hook, info := range infos {
		for _, hookInfo := range info {
			syncTask := taskhooksync.NewTask(t.packageName, hook, hookInfo, t.manager, t.logger)

			queueName := hookInfo.QueueName
			if queueName == "main" {
				queueName = t.packageName

				// Place wait tasks in separate sync queue to avoid blocking main queue
				// This prevents deadlocks when multiple hooks need to sync
				if hookInfo.KubernetesBinding.WaitForSynchronization {
					queueName = fmt.Sprintf("%s-sync", t.packageName)
				}
			}

			if hookInfo.KubernetesBinding.WaitForSynchronization {
				// Add to WaitGroup - we'll block until this completes
				t.queue.Enqueue(ctx, queueName, syncTask, queue.WithWait(wg))
				continue
			}

			// Non-blocking sync - don't wait for completion
			t.queue.Enqueue(ctx, queueName, syncTask)
		}
	}
	// Block until all WaitForSynchronization hooks complete
	// This ensures critical hooks run before startup hooks
	wg.Wait()

	// Step 3: Run package startup hooks (onStartup binding) and initial run
	t.logger.Debug("run package startup hooks", slog.String("name", t.packageName))
	if err = t.manager.StartupPackage(ctx, t.packageName); err != nil {
		return fmt.Errorf("startup package: %w", err)
	}

	return nil
}
