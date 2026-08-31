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
	"path/filepath"
	"sync"

	addontypes "github.com/flant/addon-operator/pkg/hook/types"
	addonutils "github.com/flant/addon-operator/pkg/utils"
	"github.com/flant/kube-client/manifest"
	shtypes "github.com/flant/shell-operator/pkg/hook/types"
	"go.opentelemetry.io/otel"

	sdkpkg "github.com/deckhouse/module-sdk/pkg"

	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/nelm"
	taskensurecrd "github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/runtime/tasks/ensurecrd"
	taskensurewebhooks "github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/runtime/tasks/ensurewebhooks"
	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/status"
	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/queue"
	"github.com/deckhouse/deckhouse/pkg/log"
)

const (
	taskTracer = "globalrun"
)

// Module is the minimal module view the task needs to ensure a module's CRDs and
// conversion webhooks: the webhooks are rendered from the module's chart, so the
// values and maintenance mode a render needs are part of the view.
type Module interface {
	// GetName returns the module name, used for logging and the subtask queue.
	GetName() string
	// GetPath returns the module root path that contains the crds directory and the chart.
	GetPath() string
	GetRuntimeValues() string
	GetValues() addonutils.Values
	GetMaintenance() nelm.MaintenanceState
}

// crdService applies a module's bundled CRDs and reports the GVKs applied for a
// set of modules. The ensure subtasks call Install (which records the applied
// GVKs per module); GetManagedGVKs aggregates them once every module is ensured.
type crdService interface {
	Install(ctx context.Context, name, path string) error
	GetManagedGVKs(enabled []string) []string
	HasCRDs(path string) error
}

// globalModule runs the global BeforeAll hooks and receives the enabled module
// set and the discovered CRD capabilities for the global values.
type globalModule interface {
	GetName() string
	RunHooksByBinding(ctx context.Context, binding shtypes.BindingType) error
	SetEnabledModules(modules []string)
	SetCapabilities(apiVersions []string)
}

// queueService enqueues the per-module EnsureCRDs and EnsureWebhooks subtasks.
type queueService interface {
	Enqueue(ctx context.Context, name string, task queue.Task, opts ...queue.EnqueueOption)
}

// nelmI reports whether a module's chart declares a ConversionWebhook and renders
// the module to collect them. HasConversionWebhook gates the subtask; the subtasks
// themselves call GetConversionWebhooks.
type nelmI interface {
	HasConversionWebhook(path string) (bool, error)
	GetConversionWebhooks(ctx context.Context, namespace string, pkg nelm.Package) ([]manifest.Manifest, error)
}

// patcher applies the rendered webhooks to the cluster.
// Passed straight to the EnsureWebhooks subtasks.
type patcher interface {
	ExecuteOperations(ops []sdkpkg.PatchCollectorOperation) error
}

// task ensures the CRDs of every enabled module, then publishes the enabled set
// and the applied GVKs (capabilities) into global values. It is the global
// node's unit of work, enqueued whenever the scheduler schedules global; the
// scheduler holds every module behind global (canSchedule barrier), so this runs
// before any module and modules render against a complete capability set.
type task struct {
	modules []Module

	crd     crdService
	global  globalModule
	queue   queueService
	patcher patcher
	nelm    nelmI
	status  *status.Service

	logger *log.Logger
}

// NewTask creates a task that ensures CRDs for the given enabled modules,
// publishes the resulting capabilities, and ensures their conversion webhooks.
func NewTask(
	global globalModule,
	enabled []Module,
	crd crdService,
	nelm nelmI,
	patcher patcher,
	queueService queueService,
	status *status.Service,
	logger *log.Logger,
) queue.Task {
	return &task{
		modules: enabled,
		crd:     crd,
		global:  global,
		nelm:    nelm,
		patcher: patcher,
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
//
// Last, an EnsureWebhooks subtask is enqueued and waited on for every module whose
// chart declares a ConversionWebhook — after the publish, so each render sees the
// values it will be deployed with, and before the barrier releases, so a module's
// conversions are registered before its release applies the custom resources they
// convert. A module whose chart cannot be rendered retries forever and holds the
// barrier, exactly as a broken CRD does; gating on the chart scan keeps modules
// that ship no webhooks out of that path entirely.
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
		if err := t.crd.HasCRDs(pkg.GetPath()); err == nil {
			names = append(names, fmt.Sprintf("%s-crd", pkg.GetName()))
		}

		sub := taskensurecrd.NewTask(pkg, t.crd.Install, t.status, t.logger)
		t.queue.Enqueue(ctx, filepath.Join(pkg.GetName(), "crd"), sub, queue.WithWait(wg))
	}

	wg.Wait()

	t.global.SetEnabledModules(names)
	t.global.SetCapabilities(t.crd.GetManagedGVKs(names))

	// Conversion webhooks are rendered from each module's chart, so they are
	// enqueued only after the global values above are published — a render before
	// that would see the previous cycle's enabled set and capabilities. The wait
	// below keeps them ahead of every module's release: the barrier holds until
	// each module's webhooks are applied, or the run is cancelled.
	for _, pkg := range t.modules {
		hasWebhook, err := t.nelm.HasConversionWebhook(pkg.GetPath())
		if err != nil {
			// The scan only decides whether the render is worth it, so an unreadable
			// chart falls through to the subtask, which reports the real failure.
			t.logger.Warn("scan chart for conversion webhooks", slog.String("name", pkg.GetName()), log.Err(err))
		} else if !hasWebhook {
			continue
		}

		sub := taskensurewebhooks.NewTask(pkg, t.nelm, t.patcher, t.status, t.logger)
		t.queue.Enqueue(ctx, filepath.Join(pkg.GetName(), "webhooks"), sub, queue.WithWait(wg))
	}

	wg.Wait()

	return nil
}
