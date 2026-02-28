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
	"path/filepath"
	"slices"

	addonutils "github.com/flant/addon-operator/pkg/utils"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"

	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/loader"
	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/modules"
	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/runtime/lifecycle"
	taskdisable "github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/runtime/tasks/disable"
	taskdownload "github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/runtime/tasks/download"
	taskinstall "github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/runtime/tasks/install"
	taskload "github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/runtime/tasks/load"
	taskuninstall "github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/runtime/tasks/uninstall"
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

// UpdateModule handles module creation and version changes from the module controller.
//
// Flow mirrors UpdateApp: version changes enqueue the full pipeline
// (Disable → Download → Install → Load), settings-only changes trigger
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

	if err := r.scheduler.CheckConstraints(module.Definition.Constraints()); err != nil {
		r.status.HandleError(name, err)
		return
	}

	r.status.ClearRuntimeConditions(name)
	r.status.SetConditionTrue(name, status.ConditionRequirementsMet)

	tasks := []queue.Task{
		taskdownload.NewModuleTask(name, version, repo, r.installer, r.status, r.logger),
		taskinstall.NewModuleTask(name, version, repo, r.installer, r.status, r.logger),
		taskload.NewModuleTask(name, repo, r.loadModule, r.status, r.logger),
	}

	// If there's an existing module, disable it first
	if pkg := r.modules[name]; pkg != nil {
		tasks = slices.Insert(tasks, 0, taskdisable.NewTask(pkg, modulesNamespace, true, r.nelmService, r.queueService, r.status, r.logger))
	}

	for _, task := range tasks {
		r.queueService.Enqueue(ctx, name, task)
	}
}

// loadModule builds a Module from its package files and stores it in r.modules.
// Called by the Load task after the package image is mounted on the filesystem.
// Does not apply settings or register with the scheduler — see loadApp for rationale.
func (r *Runtime) loadModule(ctx context.Context, repo registry.Remote, packagePath string) (string, error) {
	ctx, span := otel.Tracer(runtimeTracer).Start(ctx, "loadModule")
	defer span.End()

	span.SetAttributes(attribute.String("path", packagePath))
	span.SetAttributes(attribute.String("repository", repo.Name))

	conf, err := loader.LoadModuleConf(ctx, packagePath, r.logger)
	if err != nil {
		span.SetStatus(codes.Error, err.Error())
		return "", newLoadFailedErr(err)
	}

	conf.Repository = repo
	conf.Patcher = r.objectPatcher
	conf.ScheduleManager = r.scheduleManager
	conf.KubeEventsManager = r.kubeEventsManager
	conf.GlobalValuesGetter = r.addonModuleManager.GetGlobal().GetValues

	module, err := modules.NewModuleByConfig(filepath.Base(packagePath), conf, r.logger)
	if err != nil {
		span.SetStatus(codes.Error, err.Error())
		return "", newLoadFailedErr(err)
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	r.modules[module.GetName()] = module

	return module.GetVersion().String(), nil
}

// RemoveModule removes a module and cancels all its running operations.
// After uninstall, a cleanup goroutine removes the Store entry and stops the queue.
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
		r.queueService.Enqueue(ctx, name, taskdisable.NewTask(pkg, modulesNamespace, false, r.nelmService, r.queueService, r.status, r.logger))
	}

	cleanup := queue.WithOnDone(func() {
		go func() {
			r.mu.Lock()
			defer r.mu.Unlock()

			if r.packages.Delete(name) {
				r.queueService.Remove(name)
				r.status.Delete(name)
			}
		}()
	})

	r.queueService.Enqueue(ctx, name, taskuninstall.NewModuleTask(name, r.installer, r.logger), cleanup)
}
