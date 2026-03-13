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

	"sigs.k8s.io/yaml"

	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/apps"
	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/modules"
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

// Dump returns a YAML snapshot of all packages and their current state.
//
// Includes for each package:
//   - Status: Current phase (Pending/Loaded/Running)
//   - State: Scheduler state (enabled/disabled with reason)
//   - Info: Instance name and namespace, current package configuration values and hooks
//
// Used for debugging and introspection of operator internal state.
// Skips packages that have been removed from the manager.
func (r *Runtime) Dump() []byte {
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

	marshalled, _ := yaml.Marshal(d)
	return marshalled
}

// DumpByName returns a YAML snapshot of a single package by name.
// Checks apps first, then modules. Returns an empty dump if not found.
func (r *Runtime) DumpByName(name string) []byte {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var marshalled []byte

	if app := r.apps[name]; app != nil {
		marshalled, _ = yaml.Marshal(appDump{
			r.status.GetStatus(app.GetName()),
			app.GetInfo(),
		})
	}

	if mod := r.modules[name]; mod != nil {
		marshalled, _ = yaml.Marshal(moduleDump{
			r.status.GetStatus(mod.GetName()),
			mod.GetInfo(),
		})
	}

	return marshalled
}

// renderManifests renders the Helm chart for a loaded package. Used by the debug server.
func (r *Runtime) renderManifests(ctx context.Context, name string) (string, error) {
	r.mu.Lock()

	if app := r.apps[name]; app != nil {
		r.mu.Unlock()
		return r.nelmService.Render(ctx, app.GetNamespace(), app)
	}

	if module := r.modules[name]; module != nil {
		r.mu.Unlock()
		return r.nelmService.Render(ctx, modulesNamespace, module)
	}

	r.mu.Unlock()

	return "", errors.New("no package found")
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
			queues = append(queues, fmt.Sprintf("%s/%s", name, q))
			queues = append(queues, fmt.Sprintf("%s/%s/sync", name, q))
		}
	}

	if mod := r.modules[name]; mod != nil {
		queues = append(queues, mod.GetName())
		for _, q := range mod.GetHooksQueues() {
			queues = append(queues, fmt.Sprintf("%s/%s", name, q))
			queues = append(queues, fmt.Sprintf("%s/%s/sync", name, q))
		}
	}

	return queues
}
