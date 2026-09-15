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

// Package fsm implements the state machine fencing-controller runs for one
// FencingFailedNodeState. It decides transitions only: reading and writing the
// object, deleting pods and publishing events stay with the reconciler.
package fsm

import (
	v1alpha1 "fencing-controller/api/node-manager.deckhouse.io/v1alpha1"
)

// State is a state of the machine. Its values are the phases of
// FencingFailedNodeState.status.phase, so a controller that restarted in the
// middle of an incident restores the machine from the object it observes.
type State string

const (
	// StateHealthy is S0_HEALTHY, the entry state. It is never stored: a healthy
	// Node has no FencingFailedNodeState, so absence of the object is the
	// healthy signal.
	StateHealthy = State(v1alpha1.PhaseHealthy)
	// StateSuspected is S1_SUSPECTED: failure evidence exists and the evacuation
	// delay of the profile has not elapsed yet.
	StateSuspected = State(v1alpha1.PhaseSuspected)
	// StateFallbackAlive is S2_FALLBACK_ALIVE: the affected Node lost quorum but
	// confirmed within the fallback TTL that it still reaches the Kubernetes API.
	StateFallbackAlive = State(v1alpha1.PhaseFallbackAlive)
	// StateReadyToEvict is S3_READY_TO_EVICT: the conditions for evacuation hold.
	StateReadyToEvict = State(v1alpha1.PhaseReadyToEvict)
	// StateEvicting is S4_EVICTING: deletion of the pods of the Node is running.
	StateEvicting = State(v1alpha1.PhaseEvicting)
	// StateDone is S5_DONE: the operations finished.
	StateDone = State(v1alpha1.PhaseDone)
	// StateError is S_ERR: a reconcile error to retry with backoff, and the
	// state of an object whose reference to its Node is invalid or stale.
	StateError = State(v1alpha1.PhaseError)
)

// Phase returns the value the state is stored as in status.phase.
func (s State) Phase() v1alpha1.FencingFailedNodeStatePhase {
	return v1alpha1.FencingFailedNodeStatePhase(s)
}

// Event triggers a transition.
type Event string

const (
	// EventFailedDetected: the object carries a failed section.
	EventFailedDetected Event = "FailedDetected"
	// EventFallbackFresh: the affected Node confirmed liveness within the
	// fallback TTL of the profile. It always outweighs the failed section.
	EventFallbackFresh Event = "FallbackFresh"
	// EventFallbackStale: the fallback heartbeat is missing or older than the
	// fallback TTL, while the failed section remains.
	EventFallbackStale Event = "FallbackStale"
	// EventEvacuationDelayElapsed: the failure is older than the evacuation
	// delay of the profile and no fresh fallback protects the Node.
	EventEvacuationDelayElapsed Event = "EvacuationDelayElapsed"
	// EventEvictionStarted: deletion of the pods of the Node started.
	EventEvictionStarted Event = "EvictionStarted"
	// EventEvictionCompleted: deletion finished.
	EventEvictionCompleted Event = "EvictionCompleted"
	// EventReconcileFailed: an API error or a conflict interrupted the eviction.
	EventReconcileFailed Event = "ReconcileFailed"
	// EventRetryAfterBackoff: the failed eviction is retried.
	EventRetryAfterBackoff Event = "RetryAfterBackoff"
	// EventInvalidNodeReference: the object does not identify a live Node, so
	// evacuation is forbidden from whatever state the machine is in.
	EventInvalidNodeReference Event = "InvalidNodeReference"
	// EventStateDeleted: the recovered Node deleted its own object.
	EventStateDeleted Event = "StateDeleted"
)

// transitions is the transition table of the ADR and nothing more: every entry
// is one arrow of its state diagram, and no arrow the ADR does not describe is
// added. Every state is a key, so an unknown phase is caught by a lookup.
var transitions = map[State]map[Event]State{
	StateHealthy: {
		EventFailedDetected: StateSuspected,     // S0 -> S1
		EventFallbackFresh:  StateFallbackAlive, // S0 -> S2
	},
	StateSuspected: {
		EventFallbackFresh:          StateFallbackAlive, // S1 -> S2
		EventEvacuationDelayElapsed: StateReadyToEvict,  // S1 -> S3
		EventStateDeleted:           StateHealthy,       // S1 -> S0
	},
	StateFallbackAlive: {
		EventFallbackStale:          StateSuspected,    // S2 -> S1
		EventEvacuationDelayElapsed: StateReadyToEvict, // S2 -> S3
		EventStateDeleted:           StateHealthy,      // S2 -> S0
	},
	StateReadyToEvict: {
		EventEvictionStarted: StateEvicting, // S3 -> S4
	},
	StateEvicting: {
		EventEvictionCompleted: StateDone,  // S4 -> S5
		EventReconcileFailed:   StateError, // S4 -> S_ERR
	},
	// S5_DONE is terminal.
	StateDone: {},
	StateError: {
		EventRetryAfterBackoff: StateReadyToEvict, // S_ERR -> S3
		EventEvictionStarted:   StateEvicting,     // S_ERR -> S4
	},
}

// Transition returns the state the ADR describes for the pair, or false when the
// ADR describes no such arrow.
func Transition(from State, event Event) (State, bool) {
	arrows, known := transitions[from]
	if !known {
		return "", false
	}

	// The ADR allows the terminal skip on an invalid or stale Node reference
	// from any state, so it is not repeated per state in the table above. The
	// lookup above still runs first: a phase that is not a state of the machine
	// leads nowhere, not even to the skip.
	if event == EventInvalidNodeReference {
		return StateError, true
	}

	next, ok := arrows[event]

	return next, ok
}
