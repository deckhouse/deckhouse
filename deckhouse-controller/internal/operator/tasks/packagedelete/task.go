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

package packagedelete

import (
	"context"
	"fmt"
	"log/slog"

	packagemanager "github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/manager"
	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/queue"
	"github.com/deckhouse/deckhouse/pkg/log"
)

const (
	taskTracer = "packageDelete"
)

type DependencyContainer interface {
	PackageManager() *packagemanager.Manager
	QueueService() *queue.Service
}

func New(name string, dc DependencyContainer, logger *log.Logger) queue.Task {
	return &task{
		packageName: name,
		dc:          dc,
		logger:      logger.Named(taskTracer),
	}
}

type task struct {
	packageName string

	dc DependencyContainer

	logger *log.Logger
}

func (t *task) String() string {
	return fmt.Sprintf("Package:%s:Delete", t.packageName)
}

func (t *task) Execute(ctx context.Context) error {
	t.logger.Debug("delete package", slog.String("name", t.packageName))

	// get them here because manager will remove the app
	queues := t.dc.PackageManager().GetPackageQueues(t.packageName)

	// delete nelm release, stop kube monitors and schedules
	if err := t.dc.PackageManager().DisablePackage(ctx, t.packageName, false); err != nil {
		return fmt.Errorf("disable package '%s': %w", t.packageName, err)
	}

	t.logger.Debug("remove package queues", slog.String("name", t.packageName))

	// remove package hooks queues
	for _, q := range queues {
		if q == "main" || q == t.packageName {
			continue
		}

		t.logger.Debug("remove package queue", slog.String("name", t.packageName), slog.String("queue", q))
		t.dc.QueueService().Remove(q)
	}

	// remove package queue
	t.dc.QueueService().Remove(t.packageName)

	return nil
}
