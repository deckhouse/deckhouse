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

package appdisable

import (
	"context"
	"fmt"
	"log/slog"

	packagemanager "github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/manager"
	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/queue"
	"github.com/deckhouse/deckhouse/pkg/log"
)

const (
	taskTracer = "appdisable"
)

type DependencyContainer interface {
	PackageManager() *packagemanager.Manager
	QueueService() *queue.Service
}

func New(name string, dc DependencyContainer, logger *log.Logger) queue.Task {
	return &task{
		name:   name,
		dc:     dc,
		logger: logger.Named(taskTracer),
	}
}

type task struct {
	name string

	dc DependencyContainer

	logger *log.Logger
}

func (t *task) Name() string {
	return "appdisable"
}

func (t *task) Execute(ctx context.Context) error {
	t.logger.Debug("delete package", slog.String("name", t.name))

	// delete nelm release, stop kube monitors and schedules
	if err := t.dc.PackageManager().DisablePackage(ctx, t.name); err != nil {
		return fmt.Errorf("disable package '%s': %w", t.name, err)
	}

	t.logger.Debug("remove package queues", slog.String("name", t.name))

	// remove package hooks queues
	for _, q := range t.dc.PackageManager().GetPackageQueues(t.name) {
		t.dc.QueueService().Remove(q)
	}

	// remove package queue
	t.dc.QueueService().Remove(t.name)

	return nil
}
