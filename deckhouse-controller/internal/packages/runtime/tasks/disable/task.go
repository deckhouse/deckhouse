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

	addontypes "github.com/flant/addon-operator/pkg/hook/types"
	shtypes "github.com/flant/shell-operator/pkg/hook/types"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"

	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/hooks"
	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/queue"
	"github.com/deckhouse/deckhouse/pkg/log"
)

const (
	taskTracer = "package-disable"
)

// packageI abstracts package operations needed for disabling.
type packageI interface {
	GetName() string
	GetQueues() []string
	// RunHooksByBinding executes hooks matching the given binding type (e.g., AfterDeleteHelm).
	RunHooksByBinding(ctx context.Context, binding shtypes.BindingType) error
	// GetHooksByBinding returns hooks for a binding type to disable their controllers.
	GetHooksByBinding(binding shtypes.BindingType) []hooks.Hook
}

// nelmI abstracts Helm release management operations.
type nelmI interface {
	// Delete uninstalls the Helm release for the package.
	Delete(ctx context.Context, namespace, name string) error
	// RemoveMonitor stops watching Helm release resources.
	RemoveMonitor(name string)
}

type queueService interface {
	Remove(name string)
}

// statusService handles condition cleanup after disable.
type statusService interface {
	// ClearRuntimeConditions resets runtime-related conditions when package stops.
	ClearRuntimeConditions(name string)
}

// NewTask creates a disable task.
// If keep is true, the Helm release is preserved (used during updates).
// If keep is false, the release is deleted (used during removal).
func NewTask(pkg packageI, ns string, keep bool, nelm nelmI, queue queueService, status statusService, logger *log.Logger) queue.Task {
	return &task{
		pkg:          pkg,
		namespace:    ns,
		keep:         keep,
		nelm:         nelm,
		queueService: queue,
		status:       status,
		logger:       logger.Named(taskTracer).With("name", pkg.GetName()),
	}
}

// task stops a running package by uninstalling its Helm release,
// running cleanup hooks, and disabling all monitors.
type task struct {
	pkg       packageI
	namespace string
	keep      bool // if true, preserve Helm release (update flow)

	nelm         nelmI
	queueService queueService
	status       statusService

	logger *log.Logger
}

func (t *task) String() string {
	return "Disable"
}

// Execute disables the package and clears runtime conditions.
// This is typically the first task in update/remove flows.
func (t *task) Execute(ctx context.Context) error {
	t.logger.Debug("disable package")
	if err := t.disablePackage(ctx); err != nil {
		return fmt.Errorf("disable package '%s': %w", t.pkg.GetName(), err)
	}

	for _, q := range t.pkg.GetQueues() {
		t.logger.Debug("remove package queue", slog.String("queue", q))
		t.queueService.Remove(fmt.Sprintf("%s/%s", t.pkg.GetName(), q))
		t.queueService.Remove(fmt.Sprintf("%s/%s/sync", t.pkg.GetName(), q))
	}

	t.queueService.Remove(fmt.Sprintf("%s/sync", t.pkg.GetName()))

	t.status.ClearRuntimeConditions(t.pkg.GetName())

	return nil
}

// disablePackage stops monitoring, uninstalls helm release and disables all hooks for a package.
//
// Process:
//  1. Stop Helm resource monitoring
//  2. Uninstall Helm release
//  3. Run AfterDeleteHelm hooks
//  4. Disable all schedule hooks
//  5. Stop all Kubernetes event monitors
func (t *task) disablePackage(ctx context.Context) error {
	_, span := otel.Tracer(taskTracer).Start(ctx, "DisablePackage")
	defer span.End()

	span.SetAttributes(attribute.String("name", t.pkg.GetName()))

	t.logger.Debug("disable package")

	// app should not get absent events
	t.nelm.RemoveMonitor(t.pkg.GetName())

	if !t.keep {
		t.logger.Debug("delete nelm release")
		if err := t.nelm.Delete(ctx, t.namespace, t.pkg.GetName()); err != nil {
			span.SetStatus(codes.Error, err.Error())
			return err
		}

		t.logger.Debug("run after delete helm hooks")

		// Run after delete helm hooks
		if err := t.pkg.RunHooksByBinding(ctx, addontypes.AfterDeleteHelm); err != nil {
			span.SetStatus(codes.Error, err.Error())
			return fmt.Errorf("run after delete helm hooks: %w", err)
		}
	}

	// Disable all schedule-based hooks
	schHooks := t.pkg.GetHooksByBinding(shtypes.Schedule)
	for _, hook := range schHooks {
		t.logger.Debug("disable schedule hook", slog.String("hook", hook.GetName()))
		if hook.GetHookController() != nil {
			hook.GetHookController().DisableScheduleBindings()
		}
	}

	// Stop all Kubernetes event monitors
	kubeHooks := t.pkg.GetHooksByBinding(shtypes.OnKubernetesEvent)
	for _, hook := range kubeHooks {
		t.logger.Debug("disable kube hook", slog.String("hook", hook.GetName()))
		if hook.GetHookController() != nil {
			hook.GetHookController().StopMonitors()
		}
	}

	return nil
}
