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

// dump is the serialization envelope for the packages endpoint.
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
// global module has not been initialized (r.global is nil), which the handler
// serves as a null document.
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

	snapshot := dump{
		Apps:    make(map[string]appDump, len(r.apps)),
		Modules: make(map[string]moduleDump, len(r.modules)),
	}

	for _, application := range r.apps {
		snapshot.Apps[application.GetName()] = appDump{
			Status: r.status.GetStatus(application.GetName()),
			Info:   application.GetInfo(),
		}
	}

	for _, module := range r.modules {
		snapshot.Modules[module.GetName()] = moduleDump{
			Status: r.status.GetStatus(module.GetName()),
			Info:   module.GetInfo(),
		}
	}

	return snapshot
}

// DumpByName returns a snapshot of a single package by name.
// Checks apps first, then modules. Returns nil if not found.
func (r *Runtime) DumpByName(name string) any {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if application := r.apps[name]; application != nil {
		return appDump{
			Status: r.status.GetStatus(application.GetName()),
			Info:   application.GetInfo(),
		}
	}

	if module := r.modules[name]; module != nil {
		return moduleDump{
			Status: r.status.GetStatus(module.GetName()),
			Info:   module.GetInfo(),
		}
	}

	return nil
}

// Snapshots returns the hook snapshots of a package, reporting false when no
// package with that name is loaded. The lookup runs under the lock and the dump
// outside it, because collecting snapshots walks the hooks of the package.
func (r *Runtime) Snapshots(name string) (any, bool) {
	r.mu.RLock()
	application := r.apps[name]
	module := r.modules[name]
	globalModule := r.global
	r.mu.RUnlock()

	switch {
	case application != nil:
		return application.GetHookSnapshotsDump(), true
	case module != nil:
		return module.GetHookSnapshotsDump(), true
	// The global module is absent until the runtime initializes it.
	case globalModule != nil && name == globalModule.GetName():
		return globalModule.GetHookSnapshotsDump(), true
	}

	return nil, false
}

// Render renders the Helm chart of a loaded package.
func (r *Runtime) Render(ctx context.Context, name string) (string, error) {
	r.mu.Lock()

	if application := r.apps[name]; application != nil {
		r.mu.Unlock()
		return r.nelmService.Render(ctx, application.GetNamespace(), application)
	}

	if module := r.modules[name]; module != nil {
		r.mu.Unlock()
		return r.nelmService.Render(ctx, app.NamespaceDeckhouse, module)
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

	if application := r.apps[name]; application != nil {
		queues = append(queues, application.GetName())
		for _, hookQueue := range application.GetHooksQueues() {
			queues = append(queues, filepath.Join(name, hookQueue))
			queues = append(queues, filepath.Join(name, hookQueue, "sync"))
		}
	}

	if module := r.modules[name]; module != nil {
		queues = append(queues, module.GetName())
		for _, hookQueue := range module.GetHooksQueues() {
			queues = append(queues, filepath.Join(name, hookQueue))
			queues = append(queues, filepath.Join(name, hookQueue, "sync"))
		}
		// The CRD subtask queue is spawned once per module by the global run task,
		// as "<name>/crd" — it is not a per-hook-queue subqueue.
		queues = append(queues, filepath.Join(name, "crd"))
	}

	return queues
}
