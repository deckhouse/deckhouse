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

	hookcontroller "github.com/flant/shell-operator/pkg/hook/controller"

	packagemanager "github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/manager"
	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/queue"
	"github.com/deckhouse/deckhouse/pkg/log"
)

const (
	taskTracer = "hookSync"
)

type DependencyContainer interface {
	PackageManager() *packagemanager.Manager
}

type task struct {
	name string
	hook string

	info hookcontroller.BindingExecutionInfo

	dc DependencyContainer

	logger *log.Logger
}

func New(name, hook string, info hookcontroller.BindingExecutionInfo, dc DependencyContainer, logger *log.Logger) queue.Task {
	return &task{
		name:   name,
		hook:   hook,
		info:   info,
		dc:     dc,
		logger: logger.Named(taskTracer),
	}
}

func (t *task) String() string {
	return fmt.Sprintf("Package:%s:Hook:%s:Sync", t.name, t.hook)
}

func (t *task) Execute(ctx context.Context) error {
	t.logger.Debug("run sync hook", slog.String("hook", t.hook), slog.String("name", t.name))
	if !t.info.KubernetesBinding.ExecuteHookOnSynchronization {
		t.dc.PackageManager().UnlockKubernetesMonitors(t.name, t.hook)

		return nil
	}

	if _, err := t.dc.PackageManager().RunPackageHook(ctx, t.name, t.hook, t.info.BindingContext); err != nil {
		if !t.info.AllowFailure {
			return fmt.Errorf("run hook '%s': %w", t.hook, err)
		}
		t.logger.Warn("hook failed", slog.String("name", t.name), slog.String("hook", t.hook), log.Err(err))
	}

	t.dc.PackageManager().UnlockKubernetesMonitors(t.name, t.hook)

	return nil
}
