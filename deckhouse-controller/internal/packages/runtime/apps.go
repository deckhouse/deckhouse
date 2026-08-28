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
	"log/slog"
	"path/filepath"
	"slices"

	addonutils "github.com/flant/addon-operator/pkg/utils"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"

	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/apps"
	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/loader"
	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/nelm"
	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/runtime/lifecycle"
	taskdeploy "github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/runtime/tasks/deploy"
	taskdisable "github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/runtime/tasks/disable"
	taskload "github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/runtime/tasks/load"
	taskundeploy "github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/runtime/tasks/undeploy"
	taskuninstall "github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/runtime/tasks/uninstall"
	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/status"
	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/queue"
	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/registry"
)

const (
	defaultNamespace = "default"
)

// App represents an application instance as received from the Application controller.
// It carries the user-specified package identity, version constraints, settings, and
// maintenance mode.
type App struct {
	Name            string
	Namespace       string
	Definition      apps.Definition
	Settings        addonutils.Values
	SettingsVersion int // schema version from Application.Spec.Version (reserved for future use)
	Maintenance     string
}

// UpdateApp handles application creation and version changes from the Application controller.
//
// Flow:
//  1. NeedUpdate fast-path: skip if version and settings checksum are unchanged
//  2. Store.Update: if version changed → new root context, enqueue full pipeline
//     (Disable → Deploy → Load); if only settings changed → nil context,
//     trigger Reschedule so the scheduler re-runs Configure → Startup → Run
//  3. CheckConstraints: validate Kubernetes/Deckhouse version requirements before enqueuing
//
// Settings are applied lazily: the scheduler's schedulePackage reads pending settings
// from the Store via GetPendingSettings when the package is scheduled for startup.
func (r *Runtime) UpdateApp(repo registry.Remote, app App) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.logger.Debug("update app", slog.String("name", app.Name))

	if len(app.Namespace) == 0 {
		app.Namespace = defaultNamespace
	}

	if len(app.Settings) == 0 {
		app.Settings = make(addonutils.Values)
	}

	name := apps.BuildName(app.Namespace, app.Name)
	version := app.Definition.Version
	packageName := app.Definition.Name

	if !r.packages.NeedUpdate(name, version, app.Settings.Checksum(), app.SettingsVersion, app.Maintenance) {
		return
	}

	// applications have immutable tags, so a version change is the only invalidation
	ctx := r.packages.Update(name, version, app.SettingsVersion, app.Settings, app.Maintenance, false)
	if ctx == nil {
		r.scheduler.Reschedule(name)
		return
	}

	r.status.NewStatus(name)

	tasks := []queue.Task{
		taskdeploy.NewAppTask(name, packageName, version, repo, r.appDeployer, r.status, r.logger),
		taskload.NewAppTask(name, repo, r.loadApp, r.status, r.logger),
	}

	// If there's an existing app, disable it first
	if pkg := r.apps[name]; pkg != nil {
		tasks = slices.Insert(tasks, 0, taskdisable.NewTask(pkg, pkg.GetNamespace(), true, r.nelmService, r.queueService, r.logger))
	}

	for _, task := range tasks {
		r.queueService.Enqueue(ctx, name, task)
	}
}

// loadApp builds an Application from its package files and stores it in r.apps.
// Called by the Load task after the package image is mounted on the filesystem.
func (r *Runtime) loadApp(ctx context.Context, repo registry.Remote, packagePath string) (string, error) {
	ctx, span := otel.Tracer(runtimeTracer).Start(ctx, "loadApp")
	defer span.End()

	span.SetAttributes(attribute.String("path", packagePath))
	span.SetAttributes(attribute.String("repository", repo.Name))

	conf, err := loader.LoadAppConf(ctx, packagePath, r.logger)
	if err != nil {
		span.SetStatus(codes.Error, err.Error())
		return "", status.NewError("LoadFailed", err)
	}

	conf.Repository = repo
	conf.Patcher = r.objectPatcher
	conf.ScheduleManager = r.scheduleManager
	conf.KubeEventsManager = r.kubeEventsManager
	conf.GrantResolver = r.grantResolver
	conf.GlobalValuesGetter = r.addonModuleManager.GetGlobal().GetValues

	app, err := apps.NewAppByConfig(filepath.Base(packagePath), conf, r.logger)
	if err != nil {
		span.SetStatus(codes.Error, err.Error())
		return "", status.NewError("LoadFailed", err)
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	// The application was removed while this Load ran — r.mu is what serialises the two, so this is
	// the last point either can win. Publishing now would give the scheduler a node for a package
	// nothing tracks, and Enable would then register its hooks with the shared managers with no
	// removal path left to disable them.
	if err = ctx.Err(); err != nil {
		return "", err
	}

	// Optimistically register the app before AddNode so a successful schedule
	// can resolve it; if AddNode rejects the addition (dependency cycle),
	// roll back the map entry so we never expose a package the scheduler
	// never accepted.
	r.apps[app.GetName()] = app
	if err = r.scheduler.AddNode(app); err != nil {
		delete(r.apps, app.GetName())
		span.SetStatus(codes.Error, err.Error())
		return "", status.NewError("DependencyCycle", err)
	}

	return app.GetVersion().String(), nil
}

// RemoveApp removes an application, cancels all its running operations and reports whether the
// teardown has finished. The caller polls it and holds the Application's finalizer until it
// returns true, so the CR outlives the Helm release it owns.
//
// It is idempotent by contract: a call made while the teardown runs must not re-issue
// EventRemove, because that cancels the whole context tree (lifecycle.Package.newContext) and
// would restart the very uninstall the caller is waiting for.
//
// After the undeploy task succeeds, a cleanup goroutine removes the
// Store entry and stops the queue. The goroutine is necessary because
// queueService.Remove stops the queue — calling it synchronously from
// within the queue's own processing loop would deadlock on WaitGroup.
//
// Store.Delete has a state guard: if UpdateApp re-created the package between undeploy and cleanup,
// Update cleared the removal marker, so Delete is a no-op and removal reports unfinished again for
// the re-created generation.
func (r *Runtime) RemoveApp(namespace, instance string) bool {
	r.mu.Lock()
	defer r.mu.Unlock()

	name := apps.BuildName(namespace, instance)

	switch r.packages.RemovalState(name) {
	case lifecycle.RemovalDone:
		// nothing is tracked under the name: the teardown finished, or never had to run
		return true

	case lifecycle.RemovalInFlight:
		r.logger.Debug("app removal is still in flight", slog.String("name", name))

		return false
	}

	r.logger.Debug("remove app", slog.String("namespace", namespace), slog.String("instance", instance))

	r.scheduler.RemoveNode(name)

	// A removed application no longer reconciles anything, so drop its maintenance gauge.
	r.setMaintenanceMetric(name, nelm.Managed)

	ctx := r.packages.HandleEvent(lifecycle.EventRemove, name)
	if ctx == nil {
		return true
	}

	if pkg := r.apps[name]; pkg != nil {
		r.queueService.Enqueue(ctx, name, taskdisable.NewTask(pkg, pkg.GetNamespace(), false, r.nelmService, r.queueService, r.logger))
	} else {
		// A failed Load may roll the instance out of r.apps while the previous release is still live.
		r.queueService.Enqueue(ctx, name, taskuninstall.NewTask(name, namespace, r.nelmService, r.logger))
	}

	cleanup := queue.WithOnDone(func() {
		go func() {
			r.mu.Lock()
			defer r.mu.Unlock()

			if r.packages.Delete(name) {
				r.queueService.Remove(name)
				r.status.DeleteStatus(name)
				delete(r.apps, name)
			}
		}()
	})

	r.queueService.Enqueue(ctx, name, taskundeploy.NewAppTask(name, r.appDeployer, r.logger), cleanup)

	return false
}
