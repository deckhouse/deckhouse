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
	"fmt"
	"log/slog"

	bindingcontext "github.com/flant/shell-operator/pkg/hook/binding_context"
	hookcontroller "github.com/flant/shell-operator/pkg/hook/controller"
	shtypes "github.com/flant/shell-operator/pkg/hook/types"
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
func (m *Manager) RunPackageHook(ctx context.Context, name, hook string, bctx []bindingcontext.BindingContext) (bool, error) {
	ctx, span := otel.Tracer(managerTracer).Start(ctx, "RunPackageHook")
	defer span.End()

	span.SetAttributes(attribute.String("name", name))
	span.SetAttributes(attribute.String("hook", hook))

	m.logger.Debug("run package hook", slog.String("hook", hook), slog.String("name", name))

	// TODO(ipaqsa): how to work with parallel hooks?
	// t.dc.HelmResourcesManager().PauseMonitor(t.name)
	// defer t.dc.HelmResourcesManager().ResumeMonitor(t.name)

	app, err := m.getApp(name)
	if err != nil {
		span.SetStatus(codes.Error, err.Error())
		return false, err
	}

	// Track if values changed during hook execution
	oldChecksum := app.GetValuesChecksum()
	if err = app.RunHookByName(ctx, hook, bctx, m.dc); err != nil {
		span.SetStatus(codes.Error, err.Error())
		return false, err
	}

	return oldChecksum != app.GetValuesChecksum(), nil
}

// EnableKubernetesHooks enables all Kubernetes event hooks for a package and return info for sync tasks.
// It starts monitoring for the resources configured in each hook's bindings.
func (m *Manager) EnableKubernetesHooks(ctx context.Context, name string) (map[string]hookcontroller.BindingExecutionInfo, error) {
	ctx, span := otel.Tracer(managerTracer).Start(ctx, "EnableKubernetesHooks")
	defer span.End()

	span.SetAttributes(attribute.String("name", name))

	app, err := m.getApp(name)
	if err != nil {
		span.SetStatus(codes.Error, err.Error())
		return nil, err
	}

	res := make(map[string]hookcontroller.BindingExecutionInfo)
	for _, hook := range app.GetHooksByBinding(shtypes.OnKubernetesEvent) {
		hookCtrl := hook.GetHookController()
		if err = hookCtrl.HandleEnableKubernetesBindings(ctx, func(info hookcontroller.BindingExecutionInfo) {
			res[name] = info
		}); err != nil {
			span.SetStatus(codes.Error, err.Error())
			return nil, fmt.Errorf("enable kubernetes bindings: %w", err)
		}
	}

	return res, nil
}

// EnableScheduleHooks enables all schedule-based hooks for a package.
// Schedule hooks run at specified intervals (cron-like scheduling).
// This activates the cron schedules configured in each hook's bindings.
func (m *Manager) EnableScheduleHooks(ctx context.Context, name string) error {
	_, span := otel.Tracer(managerTracer).Start(ctx, "EnableScheduleHooks")
	defer span.End()

	span.SetAttributes(attribute.String("name", name))

	app, err := m.getApp(name)
	if err != nil {
		span.SetStatus(codes.Error, err.Error())
		return err
	}

	schHooks := app.GetHooksByBinding(shtypes.Schedule)
	for _, hook := range schHooks {
		hook.GetHookController().EnableScheduleBindings()
	}

	return nil
}

// TaskBuilder used to create task from event to process
type TaskBuilder func(ctx context.Context, name, hook string, info hookcontroller.BindingExecutionInfo) (string, queue.Task)

// BuildKubeTasks is called at kube event and creates tasks to process
func (m *Manager) BuildKubeTasks(ctx context.Context, kubeEvent shkubetypes.KubeEvent, builder TaskBuilder) map[string][]queue.Task {
	res := make(map[string][]queue.Task)

	for _, app := range m.apps {
		for _, hook := range app.GetHooksByBinding(shtypes.OnKubernetesEvent) {
			hookCtrl := hook.GetHookController()

			// Handle module hooks
			if !hookCtrl.CanHandleKubeEvent(kubeEvent) {
				return nil
			}

			hookCtrl.HandleKubeEvent(ctx, kubeEvent, func(info hookcontroller.BindingExecutionInfo) {
				q, t := builder(ctx, app.GetName(), hook.GetName(), info)
				res[q] = append(res[q], t)
			})
		}
	}

	return res
}

// BuildScheduleTasks is called at schedule event and creates tasks to process
func (m *Manager) BuildScheduleTasks(ctx context.Context, crontab string, builder TaskBuilder) map[string][]queue.Task {
	res := make(map[string][]queue.Task)

	for _, app := range m.apps {
		for _, hook := range app.GetHooksByBinding(shtypes.OnKubernetesEvent) {
			hookCtrl := hook.GetHookController()

			// Handle module hooks
			if !hookCtrl.CanHandleScheduleEvent(crontab) {
				return nil
			}

			hookCtrl.HandleScheduleEvent(ctx, crontab, func(info hookcontroller.BindingExecutionInfo) {
				q, t := builder(ctx, app.GetName(), hook.GetName(), info)
				res[q] = append(res[q], t)
			})
		}
	}

	return res
}
