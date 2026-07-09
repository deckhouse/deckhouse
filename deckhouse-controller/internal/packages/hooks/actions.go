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

package hooks

import (
	"fmt"
	"strings"

	gohook "github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/shell-operator/pkg/hook/controller"
	shtypes "github.com/flant/shell-operator/pkg/hook/types"
)

// ApplyBindingActions reconfigures a hook's dynamic Kubernetes monitors in
// response to the BindingActions a Go hook returns. A hook may re-point a
// binding at a different Kind/ApiVersion ("UpdateKind") or switch it off
// ("Disable") at runtime: node-manager's get_crds.go, for example, declares its
// instance-class binding with an empty Kind and only learns the real kind
// (e.g. DVPInstanceClass) once it has read the cloud-provider Secret.
//
// Without this the monitor keeps its registration-time (empty) Kind, its
// snapshot stays empty, and the hook fails. Mirrors addon-operator's
// ModuleHook.ApplyBindingActions.
func ApplyBindingActions(bindings []shtypes.OnKubernetesEventConfig, ctrl *controller.HookController, actions []gohook.BindingAction) error {
	for _, action := range actions {
		monitorID, ok := monitorIDForBinding(bindings, action.Name)

		if !ok {
			continue
		}

		switch strings.ToLower(action.Action) {
		case "disable", "updatekind":
			// UpdateMonitor stops the old monitor, repoints it at the new
			// kind/apiVersion (both empty for Disable), recreates it, emits a
			// synthetic Added event and unlocks events. It is a no-op if the
			// monitor link is unknown.
			if err := ctrl.UpdateMonitor(monitorID, action.Kind, action.ApiVersion); err != nil {
				return fmt.Errorf("update monitor for binding %q: %w", action.Name, err)
			}
		}
	}

	return nil
}

// monitorIDForBinding returns the monitor id of the kube binding with the given
// name, or false if no such binding exists or it carries no monitor.
func monitorIDForBinding(bindings []shtypes.OnKubernetesEventConfig, name string) (string, bool) {
	for _, binding := range bindings {
		if binding.BindingName != name || binding.Monitor == nil {
			continue
		}

		return binding.Monitor.Metadata.MonitorId, true
	}

	return "", false
}
