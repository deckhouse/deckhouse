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
	"path/filepath"

	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/app"
	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/apps"
	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/modules"
	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/modules/global"
	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/status"
)

// dump is the serialization envelope for the debug endpoint.
type dump struct {
	Apps    map[string]appDump    `json:"apps"`
	Modules map[string]moduleDump `json:"modules"`
}

// appDump combines status conditions and package info for a single app.
type appDump struct {
	Status status.Status `json:"status"`
	apps.Info
}

// moduleDump combines status conditions and package info for a single module.
type moduleDump struct {
	Status status.Status `json:"status"`
	modules.Info
}

// globalDump carries the package info and status conditions for the global module.
type globalDump struct {
	Status status.Status `json:"status"`
	global.Info
}

// DumpGlobal returns a snapshot of the global module's package info.
//
// The snapshot mirrors global.Info: instance name, running state, filesystem
// path, current values, and the names of registered hooks. Returns nil when the
// global module has not been initialized (r.global is nil), which the debug
// handler surfaces as an empty body.
func (r *Runtime) DumpGlobal() any {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if r.global == nil {
		return nil
	}

	return globalDump{
		Status: r.status.GetStatus(r.global.GetName()),
		Info:   r.global.GetInfo(),
	}
}

// Dump returns a snapshot of all packages and their current state.
//
// Includes for each package:
//   - Status: Current phase (Pending/Loaded/Running)
//   - State: Scheduler state (enabled/disabled with reason)
//   - Info: Instance name and namespace, current package configuration values and hooks
//
// Used for debugging and introspection of operator internal state.
// Skips packages that have been removed from the manager.
func (r *Runtime) Dump() any {
	r.mu.RLock()
	defer r.mu.RUnlock()

	d := dump{
		Apps:    make(map[string]appDump),
		Modules: make(map[string]moduleDump),
	}

	for _, app := range r.apps {
		d.Apps[app.GetName()] = appDump{
			Status: r.status.GetStatus(app.GetName()),
			Info:   app.GetInfo(),
		}
	}

	for _, module := range r.modules {
		d.Modules[module.GetName()] = moduleDump{
			Status: r.status.GetStatus(module.GetName()),
			Info:   module.GetInfo(),
		}
	}

	return d
}

// DumpByName returns a snapshot of a single package by name.
// Checks apps first, then modules. Returns nil if not found.
func (r *Runtime) DumpByName(name string) any {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if app := r.apps[name]; app != nil {
		return appDump{
			Status: r.status.GetStatus(app.GetName()),
			Info:   app.GetInfo(),
		}
	}

	if mod := r.modules[name]; mod != nil {
		return moduleDump{
			Status: r.status.GetStatus(mod.GetName()),
			Info:   mod.GetInfo(),
		}
	}

	return nil
}

// Snapshots returns the hook snapshots of a package, reporting false when no
// package with that name is loaded.
func (r *Runtime) Snapshots(name string) (any, bool) {
	r.mu.RLock()
	app := r.apps[name]
	mod := r.modules[name]
	r.mu.RUnlock()

	switch {
	case app != nil:
		return app.GetHookSnapshotsDump(), true
	case mod != nil:
		return mod.GetHookSnapshotsDump(), true
	case name == r.global.GetName():
		return r.global.GetHookSnapshotsDump(), true
	}

	return nil, false
}

// Render renders the Helm chart of a loaded package.
func (r *Runtime) Render(ctx context.Context, name string) (string, error) {
	r.mu.Lock()

	if pkg := r.apps[name]; pkg != nil {
		r.mu.Unlock()
		return r.nelmService.Render(ctx, pkg.GetNamespace(), pkg)
	}

	if pkg := r.modules[name]; pkg != nil {
		r.mu.Unlock()
		return r.nelmService.Render(ctx, app.NamespaceDeckhouse, pkg)
	}

	r.mu.Unlock()

	return "", errors.New("no package found")
}

// DumpQueues returns a snapshot of the task queues of one package, or of every
// queue when name is empty.
func (r *Runtime) DumpQueues(name string) any {
	return r.queueService.Dump(r.collectQueues(name)...)
}

// collectQueues expands a package name into all its queue names (main + hook sub-queues).
// Returns nil if name is empty (meaning include all).
func (r *Runtime) collectQueues(name string) []string {
	if name == "" {
		return nil
	}

	r.mu.RLock()
	defer r.mu.RUnlock()

	var queues []string

	if app := r.apps[name]; app != nil {
		queues = append(queues, app.GetName())
		for _, q := range app.GetHooksQueues() {
			queues = append(queues, filepath.Join(name, q))
			queues = append(queues, filepath.Join(name, q, "sync"))
		}
	}

	if mod := r.modules[name]; mod != nil {
		queues = append(queues, mod.GetName())
		for _, q := range mod.GetHooksQueues() {
			queues = append(queues, filepath.Join(name, q))
			queues = append(queues, filepath.Join(name, q, "sync"))
		}
		// The CRD subtask queue is spawned once per module by the global run task,
		// as "<name>/crd" — it is not a per-hook-queue subqueue.
		queues = append(queues, filepath.Join(name, "crd"))
	}

	return queues
}
