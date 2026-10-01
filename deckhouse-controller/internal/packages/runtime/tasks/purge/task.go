// Copyright 2026 Flant JSC
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

package purge

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/orphans"
	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/queue"
	"github.com/deckhouse/deckhouse/pkg/log"
)

const (
	// taskTracer identifies tracing and log records emitted by the purge task.
	taskTracer = "package-purge"
)

// packageI abstracts the application whose leftovers are purged.
type packageI interface {
	GetName() string
	GetNamespace() string
	GetInstance() string
	// GetOrphanResources returns the kinds of objects the package leaves beside its release.
	GetOrphanResources() ([]orphans.Resource, error)
}

// orphansI abstracts deleting the objects an instance left beside its release.
type orphansI interface {
	Delete(ctx context.Context, namespace, instance string, resources []orphans.Resource) error
}

// NewTask creates a purge task. A removal enqueues it behind the Disable task, so it runs once the
// release is uninstalled and the hooks are stopped, when nothing of the application is left to
// recreate what it deletes.
func NewTask(pkg packageI, service orphansI, logger *log.Logger) queue.Task {
	return &task{
		pkg:     pkg,
		orphans: service,
		logger:  logger.Named(taskTracer).With("name", pkg.GetName()),
	}
}

// task deletes the objects of the kinds the package declares that the instance left in its namespace.
type task struct {
	pkg packageI

	orphans orphansI

	logger *log.Logger
}

// String returns the stable queue identity of the task.
func (t *task) String() string {
	return "Purge"
}

// Execute deletes the instance's objects of the declared kinds. A declaration that cannot be read is
// reported and skipped rather than failed: it reads the same on every retry, and the removal waits
// behind this task.
func (t *task) Execute(ctx context.Context) error {
	resources, err := t.pkg.GetOrphanResources()
	if err != nil {
		t.logger.Error("skip purge, orphan resources are malformed", log.Err(err))
		return nil
	}

	if len(resources) == 0 {
		return nil
	}

	t.logger.Debug("purge orphan resources", slog.Int("kinds", len(resources)))

	if err = t.orphans.Delete(ctx, t.pkg.GetNamespace(), t.pkg.GetInstance(), resources); err != nil {
		return fmt.Errorf("delete orphan resources: %w", err)
	}

	return nil
}
