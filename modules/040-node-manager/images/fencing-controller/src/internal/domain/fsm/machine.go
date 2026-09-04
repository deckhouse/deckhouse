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
	"errors"
	"fmt"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	v1alpha1 "fencing-controller/api/node-manager.deckhouse.io/v1alpha1"
)

// Params are the profile timings a transition depends on. The controller
// resolves them from the FencingSLAProfile named by spec.profileRef.name and
// passes them into every decision: they are inputs of the choice only and are
// never written back to the object.
type Params struct {
	// FallbackTTL is fallback.ttl of the profile: how long a heartbeat of the
	// affected Node keeps blocking evacuation.
	FallbackTTL time.Duration
	// EvacuationDelay is evacuation.delay of the profile: how long the
	// controller waits after the detected failure before pods may be deleted.
	EvacuationDelay time.Duration
}

// resolved reports whether both timings came from a profile the controller could
// read. Without them no transition is chosen at all, because the fast eviction
// path must never start on an incomplete configuration.
func (p Params) resolved() bool {
	return p.FallbackTTL > 0 && p.EvacuationDelay > 0
}

// FSM is the machine of a single incident. It is not safe for concurrent use:
// every reconcile builds its own from the object it observes.
type FSM struct {
	state State
}

// NewFSMFromCR restores the machine from status.phase of the incident. An empty
// phase is the entry state S0_HEALTHY, which is what an object the controller
// has not written yet looks like.
func NewFSMFromCR(incident *v1alpha1.FencingFailedNodeState) (*FSM, error) {
	if incident == nil {
		return nil, errors.New("fencingfailednodestate is nil")
	}

	if incident.Status.Phase == "" {
		return &FSM{state: StateHealthy}, nil
	}

	state := State(incident.Status.Phase)
	if _, known := transitions[state]; !known {
		return nil, fmt.Errorf("phase %q is not a state of the fencing state machine", incident.Status.Phase)
	}

	return &FSM{state: state}, nil
}

// State returns the state the machine is in.
func (f *FSM) State() State {
	return f.state
}

// Advance crosses every arrow the decision flow of the ADR justifies for the
// observed incident and returns the events it fired, in order. It returns
// nothing when the observed signals justify no transition, and the machine then
// keeps its state until the object changes or a deadline passes.
//
// One observation can cross more than one arrow: an incident first seen after
// its evacuation delay already elapsed, for example after the controller
// restarted, enters S1_SUSPECTED and moves on to S3_READY_TO_EVICT at once.
func (f *FSM) Advance(incident *v1alpha1.FencingFailedNodeState, params Params, now time.Time) []Event {
	var fired []Event

	// The observation does not change while the arrows are crossed, so the walk
	// is finite; the bound only keeps a future table edit from looping forever.
	for range len(transitions) {
		event, ok := f.step(incident, params, now)
		if !ok {
			break
		}

		fired = append(fired, event)
	}

	return fired
}

// step crosses the strongest arrow of the current state, if the observed signals
// justify one.
func (f *FSM) step(incident *v1alpha1.FencingFailedNodeState, params Params, now time.Time) (Event, bool) {
	for _, event := range candidates(incident, params, now) {
		next, ok := Transition(f.state, event)
		if !ok {
			continue
		}

		f.state = next

		return event, true
	}

	return "", false
}

// RequeueAfter returns the delay after which time alone can change the decision,
// or zero when the machine only waits for the next update of the object.
func (f *FSM) RequeueAfter(incident *v1alpha1.FencingFailedNodeState, params Params, now time.Time) time.Duration {
	// Both deadlines below matter only where the elapsed evacuation delay is an
	// arrow of the table, which is exactly S1_SUSPECTED and S2_FALLBACK_ALIVE.
	if _, ok := Transition(f.state, EventEvacuationDelayElapsed); !ok || !params.resolved() {
		return 0
	}

	var nearest time.Duration

	consider := func(deadline time.Time) {
		left := deadline.Sub(now)
		if left <= 0 {
			return
		}

		if nearest == 0 || left < nearest {
			nearest = left
		}
	}

	if at, ok := heartbeatAt(incident.Status.Fallback); ok {
		consider(at.Add(params.FallbackTTL))
	}

	if at, ok := detectedAt(incident.Status.Failed); ok {
		consider(at.Add(params.EvacuationDelay))
	}

	return nearest
}

// candidates lists the events the observed signals justify, strongest first, and
// the caller crosses the first of them that is an arrow of the current state. A
// fresh fallback heartbeat always outweighs the failed section, so while the
// affected Node keeps confirming that it reaches the Kubernetes API nothing can
// move the machine towards evacuation.
func candidates(incident *v1alpha1.FencingFailedNodeState, params Params, now time.Time) []Event {
	// Unresolved timings mean the profile of the incident could not be read, and
	// the fast eviction path must not start on an incomplete configuration.
	if !params.resolved() {
		return nil
	}

	if fallbackIsFresh(incident.Status.Fallback, params.FallbackTTL, now) {
		return []Event{EventFallbackFresh}
	}

	failed := incident.Status.Failed
	if failed == nil {
		return nil
	}

	events := make([]Event, 0, 3)

	if evacuationDelayElapsed(failed, params.EvacuationDelay, now) {
		events = append(events, EventEvacuationDelayElapsed)
	}

	// The failed section alone: an object that just appeared enters
	// S1_SUSPECTED, and a Node whose fallback went stale returns to it.
	return append(events, EventFailedDetected, EventFallbackStale)
}

// fallbackIsFresh reports whether the affected Node confirmed within the TTL
// that it still reaches the Kubernetes API. The ADR ties the block on
// evacuation to the age of the heartbeat, so active is left informational: a
// Node that stopped writing heartbeats loses the protection either way.
func fallbackIsFresh(fallback *v1alpha1.FencingFailedNodeStateFallback, ttl time.Duration, now time.Time) bool {
	at, ok := heartbeatAt(fallback)
	if !ok {
		return false
	}

	return now.Sub(at) < ttl
}

func evacuationDelayElapsed(failed *v1alpha1.FencingFailedNodeStateFailed, delay time.Duration, now time.Time) bool {
	at, ok := detectedAt(failed)
	if !ok {
		return false
	}

	return now.Sub(at) >= delay
}

func heartbeatAt(fallback *v1alpha1.FencingFailedNodeStateFallback) (time.Time, bool) {
	if fallback == nil {
		return time.Time{}, false
	}

	return timeOf(fallback.LastHeartbeatAt)
}

func detectedAt(failed *v1alpha1.FencingFailedNodeStateFailed) (time.Time, bool) {
	if failed == nil {
		return time.Time{}, false
	}

	return timeOf(&failed.DetectedAt)
}

// timeOf treats an unset timestamp as no evidence: a writer that has not filled
// it in yet must not look like a failure detected at the zero time.
func timeOf(t *metav1.Time) (time.Time, bool) {
	if t == nil || t.IsZero() {
		return time.Time{}, false
	}

	return t.Time, true
}
