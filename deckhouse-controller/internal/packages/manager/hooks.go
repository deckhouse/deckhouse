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

package manager

import (
	"context"
	"log/slog"
	"os"

	bindingcontext "github.com/flant/shell-operator/pkg/hook/binding_context"
	hookcontroller "github.com/flant/shell-operator/pkg/hook/controller"
	shtypes "github.com/flant/shell-operator/pkg/hook/types"
	objectpatch "github.com/flant/shell-operator/pkg/kube/object_patch"
	shkubetypes "github.com/flant/shell-operator/pkg/kube_events_manager/types"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"

	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/queue"
)

// RunPackageHook executes a specific hook by name with the provided binding context.
//
// This is called when:
//   - Kubernetes events trigger a hook (resource created/updated/deleted)
//   - Schedule triggers fire (cron-like schedules)
//
// Returns:
//   - bool: true if hook modified values (may require Helm upgrade)
//   - error: if hook execution fails
func (m *Manager) RunPackageHook(ctx context.Context, name, hook string, bctx []bindingcontext.BindingContext) error {
	ctx, span := otel.Tracer(managerTracer).Start(ctx, "RunPackageHook")
	defer span.End()

	span.SetAttributes(attribute.String("name", name))
	span.SetAttributes(attribute.String("hook", hook))

	m.logger.Debug("run package hook", slog.String("hook", hook), slog.String("name", name))

	m.mu.Lock()
	app := m.apps[name]
	m.mu.Unlock()
	if app == nil {
		// package can be disabled and removed before
		return nil
	}

	// Pause NELM monitoring during hook execution to prevent race conditions.
	// Hooks may modify resources that the monitor is tracking, which could trigger
	// false-positive alerts or inconsistent state. Resume monitoring after completion.
	m.nelm.PauseMonitor(name)
	defer m.nelm.ResumeMonitor(name)

	// Track if values changed during hook execution.
	// If the checksum differs after hook execution, it indicates that the hook
	// modified application values (via patches), which may require a Helm upgrade
	// to reconcile the cluster state with the new values.
	oldChecksum := app.GetValuesChecksum()
	if err := app.RunHookByName(ctx, hook, bctx, m); err != nil {
		span.SetStatus(codes.Error, err.Error())
		return err
	}

	if m.onValuesChanged != nil && oldChecksum != app.GetValuesChecksum() {
		m.logger.Debug("values changed during the hook", slog.String("hook", hook), slog.String("name", name))
		m.onValuesChanged(name)
	}

	return nil
}

// InitializeHooks initializes hook controllers and returns info for sync tasks.
//
// This must be called after LoadApplication and before StartupPackage.
// It performs:
//  1. Creates hook controllers for each hook
//  2. Initializes Kubernetes event bindings
//  3. Initializes schedule bindings
//  4. Enables kube hooks(starting monitoring for the resources configured in each hook's bindings)
//  5. Enable schedule hooks(activating the cron schedules configured in each hook's bindings)
func (m *Manager) InitializeHooks(ctx context.Context, name string) (map[string][]hookcontroller.BindingExecutionInfo, error) {
	ctx, span := otel.Tracer(managerTracer).Start(ctx, "InitializeHooks")
	defer span.End()

	m.mu.Lock()
	defer m.mu.Unlock()

	app := m.apps[name]
	if app == nil {
		// package can be disabled and removed before
		return nil, nil
	}

	m.logger.Debug("initialize hooks", slog.String("name", name))

	// Initialize hook controllers and bind them to Kubernetes events and schedules
	for _, hook := range app.GetHooks() {
		hookCtrl := hookcontroller.NewHookController()
		hookCtrl.InitKubernetesBindings(hook.GetHookConfig().OnKubernetesEvents, m.kubeEventsManager, m.logger)
		hookCtrl.InitScheduleBindings(hook.GetHookConfig().Schedules, m.scheduleManager)

		hook.WithHookController(hookCtrl)
		hook.WithTmpDir(os.TempDir())
	}

	m.logger.Debug("enable schedule hooks", slog.String("name", name))

	schHooks := app.GetHooksByBinding(shtypes.Schedule)
	for _, hook := range schHooks {
		m.logger.Debug("enable schedule hook", slog.String("hook", hook.GetName()), slog.String("name", name))
		hook.GetHookController().EnableScheduleBindings()
	}

	m.logger.Debug("enable kubernetes hooks", slog.String("name", name))

	// Collect synchronization tasks for Kubernetes hooks.
	// When enabling a Kubernetes hook, it needs to synchronize its initial state
	// by fetching existing resources matching its watch criteria. This returns
	// BindingExecutionInfo for each resource that needs to be processed during sync.
	res := make(map[string][]hookcontroller.BindingExecutionInfo)
	for _, hook := range app.GetHooksByBinding(shtypes.OnKubernetesEvent) {
		m.logger.Debug("enable kube hook", slog.String("hook", hook.GetName()), slog.String("name", name))
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

// TaskBuilder is a function that converts a hook binding execution into a queue task.
// It returns:
//   - string: the queue name to enqueue the task into (allows routing to different queues)
//   - queue.Task: the task to be executed
//
// The builder is provided by the caller (typically the event handler) to customize
// task creation based on the specific execution context and requirements.
type TaskBuilder func(ctx context.Context, name, hook string, info hookcontroller.BindingExecutionInfo) (string, queue.Task)

// BuildKubeTasks converts a Kubernetes event into executable tasks for all matching hooks.
//
// For each application:
//  1. Find hooks that are bound to Kubernetes events
//  2. Check if the hook can handle this specific event (filtering)
//  3. Generate tasks for matching hooks using the provided builder
//
// Returns a map of queue names to tasks, allowing different hooks to be routed
// to different queues (e.g., priority queues, sequential queues).
func (m *Manager) BuildKubeTasks(ctx context.Context, kubeEvent shkubetypes.KubeEvent, builder TaskBuilder) map[string][]queue.Task {
	res := make(map[string][]queue.Task)

	m.mu.Lock()
	defer m.mu.Unlock()

	for _, app := range m.apps {
		for _, hook := range app.GetHooksByBinding(shtypes.OnKubernetesEvent) {
			hookCtrl := hook.GetHookController()

			// Check if this hook's binding criteria match the incoming event
			// (e.g., resource type, namespace, labels, event type)
			if !hookCtrl.CanHandleKubeEvent(kubeEvent) {
				m.logger.Debug("skip kube hook",
					slog.String("hook", hook.GetName()),
					slog.String("name", app.GetName()),
					slog.String("monitor", kubeEvent.MonitorId),
					slog.String("event", kubeEvent.String()))
				continue
			}

			// Process the event and generate tasks via the builder callback
			hookCtrl.HandleKubeEvent(ctx, kubeEvent, func(info hookcontroller.BindingExecutionInfo) {
				q, t := builder(ctx, app.GetName(), hook.GetName(), info)
				res[q] = append(res[q], t)
			})
		}
	}

	return res
}

// BuildScheduleTasks converts a schedule (cron) event into executable tasks for all matching hooks.
//
// For each application:
//  1. Find hooks that are bound to schedule events
//  2. Check if the hook's schedule matches the triggered crontab
//  3. Generate tasks for matching hooks using the provided builder
//
// Returns a map of queue names to tasks, allowing hooks to specify their execution queue.
func (m *Manager) BuildScheduleTasks(ctx context.Context, crontab string, builder TaskBuilder) map[string][]queue.Task {
	res := make(map[string][]queue.Task)

	m.mu.Lock()
	defer m.mu.Unlock()

	for _, app := range m.apps {
		for _, hook := range app.GetHooksByBinding(shtypes.Schedule) {
			hookCtrl := hook.GetHookController()

			// Check if this hook's cron schedule matches the triggered event
			if !hookCtrl.CanHandleScheduleEvent(crontab) {
				m.logger.Debug("skip schedule hook",
					slog.String("hook", hook.GetName()),
					slog.String("name", app.GetName()),
					slog.String("crontab", crontab))
				continue
			}

			// Process the schedule event and generate tasks via the builder callback
			hookCtrl.HandleScheduleEvent(ctx, crontab, func(info hookcontroller.BindingExecutionInfo) {
				q, t := builder(ctx, app.GetName(), hook.GetName(), info)
				res[q] = append(res[q], t)
			})
		}
	}

	return res
}

// KubeObjectPatcher returns the Kubernetes object patcher for applying patches from hooks.
//
// This implements the DependencyContainer interface required by hook execution.
// Hooks can request object patching operations (create/update/delete K8s resources)
// during their execution, which are applied through this patcher.
func (m *Manager) KubeObjectPatcher() *objectpatch.ObjectPatcher {
	return m.kubeObjectPatcher
}
