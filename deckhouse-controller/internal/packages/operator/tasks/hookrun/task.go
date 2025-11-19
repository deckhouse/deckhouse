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

package hookrun

import (
	"context"
	"fmt"
	"log/slog"

	bctx "github.com/flant/shell-operator/pkg/hook/binding_context"

	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/queue"
	"github.com/deckhouse/deckhouse/pkg/log"
)

const (
	taskTracer = "hook-run"
)

type manager interface {
	RunPackageHook(ctx context.Context, name, hook string, bctx []bctx.BindingContext) error
}

type task struct {
	packageName string
	hook        string

	bctx []bctx.BindingContext

	manager manager

	logger *log.Logger
}

func NewTask(name, hook string, bctx []bctx.BindingContext, manager manager, logger *log.Logger) queue.Task {
	return &task{
		packageName: name,
		hook:        hook,
		bctx:        bctx,
		manager:     manager,
		logger:      logger.Named(taskTracer),
	}
}

func (t *task) String() string {
	return fmt.Sprintf("Package:%s:Hook:%s:Run", t.packageName, t.hook)
}

func (t *task) Execute(ctx context.Context) error {
	t.logger.Debug("run hook", slog.String("hook", t.hook), slog.String("name", t.packageName))
	if err := t.manager.RunPackageHook(ctx, t.packageName, t.hook, t.bctx); err != nil {
		return fmt.Errorf("run hook '%s': %w", t.hook, err)
	}

	return nil
}
