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

package hooksync

import (
	"context"
	"fmt"
	"log/slog"

	bctx "github.com/flant/shell-operator/pkg/hook/binding_context"
	hookctrl "github.com/flant/shell-operator/pkg/hook/controller"

	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/queue"
	"github.com/deckhouse/deckhouse/pkg/log"
)

const (
	taskTracer = "hook-sync"
)

type manager interface {
	RunPackageHook(ctx context.Context, name, hook string, bctx []bctx.BindingContext) error
	UnlockKubernetesMonitors(name, hook string, monitors ...string)
}

type task struct {
	packageName string
	hook        string

	info hookctrl.BindingExecutionInfo

	manager manager

	logger *log.Logger
}

func NewTask(name, hook string, info hookctrl.BindingExecutionInfo, manager manager, logger *log.Logger) queue.Task {
	return &task{
		packageName: name,
		hook:        hook,
		info:        info,
		manager:     manager,
		logger:      logger.Named(taskTracer),
	}
}

func (t *task) String() string {
	return fmt.Sprintf("Package:%s:Hook:%s:Sync", t.packageName, t.hook)
}

func (t *task) Execute(ctx context.Context) error {
	// Hook synchronization task runs after hook bindings are registered
	// Purpose: Execute initial hook run and unlock monitors to allow future events
	t.logger.Debug("run sync hook", slog.String("hook", t.hook), slog.String("name", t.packageName))

	// If ExecuteHookOnSynchronization=false, just unlock and return
	// The hook will only run on future events, not during initialization
	if !t.info.KubernetesBinding.ExecuteHookOnSynchronization {
		t.manager.UnlockKubernetesMonitors(t.packageName, t.hook)
		return nil
	}

	// Execute hook with initial binding context (typically snapshot of current resources)
	if err := t.manager.RunPackageHook(ctx, t.packageName, t.hook, t.info.BindingContext); err != nil {
		// If AllowFailure=true, log warning and continue
		if !t.info.AllowFailure {
			return fmt.Errorf("run hook '%s': %w", t.hook, err)
		}
		t.logger.Warn("hook failed", slog.String("name", t.packageName), slog.String("hook", t.hook), log.Err(err))
	}

	// Unlock monitors to allow hook to process future Kubernetes events
	t.logger.Debug("unlock kubernetes monitors",
		slog.String("name", t.packageName),
		slog.String("monitor", t.info.KubernetesBinding.Monitor.Metadata.MonitorId),
		slog.String("hook", t.hook))
	t.manager.UnlockKubernetesMonitors(t.packageName, t.hook, t.info.KubernetesBinding.Monitor.Metadata.MonitorId)

	return nil
}
