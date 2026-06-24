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
	"testing"

	"github.com/deckhouse/deckhouse/modules/038-registry/hooks/helpers"
)

func TestTransition(t *testing.T) {
	cases := []struct {
		name string
		in   Inputs
		want string
	}{
		// --- derive on first reconcile (nothing persisted) ---
		{"fresh install -> New", Inputs{OldStatePresent: false}, helpers.PhaseNew},
		{"upgraded legacy, no migrate -> Legacy", Inputs{OldStatePresent: true}, helpers.PhaseLegacy},

		// --- migrate trigger from Legacy ---
		{"legacy + migrate -> TakingOver", Inputs{StoredPhase: helpers.PhaseLegacy, Migrate: true}, helpers.PhaseTakingOver},
		{"legacy + no migrate -> Legacy", Inputs{StoredPhase: helpers.PhaseLegacy}, helpers.PhaseLegacy},

		// --- TakingOver edges ---
		{"takingOver + migrate + not ready -> TakingOver", Inputs{StoredPhase: helpers.PhaseTakingOver, Migrate: true}, helpers.PhaseTakingOver},
		{"takingOver + migrate + ready -> New", Inputs{StoredPhase: helpers.PhaseTakingOver, Migrate: true, VerifyReady: true}, helpers.PhaseNew},
		{"takingOver + abort (migrate false) -> Legacy", Inputs{StoredPhase: helpers.PhaseTakingOver, Migrate: false}, helpers.PhaseLegacy},

		// --- New -> CleanupPending after the stability window ---
		{"new + cleanupReady -> CleanupPending", Inputs{StoredPhase: helpers.PhaseNew, CleanupReady: true}, helpers.PhaseCleanupPending},
		{"new + not cleanupReady -> New", Inputs{StoredPhase: helpers.PhaseNew, CleanupReady: false}, helpers.PhaseNew},
		{"new + cleanupReady + migrate false -> CleanupPending (migrate irrelevant in New)", Inputs{StoredPhase: helpers.PhaseNew, CleanupReady: true, Migrate: false}, helpers.PhaseCleanupPending},

		// --- terminal stickiness ---
		{"new is sticky even without migrate", Inputs{StoredPhase: helpers.PhaseNew, Migrate: false}, helpers.PhaseNew},
		{"new is sticky with ready", Inputs{StoredPhase: helpers.PhaseNew, VerifyReady: true}, helpers.PhaseNew},
		{"cleanupPending is sticky", Inputs{StoredPhase: helpers.PhaseCleanupPending}, helpers.PhaseCleanupPending},

		// --- precedence: stored over prev over derive ---
		{"stored beats prev", Inputs{StoredPhase: helpers.PhaseNew, PrevPhase: helpers.PhaseLegacy}, helpers.PhaseNew},
		{"prev used when stored empty", Inputs{PrevPhase: helpers.PhaseTakingOver, Migrate: true}, helpers.PhaseTakingOver},
		{"prev TakingOver aborts when migrate false", Inputs{PrevPhase: helpers.PhaseTakingOver, Migrate: false}, helpers.PhaseLegacy},

		// --- deadline rollback ---
		{"takingOver + migrate + not ready + deadline exceeded -> Legacy", Inputs{StoredPhase: helpers.PhaseTakingOver, Migrate: true, DeadlineExceeded: true}, helpers.PhaseLegacy},
		{"takingOver + migrate + ready + deadline exceeded -> New (ready wins)", Inputs{StoredPhase: helpers.PhaseTakingOver, Migrate: true, VerifyReady: true, DeadlineExceeded: true}, helpers.PhaseNew},
		{"takingOver + migrate + not ready + no deadline -> TakingOver", Inputs{StoredPhase: helpers.PhaseTakingOver, Migrate: true, DeadlineExceeded: false}, helpers.PhaseTakingOver},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := Transition(tc.in); got != tc.want {
				t.Errorf("Transition(%+v) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}

func TestResolveCurrentFreshInstallIsNew(t *testing.T) {
	// Fresh install: nothing persisted, no legacy registry-state present.
	// registry-init being present is irrelevant to phase derivation (it is
	// consumed by the PKI hook, not the takeover hook).
	got := resolveCurrent(Inputs{OldStatePresent: false})
	if got != helpers.PhaseNew {
		t.Fatalf("fresh install: resolveCurrent = %q, want %q", got, helpers.PhaseNew)
	}
}

func TestResolveCurrentLegacyClusterIsLegacy(t *testing.T) {
	got := resolveCurrent(Inputs{OldStatePresent: true})
	if got != helpers.PhaseLegacy {
		t.Fatalf("legacy cluster: resolveCurrent = %q, want %q", got, helpers.PhaseLegacy)
	}
}

func TestResolveCurrentStoredPhaseSticky(t *testing.T) {
	got := resolveCurrent(Inputs{StoredPhase: helpers.PhaseNew, OldStatePresent: true})
	if got != helpers.PhaseNew {
		t.Fatalf("stored phase must win: resolveCurrent = %q, want %q", got, helpers.PhaseNew)
	}
}
