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

package packagestartup

import (
	"context"
	"fmt"
	"log/slog"
	"sync"

	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/operator/tasks/hooksync"
	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/operator/tasks/packagerun"
	packagemanager "github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/manager"
	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/queue"
	"github.com/deckhouse/deckhouse/pkg/log"
)

const (
	taskTracer = "packageStartup"
)

type DependencyContainer interface {
	PackageManager() *packagemanager.Manager
	QueueService() *queue.Service
}

type task struct {
	name string

	dc DependencyContainer

	logger *log.Logger
}

func New(name string, dc DependencyContainer, logger *log.Logger) queue.Task {
	return &task{
		name:   name,
		dc:     dc,
		logger: logger.Named(taskTracer),
	}
}

func (t *task) String() string {
	return fmt.Sprintf("Package:%s:Startup", t.name)
}

func (t *task) Execute(ctx context.Context) error {
	t.logger.Debug("startup package", slog.String("name", t.name))

	infos, err := t.dc.PackageManager().InitializeHooks(ctx, t.name)
	if err != nil {
		return fmt.Errorf("initialize hooks for '%s': %w", t.name, err)
	}

	t.logger.Debug("wait for sync tasks to finish", slog.String("name", t.name))

	wg := new(sync.WaitGroup)
	for hook, info := range infos {
		for _, hookInfo := range info {
			syncTask := hooksync.New(t.name, hook, hookInfo, t.dc, t.logger)

			queueName := hookInfo.QueueName
			if queueName == "main" {
				queueName = t.name

				// place wait tasks in different sync queue to not block their execution
				if hookInfo.KubernetesBinding.WaitForSynchronization {
					queueName = fmt.Sprintf("%s-sync", t.name)
				}
			}

			if hookInfo.KubernetesBinding.WaitForSynchronization {
				t.dc.QueueService().Enqueue(ctx, queueName, syncTask, queue.WithWait(wg))
				continue
			}

			t.dc.QueueService().Enqueue(ctx, queueName, syncTask)
		}
	}
	wg.Wait()

	t.logger.Debug("run package startup hooks", slog.String("name", t.name))

	if err = t.dc.PackageManager().StartupPackage(ctx, t.name); err != nil {
		return fmt.Errorf("startup package '%s': %w", t.name, err)
	}

	t.dc.QueueService().Enqueue(ctx, t.name, packagerun.New(t.name, t.dc, t.logger))

	return nil
}
