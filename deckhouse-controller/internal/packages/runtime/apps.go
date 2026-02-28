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
	taskdisable "github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/runtime/tasks/disable"
	taskdownload "github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/runtime/tasks/download"
	taskinstall "github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/runtime/tasks/install"
	taskload "github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/runtime/tasks/load"
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
	defer r.mu.Unlock()

	app := r.apps[name]
	if app == nil {
		return "", fmt.Errorf("app %s not found", name)
	}

	return r.nelmService.Render(ctx, app.GetNamespace(), app)
}

// ValidateSettings checks settings against the package's OpenAPI schema.
// Returns valid if the package is not loaded yet (settings validated on load).
func (r *Runtime) ValidateSettings(ctx context.Context, name string, settings addonutils.Values) (settingscheck.Result, error) {
	ctx, span := otel.Tracer(runtimeTracer).Start(ctx, "ValidateSettings")
	defer span.End()

	r.mu.Lock()
	app := r.apps[name]
	if app == nil {
		r.mu.Unlock()
		return settingscheck.Result{Valid: true}, nil
	}
	r.mu.Unlock()

	return app.ValidateSettings(ctx, settings)
}

// UpdateApp handles application creation and version changes from the Application controller.
//
// Flow:
//  1. NeedUpdate fast-path: skip if version and settings checksum are unchanged
//  2. Store.Update: if version changed → new root context, enqueue full pipeline
//     (Disable → Download → Install → Load); if only settings changed → nil context,
//     trigger Reschedule so the scheduler re-runs ApplySettings → Startup → Run
//  3. CheckConstraints: validate Kubernetes/Deckhouse version requirements before enqueuing
//
// Settings are applied lazily: the scheduler's schedulePackage reads pending settings
// from the Store via GetPendingSettings when the package is scheduled for startup.
func (r *Runtime) UpdateApp(repo registry.Remote, app App) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if app.Namespace == "" {
		app.Namespace = "default"
	}

	if len(app.Settings) == 0 {
		app.Settings = make(addonutils.Values)
	}

	name := apps.BuildName(app.Namespace, app.Name)
	version := app.Definition.Version

	if !r.packages.NeedUpdate(name, version, app.Settings.Checksum()) {
		return
	}

	ctx := r.packages.Update(name, version, app.Settings)
	if ctx == nil {
		r.scheduler.Reschedule(name)
		return
	}

	if err := r.scheduler.CheckConstraints(app.Definition.Constraints()); err != nil {
		r.status.HandleError(name, err)
		return
	}

	r.status.ClearRuntimeConditions(name)
	r.status.SetConditionTrue(name, status.ConditionRequirementsMet)

	packageName := app.Definition.Name
	packageVersion := app.Definition.Version

	tasks := []queue.Task{
		taskdownload.NewAppTask(name, packageName, packageVersion, repo, r.installer, r.status, r.logger),
		taskinstall.NewAppTask(name, packageName, packageVersion, repo, r.installer, r.status, r.logger),
		taskload.NewAppTask(name, repo, r.loadApp, r.status, r.logger),
	}

	// If there's an existing app, disable it first
	if pkg := r.apps[name]; pkg != nil {
		tasks = slices.Insert(tasks, 0, taskdisable.NewTask(pkg, pkg.GetNamespace(), true, r.nelmService, r.queueService, r.status, r.logger))
	}

	for _, task := range tasks {
		r.queueService.Enqueue(ctx, name, task)
	}
}

// loadApp builds an Application from its package files and stores it in r.apps.
// Called by the Load task after the package image is mounted on the filesystem.
// Does not apply settings or register with the scheduler — that happens in the
// ApplySettings task which runs as part of the scheduler's schedule pipeline.
func (r *Runtime) loadApp(ctx context.Context, repo registry.Remote, packagePath string) (string, error) {
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

	r.mu.Lock()
	defer r.mu.Unlock()

	r.apps[app.GetName()] = app

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

	ctx := r.packages.HandleEvent(lifecycle.EventRemove, name)
	if ctx == nil {
		return
	}

	if pkg := r.apps[name]; pkg != nil {
		r.queueService.Enqueue(ctx, name, taskdisable.NewTask(pkg, pkg.GetNamespace(), false, r.nelmService, r.queueService, r.status, r.logger))
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

	r.queueService.Enqueue(ctx, name, taskuninstall.NewAppTask(name, r.installer, r.logger), cleanup)
}
