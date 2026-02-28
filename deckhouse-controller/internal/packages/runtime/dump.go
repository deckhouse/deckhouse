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
	"sigs.k8s.io/yaml"

	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/apps"
	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/modules"
	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/status"
)

// dump is the serialization envelope for the debug endpoint.
type dump struct {
	Apps    map[string]appDump    `json:"apps" yaml:"apps"`
	Modules map[string]moduleDump `json:"modules" yaml:"modules"`
}

// appDump combines status conditions and package info for a single app.
type appDump struct {
	status.Status
	apps.Info
}

// moduleDump combines status conditions and package info for a single module.
type moduleDump struct {
	status.Status
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
			r.status.GetStatus(app.GetName()),
			app.GetInfo(),
		}
	}

	for _, module := range r.modules {
		d.Modules[module.GetName()] = moduleDump{
			r.status.GetStatus(module.GetName()),
			module.GetInfo(),
		}
	}

	marshalled, _ := yaml.Marshal(d)
	return marshalled
}
