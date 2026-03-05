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
	"path/filepath"
	"slices"

	addonutils "github.com/flant/addon-operator/pkg/utils"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"

	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/loader"
	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/modules"
	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/runtime/lifecycle"
	taskapplysettings "github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/runtime/tasks/applysettings"
	taskdisable "github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/runtime/tasks/disable"
	taskdownload "github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/runtime/tasks/download"
	taskinstall "github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/runtime/tasks/install"
	taskload "github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/runtime/tasks/load"
	taskrun "github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/runtime/tasks/run"
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

// UpdateModule handles module updates (version changes and settings changes).
// On version change: validates requirements, then queues download → install → load.
// On settings change: queues apply-settings → run.
func (r *Runtime) UpdateModule(repo registry.Remote, module Module) {
	r.mu.Lock()
	defer r.mu.Unlock()

	settingsChecksum := ""
	if len(module.Settings) > 0 {
		settingsChecksum = module.Settings.Checksum()
	}

	name := module.Name
	version := module.Definition.Version

	r.modules.Update(name, version, settingsChecksum, func(ctx context.Context, event int, pkg *modules.Module) {
		var tasks []queue.Task
		if event == lifecycle.EventVersionChanged {
			if err := r.scheduler.Check(module.Definition.Requirements.Checks()); err != nil {
				r.status.HandleError(name, err)
				return
			}

			r.status.ClearRuntimeConditions(name)
			r.status.SetConditionTrue(name, status.ConditionRequirementsMet)

			tasks = []queue.Task{
				taskdownload.NewModuleTask(name, version, repo, r.installer, r.status, r.logger),
				taskinstall.NewModuleTask(name, version, repo, r.installer, r.status, r.logger),
				taskload.NewModuleTask(name, repo, module.Settings, r.loadModule, r.status, r.logger),
			}

			// If there's an existing module, disable it first
			if pkg != nil {
				tasks = slices.Insert(tasks, 0, taskdisable.NewTask(pkg, modulesNamespace, true, r.nelmService, r.queueService, r.status, r.logger))
			}
		}

		if event == lifecycle.EventSettingsChanged && pkg != nil {
			tasks = []queue.Task{
				taskapplysettings.NewTask(pkg, module.Settings, r.status, r.logger),
				taskrun.NewTask(pkg, modulesNamespace, r.nelmService, r.status, r.logger),
			}
		}

		for _, task := range tasks {
			r.queueService.Enqueue(ctx, name, task)
		}
	})
}

// loadModule builds a Module from its package files, validates settings, and registers it
// with the lifecycle store and scheduler. Called by the Load task after filesystem mount.
func (r *Runtime) loadModule(ctx context.Context, repo registry.Remote, settings addonutils.Values, packagePath string) (string, error) {
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

	res, err := module.ValidateSettings(ctx, settings)
	if err != nil {
		span.SetStatus(codes.Error, err.Error())
		return "", newApplyInitialSettingsErr(err)
	}

	if !res.Valid {
		span.SetStatus(codes.Error, res.Message)
		return "", newApplyInitialSettingsErr(errors.New(res.Message))
	}

	r.mu.Lock()
	r.modules.SetPackage(module.GetName(), module)
	r.mu.Unlock()

	r.scheduler.Register(module)

	return module.GetVersion(), nil
}

// RemoveModule removes a module and cancels all its running operations.
// After uninstall, a cleanup goroutine removes the Store entry and stops the queue.
// See RemoveApp for detailed rationale on the async cleanup pattern.
func (r *Runtime) RemoveModule(name string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.scheduler.Remove(name)

	r.modules.HandleEvent(lifecycle.EventRemove, name, func(ctx context.Context, _ int, pkg *modules.Module) {
		cleanup := queue.WithOnDone(func() {
			go func() {
				r.mu.Lock()
				defer r.mu.Unlock()

				if r.modules.Delete(name) {
					r.queueService.Remove(name)
					r.status.Delete(name)
				}
			}()
		})

		r.queueService.Enqueue(ctx, name, taskdisable.NewTask(pkg, modulesNamespace, false, r.nelmService, r.queueService, r.status, r.logger))
		r.queueService.Enqueue(ctx, name, taskuninstall.NewModuleTask(name, r.installer, r.logger), cleanup)
	})
}
