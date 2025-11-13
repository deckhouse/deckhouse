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
	"log/slog"

	addonutils "github.com/flant/addon-operator/pkg/utils"
	"sigs.k8s.io/yaml"

	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/manager/apps"
	taskapplysettings "github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/operator/tasks/applysettings"
	taskdisable "github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/operator/tasks/disable"
	taskinstall "github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/operator/tasks/install"
	taskload "github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/operator/tasks/load"
	taskrun "github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/operator/tasks/run"
	taskuninstall "github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/operator/tasks/uninstall"
	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/schedule"
	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/queue"
	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/registry"
	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/apis/deckhouse.io/v1alpha1"
)

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
func (o *Operator) Update(repo *v1alpha1.PackageRepository, inst Instance) {
	o.mu.Lock()
	defer o.mu.Unlock()

	if inst.Namespace == "" {
		inst.Namespace = "default"
	}

	name := apps.BuildName(inst.Namespace, inst.Name)

	if _, ok := o.packages[name]; !ok {
		o.packages[name] = &Package{
			name: name,
			status: Status{
				Phase: Pending,
			},
		}
	}

	// Cancel previous tasks before enqueueing new ones
	ctx := o.packages[name].renewContext()

	if o.packages[name].status.Phase == Pending {
		packageName := inst.Definition.Name
		packageVersion := inst.Definition.Version
		reg := registry.BuildRegistryByRepository(repo)

		o.queueService.Enqueue(ctx, name, taskinstall.NewTask(name, packageName, packageVersion, reg, o.installer, o.logger))
		o.queueService.Enqueue(ctx, name, taskload.NewTask(name, inst.Settings, o.manager, o.logger),
			queue.WithOnDone(func() {
				o.mu.Lock()
				o.packages[name].status.Phase = Loaded
				o.mu.Unlock()

				o.scheduler.Add(o.manager.GetApplication(name))
			}))

		return
	}

	if o.manager.SettingsChanged(name, inst.Settings) {
		o.queueService.Enqueue(ctx, name, taskapplysettings.NewTask(name, inst.Settings, o.manager, o.logger))
		o.queueService.Enqueue(ctx, name, taskrun.NewTask(name, o.manager, o.logger), queue.WithUnique())
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

	app := o.packages[name]
	if app == nil {
		return
	}

	// stop getting enabling/disabling event
	o.scheduler.Remove(name)

	// Capture queues before manager removes the app metadata
	queues := o.manager.GetPackageQueues(name)

	ctx := app.renewContext()
	o.queueService.Enqueue(ctx, name, taskdisable.NewTask(name, o.manager, false, o.logger), queue.WithOnDone(func() {
		for _, q := range queues {
			if q == "main" || q == name {
				continue
			}

			o.logger.Debug("remove package queue", slog.String("name", name), slog.String("queue", q))
			o.queueService.Remove(q)
		}

		o.mu.Lock()
		delete(o.packages, name)
		o.mu.Unlock()
	}))

	o.queueService.Enqueue(ctx, name, taskuninstall.NewTask(name, o.installer, o.logger), queue.WithOnDone(func() {
		// Remove package's main queue after uninstall completes
		go o.queueService.Remove(name)
	}))
}

type dump struct {
	Packages map[string]packageDump `json:"packages" yaml:"packages"`
}

type packageDump struct {
	Status
	schedule.State
	addonutils.Values `yaml:"values,omitempty" json:"values,omitempty"`
}

// Dump returns a YAML snapshot of all packages and their current state.
//
// Includes for each package:
//   - Status: Current phase (Pending/Loaded/Running)
//   - State: Scheduler state (enabled/disabled with reason)
//   - Values: Current package configuration values
//
// Used for debugging and introspection of operator internal state.
// Skips packages that have been removed from the manager.
func (o *Operator) Dump() []byte {
	o.mu.Lock()
	defer o.mu.Unlock()

	d := dump{
		Packages: make(map[string]packageDump),
	}

	for name, pkg := range o.packages {
		app := o.manager.GetApplication(name)
		if app == nil {
			continue
		}

		d.Packages[name] = packageDump{
			pkg.status,
			o.scheduler.State(name),
			app.GetValues().GetKeySection(name),
		}
	}

	marshalled, _ := yaml.Marshal(d)
	return marshalled
}
