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

package globalenable

import (
	"context"
	"fmt"
	"log/slog"

	addontypes "github.com/flant/addon-operator/pkg/hook/types"
	bctx "github.com/flant/shell-operator/pkg/hook/binding_context"
	hookcontroller "github.com/flant/shell-operator/pkg/hook/controller"
	shtypes "github.com/flant/shell-operator/pkg/hook/types"
	"go.opentelemetry.io/otel"

	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/hooks"
	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/status"
	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/queue"
	"github.com/deckhouse/deckhouse/pkg/log"
)

const taskTracer = "package-global-enable"

// globalI abstracts the global module operations needed to enable its hooks.
type globalI interface {
	GetName() string
	// HooksInitialized reports whether hook controllers were already built.
	HooksInitialized() bool
	// InitializeHooks creates hook controllers and binds them to events.
	InitializeHooks()
	GetHooksByBinding(binding shtypes.BindingType) []hooks.GlobalHook
	RunHooksByBinding(ctx context.Context, binding shtypes.BindingType) error
	RunHookByName(ctx context.Context, hook string, bctx []bctx.BindingContext) error
	// UnlockKubernetesMonitors allows events to flow to the hook after sync.
	UnlockKubernetesMonitors(hook string, monitors ...string)
}

// task enables the global module's hooks. The scheduler enqueues it whenever it
// (re)schedules global, mirroring the per-package enable task but without Helm.
type task struct {
	pkg globalI

	status *status.Service
	logger *log.Logger
}

// NewTask creates a task that initializes and runs the global module's hooks.
func NewTask(pkg globalI, status *status.Service, logger *log.Logger) queue.Task {
	return &task{
		pkg:    pkg,
		status: status,
		logger: logger.Named(taskTracer).With(slog.String("name", pkg.GetName())),
	}
}

func (t *task) String() string {
	return "GlobalEnable"
}

// Execute initializes hook controllers once, enables schedule and Kubernetes
// bindings (with initial synchronization), then runs the OnStartup and BeforeAll
// hooks so global values are ready before any package renders.
func (t *task) Execute(ctx context.Context) error {
	ctx, span := otel.Tracer(taskTracer).Start(ctx, "Execute")
	defer span.End()

	// DEBUG: temporary demo logging — remove before merge.
	t.logger.Info("DEBUG globalenable: start", slog.Bool("hooks_initialized", t.pkg.HooksInitialized()))

	if !t.pkg.HooksInitialized() {
		t.pkg.InitializeHooks()
	}

	scheduleHooks := t.pkg.GetHooksByBinding(shtypes.Schedule)
	for _, hook := range scheduleHooks {
		hook.GetHookController().EnableScheduleBindings()
	}
	t.logger.Info("DEBUG globalenable: schedule bindings enabled", slog.Int("count", len(scheduleHooks)))

	if err := t.syncKubernetesBindings(ctx); err != nil {
		t.status.HandleError(t.pkg.GetName(), status.ConditionHooksProcessed, err)
		return fmt.Errorf("sync kubernetes bindings: %w", err)
	}
	t.logger.Info("DEBUG globalenable: kubernetes bindings synced",
		slog.Int("count", len(t.pkg.GetHooksByBinding(shtypes.OnKubernetesEvent))))

	t.logger.Info("DEBUG globalenable: running onStartup hooks",
		slog.Int("count", len(t.pkg.GetHooksByBinding(shtypes.OnStartup))))
	if err := t.pkg.RunHooksByBinding(ctx, shtypes.OnStartup); err != nil {
		t.status.HandleError(t.pkg.GetName(), status.ConditionHooksProcessed, err)
		return fmt.Errorf("run onStartup hooks: %w", err)
	}

	t.logger.Info("DEBUG globalenable: running beforeAll hooks",
		slog.Int("count", len(t.pkg.GetHooksByBinding(addontypes.BeforeAll))))
	if err := t.pkg.RunHooksByBinding(ctx, addontypes.BeforeAll); err != nil {
		t.status.HandleError(t.pkg.GetName(), status.ConditionHooksProcessed, err)
		return fmt.Errorf("run beforeAll hooks: %w", err)
	}

	t.logger.Info("DEBUG globalenable: done")

	return nil
}

// syncKubernetesBindings enables every Kubernetes hook binding, runs the initial
// synchronization for the bindings that request it, and unlocks the monitors so
// subsequent cluster events reach the hooks.
func (t *task) syncKubernetesBindings(ctx context.Context) error {
	type syncUnit struct {
		hook string
		info hookcontroller.BindingExecutionInfo
	}

	var units []syncUnit
	for _, hook := range t.pkg.GetHooksByBinding(shtypes.OnKubernetesEvent) {
		name := hook.GetName()
		err := hook.GetHookController().HandleEnableKubernetesBindings(ctx, func(info hookcontroller.BindingExecutionInfo) {
			units = append(units, syncUnit{hook: name, info: info})
		})
		if err != nil {
			return fmt.Errorf("enable kubernetes bindings for hook %q: %w", name, err)
		}
	}

	for _, unit := range units {
		if unit.info.KubernetesBinding.ExecuteHookOnSynchronization {
			if err := t.pkg.RunHookByName(ctx, unit.hook, unit.info.BindingContext); err != nil {
				if !unit.info.AllowFailure {
					return fmt.Errorf("run sync hook %q: %w", unit.hook, err)
				}

				t.logger.Warn("global sync hook failed", slog.String("hook", unit.hook), log.Err(err))
			}
		}

		t.pkg.UnlockKubernetesMonitors(unit.hook, unit.info.KubernetesBinding.Monitor.Metadata.MonitorId)
	}

	return nil
}
