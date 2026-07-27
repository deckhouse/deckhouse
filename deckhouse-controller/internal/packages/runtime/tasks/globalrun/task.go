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

// Package globalrun provides the global node's unit of work: run the global
// BeforeAll hooks, ensure the CRDs of every enabled module, then publish the
// enabled set and the discovered CRD capabilities into global values, before any
// module converges behind the global barrier.
package globalrun

import (
	"context"
	"fmt"
	"log/slog"
	"sync"

	addontypes "github.com/flant/addon-operator/pkg/hook/types"
	shtypes "github.com/flant/shell-operator/pkg/hook/types"
	"go.opentelemetry.io/otel"

	taskensurecrd "github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/runtime/tasks/ensurecrd"
	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/status"
	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/queue"
	"github.com/deckhouse/deckhouse/pkg/log"
)

const (
	taskTracer = "globalrun"
)

// Module is the minimal module view the task needs to ensure a module's CRDs.
type Module interface {
	// GetName returns the module name, used for logging and the subtask queue.
	GetName() string
	// GetPath returns the module root path that contains the crds directory.
	GetPath() string
}

// crdService applies a module's bundled CRDs and reports the GVKs applied for a
// set of modules. The ensure subtasks call Install (which records the applied
// GVKs per module); GetManagedGVKs aggregates them once every module is ensured.
type crdService interface {
	Install(ctx context.Context, name, path string) error
	GetManagedGVKs(enabled []string) []string
}

// globalModule runs the global BeforeAll hooks and receives the enabled module
// set and the discovered CRD capabilities for the global values.
type globalModule interface {
	GetName() string
	RunHooksByBinding(ctx context.Context, binding shtypes.BindingType) error
	SetEnabledModules(modules []string)
	SetCapabilities(apiVersions []string)
}

// queueService enqueues the per-module EnsureCRDs subtasks.
type queueService interface {
	Enqueue(ctx context.Context, name string, task queue.Task, opts ...queue.EnqueueOption)
}

// task ensures the CRDs of every enabled module, then publishes the enabled set
// and the applied GVKs (capabilities) into global values. It is the global
// node's unit of work, enqueued whenever the scheduler schedules global; the
// scheduler holds every module behind global (canSchedule barrier), so this runs
// before any module and modules render against a complete capability set.
type task struct {
	modules []Module

	crd    crdService
	global globalModule
	queue  queueService
	status *status.Service

	logger *log.Logger
}

// NewTask creates a task that ensures CRDs for the given enabled modules and
// publishes the resulting capabilities.
func NewTask(global globalModule, enabled []Module, crd crdService, queueService queueService, status *status.Service, logger *log.Logger) queue.Task {
	return &task{
		modules: enabled,
		crd:     crd,
		global:  global,
		queue:   queueService,
		status:  status,
		logger:  logger.Named(taskTracer),
	}
}

func (t *task) String() string {
	return "Run"
}

// Execute first runs the global BeforeAll hooks, then fans out one EnsureCRDs
// subtask per enabled module under the task's own context, waits for all of them
// to finish, then publishes the enabled set and the applied GVKs (capabilities)
// into global values.
//
// BeforeAll runs before the CRDs are ensured: the hooks prepare the shared global
// values that every module renders against, all behind the global barrier.
//
// The subtasks share this task's context: cancelling it (queue shutdown, or a
// fresh global schedule) drops the in-flight ensures and releases the wait. A
// broken CRD retries forever (queue backoff) and surfaces
// ConditionCustomResourcesApplied=False on its module; the wait holds — and with
// it every module behind the global barrier — until it succeeds or is cancelled.
//
// Global values are published only on a clean, uncancelled run, so they never
// reflect a half-ensured set: each module's Install records its applied GVKs, so
// once the wait returns GetManagedGVKs reports the complete set.
func (t *task) Execute(ctx context.Context) error {
	ctx, span := otel.Tracer(taskTracer).Start(ctx, "Run")
	defer span.End()

	// Run the global BeforeAll hooks before ensuring CRDs so they prepare the
	// shared global values every module renders against, behind the global barrier.
	// The Enable task ahead of this on the global queue has already initialized and
	// synced the hooks.
	if err := t.global.RunHooksByBinding(ctx, addontypes.BeforeAll); err != nil {
		t.status.HandleError(t.global.GetName(), status.ConditionHooksProcessed, err)
		return fmt.Errorf("run beforeAll hooks: %w", err)
	}

	t.logger.Debug("ensure crds for enabled modules", slog.Int("modules", len(t.modules)))

	wg := new(sync.WaitGroup)
	names := make([]string, 0, len(t.modules))
	for _, pkg := range t.modules {
		names = append(names, pkg.GetName())
		sub := taskensurecrd.NewTask(pkg, t.crd.Install, t.status, t.logger)
		t.queue.Enqueue(ctx, pkg.GetName()+"/crd", sub, queue.WithWait(wg))
	}

	wg.Wait()

	t.global.SetEnabledModules(names)
	t.global.SetCapabilities(t.crd.GetManagedGVKs(names))

	return nil
}
