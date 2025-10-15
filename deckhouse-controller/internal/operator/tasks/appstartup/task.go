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

package appstartup

import (
	"context"
	"fmt"
	"log/slog"
	"sync"

	helmresourcesmanager "github.com/flant/addon-operator/pkg/helm_resources_manager"

	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/operator/tasks/apprun"
	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/operator/tasks/hooksync"
	packagemanager "github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/manager"
	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/queue"
	"github.com/deckhouse/deckhouse/pkg/log"
)

const (
	taskTracer = "appstartup"

	queueSync = "sync"
)

type DependencyContainer interface {
	PackageManager() *packagemanager.Manager
	QueueService() *queue.Service
	HelmResourcesManager() helmresourcesmanager.HelmResourcesManager
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

func (t *task) Name() string {
	return "appstartup"
}

func (t *task) Execute(ctx context.Context) error {
	t.logger.Debug("startup package", slog.String("name", t.name))

	infos, err := t.dc.PackageManager().EnableKubernetesHooks(ctx, t.name)
	if err != nil {
		return fmt.Errorf("enable kubernetes hooks for '%s': %w", t.name, err)
	}

	waitTasks := make(map[string]queue.Task)
	tasks := make(map[string]queue.Task)
	for hook, info := range infos {
		syncTask := hooksync.New(t.name, hook, info, t.dc, t.logger)

		queueName := info.KubernetesBinding.Queue
		if queueName == "main" {
			queueName = fmt.Sprintf("%s-%s", t.name, queueSync)
		}

		if info.KubernetesBinding.WaitForSynchronization {
			waitTasks[queueName] = syncTask
			continue
		}

		tasks[queueName] = syncTask
	}

	t.logger.Debug("wait for sync tasks to finish", slog.String("name", t.name), slog.Int("tasks", len(waitTasks)))

	wg := new(sync.WaitGroup)
	for q, waitTask := range waitTasks {
		t.dc.QueueService().Enqueue(ctx, q, waitTask, queue.WithWait(wg))
	}
	wg.Wait()

	for q, syncTask := range tasks {
		t.dc.QueueService().Enqueue(ctx, q, syncTask)
	}

	if err = t.dc.PackageManager().EnableScheduleHooks(ctx, t.name); err != nil {
		return fmt.Errorf("enable schedule hooks for '%s': %w", t.name, err)
	}

	if err = t.dc.PackageManager().StartupPackage(ctx, t.name); err != nil {
		return fmt.Errorf("startup package '%s': %w", t.name, err)
	}

	t.dc.QueueService().Enqueue(ctx, t.name, apprun.New(t.name, t.dc, t.logger))

	return nil
}
