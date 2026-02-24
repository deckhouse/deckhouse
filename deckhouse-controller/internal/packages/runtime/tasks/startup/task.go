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

package startup

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sync"

	bctx "github.com/flant/shell-operator/pkg/hook/binding_context"
	hookcontroller "github.com/flant/shell-operator/pkg/hook/controller"
	shtypes "github.com/flant/shell-operator/pkg/hook/types"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/hooks"
	taskhooksync "github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/runtime/tasks/hooksync"
	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/status"
	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/queue"
	"github.com/deckhouse/deckhouse/pkg/log"
)

const (
	taskTracer = "package-startup"
)

// packageI abstracts package operations needed for startup.
type packageI interface {
	GetName() string
	GetValuesChecksum() string
	NeedStartup() bool
	// InitializeHooks creates hook controllers and binds them to events.
	InitializeHooks()
	GetHooksByBinding(binding shtypes.BindingType) []hooks.Hook
	RunHooksByBinding(ctx context.Context, binding shtypes.BindingType) error
	RunHookByName(ctx context.Context, hook string, bctx []bctx.BindingContext) error
	// UnlockKubernetesMonitors allows events to flow after initial sync.
	UnlockKubernetesMonitors(hook string, monitors ...string)
}

// nelmI abstracts Helm monitor control during startup.
type nelmI interface {
	// PauseMonitor temporarily stops Helm release monitoring.
	PauseMonitor(name string)
	// ResumeMonitor restarts Helm release monitoring after startup.
	ResumeMonitor(name string)
}

// statusService provides condition updates and error handling.
type statusService interface {
	SetConditionTrue(name string, cond status.ConditionType)
	HandleError(name string, err error)
}

// queueService allows enqueuing hook sync tasks to separate queues.
type queueService interface {
	Enqueue(ctx context.Context, name string, task queue.Task, opts ...queue.EnqueueOption)
}

// task initializes a loaded package by enabling hooks and running startup sequence.
// Follows the Load task and precedes the Run task in the lifecycle.
type task struct {
	pkg packageI

	nelm   nelmI
	queue  queueService
	status statusService

	logger *log.Logger
}

// NewTask creates a startup task that will initialize hooks and run OnStartup bindings.
func NewTask(pkg packageI, nelm nelmI, queueService queueService, status statusService, logger *log.Logger) queue.Task {
	return &task{
		pkg:    pkg,
		nelm:   nelm,
		queue:  queueService,
		status: status,
		logger: logger.Named(taskTracer).With(slog.String("name", pkg.GetName())),
	}
}

func (t *task) String() string {
	return "Startup"
}

// Execute performs the three-step startup sequence:
// 1. Initialize hooks - enable Kubernetes monitors and schedule bindings
// 2. Synchronize - run initial sync for hooks with WaitForSynchronization
// 3. Run OnStartup hooks - execute startup bindings before normal operation
func (t *task) Execute(ctx context.Context) error {
	// skip if package already running
	if !t.pkg.NeedStartup() {
		return nil
	}

	t.logger.Debug("startup package")

	t.status.HandleError(t.pkg.GetName(), &status.Error{
		Err: errors.New("startup package"),
		Conditions: []status.Condition{
			{
				Type:   status.ConditionWaitConverge,
				Status: metav1.ConditionFalse,
			},
		},
	})

	// Step 1: Enable kubernetes/schedule hooks - registers watchers and cron schedules
	infos, err := t.initializeHooks(ctx)
	if err != nil {
		t.status.HandleError(t.pkg.GetName(), err)

		return fmt.Errorf("initialize hooks: %w", err)
	}

	// Step 2: Enqueue hook synchronization tasks
	// For each hook binding, we need to:
	// - executePlan initial synchronization if ExecuteHookOnSynchronization=true
	// - Unlock monitors to allow future events to trigger the hook
	// - If WaitForSynchronization=true, block until sync completes
	t.logger.Debug("wait for sync tasks to finish")
	wg := new(sync.WaitGroup)
	for hook, info := range infos {
		for _, hookInfo := range info {
			syncTask := taskhooksync.NewTask(t.pkg, hook, hookInfo, t.nelm, t.status, t.logger)

			// queue = <name>/<queue>
			queueName := fmt.Sprintf("%s/%s", t.pkg.GetName(), hookInfo.QueueName)

			if hookInfo.KubernetesBinding.WaitForSynchronization {
				queueName = fmt.Sprintf("%s/sync", queueName)
				// Add to WaitGroup - we'll block until this completes
				t.queue.Enqueue(ctx, queueName, syncTask, queue.WithWait(wg))
				continue
			}

			// Non-blocking sync - don't wait for completion
			t.queue.Enqueue(ctx, queueName, syncTask)
		}
	}
	// Block until all WaitForSynchronization hooks complete
	// This ensures critical hooks run before startup hooks
	wg.Wait()

	// Step 3: Run package startup hooks (onStartup binding)
	if err = t.startupPackage(ctx); err != nil {
		t.status.HandleError(t.pkg.GetName(), err)
		return fmt.Errorf("startup package: %w", err)
	}

	return nil
}

// initializeHooks initializes hook controllers and returns info for sync tasks.
//
// This must be called after LoadApplication and before StartupPackage.
// It performs:
//  1. Creates hook controllers for each hook
//  2. Initializes Kubernetes event bindings
//  3. Initializes schedule bindings
//  4. Enables kube hooks(starting monitoring for the resources configured in each hook's bindings)
//  5. Enable schedule hooks(activating the cron schedules configured in each hook's bindings)
func (t *task) initializeHooks(ctx context.Context) (map[string][]hookcontroller.BindingExecutionInfo, error) {
	ctx, span := otel.Tracer(taskTracer).Start(ctx, "InitializeHooks")
	defer span.End()

	// Initialize hook controllers and bind them to Kubernetes events and schedules
	t.logger.Debug("initialize package hooks")
	t.pkg.InitializeHooks()

	t.logger.Debug("enable schedule hooks")

	schHooks := t.pkg.GetHooksByBinding(shtypes.Schedule)
	for _, hook := range schHooks {
		t.logger.Debug("enable schedule hook", slog.String("hook", hook.GetName()))
		hook.GetHookController().EnableScheduleBindings()
	}

	t.logger.Debug("enable kubernetes hooks")

	// Collect synchronization tasks for Kubernetes hooks.
	// When enabling a Kubernetes hook, it needs to synchronize its initial state
	// by fetching existing resources matching its watch criteria. This returns
	// BindingExecutionInfo for each resource that needs to be processed during sync.
	res := make(map[string][]hookcontroller.BindingExecutionInfo)
	for _, hook := range t.pkg.GetHooksByBinding(shtypes.OnKubernetesEvent) {
		t.logger.Debug("enable kube hook", slog.String("hook", hook.GetName()))
		hookCtrl := hook.GetHookController()
		// HandleEnableKubernetesBindings starts watching resources and calls the callback
		// for each existing resource that matches the watch criteria (initial synchronization).
		err := hookCtrl.HandleEnableKubernetesBindings(ctx, func(info hookcontroller.BindingExecutionInfo) {
			res[hook.GetName()] = append(res[hook.GetName()], info)
		})
		if err != nil {
			span.SetStatus(codes.Error, err.Error())
			return nil, newInitHooksErr(err)
		}
	}

	return res, nil
}

// startupPackage runs OnStartup hooks for a package.
// This must be called after InitializeHooks and before RunPackage.
func (t *task) startupPackage(ctx context.Context) error {
	ctx, span := otel.Tracer(taskTracer).Start(ctx, "StartupPackage")
	defer span.End()

	span.SetAttributes(attribute.String("name", t.pkg.GetName()))

	t.logger.Debug("run startup hooks")
	if err := t.pkg.RunHooksByBinding(ctx, shtypes.OnStartup); err != nil {
		span.SetStatus(codes.Error, err.Error())
		return newStartupHookErr(err)
	}

	return nil
}
