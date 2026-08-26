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
	"slices"
	"testing"
	"time"

	equality "k8s.io/apimachinery/pkg/api/equality"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	v1alpha1 "fencing-controller/api/node-manager.deckhouse.io/v1alpha1"
)

// observedAt is the moment every incident below is observed at, and the timings
// are those of the critical profile.
var (
	observedAt = time.Date(2026, time.June, 2, 15, 0, 5, 0, time.UTC)

	critical = Params{FallbackTTL: time.Second, EvacuationDelay: 1200 * time.Millisecond}
)

func TestNewFSMFromCRRestoresEveryPhase(t *testing.T) {
	for _, want := range adrStates {
		t.Run(string(want), func(t *testing.T) {
			machine, err := NewFSMFromCR(incident(want.Phase(), nil, nil))
			if err != nil {
				t.Fatalf("restore from phase %q: %v", want.Phase(), err)
			}

			if got := machine.State(); got != want {
				t.Errorf("restored state %s, want %s", got, want)
			}
		})
	}
}

// TestNewFSMFromCREntersHealthyWithoutPhase covers the object the agent has just
// created: the controller has not written a phase yet, so the machine is at its
// entry state.
func TestNewFSMFromCREntersHealthyWithoutPhase(t *testing.T) {
	machine, err := NewFSMFromCR(incident("", nil, nil))
	if err != nil {
		t.Fatalf("restore without a phase: %v", err)
	}

	if got := machine.State(); got != StateHealthy {
		t.Errorf("restored state %s, want %s", got, StateHealthy)
	}
}

func TestNewFSMFromCRRejectsWhatItCannotRestore(t *testing.T) {
	t.Run("unknown phase", func(t *testing.T) {
		if _, err := NewFSMFromCR(incident("Draining", nil, nil)); err == nil {
			t.Error("restore from an unknown phase succeeded, want an error")
		}
	})

	t.Run("no object", func(t *testing.T) {
		if _, err := NewFSMFromCR(nil); err == nil {
			t.Error("restore from a nil object succeeded, want an error")
		}
	})
}

func TestAdvance(t *testing.T) {
	for name, tc := range map[string]struct {
		phase        v1alpha1.FencingFailedNodeStatePhase
		failedAgo    *time.Duration
		heartbeatAgo *time.Duration
		wantEvents   []Event
		wantState    State
	}{
		"object appears with failure evidence": {
			failedAgo:  ago(200 * time.Millisecond),
			wantEvents: []Event{EventFailedDetected},
			wantState:  StateSuspected,
		},
		"object appears with a fresh heartbeat": {
			heartbeatAgo: ago(200 * time.Millisecond),
			wantEvents:   []Event{EventFallbackFresh},
			wantState:    StateFallbackAlive,
		},
		"a fresh heartbeat outweighs an elapsed delay": {
			failedAgo:    ago(30 * time.Second),
			heartbeatAgo: ago(200 * time.Millisecond),
			wantEvents:   []Event{EventFallbackFresh},
			wantState:    StateFallbackAlive,
		},
		"suspected waits while the delay runs": {
			phase:      v1alpha1.PhaseSuspected,
			failedAgo:  ago(500 * time.Millisecond),
			wantEvents: nil,
			wantState:  StateSuspected,
		},
		"suspected reaches the delay": {
			phase:      v1alpha1.PhaseSuspected,
			failedAgo:  ago(1200 * time.Millisecond),
			wantEvents: []Event{EventEvacuationDelayElapsed},
			wantState:  StateReadyToEvict,
		},
		"suspected is protected by a new heartbeat": {
			phase:        v1alpha1.PhaseSuspected,
			failedAgo:    ago(30 * time.Second),
			heartbeatAgo: ago(100 * time.Millisecond),
			wantEvents:   []Event{EventFallbackFresh},
			wantState:    StateFallbackAlive,
		},
		"heartbeat exactly as old as the ttl is stale": {
			phase:        v1alpha1.PhaseFallbackAlive,
			failedAgo:    ago(500 * time.Millisecond),
			heartbeatAgo: ago(time.Second),
			wantEvents:   []Event{EventFallbackStale},
			wantState:    StateSuspected,
		},
		"stale heartbeat past the delay goes straight to eviction": {
			phase:        v1alpha1.PhaseFallbackAlive,
			failedAgo:    ago(2 * time.Second),
			heartbeatAgo: ago(2 * time.Second),
			wantEvents:   []Event{EventEvacuationDelayElapsed},
			wantState:    StateReadyToEvict,
		},
		"stale heartbeat without failure evidence waits": {
			phase:        v1alpha1.PhaseFallbackAlive,
			heartbeatAgo: ago(30 * time.Second),
			wantEvents:   nil,
			wantState:    StateFallbackAlive,
		},
		// A controller that was down while the delay elapsed sees the incident
		// for the first time past its deadline.
		"late first observation crosses two arrows": {
			failedAgo:  ago(30 * time.Second),
			wantEvents: []Event{EventFailedDetected, EventEvacuationDelayElapsed},
			wantState:  StateReadyToEvict,
		},
		// The ADR describes no arrow out of S3 on either signal, so an incident
		// that is ready to evict stays there until the eviction path moves it.
		"ready to evict ignores an even older failure": {
			phase:      v1alpha1.PhaseReadyToEvict,
			failedAgo:  ago(30 * time.Second),
			wantEvents: nil,
			wantState:  StateReadyToEvict,
		},
		"ready to evict is not pulled back by a fresh heartbeat": {
			phase:        v1alpha1.PhaseReadyToEvict,
			failedAgo:    ago(30 * time.Second),
			heartbeatAgo: ago(100 * time.Millisecond),
			wantEvents:   nil,
			wantState:    StateReadyToEvict,
		},
		"done is terminal": {
			phase:      v1alpha1.PhaseDone,
			failedAgo:  ago(30 * time.Second),
			wantEvents: nil,
			wantState:  StateDone,
		},
	} {
		t.Run(name, func(t *testing.T) {
			machine := restore(t, incident(tc.phase, tc.failedAgo, tc.heartbeatAgo))

			got := machine.Advance(incident(tc.phase, tc.failedAgo, tc.heartbeatAgo), critical, observedAt)

			if !slices.Equal(got, tc.wantEvents) {
				t.Errorf("fired %v, want %v", got, tc.wantEvents)
			}

			if machine.State() != tc.wantState {
				t.Errorf("ended in %s, want %s", machine.State(), tc.wantState)
			}
		})
	}
}

// TestAdvanceWaitsWhileTheProfileIsUnresolved is the configuration error case:
// without the timings of the profile the fast eviction path must not start, no
// matter how old the failure is.
func TestAdvanceWaitsWhileTheProfileIsUnresolved(t *testing.T) {
	for name, params := range map[string]Params{
		"nothing resolved":       {},
		"no fallback ttl":        {EvacuationDelay: 1200 * time.Millisecond},
		"no evacuation delay":    {FallbackTTL: time.Second},
		"negative fallback ttl":  {FallbackTTL: -time.Second, EvacuationDelay: 1200 * time.Millisecond},
		"negative eviction wait": {FallbackTTL: time.Second, EvacuationDelay: -time.Second},
	} {
		t.Run(name, func(t *testing.T) {
			state := incident(v1alpha1.PhaseSuspected, ago(30*time.Second), nil)
			machine := restore(t, state)

			if fired := machine.Advance(state, params, observedAt); len(fired) > 0 {
				t.Errorf("fired %v on an unresolved profile, want nothing", fired)
			}

			if machine.State() != StateSuspected {
				t.Errorf("ended in %s, want the incident to stay in %s", machine.State(), StateSuspected)
			}
		})
	}
}

// TestAdvanceIsNotEvidenceWithoutTimestamps guards against a half-written
// section: a zero timestamp must not read as a failure detected long ago.
func TestAdvanceIsNotEvidenceWithoutTimestamps(t *testing.T) {
	state := incident("", nil, nil)
	state.Status.Failed = &v1alpha1.FencingFailedNodeStateFailed{DetectedBy: "worker-1"}
	state.Status.Fallback = &v1alpha1.FencingFailedNodeStateFallback{Active: true}

	machine := restore(t, state)

	if fired := machine.Advance(state, critical, observedAt); !slices.Equal(fired, []Event{EventFailedDetected}) {
		t.Errorf("fired %v, want only %s: an unset detection time cannot have elapsed", fired, EventFailedDetected)
	}
}

// TestAdvanceDoesNotTouchTheObject checks the timings are decision inputs only:
// the spec of an incident is immutable and its status belongs to the writers.
func TestAdvanceDoesNotTouchTheObject(t *testing.T) {
	state := incident(v1alpha1.PhaseFallbackAlive, ago(30*time.Second), ago(30*time.Second))
	before := state.DeepCopy()

	machine := restore(t, state)
	machine.Advance(state, critical, observedAt)
	machine.RequeueAfter(state, critical, observedAt)

	if !equality.Semantic.DeepEqual(before, state) {
		t.Errorf("the object changed:\nbefore %+v\nafter  %+v", before, state)
	}
}

func TestRequeueAfter(t *testing.T) {
	for name, tc := range map[string]struct {
		phase        v1alpha1.FencingFailedNodeStatePhase
		failedAgo    *time.Duration
		heartbeatAgo *time.Duration
		params       Params
		want         time.Duration
	}{
		"suspected waits out the rest of the delay": {
			phase:     v1alpha1.PhaseSuspected,
			failedAgo: ago(200 * time.Millisecond),
			params:    critical,
			want:      time.Second,
		},
		"fallback alive wakes up when the heartbeat expires first": {
			phase:        v1alpha1.PhaseFallbackAlive,
			failedAgo:    ago(200 * time.Millisecond),
			heartbeatAgo: ago(200 * time.Millisecond),
			params:       critical,
			want:         800 * time.Millisecond,
		},
		"a deadline in the past needs no timer": {
			phase:     v1alpha1.PhaseSuspected,
			failedAgo: ago(30 * time.Second),
			params:    critical,
			want:      0,
		},
		"nothing to wait for without failure evidence": {
			phase:  v1alpha1.PhaseSuspected,
			params: critical,
			want:   0,
		},
		"states with no timing arrow do not wake up": {
			phase:     v1alpha1.PhaseReadyToEvict,
			failedAgo: ago(200 * time.Millisecond),
			params:    critical,
			want:      0,
		},
		"an unresolved profile has no deadline": {
			phase:     v1alpha1.PhaseSuspected,
			failedAgo: ago(200 * time.Millisecond),
			want:      0,
		},
	} {
		t.Run(name, func(t *testing.T) {
			state := incident(tc.phase, tc.failedAgo, tc.heartbeatAgo)

			if got := restore(t, state).RequeueAfter(state, tc.params, observedAt); got != tc.want {
				t.Errorf("RequeueAfter = %s, want %s", got, tc.want)
			}
		})
	}
}

func restore(t *testing.T, state *v1alpha1.FencingFailedNodeState) *FSM {
	t.Helper()

	machine, err := NewFSMFromCR(state)
	if err != nil {
		t.Fatalf("restore state machine: %v", err)
	}

	return machine
}

// incident builds an object observed at observedAt. A nil age leaves the section
// out, which is how a writer that owns it has not reported anything yet.
func incident(
	phase v1alpha1.FencingFailedNodeStatePhase,
	failedAgo, heartbeatAgo *time.Duration,
) *v1alpha1.FencingFailedNodeState {
	state := &v1alpha1.FencingFailedNodeState{
		ObjectMeta: metav1.ObjectMeta{Name: "worker-3"},
		Spec: v1alpha1.FencingFailedNodeStateSpec{
			NodeGroup:  "worker",
			ProfileRef: v1alpha1.ProfileRef{Name: v1alpha1.ProfileCritical},
		},
		Status: v1alpha1.FencingFailedNodeStateStatus{Phase: phase},
	}

	if failedAgo != nil {
		state.Status.Failed = &v1alpha1.FencingFailedNodeStateFailed{
			DetectedAt: metav1.NewTime(observedAt.Add(-*failedAgo)),
			DetectedBy: "worker-1",
			Reason:     v1alpha1.FailedReasonMemberlistDead,
			AliveCount: 3,
			QuorumSize: 3,
		}
	}

	if heartbeatAgo != nil {
		at := metav1.NewTime(observedAt.Add(-*heartbeatAgo))
		state.Status.Fallback = &v1alpha1.FencingFailedNodeStateFallback{
			Active:                   true,
			LastHeartbeatAt:          &at,
			HeartbeatIntervalSeconds: 1,
		}
	}

	return state
}

func ago(d time.Duration) *time.Duration {
	return &d
}
