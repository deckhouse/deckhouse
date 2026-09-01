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
	"errors"
	"log/slog"
	"slices"
	"sync"
	"time"

	addonutils "github.com/flant/addon-operator/pkg/utils"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"

	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/app"
	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/loader"
	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/modules"
	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/runtime/lifecycle"
	taskdeploy "github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/runtime/tasks/deploy"
	taskdisable "github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/runtime/tasks/disable"
	taskdummy "github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/runtime/tasks/dummy"
	taskload "github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/runtime/tasks/load"
	taskundeploy "github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/runtime/tasks/undeploy"
	taskuninstall "github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/runtime/tasks/uninstall"
	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/status"
	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/queue"
	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/registry"
)

// loadModulesTimeout bounds the bootstrap barrier, which waits on a queue that retries forever.
const loadModulesTimeout = 10 * time.Minute

// Module represents a module instance as received from the module controller.
// Unlike App, modules always run in the d8-system namespace.
type Module struct {
	Name            string
	Definition      modules.Definition
	Settings        addonutils.Values
	SettingsVersion int // schema version from ModuleConfig.Spec.Version
	Maintenance     string
	Enabled         *bool
	Repository      registry.Remote
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

// UpdateGlobalSettings applies a settings change to the global module, whose settings a
// Module carries like every other package's.
//
// Global is the one package the runtime builds itself: loadGlobal read its files out of the
// global hooks dir at startup, so nothing is ever deployed or loaded for it, it has no version
// to change and no enabled intent of its own — the scheduler holds it enabled at order 0. That
// leaves the settings, which are stashed where scheduleGlobal picks them up on its next pass,
// and a Reschedule so Configure re-runs with them.
func (r *Runtime) UpdateGlobalSettings(settingsVersion int, settings addonutils.Values) {
	r.mu.Lock()
	defer r.mu.Unlock()

	name := r.global.GetName()

	r.logger.Debug("update global settings", slog.String("name", name))

	if len(settings) == 0 {
		settings = make(addonutils.Values)
	}

	if !r.packages.UpdateSettings(name, settingsVersion, settings, "") {
		return
	}

	r.scheduler.Reschedule(name)
}

// LoadModules runs the bootstrap's whole module tree through the pipeline UpdateModule and
// UpdateEmbeddedModule run one module at a time, and blocks until every package has deployed and
// loaded. It is the barrier the caller needs before ResumeScheduler: a scheduler resumed over a
// half-loaded tree resolves its rule chain against nodes that are not there yet.
//
// Each module is registered and enqueued with a shared WaitGroup riding its tasks, so the wait
// covers Deploy as well as Load — an undeployed module is one the Load behind it cannot finish.
// Embedded modules take the embedded path (no Deploy, ReadyOnFilesystem true from the start, the
// edition's version as their package version), so the reconcile that follows this finds exactly
// the state UpdateEmbeddedModule would have stored and does not re-run the pipeline over it.
//
// The wait is bounded and never holds r.mu. It cannot hold the lock because Load calls
// registerModule, which takes r.mu itself, so waiting under it deadlocks the queue against the
// caller. It is bounded because the queue retries a failing task forever: one module whose image
// will not pull would otherwise keep the scheduler paused for the whole process, so the barrier
// gives up after loadModulesTimeout and leaves the rest to converge in the background.
func (r *Runtime) LoadModules(ctx context.Context, mods []Module) {
	wg := new(sync.WaitGroup)

	r.enqueueModules(wg, mods)

	loaded := make(chan struct{})

	go func() {
		wg.Wait()
		close(loaded)
	}()

	ctx, cancel := context.WithTimeout(ctx, loadModulesTimeout)
	defer cancel()

	select {
	case <-loaded:
		r.logger.Debug("all modules loaded", slog.Int("count", len(mods)))

	case <-ctx.Done():
		r.logger.Warn("modules are still loading, continue without them", slog.Int("count", len(mods)))
	}
}

// enqueueModules registers every module and puts its pipeline on the queue with wg riding each
// task. Split out of LoadModules so r.mu is released before the barrier waits on wg.
func (r *Runtime) enqueueModules(wg *sync.WaitGroup, mods []Module) {
	r.mu.Lock()
	defer r.mu.Unlock()

	for _, module := range mods {
		name := module.Name

		r.logger.Debug("load module", slog.String("name", name))

		if len(module.Settings) == 0 {
			module.Settings = make(addonutils.Values)
		}

		// An embedded module is the one the image ships, so it has no repository to pull from and
		// no package version of its own — the running edition's stands in for it, as in
		// UpdateEmbeddedModule, so the version this stores is the one the reconcile compares against.
		embedded := module.Repository.Name == ""

		version := module.Definition.Version
		if embedded {
			version = app.EmbeddedPackageVersion(r.edition.Version)
		}

		r.global.SetConfigEnabled(name, module.Enabled)

		ctx := r.packages.Update(name, version, module.SettingsVersion, module.Settings, module.Maintenance, false)
		if ctx == nil {
			r.scheduler.Reschedule(name)
			continue
		}

		r.status.NewStatus(name)

		if embedded {
			// The image carries the module, so nothing has to place it on disk.
			r.status.SetConditionTrue(name, status.ConditionReadyOnFilesystem)

			r.queueService.Enqueue(ctx, name, taskload.NewEmbeddedTask(name, r.loadEmbeddedModule, r.status, r.logger), queue.WithWait(wg))

			continue
		}

		// Deploy goes first: the queue holds its head until it succeeds, so a Load enqueued ahead
		// of it would spin on files nothing has placed yet and never let the Deploy behind it run.
		r.queueService.Enqueue(ctx, name, taskdeploy.NewModuleTask(name, version, module.Repository, false, r.moduleDeployer, r.status, r.logger), queue.WithWait(wg))
		r.queueService.Enqueue(ctx, name, taskload.NewModuleTask(name, module.Repository, r.loadModule, r.status, r.logger), queue.WithWait(wg))
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
func (r *Runtime) UpdateModule(module Module, force bool) {
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
		taskdeploy.NewModuleTask(name, version, module.Repository, force, r.moduleDeployer, r.status, r.logger),
		taskload.NewModuleTask(name, module.Repository, r.loadModule, r.status, r.logger),
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
// The version is the running edition's reduced to major.minor.patch — the same one the
// Module spec and its ModulePackageVersion carry — because an embedded module has no
// package version of its own, so it cannot change while the process lives — but EventRemove clears
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
	version := app.EmbeddedPackageVersion(r.edition.Version)
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
// as loadModule does for a downloaded one. The definition's version is overwritten with the
// running edition's, reduced to the version the image's packages carry, and the repository the
// Load task passes is empty — an embedded module has none, so no registry values are injected.
func (r *Runtime) loadEmbeddedModule(ctx context.Context, _ registry.Remote, packagePath string) (string, error) {
	ctx, span := otel.Tracer(runtimeTracer).Start(ctx, "loadEmbeddedModule")
	defer span.End()

	span.SetAttributes(attribute.String("path", packagePath))

	conf, err := loader.LoadEmbeddedConf(ctx, packagePath, r.logger)
	if err != nil {
		span.SetStatus(codes.Error, err.Error())
		return "", status.NewError("LoadFailed", err)
	}

	conf.Definition.Version = app.EmbeddedPackageVersion(r.edition.Version)

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
	if err = ctx.Err(); err != nil {
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

// RemoveModule removes a module, cancels all its running operations and reports whether the
// teardown has finished. After undeploy, a cleanup goroutine removes the Store entry and stops
// the queue. See RemoveApp for the idempotence contract and the async cleanup rationale.
func (r *Runtime) RemoveModule(name string) bool {
	r.mu.Lock()
	defer r.mu.Unlock()

	switch r.packages.RemovalState(name) {
	case lifecycle.RemovalDone:
		return true

	case lifecycle.RemovalInFlight:
		r.logger.Debug("module removal is still in flight", slog.String("name", name))

		return false
	}

	r.scheduler.RemoveNode(name)

	ctx := r.packages.HandleEvent(lifecycle.EventRemove, name)
	if ctx == nil {
		return true
	}

	if pkg := r.modules[name]; pkg != nil {
		r.queueService.Enqueue(ctx, name, taskdisable.NewTask(pkg, app.NamespaceDeckhouse, false, r.nelmService, r.queueService, r.logger))
	} else {
		// A failed Load may roll the instance out of r.modules while the previous release is still live.
		r.queueService.Enqueue(ctx, name, taskuninstall.NewTask(name, app.NamespaceDeckhouse, r.nelmService, r.logger))
	}

	cleanup := queue.WithOnDone(r.cleanupModule(name))

	r.queueService.Enqueue(ctx, name, taskundeploy.NewModuleTask(name, r.moduleDeployer, r.logger), cleanup)

	return false
}

// RemoveEmbeddedModule removes an embedded module and cancels all its running operations.
// It is RemoveModule without Undeploy: the image carries the files, so nothing was ever placed
// on disk for the deployer to take back. Cleanup always rides on Dummy after Disable, or after
// Uninstall when the module instance is unavailable.
func (r *Runtime) RemoveEmbeddedModule(name string) bool {
	r.mu.Lock()
	defer r.mu.Unlock()

	switch r.packages.RemovalState(name) {
	case lifecycle.RemovalDone:
		return true

	case lifecycle.RemovalInFlight:
		r.logger.Debug("embedded module removal is still in flight", slog.String("name", name))

		return false
	}

	r.scheduler.RemoveNode(name)

	ctx := r.packages.HandleEvent(lifecycle.EventRemove, name)
	if ctx == nil {
		return true
	}

	if pkg := r.modules[name]; pkg != nil {
		r.queueService.Enqueue(ctx, name, taskdisable.NewTask(pkg, app.NamespaceDeckhouse, false, r.nelmService, r.queueService, r.logger))
	} else {
		// A failed Load may roll the instance out of r.modules while the previous release is still live.
		r.queueService.Enqueue(ctx, name, taskuninstall.NewTask(name, app.NamespaceDeckhouse, r.nelmService, r.logger))
	}

	// The teardown rides the last task in the package's queue, never runs inline: it stops that queue
	// and waits up to 10s for it to drain, so from here — under r.mu, with a Load possibly still
	// running and about to want r.mu itself — it would deadlock both. RemoveModule anchors it on
	// Undeploy; an embedded module has nothing to undeploy, so it anchors on a dummy task.
	r.queueService.Enqueue(ctx, name, taskdummy.NewTask(name, r.logger), queue.WithOnDone(r.cleanupModule(name)))

	return false
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

// GetModuleDigest resolves the digest the tag currently points at. It is what a caller
// pinning a module to a mutable dev tag compares against, because the runtime's own change
// detection is blind to a repush under an unchanged tag.
func (r *Runtime) GetModuleDigest(ctx context.Context, remote registry.Remote, name, tag string) (string, error) {
	return r.registry.GetImageDigest(ctx, remote, name, tag)
}

// ValidateModuleExclusiveGroup returns an error if there is an enabled module with the same exclusive group.
func (r *Runtime) ValidateModuleExclusiveGroup(group string) error {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var found bool
	for name := range r.modules {
		if r.modules[name].GetExclusiveGroup() != group {
			continue
		}

		if r.scheduler.IsEnabled(name) {
			found = true
			break
		}
	}

	if found {
		return errors.New("module cannot be enabled because another module with same exclusiveGroup enabled")
	}

	return nil
}
