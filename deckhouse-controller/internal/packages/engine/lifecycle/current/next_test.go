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

package current

import (
	"errors"
	"fmt"
	"reflect"
	"slices"
	"testing"
	"time"

	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/engine/lifecycle/configstate"
	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/engine/lifecycle/model"
	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/engine/lifecycle/operation"
	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/engine/lifecycle/retry"
	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/engine/schedule"
	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/addonutils"
)

const (
	version   = "v1.0.0"
	ownInputs = "ownInputs"
)

var t0 = time.Date(2026, 10, 8, 12, 0, 0, 0, time.UTC)

// The configurations of the tests; their names stand for configChecksum.
var (
	config1 = &model.Configuration{Maintenance: "one"}
	config2 = &model.Configuration{Maintenance: "two"}
)

// stateOpt builds a test state.
type stateOpt func(*state)

func newState(opts ...stateOpt) state {
	s := state{version: version}
	for _, opt := range opts {
		opt(&s)
	}

	return s
}

// acceptedK1: the configuration k1 is in force.
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

func disabled(s *state) { s.disabled = true }

func busy(s *state) { s.busy = true }

// hooksStarted: a sync or a run started the hooks.
func hooksStarted(s *state) { s.hooksStarted = true }

// event returns an event of the running version echoing ownInputs.
func event(enabled, gate bool, inputs string) *schedule.Event {
	return &schedule.Event{Version: version, Enabled: enabled, Gate: gate, OwnInputs: ownInputs, Inputs: inputs}
}

func withConfig(config *model.Configuration, checksum string, ev *schedule.Event) input {
	return input{config: config, configChecksum: checksum, event: ev}
}

func TestNext(t *testing.T) {
	staleEcho := event(true, true, "x")
	staleEcho.OwnInputs = "stale"

	disabledStaleEcho := event(false, false, "d")
	disabledStaleEcho.OwnInputs = "stale"

	otherVersion := event(true, true, "x")
	otherVersion.Version = "v0.9.0"

	tests := []struct {
		name  string
		state state
		in    input
		now   time.Duration // now is t0+now.
		want  action
	}{
		// 1. Uninstall.
		{
			"uninstall", newState(acceptedK1),
			input{uninstall: true, event: event(true, true, "x")},
			0,
			action{kind: actionUninstall},
		},
		{
			"uninstall wins over a new configuration", newState(acceptedK1),
			input{uninstall: true, config: config2, configChecksum: "k2"},
			0,
			action{kind: actionUninstall},
		},
		{
			"uninstall wins for a disabled Current", newState(acceptedK1, disabled),
			input{uninstall: true},
			0,
			action{kind: actionUninstall},
		},
		{
			"a failed uninstall waits for its backoff", newState(failedOperation(operation.Uninstall, "")),
			input{uninstall: true, event: event(true, true, "x")},
			0,
			action{kind: actionWait},
		},
		{
			"a failed uninstall is retried once its backoff expired", newState(failedOperation(operation.Uninstall, "")),
			input{uninstall: true},
			retry.Min,
			action{kind: actionUninstall},
		},

		// 2. Accept.
		{
			"the first configuration is accepted without an event", newState(), withConfig(config1, "k1", nil), 0,
			action{kind: actionAccept},
		},
		{
			"a new configuration is accepted before a run", newState(acceptedK1),
			withConfig(config2, "k2", event(true, true, "x")), 0,
			action{kind: actionAccept},
		},
		{
			"a new configuration is accepted while busy (it cancels a run)", newState(acceptedK1, busy),
			withConfig(config2, "k2", event(true, true, "x")), 0,
			action{kind: actionAccept},
		},
		{
			"the configuration in force is not accepted again", newState(acceptedK1, hooksStarted),
			withConfig(config1, "k1", event(true, true, "x")), 0,
			action{kind: actionRun, inputs: "x"},
		},
		{
			"a failed configuration waits for its backoff, the old one keeps running", newState(failedK2At(0), hooksStarted),
			withConfig(config2, "k2", event(true, true, "x")), 0,
			action{kind: actionRun, inputs: "x"},
		},
		{
			"a failed configuration is retried once its backoff expired", newState(failedK2At(0)),
			withConfig(config2, "k2", event(true, true, "x")), retry.Min,
			action{kind: actionAccept},
		},
		{
			"the retry of a failed configuration waits for an idle worker", newState(failedK2At(0), busy, hooksStarted),
			withConfig(config2, "k2", event(true, true, "x")), retry.Min,
			action{kind: actionRun, inputs: "x"},
		},
		{
			"another configuration is not held by the backoff of a failed one", newState(failedK2At(0)),
			withConfig(&model.Configuration{Maintenance: "three"}, "k3", nil), 0,
			action{kind: actionAccept},
		},
		{
			"the old configuration pushed back after a failed one is accepted again", newState(failedK2At(0)),
			withConfig(config1, "k1", nil), 0,
			action{kind: actionAccept},
		},

		// 3. No event, or another revision's.
		{"no event", newState(acceptedK1), withConfig(config1, "k1", nil), 0, action{kind: actionWait}},
		{
			"an event of another revision", newState(acceptedK1), withConfig(config1, "k1", otherVersion), 0,
			action{kind: actionWait},
		},

		// 4. Disable.
		{
			"disable needs no configuration", newState(),
			input{event: event(false, false, "d")},
			0,
			action{kind: actionDisable, inputs: "d"},
		},
		{
			"disable without a pushed configuration ignores the echo", newState(),
			input{event: disabledStaleEcho},
			0,
			action{kind: actionDisable, inputs: "d"},
		},
		{
			"disable waits for the echo once a configuration was pushed", newState(acceptedK1),
			withConfig(config1, "k1", disabledStaleEcho), 0,
			action{kind: actionWait},
		},
		{
			"disable waits for the echo after a failed first accept",
			newState(func(s *state) { s.settings.Pushed = "k1"; s.accept.Failed("k1", t0) }),
			withConfig(config1, "k1", disabledStaleEcho), 0,
			action{kind: actionWait},
		},
		{
			"disable with the echo", newState(acceptedK1), withConfig(config1, "k1", event(false, false, "d")), 0,
			action{kind: actionDisable, inputs: "d"},
		},
		{
			"a failed disable waits for its backoff", newState(acceptedK1, failedOperation(operation.Disable, "d")),
			withConfig(config1, "k1", event(false, false, "d")), 0,
			action{kind: actionWait},
		},
		{
			"a failed disable holds a disable on other Inputs: a disable deletes whatever the values",
			newState(acceptedK1, failedOperation(operation.Disable, "d")),
			withConfig(config1, "k1", event(false, false, "e")), 0,
			action{kind: actionWait},
		},
		{
			"a failed run does not hold a disable", newState(acceptedK1, failedOperation(operation.Run, "d")),
			withConfig(config1, "k1", event(false, false, "d")), 0,
			action{kind: actionDisable, inputs: "d"},
		},
		{
			"disabled and done", newState(acceptedK1, disabled, applied("d")),
			withConfig(config1, "k1", event(false, false, "d")), 0,
			action{kind: actionWait},
		},
		{
			"disabled, decision changed while disabled", newState(acceptedK1, disabled, applied("d")),
			withConfig(config1, "k1", event(false, false, "e")), 0,
			action{kind: actionWait},
		},

		// 5. The own-inputs echo and an accepted configuration.
		{"no accepted configuration", newState(), input{event: event(true, true, "x")}, 0, action{kind: actionWait}},
		{
			"a failed first accept runs nothing",
			newState(func(s *state) { s.settings.Pushed = "k1"; s.accept.Failed("k1", t0) }),
			withConfig(config1, "k1", event(true, true, "x")), 0,
			action{kind: actionWait},
		},
		{
			"the event does not echo the own inputs", newState(acceptedK1), withConfig(config1, "k1", staleEcho), 0,
			action{kind: actionWait},
		},

		// 6. Run.
		{
			"run", newState(acceptedK1, hooksStarted), withConfig(config1, "k1", event(true, true, "x")), 0,
			action{kind: actionRun, inputs: "x"},
		},
		{
			"a sync after a disable: the disable stopped the hooks", newState(acceptedK1, disabled, applied("d")),
			withConfig(config1, "k1", event(true, true, "x")), 0,
			action{kind: actionSync, inputs: "x"},
		},
		{
			"gate closed", newState(acceptedK1), withConfig(config1, "k1", event(true, false, "x")), 0,
			action{kind: actionWait},
		},
		{
			"converged", newState(acceptedK1, applied("x")), withConfig(config1, "k1", event(true, true, "x")), 0,
			action{kind: actionWait},
		},
		{
			"a failed run waits for its backoff", newState(acceptedK1, hooksStarted, failedOperation(operation.Run, "x")),
			withConfig(config1, "k1", event(true, true, "x")), retry.Min - time.Nanosecond,
			action{kind: actionWait},
		},
		{
			"a failed run is retried once its backoff expired", newState(acceptedK1, hooksStarted, failedOperation(operation.Run, "x")),
			withConfig(config1, "k1", event(true, true, "x")), retry.Min,
			action{kind: actionRun, inputs: "x"},
		},
		{
			"new Inputs are not held by the backoff of a failed run",
			newState(acceptedK1, hooksStarted, failedOperation(operation.Run, "x")),
			withConfig(config1, "k1", event(true, true, "y")), 0,
			action{kind: actionRun, inputs: "y"},
		},
		{
			"a failed disable does not hold a run", newState(acceptedK1, hooksStarted, failedOperation(operation.Disable, "x")),
			withConfig(config1, "k1", event(true, true, "x")), 0,
			action{kind: actionRun, inputs: "x"},
		},
		{
			"hooks not started sync first", newState(acceptedK1), withConfig(config1, "k1", event(true, true, "x")), 0,
			action{kind: actionSync, inputs: "x"},
		},
		{
			"a converged node needs no sync", newState(acceptedK1, applied("x")),
			withConfig(config1, "k1", event(true, true, "x")), 0,
			action{kind: actionWait},
		},
		{
			"a closed gate needs no sync", newState(acceptedK1), withConfig(config1, "k1", event(true, false, "x")), 0,
			action{kind: actionWait},
		},
		{
			"a failed sync waits for its backoff", newState(acceptedK1, failedOperation(operation.Sync, "")),
			withConfig(config1, "k1", event(true, true, "x")), retry.Min - time.Nanosecond,
			action{kind: actionWait},
		},
		{
			"a failed sync holds new Inputs too", newState(acceptedK1, failedOperation(operation.Sync, "")),
			withConfig(config1, "k1", event(true, true, "y")), 0,
			action{kind: actionWait},
		},
		{
			"a failed sync is retried once its backoff expired", newState(acceptedK1, failedOperation(operation.Sync, "")),
			withConfig(config1, "k1", event(true, true, "x")), retry.Min,
			action{kind: actionSync, inputs: "x"},
		},
		{
			"OnStartup failed after the hooks started: the run retries it after the sync's backoff",
			newState(acceptedK1, hooksStarted, failedOperation(operation.Sync, "")),
			withConfig(config1, "k1", event(true, true, "x")), retry.Min,
			action{kind: actionRun, inputs: "x"},
		},
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
		wait      = action{kind: actionWait}
		accept    = action{kind: actionAccept}
		runX      = action{kind: actionRun, inputs: "x"}
		runY      = action{kind: actionRun, inputs: "y"}
		disable   = action{kind: actionDisable, inputs: "d"}
		uninstall = action{kind: actionUninstall}
	)

	run := &operation.Operation{Kind: operation.Run, Key: "x"}
	syncing := &operation.Operation{Kind: operation.Sync, Key: "x"}
	disabling := &operation.Operation{Kind: operation.Disable, Key: "d"}
	uninstalling := &operation.Operation{Kind: operation.Uninstall}

	tests := []struct {
		name string
		want action
		op   *operation.Operation
		ok   bool
	}{
		{"run: wait does not cancel", wait, run, false},
		{"run: the same run does not cancel", runX, run, false},
		{"run: other Inputs do not cancel, the run finishes", runY, run, false},
		{"run: a new configuration cancels", accept, run, true},
		{"run: disable cancels", disable, run, true},
		{"run: uninstall cancels", uninstall, run, true},

		{"sync: wait does not cancel", wait, syncing, false},
		{"sync: the same sync does not cancel", action{kind: actionSync, inputs: "x"}, syncing, false},
		{"sync: a run does not cancel, its render follows", runY, syncing, false},
		{"sync: a new configuration cancels", accept, syncing, true},
		{"sync: disable cancels", disable, syncing, true},
		{"sync: uninstall cancels", uninstall, syncing, true},

		{"disable: wait does not cancel", wait, disabling, false},
		{"disable: run does not cancel", runX, disabling, false},
		{"disable: accept does not cancel", accept, disabling, false},
		{"disable: another disable does not cancel", action{kind: actionDisable, inputs: "e"}, disabling, false},
		{"disable: uninstall cancels", uninstall, disabling, true},

		{"uninstall: wait does not cancel", wait, uninstalling, false},
		{"uninstall: run does not cancel", runX, uninstalling, false},
		{"uninstall: accept does not cancel", accept, uninstalling, false},
		{"uninstall: disable does not cancel", disable, uninstalling, false},
		{"uninstall: uninstall does not cancel", uninstall, uninstalling, false},
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

	tests := []struct {
		name   string
		state  state
		in     input
		now    time.Duration
		cancel bool
	}{
		{
			"a new configuration cancels a run", newState(acceptedK1, busy),
			withConfig(config2, "k2", event(true, true, "x")), 0, true,
		},
		{
			"a failed accept never cancels a run on the old settings", newState(failedK2At(0), busy),
			withConfig(config2, "k2", event(true, true, "x")), 0, false,
		},
		{
			"nor does its retry", newState(failedK2At(0), busy),
			withConfig(config2, "k2", event(true, true, "x")), retry.Max, false,
		},
		{
			"its retry does not hide a disable", newState(failedK2At(0), busy),
			withConfig(config2, "k2", event(false, false, "d")), retry.Max, true,
		},
		{
			"a closed gate never cancels a run", newState(acceptedK1, busy),
			withConfig(config1, "k1", event(true, false, "y")), 0, false,
		},
		{
			"other Inputs with an open gate never cancel a run", newState(acceptedK1, busy),
			withConfig(config1, "k1", event(true, true, "y")), 0, false,
		},
		{
			"a disable decision cancels a run", newState(acceptedK1, busy),
			withConfig(config1, "k1", event(false, false, "d")), 0, true,
		},
		{"uninstall cancels a run", newState(acceptedK1, busy), input{uninstall: true}, 0, true},
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

func TestConfigChecksum(t *testing.T) {
	a := &model.Configuration{Settings: model.Settings{Values: map[string]any{"a": 1.0, "b": "x"}, SchemaVersion: 1}}
	same := &model.Configuration{Settings: model.Settings{Values: map[string]any{"b": "x", "a": 1.0}, SchemaVersion: 1}}
	otherVersion := &model.Configuration{Settings: model.Settings{Values: map[string]any{"a": 1.0, "b": "x"}, SchemaVersion: 2}}
	otherMaintenance := &model.Configuration{Settings: a.Settings, Maintenance: "NoResourceReconciliation"}

	if configstate.Key(a) != configstate.Key(same) {
		t.Error("equal content in another value must have the same checksum")
	}

	if configstate.Key(a) == configstate.Key(otherVersion) {
		t.Error("another schema version must change the checksum")
	}

	if configstate.Key(a) == configstate.Key(otherMaintenance) {
		t.Error("another maintenance mode must change the checksum")
	}
}

// sectionWriter is a reportWriter over one Current section.
type sectionWriter struct {
	section model.CurrentReport
	writes  int
}

func (w *sectionWriter) UpdateCurrent(f func(*model.CurrentReport)) {
	w.writes++
	f(&w.section)
}

// The step shapes of a run, a disable and an uninstall (steps.go): hooks interleave with the
// release; an uninstall prepares the hooks first.
var (
	testPrepareSteps = []operation.Step{
		{Name: stepStartHooks, Condition: operation.HooksProcessed},
		{Name: stepOnStartup, Condition: operation.HooksProcessed},
	}
	testRunSteps = []operation.Step{
		{Name: stepStartHooks, Condition: operation.HooksProcessed},
		{Name: stepOnStartup, Condition: operation.HooksProcessed},
		{Name: stepBeforeHelm, Condition: operation.HooksProcessed},
		{Name: stepApply, Condition: operation.ManifestsApplied},
		{Name: stepAfterHelm, Condition: operation.HooksProcessed},
		{Name: stepReapply, Condition: operation.ManifestsApplied},
	}
	testDisableSteps = []operation.Step{
		{Name: stepRemoveMonitor, Condition: operation.ManifestsApplied},
		{Name: stepBeforeDeleteHelm, Condition: operation.HooksProcessed},
		{Name: stepDeleteRelease, Condition: operation.ManifestsApplied},
		{Name: stepAfterDeleteHelm, Condition: operation.HooksProcessed},
		{Name: stepStopHooks, Condition: operation.HooksProcessed},
	}
	testModuleUninstallSteps = slices.Concat(testPrepareSteps, testDisableSteps)
	testUninstallSteps       = append(slices.Clone(testModuleUninstallSteps),
		operation.Step{Name: stepPurgeOrphans, Condition: operation.ManifestsApplied})
)

// The fixtures above must keep the shape of the real step lists: the table of
// TestOperationConditions is only as good as they are.
func TestStepListsMatchTheFixtures(t *testing.T) {
	shape := func(steps []operation.Step) []string {
		out := make([]string, 0, len(steps))
		for _, s := range steps {
			out = append(out, fmt.Sprintf("%s/%d", s.Name, s.Condition))
		}

		return out
	}

	app := &Reconciler{kind: model.KindApplication}
	module := &Reconciler{kind: model.KindModule}

	tests := []struct {
		name string
		got  []operation.Step
		want []operation.Step
	}{
		{name: "sync", got: module.syncSteps(&operationState{}), want: testPrepareSteps},
		{name: "run", got: module.runSteps(&operationState{}), want: testRunSteps},
		{name: "disable", got: module.disableSteps(&operationState{}), want: testDisableSteps},
		{name: "module uninstall", got: module.uninstallSteps(&operationState{}), want: testModuleUninstallSteps},
		{name: "application uninstall", got: app.uninstallSteps(&operationState{}), want: testUninstallSteps},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got, want := shape(tt.got), shape(tt.want); !reflect.DeepEqual(got, want) {
				t.Errorf("steps = %v, want %v", got, want)
			}
		})
	}
}

// stepOf returns the step named name of steps.
func stepOf(steps []operation.Step, name string) *operation.Step {
	for i := range steps {
		if steps[i].Name == name {
			return &steps[i]
		}
	}

	panic("no step " + name)
}

func TestOperationConditions(t *testing.T) {
	hookFailed := model.FalseCondition(model.ReasonHookFailed, "beforeHelm: boom")
	applyFailed := model.FalseCondition(model.ReasonManifestsApplyFailed, "apply: boom")
	afterHelmFailed := model.FalseCondition(model.ReasonHookFailed, "afterHelm: boom")
	applying := model.FalseCondition(model.ReasonApplyingManifests, "")
	reapplyFailed := model.FalseCondition(model.ReasonManifestsApplyFailed, "reapply: boom")
	deleteFailed := model.FalseCondition(model.ReasonReleaseDeleteFailed, "deleteRelease: boom")
	afterDeleteFailed := model.FalseCondition(model.ReasonHookFailed, "afterDeleteHelm: boom")
	purgeFailed := model.FalseCondition(model.ReasonOrphansPurgeFailed, "purgeOrphans: boom")

	// v1's run failed at a hook; its release is installed.
	v1HookFailed := model.CurrentReport{
		Version: "v1", HooksProcessed: hookFailed, ManifestsApplied: model.TrueCondition(), Installed: model.TrueCondition(),
	}

	tests := []struct {
		name     string
		prev     model.CurrentReport
		revision string
		steps    []operation.Step
		failed   string
		failure  model.Condition
		want     model.CurrentReport
	}{
		{
			name:     "v2 apply fails after v1's hook failure: v2's hooks before the apply passed",
			prev:     model.CurrentReport{Version: "v2", HooksProcessed: model.Condition{}, ManifestsApplied: applying, Installed: model.TrueCondition()},
			revision: "v2", steps: testRunSteps, failed: stepApply, failure: applyFailed,
			want: model.CurrentReport{Version: "v2", HooksProcessed: model.TrueCondition(), ManifestsApplied: applyFailed, Installed: model.TrueCondition()},
		},
		{
			name:     "afterHelm fails after a successful apply in the same revision: the mark goes",
			prev:     model.CurrentReport{Version: "v1", HooksProcessed: model.TrueCondition(), ManifestsApplied: applying, Installed: model.TrueCondition()},
			revision: "v1", steps: testRunSteps, failed: stepAfterHelm, failure: afterHelmFailed,
			want: model.CurrentReport{Version: "v1", HooksProcessed: afterHelmFailed, ManifestsApplied: model.TrueCondition(), Installed: model.TrueCondition()},
		},
		{
			name:     "a first step fails within the revision: the unreached condition keeps its value",
			prev:     model.CurrentReport{Version: "v1", HooksProcessed: model.TrueCondition(), ManifestsApplied: model.TrueCondition()},
			revision: "v1", steps: testRunSteps, failed: stepStartHooks, failure: hookFailed,
			want: model.CurrentReport{Version: "v1", HooksProcessed: hookFailed, ManifestsApplied: model.TrueCondition()},
		},
		{
			name:     "a first step fails on a new revision: the unreached condition is Unknown",
			prev:     v1HookFailed,
			revision: "v2", steps: testRunSteps, failed: stepStartHooks, failure: hookFailed,
			want: model.CurrentReport{Version: "v2", HooksProcessed: hookFailed, ManifestsApplied: model.Condition{}, Installed: model.TrueCondition()},
		},
		{
			name:     "success: every condition True",
			prev:     v1HookFailed,
			revision: "v2", steps: testRunSteps,
			want: model.CurrentReport{Version: "v2", HooksProcessed: model.TrueCondition(), ManifestsApplied: model.TrueCondition(), Installed: model.TrueCondition()},
		},
		{
			name:     "reapply fails after AfterHelm changed the values: the hooks passed, the release did not",
			prev:     model.CurrentReport{Version: "v1", HooksProcessed: model.TrueCondition(), ManifestsApplied: applying, Installed: model.TrueCondition()},
			revision: "v1", steps: testRunSteps, failed: stepReapply, failure: reapplyFailed,
			want: model.CurrentReport{Version: "v1", HooksProcessed: model.TrueCondition(), ManifestsApplied: reapplyFailed, Installed: model.TrueCondition()},
		},
		{
			name:     "disable fails at the release delete: the run's hook failure is replaced by the passed deletion hook",
			prev:     v1HookFailed,
			revision: "v1", steps: testDisableSteps, failed: stepDeleteRelease, failure: deleteFailed,
			want: model.CurrentReport{Version: "v1", HooksProcessed: model.TrueCondition(), ManifestsApplied: deleteFailed, Installed: model.TrueCondition()},
		},
		{
			name:     "disable fails at AfterDeleteHelm: the release is gone, the hooks failed",
			prev:     model.CurrentReport{Version: "v1", HooksProcessed: model.TrueCondition(), ManifestsApplied: model.TrueCondition(), Installed: model.TrueCondition()},
			revision: "v1", steps: testDisableSteps, failed: stepAfterDeleteHelm, failure: afterDeleteFailed,
			want: model.CurrentReport{Version: "v1", HooksProcessed: afterDeleteFailed, ManifestsApplied: model.TrueCondition(), Installed: model.TrueCondition()},
		},
		{
			name:     "uninstall of an application fails at the purge: ManifestsApplied carries it",
			prev:     model.CurrentReport{Version: "v1", HooksProcessed: model.TrueCondition(), ManifestsApplied: model.TrueCondition(), Installed: model.TrueCondition()},
			revision: "v1", steps: testUninstallSteps, failed: stepPurgeOrphans, failure: purgeFailed,
			want: model.CurrentReport{Version: "v1", HooksProcessed: model.TrueCondition(), ManifestsApplied: purgeFailed, Installed: model.TrueCondition()},
		},
		{
			name:     "disable success: every condition True",
			prev:     v1HookFailed,
			revision: "v1", steps: testDisableSteps,
			want: model.CurrentReport{Version: "v1", HooksProcessed: model.TrueCondition(), ManifestsApplied: model.TrueCondition(), Installed: model.TrueCondition()},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var failed *operation.Step
			if tt.failed != "" {
				failed = stepOf(tt.steps, tt.failed)
			}

			if got := operationConditions(tt.prev, tt.revision, tt.steps, failed, tt.failure); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("operationConditions() =\n%+v\nwant\n%+v", got, tt.want)
			}
		})
	}
}

func TestApplyingOnANewRevisionResetsHooks(t *testing.T) {
	// v1's run failed at a hook; v2 reaches its apply.
	w := &sectionWriter{section: model.CurrentReport{
		Version:          "v1",
		HooksProcessed:   model.FalseCondition(model.ReasonHookFailed, "beforeHelm: boom"),
		ManifestsApplied: model.TrueCondition(),
		Installed:        model.TrueCondition(),
	}}
	c := &Reconciler{reporter: w, state: state{version: "v2"}}

	c.reportApplying()

	want := model.CurrentReport{
		Version:          "v2",
		ManifestsApplied: model.FalseCondition(model.ReasonApplyingManifests, ""),
		Installed:        model.TrueCondition(),
	}
	if !reflect.DeepEqual(w.section, want) {
		t.Errorf("section = %+v, want %+v", w.section, want)
	}
}

func TestSuccessReportsInstalledByKind(t *testing.T) {
	tests := []struct {
		kind  operation.Kind
		steps []operation.Step
		want  model.Condition
	}{
		{kind: operation.Run, steps: testRunSteps, want: model.TrueCondition()},
		{kind: operation.Disable, steps: testDisableSteps, want: model.FalseCondition(model.ReasonDisabled, "")},
	}

	for _, tt := range tests {
		t.Run(string(tt.kind), func(t *testing.T) {
			w := &sectionWriter{}
			// A run's success sets ran before it reports (finishOperation).
			c := &Reconciler{reporter: w, state: state{version: "v1", ran: true}}

			cp := &configstate.Accepted{Values: addonutils.Values{"a": 1}, Checksum: "sum"}

			c.reportSuccess(tt.kind, tt.steps, cp)

			if w.section.Installed != tt.want || w.section.Version != "v1" {
				t.Errorf("section = %+v, want Installed %+v at v1", w.section, tt.want)
			}

			if w.writes != 1 || w.section.AppliedChecksum != "sum" || w.section.AppliedValues["a"] != 1 {
				t.Errorf("writes = %d, section = %+v, want the checkpoint in the success's one write", w.writes, w.section)
			}
		})
	}
}

// PF1: a disable of a package that never ran writes no checkpoint; one with a checkpoint of an
// earlier revision in the Report keeps it current.
func TestDisableCheckpointsOnlyAPackageThatRan(t *testing.T) {
	cp := &configstate.Accepted{Values: addonutils.Values{"a": 1}, Checksum: "sum"}

	w := &sectionWriter{}
	c := &Reconciler{reporter: w, state: state{version: "v1"}}
	c.reportSuccess(operation.Disable, testDisableSteps, cp)

	if w.section.AppliedChecksum != "" || w.section.AppliedValues != nil {
		t.Errorf("never ran: section = %+v, want no checkpoint", w.section)
	}

	w = &sectionWriter{section: model.CurrentReport{AppliedChecksum: "old"}}
	c = &Reconciler{reporter: w, state: state{version: "v2"}}
	c.reportSuccess(operation.Disable, testDisableSteps, cp)

	if w.section.AppliedChecksum != "sum" {
		t.Errorf("an earlier revision ran: section = %+v, want the new checkpoint", w.section)
	}
}

func TestCancelledOperationIsNotReported(t *testing.T) {
	prev := model.CurrentReport{Version: "v1", HooksProcessed: model.TrueCondition(), ManifestsApplied: model.TrueCondition()}
	w := &sectionWriter{section: prev}
	c := &Reconciler{reporter: w, state: state{version: "v2"}, operationState: &operationState{steps: testRunSteps}}

	c.finishOperation(operation.Result{Kind: operation.Run, Failed: stepOf(testRunSteps, stepApply), Err: errors.New("canceled"), Canceled: true})

	if !reflect.DeepEqual(w.section, prev) {
		t.Errorf("section = %+v, want it unchanged %+v", w.section, prev)
	}
}

func TestUninstallStepsPurgeOnlyForApplications(t *testing.T) {
	tests := []struct {
		kind model.Kind
		last string
	}{
		{kind: model.KindApplication, last: stepPurgeOrphans},
		{kind: model.KindModule, last: stepStopHooks},
	}

	for _, tt := range tests {
		t.Run(string(tt.kind), func(t *testing.T) {
			r := &Reconciler{kind: tt.kind}

			steps := r.uninstallSteps(&operationState{})
			if got := steps[len(steps)-1].Name; got != tt.last {
				t.Errorf("last uninstall step = %q, want %q", got, tt.last)
			}
		})
	}
}

func TestNewInputsRunAfterTheRunInFlight(t *testing.T) {
	in := withConfig(config1, "k1", event(true, true, "y"))

	// While run "x" is in flight, Inputs "y" decide a run that waits for it.
	want := next(newState(acceptedK1, busy, hooksStarted), in, ownInputs, t0)
	if want.kind != actionRun || want.inputs != "y" || supersedes(want, &operation.Operation{Kind: operation.Run, Key: "x"}) {
		t.Fatalf("next() = %+v, want a run on y that does not cancel x", want)
	}

	// Run "x" succeeded: applied is x, the node is not converged on y, and y runs.
	done := newState(acceptedK1, hooksStarted, func(s *state) { s.applied = "x" })
	if got := next(done, in, ownInputs, t0); got.kind != actionRun || got.inputs != "y" {
		t.Errorf("next() after the run = %+v, want a run on y", got)
	}
}
