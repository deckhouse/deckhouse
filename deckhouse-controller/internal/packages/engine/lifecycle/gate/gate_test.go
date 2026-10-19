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

package gate

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/Masterminds/semver/v3"

	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/engine/lifecycle/model"
	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/engine/schedule"
)

// The desired requirements and the object's constraints differ in Order only, so the fake
// scheduler can reject either.
var (
	requirements = schedule.Constraints{Order: 1}
	constraints  = schedule.Constraints{Order: 2}
	now          = time.Date(2026, 10, 8, 12, 0, 0, 0, time.UTC)
	nextWindow   = now.Add(3 * time.Hour)
)

type fakeScheduler struct {
	reject map[schedule.Order]bool
	// dependents makes CheckDependents reject: an enabled dependent does not accept the version.
	dependents bool
}

func (f fakeScheduler) CheckConstraints(_ string, c schedule.Constraints) error {
	if f.reject[c.Order] {
		return &schedule.ConstraintError{Reason: schedule.ReasonDependencyCycle, Message: "cycle"}
	}

	return nil
}

func (f fakeScheduler) CheckDependents(string, *semver.Version) error {
	if f.dependents {
		return &schedule.ConstraintError{Reason: schedule.ReasonDependentRequirementsUnmet, Message: "dependents"}
	}

	return nil
}

type fakeSettings struct{ err error }

func (f fakeSettings) CheckSettings(context.Context, model.Package, model.Settings, time.Time) error {
	return f.err
}

// object is a prepared object; only GetConstraints and GetVersion are called.
type object struct{ model.Package }

func (object) GetConstraints() schedule.Constraints { return constraints }
func (object) GetVersion() *semver.Version          { return semver.MustParse("2.0.0") }

type fakeWindows struct{ allowed bool }

func (f fakeWindows) IsAllowed(time.Time) bool            { return f.allowed }
func (f fakeWindows) NextAllowedTime(time.Time) time.Time { return nextWindow }

func TestDecide(t *testing.T) {
	current := &model.Image{Version: "v1"}
	ready := &Prepared{Ready: true, Object: object{}}
	outside := model.Policy{Mode: model.UpdateModeAuto, Windows: fakeWindows{}}

	tests := []struct {
		name     string
		in       Input
		reject   []schedule.Order // reject are the constraints the scheduler refuses.
		settings error
		// dependents: an enabled dependent rejects the candidate's version.
		dependents bool
		want       Decision
	}{
		// current
		{
			name: "same version: up to date",
			in:   Input{Desired: model.Desired{Version: "v1"}, Current: current},
			want: Decision{Kind: UpToDate},
		},
		{
			name: "another version: an ordinary update",
			in:   Input{Desired: model.Desired{Version: "v2"}, Current: current},
			want: Decision{Kind: Prepare},
		},
		{
			name:   "up to date before the requirements",
			in:     Input{Desired: model.Desired{Version: "v1", Requirements: requirements}, Current: current},
			reject: []schedule.Order{requirements.Order},
			want:   Decision{Kind: UpToDate},
		},
		// requirements
		{
			name:   "requirements not met, before anything is prepared",
			in:     Input{Desired: model.Desired{Version: "v2", Requirements: requirements}, Current: current, Prepared: ready},
			reject: []schedule.Order{requirements.Order},
			want:   Decision{Kind: Blocked, Reason: model.ReasonRequirementsNotMet, Cause: schedule.ReasonDependencyCycle, Message: "cycle"},
		},
		// prepared
		{
			name: "no result yet",
			in:   Input{Desired: model.Desired{Version: "v2"}, Current: current},
			want: Decision{Kind: Prepare},
		},
		{
			name: "a failed attempt is reported as the preparation reported it",
			in: Input{Desired: model.Desired{Version: "v2"}, Current: current,
				Prepared: &Prepared{Step: "attach", Reason: "AttachFailed", Message: "mount"}},
			want: Decision{Kind: Prepare, Step: "attach", Reason: "AttachFailed", Message: "mount"},
		},
		// final
		{
			name:     "settings rejected by the candidate",
			in:       Input{Desired: model.Desired{Version: "v2"}, Current: current, Prepared: ready},
			settings: errors.New("replicas: must be positive"),
			want:     Decision{Kind: Blocked, Reason: model.ReasonSettingsInvalid, Message: "replicas: must be positive"},
		},
		{
			// A first install has no Current to protect: its Current rejects the settings on accept.
			name:     "a first install is not stopped by settings",
			in:       Input{Desired: model.Desired{Version: "v2"}, Prepared: ready},
			settings: errors.New("replicas: must be positive"),
			want:     Decision{Kind: Activate},
		},
		{
			name:   "the candidate's constraints not met",
			in:     Input{Desired: model.Desired{Version: "v2"}, Current: current, Prepared: ready},
			reject: []schedule.Order{constraints.Order},
			want:   Decision{Kind: Blocked, Reason: model.ReasonRequirementsNotMet, Cause: schedule.ReasonDependencyCycle, Message: "cycle"},
		},
		{
			name:       "an enabled dependent rejects the candidate's version",
			in:         Input{Desired: model.Desired{Version: "v2"}, Current: current, Prepared: ready},
			dependents: true,
			want:       Decision{Kind: Blocked, Reason: model.ReasonRequirementsNotMet, Cause: schedule.ReasonDependentRequirementsUnmet, Message: "dependents"},
		},
		{
			name:       "dependents before the window",
			in:         Input{Desired: model.Desired{Version: "v2", Policy: outside}, Current: current, Prepared: ready},
			dependents: true,
			want:       Decision{Kind: Blocked, Reason: model.ReasonRequirementsNotMet, Cause: schedule.ReasonDependentRequirementsUnmet, Message: "dependents"},
		},
		{
			name:       "force does not skip the dependents",
			in:         Input{Desired: model.Desired{Version: "v2", Policy: model.Policy{Force: true}}, Current: current, Prepared: ready},
			dependents: true,
			want:       Decision{Kind: Blocked, Reason: model.ReasonRequirementsNotMet, Cause: schedule.ReasonDependentRequirementsUnmet, Message: "dependents"},
		},
		{
			name:       "first install: a present optional dependency must match too",
			in:         Input{Desired: model.Desired{Version: "v1"}, Prepared: ready},
			dependents: true,
			want:       Decision{Kind: Blocked, Reason: model.ReasonRequirementsNotMet, Cause: schedule.ReasonDependentRequirementsUnmet, Message: "dependents"},
		},
		{
			name:     "final before window",
			in:       Input{Desired: model.Desired{Version: "v2", Policy: outside}, Current: current, Prepared: ready},
			settings: errors.New("invalid"),
			want:     Decision{Kind: Blocked, Reason: model.ReasonSettingsInvalid, Message: "invalid"},
		},
		{
			name: "a settings check that errors is reported as invalid settings, not retried (F62)",
			in:   Input{Desired: model.Desired{Version: "v2"}, Current: current, Prepared: ready},
			// The memo returns ValidateSettings' error as is: a schema rejection of a module, or a
			// settings-check hook that could not reach the cluster.
			settings: errors.New("settings check hook: connection refused"),
			want:     Decision{Kind: Blocked, Reason: model.ReasonSettingsInvalid, Message: "settings check hook: connection refused"},
		},
		{
			name:     "force skips the window, never the final gate",
			in:       Input{Desired: model.Desired{Version: "v2", Policy: model.Policy{Mode: model.UpdateModeAuto, Windows: fakeWindows{}, Force: true}}, Current: current, Prepared: ready},
			settings: errors.New("invalid"),
			want:     Decision{Kind: Blocked, Reason: model.ReasonSettingsInvalid, Message: "invalid"},
		},
		{
			name:   "force does not skip the requirements",
			in:     Input{Desired: model.Desired{Version: "v2", Requirements: requirements, Policy: model.Policy{Force: true}}, Current: current, Prepared: ready},
			reject: []schedule.Order{requirements.Order},
			want:   Decision{Kind: Blocked, Reason: model.ReasonRequirementsNotMet, Cause: schedule.ReasonDependencyCycle, Message: "cycle"},
		},
		{
			name: "a failed preparation is reported even while the window is closed",
			in: Input{Desired: model.Desired{Version: "v2", Policy: outside}, Current: current,
				Prepared: &Prepared{Step: "load", Reason: model.ReasonLoadFromFilesystemFailed, Message: "load"}},
			want: Decision{Kind: Prepare, Step: "load", Reason: model.ReasonLoadFromFilesystemFailed, Message: "load"},
		},
		// window
		{
			name: "outside the window",
			in:   Input{Desired: model.Desired{Version: "v2", Policy: outside}, Current: current, Prepared: ready},
			want: Decision{
				Kind: Blocked, Reason: model.ReasonWaitingWindow, WakeAt: nextWindow,
				Message: "waiting for the update window at 2026-10-08T15:00:00Z",
			},
		},
		{
			name: "inside the window",
			in: Input{Desired: model.Desired{Version: "v2", Policy: model.Policy{Mode: model.UpdateModeAuto, Windows: fakeWindows{allowed: true}}},
				Current: current, Prepared: ready},
			want: Decision{Kind: Activate},
		},
		{
			name: "manual mode ignores the window",
			in: Input{Desired: model.Desired{Version: "v2", Policy: model.Policy{Mode: model.UpdateModeManual, Windows: fakeWindows{}}},
				Current: current, Prepared: ready},
			want: Decision{Kind: Activate},
		},
		{
			name: "force ignores the window",
			in: Input{Desired: model.Desired{Version: "v2", Policy: model.Policy{Mode: model.UpdateModeAuto, Windows: fakeWindows{}, Force: true}},
				Current: current, Prepared: ready},
			want: Decision{Kind: Activate},
		},
		{
			name: "no windows",
			in:   Input{Desired: model.Desired{Version: "v2", Policy: model.Policy{Mode: model.UpdateModeAuto}}, Current: current, Prepared: ready},
			want: Decision{Kind: Activate},
		},
		// first install
		{
			name: "first install: no Current to be up to date with",
			in:   Input{Desired: model.Desired{Version: "v1"}},
			want: Decision{Kind: Prepare},
		},
		{
			name:   "first install: requirements not met",
			in:     Input{Desired: model.Desired{Version: "v1", Requirements: requirements}},
			reject: []schedule.Order{requirements.Order},
			want:   Decision{Kind: Blocked, Reason: model.ReasonRequirementsNotMet, Cause: schedule.ReasonDependencyCycle, Message: "cycle"},
		},
		{
			name: "first install ignores the window",
			in:   Input{Desired: model.Desired{Version: "v1", Policy: outside}, Prepared: ready},
			want: Decision{Kind: Activate},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sched := fakeScheduler{reject: map[schedule.Order]bool{}, dependents: tt.dependents}
			for _, o := range tt.reject {
				sched.reject[o] = true
			}

			tt.in.Name, tt.in.Now = "mod", now

			got := Decide(context.Background(), tt.in, Deps{Scheduler: sched, Settings: fakeSettings{err: tt.settings}})
			if got != tt.want {
				t.Errorf("Decide = %+v, want %+v", got, tt.want)
			}
		})
	}
}
