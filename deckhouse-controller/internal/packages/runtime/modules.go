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
	"log/slog"
	"slices"

	addonutils "github.com/flant/addon-operator/pkg/utils"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"

	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/app"
	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/loader"
	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/modules"
	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/runtime/lifecycle"
	taskcleanup "github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/runtime/tasks/cleanup"
	taskdeploy "github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/runtime/tasks/deploy"
	taskdisable "github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/runtime/tasks/disable"
	taskload "github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/runtime/tasks/load"
	taskundeploy "github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/runtime/tasks/undeploy"
	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/status"
	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/queue"
	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/registry"
)

// Module represents a module instance as received from the module controller.
// Unlike App, modules always run in the d8-system namespace.
type Module struct {
	Name            string
	Definition      modules.Definition
	Settings        addonutils.Values
	SettingsVersion int // schema version from ModuleConfig.Spec.Version
	Maintenance     string
	Enabled         *bool
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
// Settings and the enabled intent diverge when the package is not tracked yet.
// The enabled intent is always recorded: it lives in the global module, which
// has no notion of tracking, so the scheduler's config rule sees the user intent
// the moment the package is registered. Pending settings, by contrast, are
// dropped — there is no per-package store to stash them in yet; the eventual
// UpdateModule registers the package and supplies its settings. Either way, an
// untracked package has no node to reschedule, so no Reschedule happens here.
func (r *Runtime) UpdateModulesSettings(name string, settingsVersion int, settings addonutils.Values, maintenance string, enabled *bool) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.logger.Debug("update module settings", slog.String("name", name))

	// Settings live in the per-package store; the ModuleConfig enabled intent
	// lives in the global module (thread-safe for the scheduler's enabled getter).
	// Reschedule if either actually changed.
	settingsChanged := r.packages.UpdateSettings(name, settingsVersion, settings, maintenance)
	enabledChanged := r.global.SetConfigEnabled(name, enabled)

	if settingsChanged || enabledChanged {
		r.scheduler.Reschedule(name)
	}
}

// UpdateModule handles module creation, version changes, and enabled intent from the module controller.
//
// Flow mirrors UpdateApp: version changes enqueue the full pipeline
// (Disable → Deploy → Load), settings-only changes trigger
// Reschedule to re-apply settings through the scheduler's schedule pipeline.
// See UpdateApp for detailed flow documentation.
//
// force runs the pipeline even when nothing the runtime tracks changed and makes the
// Deploy task discard the cached copy of the version. It is for callers that resolved the
// image digest and found it changed under a tag the runtime still sees as unchanged, and
// is transitional: it goes away once module tags are immutable.
func (r *Runtime) UpdateModule(repo registry.Remote, module Module, force bool) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.logger.Debug("update module", slog.String("name", module.Name), slog.Bool("force", force))

	if len(module.Settings) == 0 {
		module.Settings = make(addonutils.Values)
	}

	name := module.Name
	version := module.Definition.Version
	enabledChanged := r.global.SetConfigEnabled(name, module.Enabled)

	// A forced update skips change detection it would fail anyway.
	if !force && !r.packages.NeedUpdate(name, version, module.Settings.Checksum(), module.SettingsVersion, module.Maintenance) {
		if enabledChanged {
			r.scheduler.Reschedule(name)
		}

		return
	}

	ctx := r.packages.Update(name, version, module.SettingsVersion, module.Settings, module.Maintenance, force)
	if ctx == nil {
		r.scheduler.Reschedule(name)
		return
	}

	r.status.NewStatus(name)

	tasks := []queue.Task{
		taskdeploy.NewModuleTask(name, version, repo, force, r.moduleDeployer, r.status, r.logger),
		taskload.NewModuleTask(name, repo, r.loadModule, r.status, r.logger),
	}

	// If there's an existing module, disable it first
	if pkg := r.modules[name]; pkg != nil {
		tasks = slices.Insert(tasks, 0, taskdisable.NewTask(pkg, app.NamespaceDeckhouse, true, r.nelmService, r.queueService, r.logger))
	}

	for _, task := range tasks {
		r.queueService.Enqueue(ctx, name, task)
	}
}

// UpdateEmbeddedModule handles creation, settings and enabled intent of an embedded module —
// one shipped inside the Deckhouse image rather than pulled from a repository.
//
// The pipeline is UpdateModule's without the Deploy task: the files already sit under
// app.EmbeddedModulesDir, so ReadyOnFilesystem holds from the start and only Load runs.
// The version is the running edition's, because an embedded module carries no package
// version of its own, so it cannot change while the process lives — but EventRemove clears
// the stored version, so a delete-then-recreate still lands here with the previous instance
// registered, and Disable goes ahead of Load to tear it down.
//
// Settings-only and enabled-only changes behave as in UpdateModule: they stash the new
// values and Reschedule, so the scheduler re-runs Configure → Startup → Run with them.
func (r *Runtime) UpdateEmbeddedModule(module Module) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.logger.Debug("update embedded module", slog.String("name", module.Name))

	if len(module.Settings) == 0 {
		module.Settings = make(addonutils.Values)
	}

	name := module.Name
	version := r.edition.Version
	enabledChanged := r.global.SetConfigEnabled(name, module.Enabled)

	if !r.packages.NeedUpdate(name, version, module.Settings.Checksum(), module.SettingsVersion, module.Maintenance) {
		if enabledChanged {
			r.scheduler.Reschedule(name)
		}

		return
	}

	ctx := r.packages.Update(name, version, module.SettingsVersion, module.Settings, module.Maintenance, false)
	if ctx == nil {
		r.scheduler.Reschedule(name)
		return
	}

	r.status.NewStatus(name)

	// The image carries the module, so nothing has to place it on disk.
	r.status.SetConditionTrue(name, status.ConditionReadyOnFilesystem)

	tasks := []queue.Task{
		taskload.NewEmbeddedTask(name, r.loadEmbeddedModule, r.status, r.logger),
	}

	// If there's an existing module, disable it first
	if pkg := r.modules[name]; pkg != nil {
		tasks = slices.Insert(tasks, 0, taskdisable.NewTask(pkg, app.NamespaceDeckhouse, true, r.nelmService, r.queueService, r.logger))
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

	module, err := r.registerModule(ctx, conf)
	if err != nil {
		span.SetStatus(codes.Error, err.Error())
		return "", err
	}

	return module.GetVersion().String(), nil
}

// loadEmbeddedModule builds a Module from an embedded package directory and registers it,
// as loadModule does for a downloaded one. The definition's version is overwritten with
// the running edition's, and the repository the Load task passes is empty — an embedded
// module has none, so no registry values are injected.
func (r *Runtime) loadEmbeddedModule(ctx context.Context, _ registry.Remote, packagePath string) (string, error) {
	ctx, span := otel.Tracer(runtimeTracer).Start(ctx, "loadEmbeddedModule")
	defer span.End()

	span.SetAttributes(attribute.String("path", packagePath))

	conf, err := loader.LoadEmbeddedConf(ctx, packagePath, r.logger)
	if err != nil {
		span.SetStatus(codes.Error, err.Error())
		return "", status.NewError("LoadFailed", err)
	}

	conf.Definition.Version = r.edition.Version

	module, err := r.registerModule(ctx, conf)
	if err != nil {
		span.SetStatus(codes.Error, err.Error())
		return "", err
	}

	return module.GetVersion().String(), nil
}

// registerModule wires the runtime's shared managers into conf, builds the module and
// publishes it to r.modules and the scheduler. Returns a status error, so both loaders
// pass it straight to the Load task's condition.
func (r *Runtime) registerModule(ctx context.Context, conf *modules.Config) (*modules.Module, error) {
	conf.Patcher = r.objectPatcher
	conf.ScheduleManager = r.scheduleManager
	conf.KubeEventsManager = r.kubeEventsManager
	conf.GlobalValuesGetter = r.global.GetValues

	module, err := modules.NewModuleByConfig(conf.Definition.Name, conf, r.logger)
	if err != nil {
		return nil, status.NewError("LoadFailed", err)
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	// The package was removed while this Load ran — r.mu is what serialises the two, so this is the
	// last point either can win. Publishing now would give the scheduler a node for a package nothing
	// tracks, and Enable would then register its hooks with the shared managers with no removal path
	// left to disable them.
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	// Optimistically register the module before AddNode so a successful
	// schedule can resolve it; if AddNode rejects the addition (dependency
	// cycle), roll back the map entry so we never expose a package the
	// scheduler never accepted.
	r.modules[module.GetName()] = module
	if err = r.scheduler.AddNode(module); err != nil {
		delete(r.modules, module.GetName())
		return nil, status.NewError("DependencyCycle", err)
	}

	return module, nil
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
		r.queueService.Enqueue(ctx, name, taskdisable.NewTask(pkg, app.NamespaceDeckhouse, false, r.nelmService, r.queueService, r.logger))
	}

	cleanup := queue.WithOnDone(r.cleanupModule(name))

	r.queueService.Enqueue(ctx, name, taskundeploy.NewModuleTask(name, r.moduleDeployer, r.logger), cleanup)
}

// RemoveEmbeddedModule removes an embedded module and cancels all its running operations.
// It is RemoveModule without Undeploy: the image carries the files, so nothing was ever placed
// on disk for the deployer to take back. The cleanup therefore rides on Disable, or runs on its
// own when the module never loaded and there is nothing to disable.
func (r *Runtime) RemoveEmbeddedModule(name string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.scheduler.RemoveNode(name)

	ctx := r.packages.HandleEvent(lifecycle.EventRemove, name)
	if ctx == nil {
		return
	}

	if pkg := r.modules[name]; pkg != nil {
		r.queueService.Enqueue(ctx, name, taskdisable.NewTask(pkg, app.NamespaceDeckhouse, false, r.nelmService, r.queueService, r.logger))
	}

	// The teardown rides the last task in the package's queue, never runs inline: it stops that queue
	// and waits up to 10s for it to drain, so from here — under r.mu, with a Load possibly still
	// running and about to want r.mu itself — it would deadlock both. RemoveModule anchors it on
	// Undeploy; an embedded module has nothing to undeploy, so it anchors on a barrier.
	r.queueService.Enqueue(ctx, name, taskcleanup.NewTask(name, r.logger), queue.WithOnDone(r.cleanupModule(name)))
}

// cleanupModule returns the teardown that drops the Store entry, stops the queue and deletes the
// status once a removal's last task is done. It takes r.mu, so it never runs under the caller's.
func (r *Runtime) cleanupModule(name string) func() {
	return func() {
		go func() {
			r.mu.Lock()
			defer r.mu.Unlock()

			if r.packages.Delete(name) {
				r.queueService.Remove(name)
				r.status.DeleteStatus(name)
				delete(r.modules, name)
			}
		}()
	}
}
