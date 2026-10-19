// Copyright 2026 Flant JSC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//	http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package update_test

import (
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/Masterminds/semver/v3"
	"github.com/stretchr/testify/suite"

	et "github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/engine/enginetest"
	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/engine/lifecycle/model"
	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/engine/lifecycle/retry"
	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/engine/schedule"
	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/engine/schedule/rule"
	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/addonutils"
)

const (
	mod = "mod"
	v1  = "v1.0.0"
	v2  = "v2.0.0"
	v3  = "v3.0.0"
)

// updateSuite covers scenarios C: updates of an installed package.
type updateSuite struct {
	suite.Suite

	k *et.Kit
}

func TestUpdate(t *testing.T) {
	suite.Run(t, new(updateSuite))
}

// SetupTest boots a V2 engine and installs mod at v1.
func (s *updateSuite) SetupTest() {
	s.k = et.New(s.T(), et.WithGlobal())
	s.k.MustBoot()
	s.k.ApplyModule(et.Module(mod, v1), true)
	s.waitRunning(v1)
}

// C1: v1's Current keeps accepting settings and running while v2 prepares.
func (s *updateSuite) TestC1CurrentRunsWhileTheCandidatePrepares() {
	k := s.k
	release := k.Faults.Block("attach:" + mod + "@" + v2)
	defer release()

	mark := k.Mark()
	k.ApplyModule(et.Module(mod, v2), true)
	k.WaitLog(mark, "blocked:attach:"+mod+"@"+v2)

	k.ApplyModule(et.WithSettings(et.Module(mod, v2), addonutils.Values{"a": 1}), true)

	r := k.WaitReport(mod, "v1 accepted and applied the new settings", func(r model.Report) bool {
		return r.Current.AcceptedValues["a"] == 1 && r.Current.AppliedValues["a"] == 1
	})
	s.Equal(v1, r.CurrentVersion)
	s.Equal(v2, r.Candidate.Version, "the candidate is still being prepared")
	s.GreaterOrEqual(k.Count(mark, "upgraded:"+mod+"@"+v1), 1, "v1 ran with the new settings")
	s.Equal(-1, k.Index(mark, "disable:"+mod), "v1's hooks were not stopped")
}

// C1: activation order — AddNode v2, stop v1 (its hooks), CurrentVersion and the cleared
// Candidate, then the detach of v1 last (L1).
func (s *updateSuite) TestC1ActivationOrder() {
	k := s.k
	releaseDetach := k.Faults.Block("detach:" + mod + "@" + v1)
	defer releaseDetach()

	mark := k.Mark()
	k.ApplyModule(et.Module(mod, v2), true)
	k.WaitLog(mark, "blocked:detach:"+mod+"@"+v1)

	r := k.Report(mod)
	s.Equal(v2, r.CurrentVersion, "CurrentVersion is written before the detach")
	s.Equal(model.CandidateReport{}, r.Candidate, "the Candidate section is cleared before the detach (L1)")

	s.inOrder(mark, "attach:"+mod+"@"+v2, "load:"+mod+"@"+v2, "add-node:"+mod+"@"+v2, "disable:"+mod,
		"detach:"+mod+"@"+v1)

	releaseDetach()
	s.waitRunning(v2)
	s.Equal([]string{mod + "@" + v2}, k.Disk.Attached())

}

// C1/L1: no Report a concurrent reader (the status projection) sees shows the activated version as
// a pending candidate. Intermittent: run with -count=20.
func (s *updateSuite) TestC1ActivatedVersionNeverShownAsCandidate() {
	k := s.k
	k.ApplyModule(et.Module(mod, v2), true)
	s.waitRunning(v2)

	for _, h := range k.History(mod) {
		if h.CurrentVersion == v2 {
			s.NotEqual(v2, h.Candidate.Version, "the activated version shows as a pending candidate: %s", et.Dump(h))
		}
	}
}

// C1/G2: the Current section stays v1's until v2's own operation reports.
func (s *updateSuite) TestC1CurrentSectionStaysV1UntilV2Reports() {
	k := s.k
	release := k.Faults.Block("run:" + mod + ":beforeHelm")
	defer release()

	mark := k.Mark()
	k.ApplyModule(et.Module(mod, v2), true)
	k.WaitLog(mark, "blocked:run:"+mod+":beforeHelm")

	r := k.Report(mod)
	s.Equal(v2, r.CurrentVersion)
	s.Equal(v1, r.Current.Version, "v2 has not reported an operation yet (G2)")
	s.True(et.True(r.Current.Installed), "Installed never flaps across an update")

	release()
	s.waitRunning(v2)
}

// C1/M1: on v2's first apply the previous revision's HooksProcessed is reset, Installed is kept.
func (s *updateSuite) TestC1ApplyingOnTheNewRevisionResetsHooksProcessed() {
	k := s.k
	k.Loader.Set(mod, v2, et.Spec{Defaults: addonutils.Values{"x": 2}}) // v2 changes the release
	release := k.Faults.Block("upgrade:" + mod)
	defer release()

	mark := k.Mark()
	k.ApplyModule(et.Module(mod, v2), true)
	k.WaitLog(mark, "blocked:upgrade:"+mod)

	r := k.WaitReport(mod, "v2 applying", func(r model.Report) bool {
		return r.Current.Version == v2 && et.False(r.Current.ManifestsApplied, model.ReasonApplyingManifests)
	})
	s.True(et.Unknown(r.Current.HooksProcessed), "v1's HooksProcessed is not shown as v2's: %s", et.Dump(r))
	s.True(et.True(r.Current.Installed))

	release()
	s.waitRunning(v2)
}

// C2: requirements unmet at the early gate (Desired.Requirements): nothing is downloaded, v1 keeps
// running, the Candidate names the scheduler's cause; installing the dependency unblocks.
func (s *updateSuite) TestC2BlockedByRequirementsAtTheEarlyGate() {
	k := s.k
	mark := k.Mark()

	d := et.Module(mod, v2)
	d.Requirements = schedule.Constraints{Dependencies: map[string]schedule.Dependency{"dep": {}}}
	k.ApplyModule(d, true)

	r := k.WaitReport(mod, "blocked by requirements", func(r model.Report) bool {
		return r.Candidate.Blocked == model.ReasonRequirementsNotMet
	})
	s.Equal("DependencyNotEnabled", r.Candidate.Reason, "the scheduler's cause is carried (G3)")
	s.Equal(v2, r.Candidate.Version)
	s.True(et.True(r.Current.Installed))
	s.Equal(-1, k.Index(mark, "attach:"+mod+"@"+v2), "nothing downloaded before the requirements pass")
	s.Equal(-1, k.Index(mark, "disable:"+mod), "Current untouched")

	k.ApplyModule(et.Module("dep", v1), true)
	s.waitRunning(v2)
}

// C2: requirements of the loaded revision unmet at the final gate: v2 stays prepared, v1 runs.
func (s *updateSuite) TestC2BlockedByRequirementsAtTheFinalGate() {
	k := s.k
	k.Loader.Set(mod, v2, et.Spec{Constraints: schedule.Constraints{
		Dependencies: map[string]schedule.Dependency{"dep": {}},
	}})

	mark := k.Mark()
	k.ApplyModule(et.Module(mod, v2), true)

	r := k.WaitReport(mod, "blocked at the final gate", func(r model.Report) bool {
		return r.Candidate.Blocked == model.ReasonRequirementsNotMet
	})
	s.Equal("DependencyNotEnabled", r.Candidate.Reason)
	s.GreaterOrEqual(k.Index(mark, "load:"+mod+"@"+v2), 0, "the final gate checks a loaded revision")
	s.Equal(v1, r.CurrentVersion)
	s.True(et.True(r.Current.Installed))
	s.Equal(-1, k.Index(mark, "disable:"+mod))

	k.ApplyModule(et.Module("dep", v1), true)
	s.waitRunning(v2)
	s.Equal(1, k.Count(mark, "attach:"+mod+"@"+v2), "the prepared candidate was kept while blocked")
}

// C2: AddNode rejects the candidate (a cycle closed after the final gate): v1 keeps running with
// its node, the Candidate reports RequirementsNotMet/DependencyCycle, the next wake blocks at the
// final gate; removing the cycle unblocks.
func (s *updateSuite) TestC2RejectedAtAddNode() {
	k := s.k
	k.Loader.Set(mod, v2, et.Spec{Constraints: schedule.Constraints{
		Dependencies: map[string]schedule.Dependency{"other": {Optional: true}},
	}})

	// The window gate runs after the final gate's checks and before AddNode: the first call closes
	// the cycle, as another package's activation would.
	w := &closingWindow{fn: func() {
		s.Require().NoError(k.Scheduler.AddNode(schedule.Node{
			Name:     "other",
			Version:  semver.MustParse(v1),
			Revision: v1,
			Constraints: schedule.Constraints{
				Dependencies: map[string]schedule.Dependency{mod: {}},
				Floor:        rule.Static(rule.Enable),
			},
		}))
	}}

	mark := k.Mark()
	d := et.Module(mod, v2)
	d.Policy.Windows = w
	k.ApplyModule(d, true)

	k.WaitLog(mark, "add-node-rejected:"+mod+"@"+v2)

	r := k.WaitReport(mod, "rejected at AddNode", func(r model.Report) bool {
		return r.Candidate.Blocked == model.ReasonRequirementsNotMet
	})
	s.Equal(schedule.ReasonDependencyCycle, r.Candidate.Reason)
	s.Equal(v1, r.CurrentVersion)
	s.True(et.True(r.Current.Installed))
	s.Equal(-1, k.Index(mark, "disable:"+mod), "v1 was not stopped")

	k.Scheduler.RemoveNode("other")
	s.waitRunning(v2)
	s.Equal(1, k.Count(mark, "attach:"+mod+"@"+v2), "the rejected candidate was kept, not attached again: %v", k.Log(mark))
	s.Equal(-1, k.Index(mark, "detach:"+mod+"@"+v2))
}

// C3: the candidate is prepared ahead and activated at the window start, with no further input.
func (s *updateSuite) TestC3ActivatesAtTheWindowStart() {
	k := s.k
	opens := k.Clock.Now().Add(time.Hour)
	w := &et.Window{}
	w.OpenAt(opens)

	mark := k.Mark()
	d := et.Module(mod, v2)
	d.Policy.Windows = w
	k.ApplyModule(d, true)

	r := k.WaitReport(mod, "waiting for the window", func(r model.Report) bool {
		return r.Candidate.Blocked == model.ReasonWaitingWindow
	})
	s.Contains(r.Candidate.Message, opens.UTC().Format(time.RFC3339))
	s.GreaterOrEqual(k.Index(mark, "load:"+mod+"@"+v2), 0, "prepared ahead of the window")
	s.Equal(v1, r.CurrentVersion)

	s.Equal(time.Hour, k.AdvanceToNextTimer("the window start"))
	s.waitRunning(v2)
	s.Equal(opens, k.Clock.Now())
}

// C3: a settings change while the update waits for the window reaches v1's Current.
func (s *updateSuite) TestC3SettingsReachCurrentWhileWaiting() {
	k := s.k
	w := &et.Window{} // closed

	d := et.Module(mod, v2)
	d.Policy.Windows = w
	k.ApplyModule(d, true)
	k.WaitReport(mod, "waiting", func(r model.Report) bool { return r.Candidate.Blocked == model.ReasonWaitingWindow })

	mark := k.Mark()
	k.ApplyModule(et.WithSettings(d, addonutils.Values{"a": 1}), true)

	r := k.WaitReport(mod, "v1 applied the settings", func(r model.Report) bool {
		return r.Current.AppliedValues["a"] == 1
	})
	s.Equal(v1, r.CurrentVersion)
	s.Equal(model.ReasonWaitingWindow, r.Candidate.Blocked)
	s.GreaterOrEqual(k.Count(mark, "upgraded:"+mod+"@"+v1), 1)
}

// C4: a failed load is retried after its backoff, without attaching again; Current is untouched
// and keeps receiving settings meanwhile.
func (s *updateSuite) TestC4PreparationFailureIsRetried() {
	k := s.k
	k.Faults.FailTimes("load:"+mod+"@"+v2, 1, errors.New("broken image"))

	mark := k.Mark()
	k.ApplyModule(et.Module(mod, v2), true)

	r := k.WaitReport(mod, "load failed", func(r model.Report) bool { return r.Candidate.Step == "load" })
	s.Equal(model.ReasonLoadFromFilesystemFailed, r.Candidate.Reason)
	s.Contains(r.Candidate.Message, "broken image")
	s.True(et.True(r.Current.Installed))

	k.ApplyModule(et.WithSettings(et.Module(mod, v2), addonutils.Values{"a": 1}), true)
	k.WaitReport(mod, "v1 accepted the settings", func(r model.Report) bool { return r.Current.AcceptedValues["a"] == 1 })

	s.Equal(retry.Min, k.AdvanceToNextTimer("the preparation's retry"))
	s.waitRunning(v2)
	s.Equal(1, k.Count(mark, "attach:"+mod+"@"+v2), "the retry continues after the attach")
	s.Equal(2, k.Count(mark, "load:"+mod+"@"+v2))
	s.Equal(1, k.Report(mod).Current.AppliedValues["a"], "v2 runs with the latest settings")
}

// C5: settings invalid for v2: blocked, v1 keeps running; valid settings unblock.
func (s *updateSuite) TestC5CandidateSettingsInvalid() {
	k := s.k
	k.Loader.Set(mod, v2, et.Spec{Validate: func(v addonutils.Values) error {
		if _, ok := v["required"]; !ok {
			return errors.New("required is missing")
		}

		return nil
	}})

	mark := k.Mark()
	k.ApplyModule(et.Module(mod, v2), true)

	r := k.WaitReport(mod, "blocked by settings", func(r model.Report) bool {
		return r.Candidate.Blocked == model.ReasonSettingsInvalid
	})
	s.Contains(r.Candidate.Message, "required is missing")
	s.Equal(v1, r.CurrentVersion)
	s.True(et.True(r.Current.Configured))
	s.Equal(-1, k.Index(mark, "disable:"+mod))

	k.ApplyModule(et.WithSettings(et.Module(mod, v2), addonutils.Values{"required": true}), true)
	s.waitRunning(v2)
}

// C6: v2 → v3 during preparation: v2's attempt is cancelled and its image detached, v3 activates.
func (s *updateSuite) TestC6DesiredVersionChangesDuringPreparation() {
	k := s.k
	release := k.Faults.Block("load:" + mod + "@" + v2)
	defer release()

	mark := k.Mark()
	k.ApplyModule(et.Module(mod, v2), true)
	k.WaitLog(mark, "blocked:load:"+mod+"@"+v2)

	k.ApplyModule(et.Module(mod, v3), true)
	s.waitRunning(v3)

	s.inOrder(mark, "cancelled:load:"+mod+"@"+v2, "detach:"+mod+"@"+v2, "attach:"+mod+"@"+v3)
	s.Equal([]string{mod + "@" + v3}, k.Disk.Attached())
}

// C7: the desired version reverts to Current during preparation: the candidate is released.
func (s *updateSuite) TestC7RevertDuringPreparation() {
	k := s.k
	release := k.Faults.Block("load:" + mod + "@" + v2)
	defer release()

	mark := k.Mark()
	k.ApplyModule(et.Module(mod, v2), true)
	k.WaitLog(mark, "blocked:load:"+mod+"@"+v2)

	k.ApplyModule(et.Module(mod, v1), true)
	k.WaitLog(mark, "detach:"+mod+"@"+v2)

	r := k.WaitReport(mod, "candidate cleared", func(r model.Report) bool { return r.Candidate == model.CandidateReport{} })
	s.True(et.Running(r, v1))
	s.Equal([]string{mod + "@" + v1}, k.Disk.Attached())
}

// C7: the desired version reverts to Current while the update is blocked.
func (s *updateSuite) TestC7RevertWhileBlocked() {
	k := s.k
	k.Loader.Set(mod, v2, et.Spec{Constraints: schedule.Constraints{
		Dependencies: map[string]schedule.Dependency{"dep": {}},
	}})

	mark := k.Mark()
	k.ApplyModule(et.Module(mod, v2), true)
	k.WaitReport(mod, "blocked", func(r model.Report) bool { return r.Candidate.Blocked == model.ReasonRequirementsNotMet })

	k.ApplyModule(et.Module(mod, v1), true)
	k.WaitLog(mark, "detach:"+mod+"@"+v2)
	k.WaitReport(mod, "candidate cleared", func(r model.Report) bool { return r.Candidate == model.CandidateReport{} })
	s.Equal([]string{mod + "@" + v1}, k.Disk.Attached())
}

// C8: v2's first run fails after the activation: no rollback, an ordinary failure and retry.
func (s *updateSuite) TestC8NewCurrentFailsAfterActivation() {
	k := s.k
	k.Faults.FailTimes("run:"+mod+":beforeHelm", 1, errors.New("hook broke"))

	mark := k.Mark()
	k.ApplyModule(et.Module(mod, v2), true)

	r := k.WaitReport(mod, "v2's run failed", func(r model.Report) bool {
		return r.Current.Version == v2 && et.False(r.Current.HooksProcessed, model.ReasonHookFailed)
	})
	s.Equal(v2, r.CurrentVersion)
	s.True(et.True(r.Current.Installed))

	s.Equal(retry.Min, k.AdvanceToNextTimer("the run's retry"))
	s.waitRunning(v2)
	s.Equal(-1, k.Index(mark, "attach:"+mod+"@"+v1), "no rollback")

	for _, h := range k.History(mod) {
		s.False(et.False(h.Current.Installed, ""), "Installed never went False: %s", et.Dump(h))
	}
}

// C9: an application update waits for its window and Force (ApplyNow) skips it.
func (s *updateSuite) TestC9ApplicationUpdateWindowAndApplyNow() {
	k := et.New(s.T())
	k.MustBoot()

	app := et.App("ns", "app", v1)
	k.Engine.Apply(app)
	k.WaitReport(app.Name, "installed", func(r model.Report) bool { return et.Running(r, v1) })

	d := et.App("ns", "app", v2)
	d.Policy.Windows = &et.Window{}
	k.Engine.Apply(d)

	r := k.WaitReport(app.Name, "waiting for the window", func(r model.Report) bool {
		return r.Candidate.Blocked == model.ReasonWaitingWindow
	})
	s.Equal(v1, r.CurrentVersion)

	d.Policy.Force = true
	k.Engine.Apply(d)
	k.WaitReport(app.Name, "updated", func(r model.Report) bool { return et.Running(r, v2) })
}

// C9: the Manual mode activates at once, whatever the windows.
func (s *updateSuite) TestC9ApplicationManualModeIgnoresWindows() {
	k := et.New(s.T())
	k.MustBoot()

	app := et.App("ns", "app", v1)
	k.Engine.Apply(app)
	k.WaitReport(app.Name, "installed", func(r model.Report) bool { return et.Running(r, v1) })

	d := et.App("ns", "app", v2)
	d.Policy = model.Policy{Mode: model.UpdateModeManual, Windows: &et.Window{}}
	k.Engine.Apply(d)
	k.WaitReport(app.Name, "updated", func(r model.Report) bool { return et.Running(r, v2) })
}

func (s *updateSuite) waitRunning(version string) {
	s.T().Helper()
	s.k.WaitReport(mod, "running "+version, func(r model.Report) bool {
		return et.Running(r, version)
	})
}

func (s *updateSuite) inOrder(mark int, entries ...string) {
	s.T().Helper()

	last := -1
	for _, e := range entries {
		i := s.k.Index(mark, e)
		s.Require().GreaterOrEqual(i, 0, "missing %s in %v", e, s.k.Log(mark))
		s.Greater(i, last, "%s out of order in %v", e, s.k.Log(mark))
		last = i
	}
}

// closingWindow is always open; its first IsAllowed runs fn (between the final gate and AddNode).
type closingWindow struct {
	once sync.Once
	fn   func()
}

func (w *closingWindow) IsAllowed(time.Time) bool {
	w.once.Do(w.fn)
	return true
}

func (w *closingWindow) NextAllowedTime(t time.Time) time.Time { return t }
