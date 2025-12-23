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

package operator

import (
	"context"
	"fmt"
	"log/slog"

	addonutils "github.com/flant/addon-operator/pkg/utils"
	"sigs.k8s.io/yaml"

	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/manager/apps"
	taskapplysettings "github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/operator/tasks/applysettings"
	taskdisable "github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/operator/tasks/disable"
	taskdownload "github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/operator/tasks/download"
	taskinstall "github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/operator/tasks/install"
	taskload "github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/operator/tasks/load"
	taskrun "github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/operator/tasks/run"
	taskuninstall "github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/operator/tasks/uninstall"
	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/status"
	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/queue"
	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/registry"
)

const (
	eventSettingsChanged = iota
	eventVersionChanged
	eventRemove
	eventStartup
	eventDisable
	eventRerun
)

type lifecyclePackage struct {
	version  string
	settings addonutils.Values

	ctx    context.Context
	cancel context.CancelFunc

	rerunCancel   context.CancelFunc
	settingCancel context.CancelFunc
	disableCancel context.CancelFunc
	startupCancel context.CancelFunc
}

type Instance struct {
	Name       string
	Namespace  string
	Definition apps.Definition
	Settings   map[string]interface{}
}

// Update installs a new package or updates an existing package's configuration.
//
// For new packages (Pending phase):
//  1. Install package from repository (download and extract)
//  2. Load package hooks and configuration into memory
//  3. Register with scheduler for enable/disable lifecycle management
//
// For existing packages:
//   - If settings changed, apply new settings and trigger hook re-execution
//
// Cancels any in-flight tasks from previous Update calls via context renewal.
func (o *Operator) Update(reg registry.Registry, inst Instance) {
	if inst.Namespace == "" {
		inst.Namespace = "default"
	}

	o.mu.Lock()
	defer o.mu.Unlock()

	name := apps.BuildName(inst.Namespace, inst.Name)
	if _, ok := o.packages[name]; !ok {
		o.packages[name] = new(lifecyclePackage)
	}

	if o.packages[name].versionChanged(inst.Definition.Version) {
		if err := o.scheduler.Check(inst.Definition.Requirements.Checks()); err != nil {
			o.status.HandleError(name, err)
			return
		}
		o.status.SetConditionTrue(name, status.ConditionRequirementsMet)

		o.packages[name].settings = inst.Settings

		// Cancel previous tasks before enqueueing new ones
		ctx := o.packages[name].renewContext(eventVersionChanged)

		packageName := inst.Definition.Name
		packageVersion := inst.Definition.Version

		o.logger.Debug("update package", slog.String("name", name), slog.String("version", packageVersion))

		o.queueService.Enqueue(ctx, name, taskdisable.NewTask(name, o.status, o.manager, true, o.logger))
		o.queueService.Enqueue(ctx, name, taskdownload.NewTask(name, packageName, packageVersion, reg, o.status, o.installer, o.logger))
		o.queueService.Enqueue(ctx, name, taskinstall.NewTask(name, packageName, packageVersion, reg, o.status, o.installer, o.logger))
		o.queueService.Enqueue(ctx, name, taskload.NewTask(reg, inst.Namespace, name, inst.Settings, o.status, o.manager, o.logger))

		return
	}

	if o.packages[name].settingsChanged(inst.Settings) {
		// Cancel previous tasks before enqueueing new ones
		ctx := o.packages[name].renewContext(eventSettingsChanged)

		o.logger.Debug("update package settings", slog.String("name", name))

		o.queueService.Enqueue(ctx, name, taskapplysettings.NewTask(name, inst.Settings, o.status, o.manager, o.logger))
		o.queueService.Enqueue(ctx, name, taskrun.NewTask(name, o.status, o.manager, o.logger), queue.WithUnique())
	}
}

// Remove uninstalls a package and cleans up all associated resources.
//
// Cleanup sequence:
//  1. Disable package hooks and stop monitoring (taskdisable)
//  2. Clean up custom queues created by package hooks
//  3. Uninstall package resources (taskuninstall)
//  4. Remove package's main queue
func (o *Operator) Remove(namespace, instance string) {
	o.mu.Lock()
	defer o.mu.Unlock()

	name := apps.BuildName(namespace, instance)
	if _, ok := o.packages[name]; !ok {
		return
	}

	// Capture queues before manager removes the app metadata
	queues := o.manager.GetPackageQueues(name)

	ctx := o.packages[name].renewContext(eventRemove)
	o.queueService.Enqueue(ctx, name, taskdisable.NewTask(name, o.status, o.manager, false, o.logger), queue.WithOnDone(func() {
		for _, q := range queues {
			o.logger.Debug("remove package queue", slog.String("name", name), slog.String("queue", q))
			o.queueService.Remove(fmt.Sprintf("%s/%s", name, q))
		}
	}))

	o.queueService.Enqueue(ctx, name, taskuninstall.NewTask(name, o.installer, o.logger), queue.WithOnDone(func() {
		// Remove package's main queue after uninstall completes
		o.queueService.Remove(fmt.Sprintf("%s/sync", name))
		go o.queueService.Remove(name)

		o.mu.Lock()
		delete(o.packages, name)
		o.mu.Unlock()

		o.status.Delete(name)
	}))
}

func (p *lifecyclePackage) renewContext(event int) context.Context {
	var ctx context.Context

	switch event {
	case eventVersionChanged, eventRemove:
		// cancel all the current tasks
		if p.cancel != nil {
			p.cancel()
		}
		p.ctx, p.cancel = context.WithCancel(context.Background())
		ctx = p.ctx

	case eventSettingsChanged:
		// cancel the previous settings changed
		if p.settingCancel != nil {
			p.settingCancel()
		}
		ctx, p.settingCancel = context.WithCancel(p.ctx)

	case eventRerun:
		// cancel the previous rerun
		if p.rerunCancel != nil {
			p.rerunCancel()
			p.rerunCancel = nil
		}
		ctx, p.rerunCancel = context.WithCancel(p.ctx)

	case eventStartup:
		// cancel disable task
		if p.disableCancel != nil {
			p.disableCancel()
			p.disableCancel = nil
		}
		ctx, p.startupCancel = context.WithCancel(p.ctx)

	case eventDisable:
		// cancel startup task
		if p.startupCancel != nil {
			p.startupCancel()
		}
		ctx, p.disableCancel = context.WithCancel(p.ctx)
	}

	return ctx
}

func (p *lifecyclePackage) versionChanged(version string) bool {
	if p.version != version {
		p.version = version
		return true
	}

	return false
}

func (p *lifecyclePackage) settingsChanged(settings addonutils.Values) bool {
	if len(p.settings) == 0 {
		p.settings = make(addonutils.Values)
	}

	if len(settings) == 0 {
		settings = make(addonutils.Values)
	}

	if settings.Checksum() != p.settings.Checksum() {
		p.settings = settings
		return true
	}

	return false
}

type dump struct {
	Packages map[string]packageDump `json:"packages" yaml:"packages"`
}

type packageDump struct {
	status.Status
	apps.Info
}

// Dump returns a YAML snapshot of all packages and their current state.
//
// Includes for each package:
//   - Status: Current phase (Pending/Loaded/Running)
//   - State: Scheduler state (enabled/disabled with reason)
//   - Info: Instance name and namespace, current package configuration values and hooks
//
// Used for debugging and introspection of operator internal state.
// Skips packages that have been removed from the manager.
func (o *Operator) Dump() []byte {
	o.mu.Lock()
	defer o.mu.Unlock()

	d := dump{
		Packages: make(map[string]packageDump),
	}

	for name := range o.packages {
		d.Packages[name] = packageDump{
			o.status.GetStatus(name),
			o.manager.GetAppInfo(name),
		}
	}

	marshalled, _ := yaml.Marshal(d)
	return marshalled
}
