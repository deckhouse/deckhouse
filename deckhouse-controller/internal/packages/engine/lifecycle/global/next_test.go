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

package global

import (
	"testing"
	"time"

	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/engine/lifecycle/configstate"
	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/engine/lifecycle/model"
	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/engine/lifecycle/operation"
	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/engine/lifecycle/retry"
	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/engine/schedule"
)

const ownInputs = "ownInputs"

var t0 = time.Date(2026, 10, 8, 12, 0, 0, 0, time.UTC)

// The settings of the tests; their names stand for settingsChecksum.
var (
	settings1 = &model.Settings{SchemaVersion: 1}
	settings2 = &model.Settings{SchemaVersion: 2}
)

// stateOpt builds a test state.
type stateOpt func(*state)

func newState(opts ...stateOpt) state {
	var s state
	for _, opt := range opts {
		opt(&s)
	}

	return s
}

// synced: the hooks are started.
func synced(s *state) { s.synced = true }

// acceptedK1: the settings k1 are in force.
func acceptedK1(s *state) {
	s.settings = configstate.State{Pushed: "k1", AcceptedChecksum: "k1", Accepted: &configstate.Accepted{Checksum: "c1"}}
}

// failedK2At: k1 is in force, the accept of k2 failed at t0+at.
func failedK2At(at time.Duration) stateOpt {
	return func(s *state) {
		acceptedK1(s)
		s.settings.Pushed = "k2"
		s.accept.Failed("k2", t0.Add(at))
	}
}

func failedOperation(kind operation.Kind, inputs string) stateOpt {
	return func(s *state) { s.operations.Failed(operationKey(kind, inputs), t0) }
}

func applied(inputs string) stateOpt { return func(s *state) { s.applied = inputs } }

func busy(s *state) { s.busy = true }

// event returns an event echoing ownInputs.
func event(gate bool, inputs string) *schedule.Event {
	return &schedule.Event{Name: schedule.GlobalNode, Enabled: true, Gate: gate, OwnInputs: ownInputs, Inputs: inputs}
}

func withSettings(settings *model.Settings, checksum string, ev *schedule.Event) input {
	return input{settings: settings, settingsChecksum: checksum, event: ev}
}

func TestNext(t *testing.T) {
	staleEcho := event(true, "x")
	staleEcho.OwnInputs = "stale"

	tests := []struct {
		name  string
		state state
		in    input
		now   time.Duration // now is t0+now.
		want  action
	}{
		// 1. Accept.
		{"the checkpoint is accepted before the sync", newState(), withSettings(settings1, "k1", nil), 0,
			action{kind: actionAccept}},
		{"new settings are accepted before a run", newState(synced, acceptedK1),
			withSettings(settings2, "k2", event(true, "x")), 0, action{kind: actionAccept}},
		{"new settings are accepted while busy (they cancel a run)", newState(synced, acceptedK1, busy),
			withSettings(settings2, "k2", event(true, "x")), 0, action{kind: actionAccept}},
		{"the settings in force are not accepted again", newState(synced, acceptedK1),
			withSettings(settings1, "k1", event(true, "x")), 0, action{kind: actionRun, inputs: "x"}},
		{"failed settings wait for their backoff, the old ones keep running", newState(synced, failedK2At(0)),
			withSettings(settings2, "k2", event(true, "x")), 0, action{kind: actionRun, inputs: "x"}},
		{"failed settings are retried once their backoff expired", newState(synced, failedK2At(0)),
			withSettings(settings2, "k2", event(true, "x")), retry.Min, action{kind: actionAccept}},
		{"the retry of failed settings waits for an idle worker", newState(synced, failedK2At(0), busy),
			withSettings(settings2, "k2", event(true, "x")), retry.Min, action{kind: actionRun, inputs: "x"}},
		{"other settings are not held by the backoff of failed ones", newState(synced, failedK2At(0)),
			withSettings(&model.Settings{SchemaVersion: 3}, "k3", nil), 0, action{kind: actionAccept}},

		// 2. Sync.
		{"sync without settings", newState(), input{}, 0, action{kind: actionSync}},
		{"sync after the checkpoint was accepted", newState(acceptedK1), withSettings(settings1, "k1", nil), 0,
			action{kind: actionSync}},
		{"sync although the checkpoint failed", newState(failedK2At(0)), withSettings(settings2, "k2", nil), 0,
			action{kind: actionSync}},
		{"sync comes before a run", newState(acceptedK1), withSettings(settings1, "k1", event(true, "x")), 0,
			action{kind: actionSync}},
		{"a failed sync waits for its backoff", newState(acceptedK1, failedOperation(operation.Sync, "")),
			withSettings(settings1, "k1", event(true, "x")), retry.Min - time.Nanosecond, action{kind: actionWait}},
		{"a failed sync is retried once its backoff expired", newState(acceptedK1, failedOperation(operation.Sync, "")),
			withSettings(settings1, "k1", event(true, "x")), retry.Min, action{kind: actionSync}},
		{"the sync in flight is decided again (and not cancelled)", newState(busy), input{}, 0,
			action{kind: actionSync}},

		// 3. No event.
		{"no event", newState(synced, acceptedK1), withSettings(settings1, "k1", nil), 0, action{kind: actionWait}},

		// 4. The own-inputs echo and accepted settings.
		{"no accepted settings", newState(synced), input{event: event(true, "x")}, 0, action{kind: actionWait}},
		{"the event does not echo the own inputs", newState(synced, acceptedK1), withSettings(settings1, "k1", staleEcho), 0,
			action{kind: actionWait}},

		// 5. Run.
		{"run", newState(synced, acceptedK1), withSettings(settings1, "k1", event(true, "x")), 0,
			action{kind: actionRun, inputs: "x"}},
		{"gate closed", newState(synced, acceptedK1), withSettings(settings1, "k1", event(false, "x")), 0,
			action{kind: actionWait}},
		{"converged", newState(synced, acceptedK1, applied("x")), withSettings(settings1, "k1", event(true, "x")), 0,
			action{kind: actionWait}},
		{"a failed run waits for its backoff", newState(synced, acceptedK1, failedOperation(operation.Run, "x")),
			withSettings(settings1, "k1", event(true, "x")), retry.Min - time.Nanosecond, action{kind: actionWait}},
		{"a failed run is retried once its backoff expired", newState(synced, acceptedK1, failedOperation(operation.Run, "x")),
			withSettings(settings1, "k1", event(true, "x")), retry.Min, action{kind: actionRun, inputs: "x"}},
		{"new Inputs are not held by the backoff of a failed run (a broken module was disabled)",
			newState(synced, acceptedK1, failedOperation(operation.Run, "x")),
			withSettings(settings1, "k1", event(true, "y")), 0, action{kind: actionRun, inputs: "y"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := next(tt.state, tt.in, ownInputs, t0.Add(tt.now)); got != tt.want {
				t.Errorf("next() = %+v, want %+v", got, tt.want)
			}
		})
	}
}

func TestSupersedes(t *testing.T) {
	var (
		wait   = action{kind: actionWait}
		accept = action{kind: actionAccept}
		sync   = action{kind: actionSync}
		runX   = action{kind: actionRun, inputs: "x"}
		runY   = action{kind: actionRun, inputs: "y"}
	)

	run := &operation.Operation{Kind: operation.Run, Key: "x"}
	syncing := &operation.Operation{Kind: operation.Sync}

	tests := []struct {
		name string
		want action
		op   *operation.Operation
		ok   bool
	}{
		{"run: wait does not cancel", wait, run, false},
		{"run: the same run does not cancel", runX, run, false},
		{"run: other Inputs do not cancel, the run finishes", runY, run, false},
		{"run: new settings cancel", accept, run, true},

		{"sync: wait does not cancel", wait, syncing, false},
		{"sync: the same sync does not cancel", sync, syncing, false},
		{"sync: new settings do not cancel", accept, syncing, false},
		{"sync: a run does not cancel", runX, syncing, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := supersedes(tt.want, tt.op); got != tt.ok {
				t.Errorf("supersedes(%+v, %s %q) = %v, want %v", tt.want, tt.op.Kind, tt.op.Key, got, tt.ok)
			}
		})
	}
}

// TestCancellation checks the cancellation rules end to end over next and supersedes: the
// decision of the loop's next iteration while a run on Inputs x is in flight.
func TestCancellation(t *testing.T) {
	run := &operation.Operation{Kind: operation.Run, Key: "x"}

	staleEcho := event(true, "y")
	staleEcho.OwnInputs = "stale"

	tests := []struct {
		name   string
		state  state
		in     input
		now    time.Duration
		cancel bool
	}{
		{"new settings cancel a run", newState(synced, acceptedK1, busy),
			withSettings(settings2, "k2", event(true, "x")), 0, true},
		{"failed settings never cancel a run on the old ones", newState(synced, failedK2At(0), busy),
			withSettings(settings2, "k2", event(true, "x")), 0, false},
		{"nor does their retry", newState(synced, failedK2At(0), busy),
			withSettings(settings2, "k2", event(true, "x")), retry.Max, false},
		{"nor does their retry with other Inputs", newState(synced, failedK2At(0), busy),
			withSettings(settings2, "k2", event(true, "y")), retry.Max, false},
		{"other Inputs never cancel a run (a module was disabled)", newState(synced, acceptedK1, busy),
			withSettings(settings1, "k1", event(true, "y")), 0, false},
		{"an event that does not echo the own inputs does not cancel", newState(synced, acceptedK1, busy),
			withSettings(settings1, "k1", staleEcho), 0, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			want := next(tt.state, tt.in, ownInputs, t0.Add(tt.now))
			if got := supersedes(want, run); got != tt.cancel {
				t.Errorf("next() = %+v, supersedes = %v, want %v", want, got, tt.cancel)
			}
		})
	}
}

func TestSettingsChecksum(t *testing.T) {
	a := &model.Settings{Values: map[string]any{"a": 1.0, "b": "x"}, SchemaVersion: 1}
	same := &model.Settings{Values: map[string]any{"b": "x", "a": 1.0}, SchemaVersion: 1}
	otherVersion := &model.Settings{Values: map[string]any{"a": 1.0, "b": "x"}, SchemaVersion: 2}

	if configstate.Key(a) != configstate.Key(same) {
		t.Error("equal content in another value must have the same checksum")
	}

	if configstate.Key(a) == configstate.Key(otherVersion) {
		t.Error("another schema version must change the checksum")
	}
}

func TestNewInputsRunAfterTheRunInFlight(t *testing.T) {
	in := withSettings(settings1, "k1", event(true, "y"))

	// While run "x" is in flight, Inputs "y" (a module was disabled) decide a run that waits for it.
	want := next(newState(synced, acceptedK1, busy), in, ownInputs, t0)
	if want.kind != actionRun || want.inputs != "y" || supersedes(want, &operation.Operation{Kind: operation.Run, Key: "x"}) {
		t.Fatalf("next() = %+v, want a run on y that does not cancel x", want)
	}

	// Run "x" ended (failed on the disabled module's CRD): applied is empty, and y runs at once.
	if got := next(newState(synced, acceptedK1), in, ownInputs, t0); got.kind != actionRun || got.inputs != "y" {
		t.Errorf("next() after the run = %+v, want a run on y", got)
	}
}
