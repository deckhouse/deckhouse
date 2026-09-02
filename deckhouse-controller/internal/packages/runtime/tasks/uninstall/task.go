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

package uninstall

import (
	"context"
	"fmt"

	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/queue"
	"github.com/deckhouse/deckhouse/pkg/log"
)

const (
	// taskTracer identifies tracing and log records emitted by the uninstall task.
	taskTracer = "package-uninstall"
)

// nelmI abstracts release operations needed when no loaded package is available.
type nelmI interface {
	// Delete uninstalls the named release and succeeds when it is already absent.
	Delete(ctx context.Context, namespace, name string) error
	// RemoveMonitor stops watching resources owned by the named release.
	RemoveMonitor(name string)
}

// NewTask creates an uninstall task for a release whose loaded package is unavailable.
// A missing release is successful because the Nelm delete operation is idempotent.
func NewTask(name, namespace string, nelm nelmI, logger *log.Logger) queue.Task {
	return &task{
		name:      name,
		namespace: namespace,
		nelm:      nelm,
		logger:    logger.Named(taskTracer).With("name", name),
	}
}

// task removes a release without requiring its loaded package or lifecycle hooks.
type task struct {
	name      string
	namespace string

	nelm nelmI

	logger *log.Logger
}

// String returns the stable queue identity of the task.
func (t *task) String() string {
	return "Uninstall"
}

// Execute stops release monitoring and deletes the release by namespace and name.
func (t *task) Execute(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	t.logger.Debug("uninstall package release")
	t.nelm.RemoveMonitor(t.name)

	if err := ctx.Err(); err != nil {
		return err
	}

	if err := t.nelm.Delete(ctx, t.namespace, t.name); err != nil {
		return fmt.Errorf("delete release '%s': %w", t.name, err)
	}

	return nil
}
