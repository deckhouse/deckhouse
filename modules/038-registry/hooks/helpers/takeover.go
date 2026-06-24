/*
Copyright 2026 Flant JSC

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

	 http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package helpers

import (
	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
)

// Registry takeover phases. These string values are shared verbatim with the
// Helm templates (registry.internal.takeover.phase) and the registry-takeover
// secret.
const (
	PhaseLegacy         = "Legacy"
	PhaseTakingOver     = "TakingOver"
	PhaseNew            = "New"
	PhaseCleanupPending = "CleanupPending"

	takeoverPhasePath = "registry.internal.takeover.phase"
)

// TakeoverPhase returns the current takeover phase from the values store.
// An unset, empty, or unreadable value is reported as PhaseLegacy — the
// fail-safe that never disables the legacy orchestrator on a missing value.
func TakeoverPhase(input *go_hook.HookInput) string {
	phase, err := GetValue[string](input, takeoverPhasePath)
	if err != nil || phase == "" {
		return PhaseLegacy
	}
	return phase
}

// IsNewArchControl reports whether the new architecture is in control of the
// cluster, i.e. the legacy orchestrator must yield.
func IsNewArchControl(input *go_hook.HookInput) bool {
	switch TakeoverPhase(input) {
	case PhaseNew, PhaseCleanupPending:
		return true
	default:
		return false
	}
}
