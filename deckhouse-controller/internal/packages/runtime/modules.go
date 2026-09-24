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
	"sync"
	"time"

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
	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/addonutils"
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

// IsEmbedded reports whether the image ships the module: it has no repository to pull from.
func (m Module) IsEmbedded() bool {
	return m.Repository.Name == ""
}

// LoadModules runs the bootstrap's whole module tree through the pipeline UpdateModule runs one
// module at a time and blocks until every package has deployed and loaded. It is the barrier the
// caller needs before ResumeScheduler: a scheduler resumed over a half-loaded tree resolves its
// rule chain against nodes that are not there yet.
//
// The wait rides every task of every module, so it covers Deploy as well as Load. It never holds
// r.mu — Load takes it itself, so waiting under it deadlocks the queue against the caller — and it
// is bounded, because the queue retries a failing task forever: after loadModulesTimeout the rest
// is left to converge in the background.
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

// enqueueModules runs the bootstrap's tree through the same path as a single update, with wg riding
// every task. Split out of LoadModules so r.mu is released before the barrier waits on wg.
func (r *Runtime) enqueueModules(wg *sync.WaitGroup, mods []Module) {
	r.mu.Lock()
	defer r.mu.Unlock()

	for _, module := range mods {
		r.logger.Debug("load module", slog.String("name", module.Name))

		r.updateModule(module, false, queue.WithWait(wg))
	}
}

// UpdateModule handles module creation, version changes, settings and enabled intent from the
// module controller. A module the image ships arrives without a repository: it takes the running
// edition's version and its pipeline skips Deploy, since the files are in place already.
//
// force runs the pipeline even when nothing the runtime tracks changed and makes the Deploy task
// discard the cached copy of the version. It is for callers that resolved the image digest and
// found it changed under a tag the runtime still sees as unchanged, and is transitional: it goes
// away once module tags are immutable.
func (r *Runtime) UpdateModule(module Module, force bool) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.logger.Debug("update module", slog.String("name", module.Name), slog.Bool("force", force))

	r.updateModule(module, force)
}

// updateModule runs the desired state past the store and acts on its decision. opts ride every task
// the pipeline enqueues, which is how the bootstrap barrier waits on it. Callers hold r.mu.
func (r *Runtime) updateModule(module Module, force bool, opts ...queue.EnqueueOption) {
	name := module.Name

	if len(module.Settings) == 0 {
		module.Settings = make(addonutils.Values)
	}

	// An embedded module has no package version of its own; the running edition's stands in.
	if module.IsEmbedded() {
		module.Definition.Version = app.EmbeddedPackageVersion()
	}

	// The enabled intent lives in the global module, which has no notion of tracking, so the
	// scheduler's config rule sees it even for a package that is not registered yet.
	enabledChanged := r.global.SetConfigEnabled(name, module.Enabled)

	decision := r.packages.Reconcile(name, lifecycle.DesiredState{
		Version:         module.Definition.Version,
		Settings:        module.Settings,
		SettingsVersion: module.SettingsVersion,
		Maintenance:     module.Maintenance,
		ForceReload:     force,
	})

	switch decision.Kind {
	case lifecycle.DecisionNone:
		if enabledChanged {
			r.scheduler.Reschedule(name, reasonEnabledChanged)
		}

	case lifecycle.DecisionReconfigure:
		r.scheduler.Reschedule(name, rescheduleReason(decision.Changes))

	case lifecycle.DecisionUpdate:
		r.enqueueModule(name, module, force, opts...)
	}
}

// UpdateGlobalModule applies a settings change to the global module.
//
// The runtime built global itself out of the global hooks dir, so its version never changes and
// nothing is deployed or loaded for it: the settings are stashed for the next scheduleGlobal pass,
// which a reschedule triggers. The scheduler holds global enabled at order 0, so its enabled
// intent is ignored.
func (r *Runtime) UpdateGlobalModule(settings addonutils.Values, settingsVersion int) {
	r.mu.Lock()
	defer r.mu.Unlock()

	name := r.global.GetName()

	r.logger.Debug("update global module", slog.String("name", name))

	if len(settings) == 0 {
		settings = make(addonutils.Values)
	}

	decision := r.packages.Reconcile(name, lifecycle.DesiredState{
		Version:         r.global.GetVersion().String(),
		Settings:        settings,
		SettingsVersion: settingsVersion,
	})

	if decision.Kind == lifecycle.DecisionNone {
		return
	}

	r.scheduler.Reschedule(name, rescheduleReason(decision.Changes))
}

// enqueueModule puts the module's pipeline on its queue: Disable the live instance, then Deploy
// and Load the new version. Enqueues nothing if a removal owns the package.
func (r *Runtime) enqueueModule(name string, module Module, force bool, opts ...queue.EnqueueOption) {
	ctx, ok := r.packages.BeginUpdate(name)
	if !ok {
		r.logger.Debug("module removal is in flight, skip the update", slog.String("name", name))

		return
	}

	r.status.NewStatus(name)

	tasks := make([]queue.Task, 0, 3)

	// A registered instance keeps its hooks and its release, so it is torn down before the new
	// version takes its place.
	if pkg := r.modules[name]; pkg != nil {
		tasks = append(tasks, taskdisable.NewTask(pkg, app.NamespaceDeckhouse, true, r.nelmService, r.queueService, r.logger))
	}

	if module.IsEmbedded() {
		// The image carries the module, so nothing has to place it on disk.
		r.status.SetConditionTrue(name, status.ConditionReadyOnFilesystem)

		tasks = append(tasks, taskload.NewEmbeddedTask(name, r.loadEmbeddedModule, r.status, r.logger))
	} else {
		// Deploy goes first: the queue holds its head until it succeeds, so a Load enqueued ahead
		// of it would spin on files nothing has placed yet and never let the Deploy behind it run.
		tasks = append(tasks,
			taskdeploy.NewModuleTask(name, module.Definition.Version, module.Repository, force, r.moduleDeployer, r.status, r.logger),
			taskload.NewModuleTask(name, module.Repository, r.loadModule, r.status, r.logger))
	}

	for _, task := range tasks {
		r.queueService.Enqueue(ctx, name, task, opts...)
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

	conf.Definition.Version = app.EmbeddedPackageVersion()

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
	conf.MetricStorage = r.metricStorage
	conf.HookMetricStorage = r.hookMetricStorage

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

	ctx, state := r.packages.BeginRemoval(name)

	switch state {
	case lifecycle.RemovalDone:
		return true

	case lifecycle.RemovalInFlight:
		r.logger.Debug("module removal is still in flight", slog.String("name", name))

		return false
	}

	r.scheduler.RemoveNode(name)

	r.status.SetDeleting(name)

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

	ctx, state := r.packages.BeginRemoval(name)

	switch state {
	case lifecycle.RemovalDone:
		return true

	case lifecycle.RemovalInFlight:
		r.logger.Debug("embedded module removal is still in flight", slog.String("name", name))

		return false
	}

	r.scheduler.RemoveNode(name)

	r.status.SetDeleting(name)

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

			if r.packages.CompleteRemoval(name) {
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
