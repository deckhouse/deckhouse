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

package fsm

import (
	"testing"

	v1alpha1 "fencing-controller/api/node-manager.deckhouse.io/v1alpha1"
)

// adrStates is the state list of the ADR, S0 to S_ERR.
var adrStates = []State{
	StateHealthy,
	StateSuspected,
	StateFallbackAlive,
	StateReadyToEvict,
	StateEvicting,
	StateDone,
	StateError,
}

// adrEvents is every trigger the transitions of the ADR are described with.
var adrEvents = []Event{
	EventFailedDetected,
	EventFallbackFresh,
	EventFallbackStale,
	EventEvacuationDelayElapsed,
	EventEvictionStarted,
	EventEvictionCompleted,
	EventReconcileFailed,
	EventRetryAfterBackoff,
	EventInvalidNodeReference,
	EventStateDeleted,
}

// arrow is one transition of the ADR, quoted by its label in the document.
type arrow struct {
	adr   string
	from  State
	event Event
	to    State
}

// adrArrows is the transition list of the ADR, written out independently of the
// table under test: a transition the ADR does not describe fails the comparison,
// and so does a described transition that went missing.
var adrArrows = []arrow{
	{"S0 -> S1: created CR with status.failed", StateHealthy, EventFailedDetected, StateSuspected},
	{"S0 -> S2: created CR with fresh status.fallback", StateHealthy, EventFallbackFresh, StateFallbackAlive},
	{"S1 -> S2: fresh fallback heartbeat", StateSuspected, EventFallbackFresh, StateFallbackAlive},
	{"S2 -> S1: fallback stale or missing", StateFallbackAlive, EventFallbackStale, StateSuspected},
	{"S1 -> S3: failed age reached the evacuation delay", StateSuspected, EventEvacuationDelayElapsed, StateReadyToEvict},
	{"S2 -> S3: fallback stale and failed exists", StateFallbackAlive, EventEvacuationDelayElapsed, StateReadyToEvict},
	{"S3 -> S4: start pod deletion", StateReadyToEvict, EventEvictionStarted, StateEvicting},
	{"S4 -> S5: deletion complete", StateEvicting, EventEvictionCompleted, StateDone},
	{"S4 -> S_ERR: API or reconcile error", StateEvicting, EventReconcileFailed, StateError},
	{"S_ERR -> S3: retry with backoff", StateError, EventRetryAfterBackoff, StateReadyToEvict},
	{"S_ERR -> S4: retry the deletion itself", StateError, EventEvictionStarted, StateEvicting},
	{"S1 -> S0: CR deleted by recovered node", StateSuspected, EventStateDeleted, StateHealthy},
	{"S2 -> S0: CR deleted by recovered node", StateFallbackAlive, EventStateDeleted, StateHealthy},
}

func TestTransitionsAreExactlyTheOnesTheADRDescribes(t *testing.T) {
	described := make(map[State]map[Event]State, len(adrStates))

	for _, state := range adrStates {
		described[state] = make(map[Event]State)

		// The terminal skip on an invalid or stale Node reference is described
		// for every state, so it is not listed per state above.
		described[state][EventInvalidNodeReference] = StateError
	}

	for _, a := range adrArrows {
		if _, ok := described[a.from][a.event]; ok {
			t.Fatalf("%s: the arrow list describes %s on %s twice", a.adr, a.event, a.from)
		}

		described[a.from][a.event] = a.to
	}

	for _, from := range adrStates {
		for _, event := range adrEvents {
			want, wantOK := described[from][event]

			got, gotOK := Transition(from, event)
			if gotOK != wantOK {
				t.Errorf("Transition(%s, %s) described = %t, ADR describes it = %t", from, event, gotOK, wantOK)

				continue
			}

			if gotOK && got != want {
				t.Errorf("Transition(%s, %s) = %s, ADR leads to %s", from, event, got, want)
			}
		}
	}
}

// TestTransitionRejectsAnUnknownState walks every event, the terminal skip
// included: the skip is described for every state of the ADR, and a phase that
// names no state of the machine is not one of them.
func TestTransitionRejectsAnUnknownState(t *testing.T) {
	for _, event := range adrEvents {
		t.Run(string(event), func(t *testing.T) {
			if got, ok := Transition(State("Draining"), event); ok {
				t.Errorf("Transition from an unknown state on %s returned %s, want no transition", event, got)
			}
		})
	}
}

func TestTransitionRejectsAnUnknownEvent(t *testing.T) {
	if got, ok := Transition(StateSuspected, Event("PodsDrained")); ok {
		t.Errorf("Transition on an unknown event returned %s, want no transition", got)
	}
}

// TestEveryStateIsInTheTable guards NewFSMFromCR: a phase is accepted only if
// the state it names is a key of the table.
func TestEveryStateIsInTheTable(t *testing.T) {
	if len(transitions) != len(adrStates) {
		t.Errorf("the table holds %d states, the ADR describes %d", len(transitions), len(adrStates))
	}

	for _, state := range adrStates {
		if _, ok := transitions[state]; !ok {
			t.Errorf("state %s is missing from the table", state)
		}
	}
}

// TestStatesAndPhasesMatch keeps the machine and the CRD enum in sync: a state
// that cannot be stored could not be restored after a controller restart.
func TestStatesAndPhasesMatch(t *testing.T) {
	phases := []v1alpha1.FencingFailedNodeStatePhase{
		v1alpha1.PhaseHealthy,
		v1alpha1.PhaseSuspected,
		v1alpha1.PhaseFallbackAlive,
		v1alpha1.PhaseReadyToEvict,
		v1alpha1.PhaseEvicting,
		v1alpha1.PhaseDone,
		v1alpha1.PhaseError,
	}

	if len(phases) != len(adrStates) {
		t.Fatalf("the CRD has %d phases, the machine has %d states", len(phases), len(adrStates))
	}

	for i, state := range adrStates {
		if got := state.Phase(); got != phases[i] {
			t.Errorf("state %s is stored as phase %q, want %q", state, got, phases[i])
		}
	}
}
