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

package run

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/queue"
	"github.com/deckhouse/deckhouse/pkg/log"
)

const (
	taskTracer = "package-run"
)

type manager interface {
	RunPackage(ctx context.Context, name string) error
}

type task struct {
	packageName string

	manager manager

	logger *log.Logger
}

func NewTask(name string, manager manager, logger *log.Logger) queue.Task {
	return &task{
		packageName: name,
		manager:     manager,
		logger:      logger.Named(taskTracer),
	}
}

func (t *task) String() string {
	return "Run"
}

func (t *task) Execute(ctx context.Context) error {
	// Run package lifecycle: beforeHelm hooks → Helm upgrade → afterHelm hooks
	// Also starts Helm resource monitoring to detect drift/deletions
	t.logger.Debug("run package", slog.String("name", t.packageName))
	if err := t.manager.RunPackage(ctx, t.packageName); err != nil {
		return fmt.Errorf("run package: %w", err)
	}

	return nil
}
