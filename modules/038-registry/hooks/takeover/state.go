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

package takeover

import (
	"github.com/deckhouse/deckhouse/modules/038-registry/hooks/helpers"
)

// resolveCurrent returns the phase as currently persisted or derived, applying
// NO transition.
//
// The phase is STICKY: once persisted (registry-takeover secret, then the values
// store) it is authoritative. Only on the first reconcile — when nothing is
// persisted — is the phase DERIVED from the presence of the legacy
// orchestrator's registry-state secret:
//
//   - registry-state present -> Legacy (an upgraded cluster running the old arch)
//   - registry-state absent  -> New    (a fresh install of the new arch)
func resolveCurrent(in Inputs) string {
	switch {
	case in.StoredPhase != "":
		return in.StoredPhase
	case in.PrevPhase != "":
		return in.PrevPhase
	case in.OldStatePresent:
		return helpers.PhaseLegacy
	default:
		return helpers.PhaseNew
	}
}

// Transition resolves the current phase and applies the migrate-driven edges of
// the takeover state machine, returning the next phase. It is pure: the
// flippedAt wall-clock stamp is applied by the hook, not here.
//
//	Legacy        --migrate=true-->  TakingOver
//	TakingOver    --migrate=false--> Legacy        (operator aborted; old never
//	                                                 yielded yet, so it resumes)
//	TakingOver    --VerifyReady-->   New
//	New           (terminal, sticky)
//	CleanupPending(terminal, sticky)
func Transition(in Inputs) string {
	switch resolveCurrent(in) {
	case helpers.PhaseLegacy:
		if in.Migrate {
			return helpers.PhaseTakingOver
		}
		return helpers.PhaseLegacy

	case helpers.PhaseTakingOver:
		if !in.Migrate {
			// Abort/rollback. During TakingOver the legacy orchestrator has not
			// yielded (it yields only on New), so returning to Legacy restores
			// old ownership cleanly. The marker / registry-bashible-config
			// rollback side-effects are handled separately.
			return helpers.PhaseLegacy
		}
		if in.VerifyReady {
			return helpers.PhaseNew
		}
		if in.DeadlineExceeded {
			// Timed out before the new stack proved ready — roll back. (Marker /
			// registry-bashible-config rollback side-effects are handled separately.)
			return helpers.PhaseLegacy
		}
		return helpers.PhaseTakingOver

	case helpers.PhaseNew:
		if in.CleanupReady {
			return helpers.PhaseCleanupPending
		}
		return helpers.PhaseNew

	case helpers.PhaseCleanupPending:
		return helpers.PhaseCleanupPending

	default:
		// Unknown phase -> fail safe to Legacy (never disable the orchestrator).
		return helpers.PhaseLegacy
	}
}
