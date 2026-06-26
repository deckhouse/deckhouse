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

package runtime

import (
	"context"
	"fmt"
	"log/slog"

	bctx "github.com/flant/shell-operator/pkg/hook/binding_context"
	hookcontroller "github.com/flant/shell-operator/pkg/hook/controller"
	shtypes "github.com/flant/shell-operator/pkg/hook/types"
	shkubetypes "github.com/flant/shell-operator/pkg/kube_events_manager/types"

	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/hooks"
	taskhookrun "github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/runtime/tasks/hookrun"
	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/queue"
)

// eventSource is a loaded package whose hooks can be triggered by live Kubernetes
// or schedule events: it exposes its hooks for routing and the operations the
// hookrun task needs to execute them. Both applications and the global module
// satisfy it.
type eventSource interface {
	GetName() string
	GetValuesChecksum() string
	RunHookByName(ctx context.Context, hook string, bctx []bctx.BindingContext) error
	GetHooksByBinding(binding shtypes.BindingType) []hooks.ControllableHook
}

// BuildKubeTasks converts a Kubernetes event into executable tasks for all matching hooks.
//
// For each package (applications and the global module):
//  1. Find hooks that are bound to Kubernetes events
//  2. Check if the hook can handle this specific event (filtering)
//  3. Generate tasks for matching hooks using the provided builder
//
// Returns a map of queue names to tasks, allowing different hooks to be routed
// to different queues (e.g., priority queues, sequential queues).
func (r *Runtime) BuildKubeTasks(ctx context.Context, kubeEvent shkubetypes.KubeEvent) map[string][]queue.Task {
	res := make(map[string][]queue.Task)

	r.mu.RLock()
	defer r.mu.RUnlock()

	for _, app := range r.apps {
		r.routeKubeEvent(ctx, app, kubeEvent, res)
	}
	r.routeKubeEvent(ctx, r.global, kubeEvent, res)

	return res
}

// routeKubeEvent appends a hookrun task to res for every Kubernetes hook of src
// that matches the event.
func (r *Runtime) routeKubeEvent(ctx context.Context, src eventSource, kubeEvent shkubetypes.KubeEvent, res map[string][]queue.Task) {
	for _, hook := range src.GetHooksByBinding(shtypes.OnKubernetesEvent) {
		hookCtrl := hook.GetHookController()

		// Check if this hook's binding criteria match the incoming event
		// (e.g., resource type, namespace, labels, event type)
		if !hookCtrl.CanHandleKubeEvent(kubeEvent) {
			r.logger.Debug("skip kube hook",
				slog.String("hook", hook.GetName()),
				slog.String("name", src.GetName()),
				slog.String("monitor", kubeEvent.MonitorId),
				slog.String("event", kubeEvent.String()))
			continue
		}

		// Process the event and generate tasks via the builder callback
		hookCtrl.HandleKubeEvent(ctx, kubeEvent, func(info hookcontroller.BindingExecutionInfo) {
			r.logger.Debug("create task by kube event",
				slog.String("hook", hook.GetName()),
				slog.String("name", src.GetName()),
				slog.String("event", kubeEvent.String()))

			queueName := fmt.Sprintf("%s/%s", src.GetName(), info.QueueName)
			t := taskhookrun.NewTask(src, hook.GetName(), info.BindingContext, r.scheduler.Reschedule, r.nelmService, r.status, r.logger)
			res[queueName] = append(res[queueName], t)
		})
	}
}

// BuildScheduleTasks converts a schedule (cron) event into executable tasks for all matching hooks.
//
// For each package (applications and the global module):
//  1. Find hooks that are bound to schedule events
//  2. Check if the hook's schedule matches the triggered crontab
//  3. Generate tasks for matching hooks using the provided builder
//
// Returns a map of queue names to tasks, allowing hooks to specify their execution queue.
func (r *Runtime) BuildScheduleTasks(ctx context.Context, crontab string) map[string][]queue.Task {
	res := make(map[string][]queue.Task)

	r.mu.RLock()
	defer r.mu.RUnlock()

	for _, app := range r.apps {
		r.routeScheduleEvent(ctx, app, crontab, res)
	}
	r.routeScheduleEvent(ctx, r.global, crontab, res)

	return res
}

// routeScheduleEvent appends a hookrun task to res for every schedule hook of src
// whose crontab matches.
func (r *Runtime) routeScheduleEvent(ctx context.Context, src eventSource, crontab string, res map[string][]queue.Task) {
	for _, hook := range src.GetHooksByBinding(shtypes.Schedule) {
		hookCtrl := hook.GetHookController()

		// Check if this hook's cron schedule matches the triggered event
		if !hookCtrl.CanHandleScheduleEvent(crontab) {
			r.logger.Debug("skip schedule hook",
				slog.String("hook", hook.GetName()),
				slog.String("name", src.GetName()),
				slog.String("crontab", crontab))
			continue
		}

		// Process the schedule event and generate tasks via the builder callback
		hookCtrl.HandleScheduleEvent(ctx, crontab, func(info hookcontroller.BindingExecutionInfo) {
			r.logger.Debug("create task by schedule event",
				slog.String("hook", hook.GetName()),
				slog.String("name", src.GetName()),
				slog.String("event", crontab))

			// queue = <name>/<queue>
			queueName := fmt.Sprintf("%s/%s", src.GetName(), info.QueueName)
			t := taskhookrun.NewTask(src, hook.GetName(), info.BindingContext, r.scheduler.Reschedule, r.nelmService, r.status, r.logger)

			res[queueName] = append(res[queueName], t)
		})
	}
}
