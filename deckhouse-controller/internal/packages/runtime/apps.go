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
	"errors"
	"fmt"
	"path/filepath"
	"slices"

	addonutils "github.com/flant/addon-operator/pkg/utils"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"

	"github.com/deckhouse/module-sdk/pkg/settingscheck"

	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/apps"
	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/loader"
	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/runtime/lifecycle"
	taskapplysettings "github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/runtime/tasks/applysettings"
	taskdisable "github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/runtime/tasks/disable"
	taskdownload "github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/runtime/tasks/download"
	taskinstall "github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/runtime/tasks/install"
	taskload "github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/runtime/tasks/load"
	taskrun "github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/runtime/tasks/run"
	taskstartup "github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/runtime/tasks/startup"
	taskuninstall "github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/runtime/tasks/uninstall"
	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/status"
	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/queue"
	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/registry"
)

// App represents an application instance as received from the Application controller.
// It carries the user-specified package identity, version constraints, and settings.
type App struct {
	Name       string
	Namespace  string
	Definition apps.Definition
	Settings   addonutils.Values
}

// renderManifests renders the Helm chart for a loaded package. Used by the debug server.
func (r *Runtime) renderManifests(ctx context.Context, name string) (string, error) {
	r.mu.Lock()
	app := r.apps.GetPackage(name)
	if app == nil {
		r.mu.Unlock()
		return "", fmt.Errorf("app %s not found", name)
	}
	r.mu.Unlock()

	return r.nelmService.Render(ctx, app.GetNamespace(), app)
}

// ValidateSettings checks settings against the package's OpenAPI schema.
// Returns valid if the package is not loaded yet (settings validated on load).
func (r *Runtime) ValidateSettings(ctx context.Context, name string, settings addonutils.Values) (settingscheck.Result, error) {
	ctx, span := otel.Tracer(runtimeTracer).Start(ctx, "ValidateSettings")
	defer span.End()

	r.mu.Lock()
	app := r.apps.GetPackage(name)
	if app == nil {
		r.mu.Unlock()
		return settingscheck.Result{Valid: true}, nil
	}
	r.mu.Unlock()

	return app.ValidateSettings(ctx, settings)
}

// UpdateApp handles application updates (version changes and settings changes).
// Version changes and settings changes are handled independently with separate contexts.
func (r *Runtime) UpdateApp(repo registry.Remote, app App) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if app.Namespace == "" {
		app.Namespace = "default"
	}

	settingsChecksum := ""
	if len(app.Settings) > 0 {
		settingsChecksum = app.Settings.Checksum()
	}

	name := apps.BuildName(app.Namespace, app.Name)
	version := app.Definition.Version

	r.apps.Update(name, version, settingsChecksum, func(ctx context.Context, event int, pkg *apps.Application) {
		var tasks []queue.Task
		if event == lifecycle.EventVersionChanged {
			if err := r.scheduler.CheckByConstraints(app.Definition.Requirements.Constraints()); err != nil {
				r.status.HandleError(name, err)
				return
			}

			r.status.ClearRuntimeConditions(name)
			r.status.SetConditionTrue(name, status.ConditionRequirementsMet)

			packageName := app.Definition.Name
			packageVersion := app.Definition.Version

			tasks = []queue.Task{
				taskdownload.NewAppTask(name, packageName, packageVersion, repo, r.installer, r.status, r.logger),
				taskinstall.NewAppTask(name, packageName, packageVersion, repo, r.installer, r.status, r.logger),
				taskload.NewAppTask(name, repo, app.Settings, r.loadApp, r.status, r.logger),
			}

			// If there's an existing app, disable it first
			if pkg != nil {
				tasks = slices.Insert(tasks, 0, taskdisable.NewTask(pkg, pkg.GetNamespace(), true, r.nelmService, r.queueService, r.status, r.logger))
			}
		}

		if event == lifecycle.EventSettingsChanged && pkg != nil {
			tasks = []queue.Task{
				taskapplysettings.NewTask(pkg, app.Settings, r.status, r.logger),
				taskrun.NewTask(pkg, pkg.GetNamespace(), r.nelmService, r.status, r.logger),
			}
		}

		for _, task := range tasks {
			r.queueService.Enqueue(ctx, name, task)
		}
	})
}

// loadApp builds an Application from its package files, validates settings, and registers it
// with the lifecycle store and scheduler. Called by the Load task after filesystem mount.
func (r *Runtime) loadApp(ctx context.Context, repo registry.Remote, settings addonutils.Values, packagePath string) (string, error) {
	ctx, span := otel.Tracer(runtimeTracer).Start(ctx, "loadApp")
	defer span.End()

	span.SetAttributes(attribute.String("path", packagePath))
	span.SetAttributes(attribute.String("repository", repo.Name))

	conf, err := loader.LoadAppConf(ctx, packagePath, r.logger)
	if err != nil {
		span.SetStatus(codes.Error, err.Error())
		return "", newLoadFailedErr(err)
	}

	conf.Repository = repo
	conf.Patcher = r.objectPatcher
	conf.ScheduleManager = r.scheduleManager
	conf.KubeEventsManager = r.kubeEventsManager

	app, err := apps.NewAppByConfig(filepath.Base(packagePath), conf, r.logger)
	if err != nil {
		span.SetStatus(codes.Error, err.Error())
		return "", newLoadFailedErr(err)
	}

	res, err := app.ValidateSettings(ctx, settings)
	if err != nil {
		span.SetStatus(codes.Error, err.Error())
		return "", newApplyInitialSettingsErr(err)
	}

	if !res.Valid {
		span.SetStatus(codes.Error, res.Message)
		return "", newApplyInitialSettingsErr(errors.New(res.Message))
	}

	r.mu.Lock()
	r.apps.SetPackage(app.GetName(), app)
	r.mu.Unlock()

	r.scheduler.AddNode(app)

	return app.GetVersion().String(), nil
}

// RemoveApp removes an application and cancels all its running operations.
//
// After the uninstall task succeeds, a cleanup goroutine removes the
// Store entry and stops the queue. The goroutine is necessary because
// queueService.Remove stops the queue — calling it synchronously from
// within the queue's own processing loop would deadlock on WaitGroup.
//
// Store.Delete has a state guard: if UpdateApp re-created the package
// between uninstall and cleanup, Delete is a no-op (version != "").
func (r *Runtime) RemoveApp(namespace, instance string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	name := apps.BuildName(namespace, instance)
	r.scheduler.RemoveNode(name)

	r.apps.HandleEvent(lifecycle.EventRemove, name, func(ctx context.Context, _ int, pkg *apps.Application) {
		cleanup := queue.WithOnDone(func() {
			go func() {
				r.mu.Lock()
				defer r.mu.Unlock()

				if r.apps.Delete(name) {
					r.queueService.Remove(name)
					r.status.Delete(name)
				}
			}()
		})

		r.queueService.Enqueue(ctx, name, taskdisable.NewTask(pkg, pkg.GetNamespace(), false, r.nelmService, r.queueService, r.status, r.logger))
		r.queueService.Enqueue(ctx, name, taskuninstall.NewAppTask(name, r.installer, r.logger), cleanup)
	})
}

// enableApp is called by the scheduler when a package becomes enabled.
func (r *Runtime) enableApp(name string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.apps.HandleEvent(lifecycle.EventSchedule, name, func(ctx context.Context, _ int, pkg *apps.Application) {
		tasks := []queue.Task{
			taskstartup.NewTask(pkg, r.nelmService, r.queueService, r.status, r.logger),
			taskrun.NewTask(pkg, pkg.GetNamespace(), r.nelmService, r.status, r.logger),
		}

		for _, task := range tasks {
			r.queueService.Enqueue(ctx, name, task)
		}
	})
}

// disableApp is called by the scheduler when a package becomes disabled.
func (r *Runtime) disableApp(name string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.apps.HandleEvent(lifecycle.EventSchedule, name, func(ctx context.Context, _ int, pkg *apps.Application) {
		tasks := []queue.Task{
			taskdisable.NewTask(pkg, pkg.GetNamespace(), true, r.nelmService, r.queueService, r.status, r.logger),
		}

		for _, task := range tasks {
			r.queueService.Enqueue(ctx, name, task)
		}
	})
}

// runApp is called by NELM monitor when a package needs to re-run.
func (r *Runtime) runApp(name string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.apps.HandleEvent(lifecycle.EventRun, name, func(ctx context.Context, _ int, pkg *apps.Application) {
		tasks := []queue.Task{
			taskrun.NewTask(pkg, pkg.GetNamespace(), r.nelmService, r.status, r.logger),
		}

		for _, task := range tasks {
			r.queueService.Enqueue(ctx, name, task)
		}
	})
}
