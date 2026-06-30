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

package runtime

import (
	"context"
	"slices"

	addonutils "github.com/flant/addon-operator/pkg/utils"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"

	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/loader"
	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/modules"
	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/runtime/lifecycle"
	taskdeploy "github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/runtime/tasks/deploy"
	taskdisable "github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/runtime/tasks/disable"
	taskload "github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/runtime/tasks/load"
	taskundeploy "github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/runtime/tasks/undeploy"
	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/status"
	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/queue"
	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/registry"
)

const (
	modulesNamespace = "d8-system"
)

// Module represents a module instance as received from the module controller.
// Unlike App, modules always run in the d8-system namespace.
type Module struct {
	Name       string
	Definition modules.Definition
	Settings   addonutils.Values
}

// UpdateModulesSettings applies a settings-and-enabled change to an
// already-tracked package without redeploying or reloading it. It is meant to be
// wired into the packages-config-controller, which owns package settings and the
// ModuleConfig enabled intent independently of the package version handled by
// UpdateModule. enabled is the tri-state user intent (*true/*false set by a
// ModuleConfig, nil when unset) consumed by the scheduler's config rule.
//
// Unlike UpdateModule, this never enqueues Deploy/Load tasks and never cancels
// the package's context tree: it only stashes the new pending settings and
// enabled intent and, if either actually changed, triggers Reschedule so the
// scheduler re-resolves the rule chain (re-evaluating the config rule) and, when
// the package stays enabled, re-runs the Configure → Startup → Run pipeline (see
// schedulePackage) with the new values. Any in-flight deploy or load for the
// package keeps running untouched.
//
// If the package is not tracked yet, the change is dropped: there is nothing to
// reschedule, and the eventual UpdateModule will register the package, which
// then picks up settings and enabled on the next config event.
func (r *Runtime) UpdateModulesSettings(name string, settings addonutils.Values, enabled *bool) {
	r.mu.Lock()
	defer r.mu.Unlock()

	// Settings live in the per-package store; the ModuleConfig enabled intent
	// lives in the global module (thread-safe for the scheduler's enabled getter).
	// Reschedule if either actually changed.
	settingsChanged := r.packages.UpdateSettings(name, settings)
	enabledChanged := r.global.SetConfigEnabled(name, enabled)

	if settingsChanged || enabledChanged {
		r.scheduler.Reschedule(name)
	}
}

// UpdateModule handles module creation and version changes from the module controller.
//
// Flow mirrors UpdateApp: version changes enqueue the full pipeline
// (Disable → Deploy → Load), settings-only changes trigger
// Reschedule to re-apply settings through the scheduler's schedule pipeline.
// See UpdateApp for detailed flow documentation.
func (r *Runtime) UpdateModule(repo registry.Remote, module Module) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if len(module.Settings) == 0 {
		module.Settings = make(addonutils.Values)
	}

	name := module.Name
	version := module.Definition.Version

	if !r.packages.NeedUpdate(name, version, module.Settings.Checksum()) {
		return
	}

	ctx := r.packages.Update(name, version, module.Settings)
	if ctx == nil {
		r.scheduler.Reschedule(name)
		return
	}

	r.status.NewStatus(name)

	tasks := []queue.Task{
		taskdeploy.NewModuleTask(name, version, repo, r.moduleDeployer, r.status, r.logger),
		taskload.NewModuleTask(name, repo, r.loadModule, r.status, r.logger),
	}

	// If there's an existing module, disable it first
	if pkg := r.modules[name]; pkg != nil {
		tasks = slices.Insert(tasks, 0, taskdisable.NewTask(pkg, modulesNamespace, true, r.nelmService, r.queueService, r.logger))
	}

	for _, task := range tasks {
		r.queueService.Enqueue(ctx, name, task)
	}
}

// loadModule builds a Module from its package files, stores it in r.modules,
// and registers it with the scheduler via AddNode. Called by the Load task
// after the package image is mounted on the filesystem.
func (r *Runtime) loadModule(ctx context.Context, repo registry.Remote, packagePath string) (string, error) {
	ctx, span := otel.Tracer(runtimeTracer).Start(ctx, "loadModule")
	defer span.End()

	span.SetAttributes(attribute.String("path", packagePath))
	span.SetAttributes(attribute.String("repository", repo.Name))

	conf, err := loader.LoadModuleConf(ctx, packagePath, r.logger)
	if err != nil {
		span.SetStatus(codes.Error, err.Error())
		return "", status.NewError("LoadFailed", err)
	}

	conf.Repository = repo
	conf.Patcher = r.objectPatcher
	conf.ScheduleManager = r.scheduleManager
	conf.KubeEventsManager = r.kubeEventsManager
	conf.GlobalValuesGetter = r.global.GetValues

	module, err := modules.NewModuleByConfig(conf.Definition.Name, conf, r.logger)
	if err != nil {
		span.SetStatus(codes.Error, err.Error())
		return "", status.NewError("LoadFailed", err)
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	// Optimistically register the module before AddNode so a successful
	// schedule can resolve it; if AddNode rejects the addition (dependency
	// cycle), roll back the map entry so we never expose a package the
	// scheduler never accepted.
	r.modules[module.GetName()] = module
	if err = r.scheduler.AddNode(module); err != nil {
		delete(r.modules, module.GetName())
		span.SetStatus(codes.Error, err.Error())
		return "", status.NewError("DependencyCycle", err)
	}

	return module.GetVersion().String(), nil
}

// RemoveModule removes a module and cancels all its running operations.
// After undeploy, a cleanup goroutine removes the Store entry and stops the queue.
// See RemoveApp for detailed rationale on the async cleanup pattern.
func (r *Runtime) RemoveModule(name string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.scheduler.RemoveNode(name)

	ctx := r.packages.HandleEvent(lifecycle.EventRemove, name)
	if ctx == nil {
		return
	}

	if pkg := r.modules[name]; pkg != nil {
		r.queueService.Enqueue(ctx, name, taskdisable.NewTask(pkg, modulesNamespace, false, r.nelmService, r.queueService, r.logger))
	}

	cleanup := queue.WithOnDone(func() {
		go func() {
			r.mu.Lock()
			defer r.mu.Unlock()

			if r.packages.Delete(name) {
				r.queueService.Remove(name)
				r.status.DeleteStatus(name)
				delete(r.modules, name)
			}
		}()
	})

	r.queueService.Enqueue(ctx, name, taskundeploy.NewModuleTask(name, r.moduleDeployer, r.logger), cleanup)
}
