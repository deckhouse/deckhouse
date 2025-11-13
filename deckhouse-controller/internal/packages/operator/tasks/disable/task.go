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

package disable

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/queue"
	"github.com/deckhouse/deckhouse/pkg/log"
)

const (
	taskTracer = "package-disable"
)

type manager interface {
	DisablePackage(ctx context.Context, name string, keep bool) error
}

func NewTask(name string, manager manager, keep bool, logger *log.Logger) queue.Task {
	return &task{
		packageName: name,
		keep:        keep,
		manager:     manager,
		logger:      logger.Named(taskTracer),
	}
}

type task struct {
	packageName string
	keep        bool

	manager manager

	logger *log.Logger
}

func (t *task) String() string {
	return "Disable"
}

func (t *task) Execute(ctx context.Context) error {
	// stop kube monitors and schedules
	t.logger.Debug("disable package", slog.String("name", t.packageName))
	if err := t.manager.DisablePackage(ctx, t.packageName, t.keep); err != nil {
		return fmt.Errorf("disable package '%s': %w", t.packageName, err)
	}

	return nil
}
