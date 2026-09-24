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

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"

	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/apps"
	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/loader"
	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/nelm"
	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/resourcerequests"
	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/runtime/lifecycle"
	taskdeploy "github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/runtime/tasks/deploy"
	taskdisable "github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/runtime/tasks/disable"
	taskload "github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/runtime/tasks/load"
	taskundeploy "github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/runtime/tasks/undeploy"
	taskuninstall "github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/runtime/tasks/uninstall"
	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/status"
	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/queue"
	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/registry"
	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/addonutils"
)

const (
	defaultNamespace = "default"
)

// App represents an application instance as received from the Application controller.
// It carries the user-specified package identity, version constraints, settings,
// maintenance mode, and per-workload resource overrides.
type App struct {
	Name            string
	Namespace       string
	Definition      apps.Definition
	Settings        addonutils.Values
	SettingsVersion int // schema version from Application.Spec.Version (reserved for future use)
	Maintenance     string
	Repository      registry.Remote

	// ResourceRequests are the per-workload replicas and container resources from
	// Application.spec.resourceRequests. Honoured only behind the resource-requests
	// feature gate, which the nelm layer reads.
	ResourceRequests []resourcerequests.Request
}

// UpdateApp handles application creation and version changes from the Application controller.
//
// The store decides: a version change restarts the pipeline (Disable → Deploy → Load), any other
// change only reschedules, so the scheduler re-runs Configure → Startup → Run. Applications have
// immutable tags, so a version change is the only invalidation.
//
// Settings are applied lazily: schedulePackage reads them back from the store when the package is
// scheduled, so a change that lands mid-pipeline is still picked up.
func (r *Runtime) UpdateApp(app App) {
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

	decision := r.packages.Reconcile(name, lifecycle.DesiredState{
		Version:         app.Definition.Version,
		Settings:        app.Settings,
		SettingsVersion: app.SettingsVersion,
		Maintenance:     app.Maintenance,
		Resources:       app.ResourceRequests,
	})

	switch decision.Kind {
	case lifecycle.DecisionNone:
		return

	case lifecycle.DecisionReconfigure:
		r.scheduler.Reschedule(name, rescheduleReason(decision.Changes))

	case lifecycle.DecisionUpdate:
		r.enqueueApp(name, app)
	}
}

// enqueueApp puts the application's pipeline on its queue: Disable the live instance, then Deploy
// and Load the new version. Enqueues nothing if a removal owns the package.
func (r *Runtime) enqueueApp(name string, app App) {
	ctx, ok := r.packages.BeginUpdate(name)
	if !ok {
		r.logger.Debug("application removal is in flight, skip the update", slog.String("name", name))

		return
	}

	r.status.NewStatus(name)

	tasks := make([]queue.Task, 0, 3)

	// A registered instance keeps its hooks and its release, so it is torn down before the new
	// version takes its place.
	if pkg := r.apps[name]; pkg != nil {
		tasks = append(tasks, taskdisable.NewTask(pkg, pkg.GetNamespace(), true, r.nelmService, r.queueService, r.logger))
	}

	// Deploy goes first: the queue holds its head until it succeeds, so a Load enqueued ahead of it
	// would spin on files nothing has placed yet and never let the Deploy behind it run.
	tasks = append(tasks,
		taskdeploy.NewAppTask(name, app.Definition.Name, app.Definition.Version, app.Repository, r.appDeployer, r.status, r.logger),
		taskload.NewAppTask(name, app.Repository, r.loadApp, r.status, r.logger))

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
// It is idempotent by contract: BeginRemoval reports a teardown already in flight instead of
// beginning a second one, which would cancel the very uninstall the caller is waiting for.
//
// After the undeploy task succeeds, a cleanup goroutine removes the store entry and stops the
// queue. The goroutine is necessary because queueService.Remove stops the queue — calling it
// synchronously from within the queue's own processing loop would deadlock on WaitGroup.
func (r *Runtime) RemoveApp(namespace, instance string) bool {
	r.mu.Lock()
	defer r.mu.Unlock()

	name := apps.BuildName(namespace, instance)

	ctx, state := r.packages.BeginRemoval(name)

	switch state {
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

	r.status.SetDeleting(name)

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

			if r.packages.CompleteRemoval(name) {
				r.queueService.Remove(name)
				r.status.DeleteStatus(name)
				delete(r.apps, name)
			}
		}()
	})

	r.queueService.Enqueue(ctx, name, taskundeploy.NewAppTask(name, r.appDeployer, r.logger), cleanup)

	return false
}
