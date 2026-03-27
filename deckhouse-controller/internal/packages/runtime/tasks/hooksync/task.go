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
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"

	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/status"
	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/queue"
	"github.com/deckhouse/deckhouse/pkg/log"
)

const (
	taskTracer = "hook-sync"
)

// packageI abstracts package operations needed for hook synchronization.
type packageI interface {
	GetName() string
	GetValuesChecksum() string
	// RunHookByName executes a specific hook with the given binding context.
	RunHookByName(ctx context.Context, hook string, bctx []bctx.BindingContext) error
	// UnlockKubernetesMonitors allows events to flow to the hook after sync.
	UnlockKubernetesMonitors(hook string, monitors ...string)
}

// nelmI abstracts Helm monitor control during hook execution.
type nelmI interface {
	// PauseMonitor stops monitoring while hook runs (hook may modify resources).
	PauseMonitor(name string)
	ResumeMonitor(name string)
}

// statusService provides condition updates and error handling.
type statusService interface {
	SetConditionTrue(name string, cond status.ConditionType)
	HandleError(name string, err error)
}

// task synchronizes a single hook binding during package startup.
// Runs the hook with initial resource snapshot, then unlocks the monitor.
type task struct {
	pkg  packageI
	hook string

	info hookctrl.BindingExecutionInfo // contains binding context and sync options

	nelm   nelmI
	status statusService

	logger *log.Logger
}

// NewTask creates a sync task for a specific hook binding.
// Created by the Startup task for each Kubernetes hook binding.
func NewTask(pkg packageI, hook string, info hookctrl.BindingExecutionInfo, nelm nelmI, status statusService, logger *log.Logger) queue.Task {
	return &task{
		pkg:    pkg,
		hook:   hook,
		info:   info,
		nelm:   nelm,
		status: status,
		logger: logger.Named(taskTracer).With(slog.String("hook", hook), slog.String("name", pkg.GetName())),
	}
}

func (t *task) String() string {
	return fmt.Sprintf("%s:%s:Sync", t.pkg.GetName(), t.hook)
}

// Execute runs the initial hook synchronization and unlocks the monitor.
// If ExecuteHookOnSynchronization is false, only unlocks without running.
// If AllowFailure is true, logs warning and continues on error.
func (t *task) Execute(ctx context.Context) error {
	t.logger.Debug("run sync hook")

	// If ExecuteHookOnSynchronization=false, just unlock and return
	// The hook will only run on future events, not during initialization
	if !t.info.KubernetesBinding.ExecuteHookOnSynchronization {
		t.pkg.UnlockKubernetesMonitors(t.hook, t.info.KubernetesBinding.Monitor.Metadata.MonitorId)
		return nil
	}

	// Execute hook with initial binding context (typically snapshot of current resources)
	if err := t.runPackageSyncHook(ctx); err != nil {
		// If AllowFailure=true, log warning and continue
		if !t.info.AllowFailure {
			t.status.HandleError(t.pkg.GetName(), err)
			return fmt.Errorf("run hook '%s': %w", t.hook, err)
		}
		t.logger.Warn("hook failed", log.Err(err))
	}

	// Unlock monitors to allow hook to process future Kubernetes events
	t.logger.Debug("unlock kubernetes monitors",
		slog.String("monitor", t.info.KubernetesBinding.Monitor.Metadata.MonitorId))
	t.pkg.UnlockKubernetesMonitors(t.hook, t.info.KubernetesBinding.Monitor.Metadata.MonitorId)

	return nil
}

// runPackageSyncHook executes the hook with the initial binding context.
// Pauses Helm monitoring during execution to prevent false alerts.
func (t *task) runPackageSyncHook(ctx context.Context) error {
	ctx, span := otel.Tracer(taskTracer).Start(ctx, "RunPackageSyncHook")
	defer span.End()

	span.SetAttributes(attribute.String("name", t.pkg.GetName()))
	span.SetAttributes(attribute.String("hook", t.hook))

	t.logger.Debug("run package sync hook")

	// Pause NELM monitoring during hook execution to prevent race conditions.
	// Hooks may modify resources that the monitor is tracking, which could trigger
	// false-positive alerts or inconsistent state. Resume monitoring after completion.
	t.nelm.PauseMonitor(t.pkg.GetName())
	defer t.nelm.ResumeMonitor(t.pkg.GetName())

	if err := t.pkg.RunHookByName(ctx, t.hook, t.info.BindingContext); err != nil {
		span.SetStatus(codes.Error, err.Error())
		return newEventHookErr(err)
	}

	return nil
}
