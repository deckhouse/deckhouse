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
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"

	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/status"
	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/queue"
	"github.com/deckhouse/deckhouse/pkg/log"
)

const (
	taskTracer = "hook-run"
)

// packageI abstracts package operations needed for hook execution.
type packageI interface {
	GetName() string
	// GetValuesChecksum returns hash to detect if hook modified values.
	GetValuesChecksum() string
	// RunHookByName executes a specific hook with the given binding context.
	RunHookByName(ctx context.Context, hook string, bctx []bctx.BindingContext) error
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

// task executes a hook in response to a Kubernetes event or schedule trigger.
// Created by the event handler when monitors detect changes.
type task struct {
	pkg  packageI
	hook string

	bctx []bctx.BindingContext // event details passed to the hook

	onValuesChanged func(name string)

	nelm   nelmI
	status statusService

	logger *log.Logger
}

// NewTask creates a task to run a hook with the given binding context.
// The binding context contains event details (added/modified/deleted objects).
func NewTask(pkg packageI, hook string, bctx []bctx.BindingContext, onValuesChange func(name string), nelm nelmI, status statusService, logger *log.Logger) queue.Task {
	return &task{
		pkg:             pkg,
		hook:            hook,
		bctx:            bctx,
		nelm:            nelm,
		status:          status,
		onValuesChanged: onValuesChange,
		logger:          logger.Named(taskTracer).With(slog.String("hook", hook), slog.String("name", pkg.GetName())),
	}
}

func (t *task) String() string {
	return fmt.Sprintf("%s:%s:Run", t.pkg.GetName(), t.hook)
}

// Execute runs the hook and delegates errors to the status service.
func (t *task) Execute(ctx context.Context) error {
	t.logger.Debug("run hook")
	if err := t.runPackageHook(ctx); err != nil {
		t.status.HandleError(t.pkg.GetName(), err)
		return fmt.Errorf("run hook '%s': %w", t.hook, err)
	}

	return nil
}

// runPackageHook executes the hook with the binding context.
// Pauses Helm monitoring during execution to prevent false alerts.
func (t *task) runPackageHook(ctx context.Context) error {
	ctx, span := otel.Tracer(taskTracer).Start(ctx, "RunPackageHook")
	defer span.End()

	span.SetAttributes(attribute.String("name", t.pkg.GetName()))
	span.SetAttributes(attribute.String("hook", t.hook))

	t.logger.Debug("run package hook")

	// Pause NELM monitoring during hook execution to prevent race conditions.
	// Hooks may modify resources that the monitor is tracking, which could trigger
	// false-positive alerts or inconsistent state. Resume monitoring after completion.
	t.nelm.PauseMonitor(t.pkg.GetName())
	defer t.nelm.ResumeMonitor(t.pkg.GetName())

	// Track if values changed during hook execution.
	// If the checksum differs after hook execution, it indicates that the hook
	// modified application values (via patches), which may require a Helm upgrade
	// to reconcile the cluster state with the new values.
	oldChecksum := t.pkg.GetValuesChecksum()
	if err := t.pkg.RunHookByName(ctx, t.hook, t.bctx); err != nil {
		span.SetStatus(codes.Error, err.Error())
		return newEventHookErr(err)
	}

	if oldChecksum != t.pkg.GetValuesChecksum() {
		t.logger.Debug("values changed during the hook")
		t.onValuesChanged(t.pkg.GetName())
	}

	return nil
}
