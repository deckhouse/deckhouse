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
	"fmt"
	"log/slog"
	"path/filepath"
	"slices"
	"strings"

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

	// embeddedModulesDir is the on-disk root of built-in (embedded) modules
	// shipped inside the deckhouse image. Modules under this path are loaded
	// with the embedded loader (no version-by-symlink resolution).
	embeddedModulesDir = "/deckhouse/modules"
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

// ProcessFunctionalModules consumes a functional-modules handoff signal from
// addon-operator. The signal arrives once all critical modules have finished
// converging in addon-operator and carries the names of the enabled functional
// (non-critical) modules that the new controller should now own.
//
// For each module it runs the adoption flow: the module is already loaded on the
// filesystem by addon-operator's module loader, so instead of re-downloading it
// the runtime loads it in place (embedded or downloaded layout), takes its
// settings from addon-operator's resolved config values, and registers it with
// the scheduler. The scheduler then drives the regular pipeline
// (ensureCRD -> configure -> enable -> run).
func (r *Runtime) ProcessFunctionalModules(names []string) {
	r.logger.Info("received functional modules handoff from addon-operator",
		slog.Int("count", len(names)),
		slog.Any("modules", names))

	desired := make(map[string]struct{}, len(names))
	for _, name := range names {
		desired[name] = struct{}{}
	}

	for _, name := range names {
		if err := r.adoptModule(context.Background(), name); err != nil {
			r.logger.Error("adopt functional module",
				slog.String("module", name),
				slog.Any("error", err))
		}
	}

	// The handoff list is authoritative for the set of enabled functional
	// modules. Any previously adopted module absent from the new list has been
	// disabled (or removed) in addon-operator, so tear it down here. Collect the
	// names under a read lock, then call RemoveModule outside it (it takes the
	// write lock itself).
	r.mu.RLock()
	var stale []string
	for name := range r.adopted {
		if _, ok := desired[name]; !ok {
			stale = append(stale, name)
		}
	}
	r.mu.RUnlock()

	for _, name := range stale {
		r.logger.Info("removing functional module no longer handed off by addon-operator",
			slog.String("module", name))
		r.RemoveModule(name)
	}
}

// adoptModule brings a module that is already present on the filesystem (loaded
// by addon-operator) into the new runtime without re-downloading it.
//
// It resolves the module's on-disk path and settings from the addon-operator
// module manager, loads the package definition in place (choosing the embedded
// or downloaded loader by path), registers a Store entry with the settings, adds
// the module to the scheduler, and lets the scheduler schedule the pipeline.
//
// Re-adoption is idempotent: if neither the loaded version nor the settings
// changed, it is a no-op; a settings-only change triggers a Reschedule.
func (r *Runtime) adoptModule(ctx context.Context, name string) error {
	ctx, span := otel.Tracer(runtimeTracer).Start(ctx, "adoptModule")
	defer span.End()

	span.SetAttributes(attribute.String("module", name))

	basic := r.addonModuleManager.GetModule(name)
	if basic == nil {
		return fmt.Errorf("module %q is not known to the module manager", name)
	}

	path := basic.GetPath()
	embedded := isEmbeddedPath(path)
	settings := addonutils.Values(basic.GetConfigValues(false))

	conf, err := r.loadModuleConfFromPath(ctx, path, embedded)
	if err != nil {
		span.SetStatus(codes.Error, err.Error())
		return status.NewError("LoadFailed", fmt.Errorf("load module conf from %q: %w", path, err))
	}

	module, err := modules.NewModuleByConfig(name, conf, r.logger)
	if err != nil {
		span.SetStatus(codes.Error, err.Error())
		return status.NewError("LoadFailed", fmt.Errorf("new module: %w", err))
	}

	version := module.GetVersion().String()

	r.mu.Lock()
	defer r.mu.Unlock()

	if !r.packages.NeedUpdate(name, version, settings.Checksum()) {
		return nil
	}

	rctx := r.packages.Update(name, version, settings)
	if rctx == nil {
		// settings-only change: the loaded instance stays, re-run the pipeline
		// so the new settings are applied through Configure.
		r.scheduler.Reschedule(name)
		return nil
	}

	r.status.NewStatus(name)

	// Optimistically register before AddNode so a successful schedule can
	// resolve it; roll back on a dependency cycle (mirror loadModule).
	r.modules[name] = module
	if err = r.scheduler.AddNode(module); err != nil {
		delete(r.modules, name)
		span.SetStatus(codes.Error, err.Error())
		return status.NewError("DependencyCycle", fmt.Errorf("add node: %w", err))
	}

	// Mark as adopted: its filesystem content is owned by addon-operator, so a
	// later RemoveModule must not undeploy/remove the package files.
	r.adopted[name] = struct{}{}

	r.logger.Info("adopted functional module",
		slog.String("module", name),
		slog.String("version", version),
		slog.Bool("embedded", embedded),
		slog.String("path", path))

	return nil
}

// loadModuleConfFromPath loads a module package config directly from an on-disk
// path, choosing the embedded loader (no version-by-symlink resolution) or the
// downloaded loader by path, and wires the shared runtime managers so the
// module's hooks can patch objects, schedule crons and watch Kubernetes events.
func (r *Runtime) loadModuleConfFromPath(ctx context.Context, path string, embedded bool) (*modules.Config, error) {
	var (
		conf *modules.Config
		err  error
	)

	if embedded {
		conf, err = loader.LoadEmbeddedConf(ctx, path, r.logger)
	} else {
		conf, err = loader.LoadModuleConf(ctx, path, r.logger)
	}
	if err != nil {
		return nil, err
	}

	conf.Patcher = r.objectPatcher
	conf.ScheduleManager = r.scheduleManager
	conf.KubeEventsManager = r.kubeEventsManager
	conf.GlobalValuesGetter = r.addonModuleManager.GetGlobal().GetValues

	return conf, nil
}

// isEmbeddedPath reports whether the module path points at a built-in (embedded)
// module shipped inside the deckhouse image rather than a downloaded one.
func isEmbeddedPath(path string) bool {
	return strings.HasPrefix(path, embeddedModulesDir)
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
	conf.GlobalValuesGetter = r.addonModuleManager.GetGlobal().GetValues

	module, err := modules.NewModuleByConfig(filepath.Base(packagePath), conf, r.logger)
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
//
// Adopted modules (loaded in place from addon-operator's filesystem layout via
// adoptModule) do not own their on-disk content, so the undeploy step is
// skipped for them: only hooks and the Helm release are torn down, and the
// in-memory bookkeeping is cleared once the disable task completes.
func (r *Runtime) RemoveModule(name string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.scheduler.RemoveNode(name)

	ctx := r.packages.HandleEvent(lifecycle.EventRemove, name)
	if ctx == nil {
		return
	}

	_, adopted := r.adopted[name]

	cleanup := queue.WithOnDone(func() {
		go func() {
			r.mu.Lock()
			defer r.mu.Unlock()

			if r.packages.Delete(name) {
				r.queueService.Remove(name)
				r.status.DeleteStatus(name)
				delete(r.modules, name)
				delete(r.adopted, name)
			}
		}()
	})

	// For adopted modules, disabling is the terminal step: tear down hooks and
	// the Helm release, then run cleanup. The filesystem content stays in place
	// (owned by addon-operator), so no undeploy task is enqueued.
	if adopted {
		if pkg := r.modules[name]; pkg != nil {
			r.queueService.Enqueue(ctx, name, taskdisable.NewTask(pkg, modulesNamespace, false, r.nelmService, r.queueService, r.logger), cleanup)
			return
		}

		// Nothing loaded to disable: drop the bookkeeping directly (we already
		// hold the lock) so the Store does not get stuck in the removing state.
		if r.packages.Delete(name) {
			r.queueService.Remove(name)
			r.status.DeleteStatus(name)
			delete(r.modules, name)
			delete(r.adopted, name)
		}
		return
	}

	if pkg := r.modules[name]; pkg != nil {
		r.queueService.Enqueue(ctx, name, taskdisable.NewTask(pkg, modulesNamespace, false, r.nelmService, r.queueService, r.logger))
	}

	r.queueService.Enqueue(ctx, name, taskundeploy.NewModuleTask(name, r.moduleDeployer, r.logger), cleanup)
}
