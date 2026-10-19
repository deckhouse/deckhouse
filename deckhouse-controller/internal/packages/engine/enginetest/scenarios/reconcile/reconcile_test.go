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

package reconcile_test

import (
	"errors"
	"testing"

	addontypes "github.com/flant/addon-operator/pkg/hook/types"
	"github.com/stretchr/testify/suite"

	et "github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/engine/enginetest"
	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/engine/lifecycle/model"
	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/engine/lifecycle/retry"
	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/nelm"
	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/addonutils"
)

// reconcileSuite covers scenarios B: a module installed at v1.0.0 and reconciled in place.
type reconcileSuite struct {
	base
}

func TestReconcile(t *testing.T) {
	suite.Run(t, new(reconcileSuite))
}

const v1 = "v1.0.0"

// start boots an engine with Global and installs mod with settings a=1; settings with bad=true are
// invalid.
func (s *reconcileSuite) start() *et.Kit {
	k := et.New(s.T(), et.WithGlobal())
	k.Loader.Set("mod", "", et.Spec{Validate: func(v addonutils.Values) error {
		if v["bad"] == true {
			return errors.New("bad is not allowed")
		}

		return nil
	}})
	k.MustBoot()

	k.ApplyModule(et.WithSettings(et.Module("mod", v1), addonutils.Values{"a": 1}), true)
	s.installed(k, "mod", v1)
	s.converged(k)

	return k
}

// B1: a settings change without a candidate is accepted and applied by one run.
func (s *reconcileSuite) TestB1SettingsChangeRunsOnceWithTheNewSettings() {
	k := s.start()
	mark := k.Mark()

	k.ApplyModule(et.WithSettings(et.Module("mod", v1), addonutils.Values{"a": 2}), true)

	r := k.WaitReport("mod", "a=2 applied", func(r model.Report) bool { return r.Current.AppliedValues["a"] == 2 })
	s.True(et.True(r.Current.Configured))
	s.Equal(2, r.Current.AcceptedValues["a"])
	s.Equal(r.Current.AcceptedChecksum, r.Current.AppliedChecksum)
	s.Equal(2, k.Loader.Latest("mod").GetSettings()["a"])
	s.exactly(k, mark, "mod", v1, 1)
	s.Equal(model.CandidateReport{}, k.Report("mod").Candidate, "no candidate for a settings change")
}

// B2: invalid settings keep the last valid ones in force, report SettingsInvalid and are retried;
// valid settings recover.
func (s *reconcileSuite) TestB2InvalidSettingsKeepTheLastValidInForce() {
	k := s.start()
	mark := k.Mark()

	k.ApplyModule(et.WithSettings(et.Module("mod", v1), addonutils.Values{"bad": true}), true)

	r := k.WaitReport("mod", "SettingsInvalid", func(r model.Report) bool {
		return et.False(r.Current.Configured, model.ReasonSettingsInvalid)
	})
	s.Contains(r.Current.Configured.Message, "bad is not allowed")
	s.Equal(1, r.Current.AcceptedValues["a"], "the last valid settings stay accepted")
	s.Equal(1, r.Current.AppliedValues["a"])
	s.True(et.True(r.Current.Installed))
	s.Equal(1, k.Loader.Latest("mod").GetSettings()["a"], "the object keeps the last valid settings")

	// Retried with backoff: after retry.Min, then after twice that.
	validations := k.Count(mark, "validate:mod@v1.0.0")
	s.Equal(retry.Min, k.AdvanceToNextTimer("the first accept retry"))
	k.Wait("accept retried", func() bool { return k.Count(mark, "validate:mod@v1.0.0") == validations+1 })
	s.Equal(2*retry.Min, k.AdvanceToNextTimer("the second accept retry"))
	k.Wait("accept retried twice", func() bool { return k.Count(mark, "validate:mod@v1.0.0") == validations+2 })
	s.Equal(0, upgrades(k, mark, "mod", v1), "no run with invalid settings")

	k.ApplyModule(et.WithSettings(et.Module("mod", v1), addonutils.Values{"a": 3}), true)
	k.WaitReport("mod", "recovered", func(r model.Report) bool {
		return et.True(r.Current.Configured) && r.Current.AppliedValues["a"] == 3
	})
}

// B3: maintenance reaches the object and the Report and reruns; turning it off reruns again.
func (s *reconcileSuite) TestB3MaintenanceOnAndOff() {
	k := s.start()
	mark := k.Mark()

	d := et.WithSettings(et.Module("mod", v1), addonutils.Values{"a": 1})
	d.Configuration.Maintenance = nelm.NoResourceReconciliation
	k.ApplyModule(d, true)

	k.WaitReport("mod", "maintenance accepted", func(r model.Report) bool {
		return r.Current.Maintenance == nelm.NoResourceReconciliation
	})
	s.exactly(k, mark, "mod", v1, 1)
	s.Equal(nelm.NoResourceReconciliation, k.Loader.Latest("mod").GetMaintenance())
	s.Equal(k.Report("mod").Current.AcceptedChecksum, k.Report("mod").Current.AppliedChecksum)

	mark = k.Mark()
	d.Configuration.Maintenance = nelm.Managed
	k.ApplyModule(d, true)

	k.WaitReport("mod", "maintenance off", func(r model.Report) bool { return r.Current.Maintenance == nelm.Managed })
	s.exactly(k, mark, "mod", v1, 1)
	s.Equal(nelm.Managed, k.Loader.Latest("mod").GetMaintenance())
}

// B4: drift of the release leads to exactly one run.
func (s *reconcileSuite) TestB4DriftRunsOnce() {
	k := s.start()
	mark := k.Mark()

	s.Require().True(k.Nelm.Drift("mod"), "the release has a running monitor")
	s.exactly(k, mark, "mod", v1, 1)
}

// B5: a runtime hook that changes the values leads to exactly one run.
func (s *reconcileSuite) TestB5RuntimeHookValuesChangeRunsOnce() {
	k := s.start()
	mark := k.Mark()

	s.Require().True(k.Hooks.RuntimeChange("mod", "discovered", "x"))
	s.exactly(k, mark, "mod", v1, 1)
}

// B6: new inputs during a run do not cancel it; the next run follows.
func (s *reconcileSuite) TestB6NewInputsDuringARunDoNotCancelIt() {
	k := s.start()
	mark := k.Mark()

	release := k.Faults.Block("upgrade:mod")
	defer release()

	s.Require().True(k.Nelm.Drift("mod"))
	k.WaitLog(mark, "blocked:upgrade:mod")

	s.Require().True(k.Hooks.RuntimeChange("mod", "discovered", "x"))
	s.False(k.WaitFor(quietPeriod, func() bool { return k.Index(mark, "cancelled:upgrade:mod") >= 0 }),
		"new inputs cancelled the run: %v", k.Log(mark))

	release()

	s.exactly(k, mark, "mod", v1, 2)
	s.Equal(-1, k.Index(mark, "cancelled:upgrade:mod"))
}

// B7: a new configuration during a run cancels it; the next run applies the new configuration.
func (s *reconcileSuite) TestB7NewConfigurationDuringARunCancelsIt() {
	k := s.start()
	mark := k.Mark()

	release := k.Faults.Block("upgrade:mod")
	defer release()

	s.Require().True(k.Nelm.Drift("mod"))
	k.WaitLog(mark, "blocked:upgrade:mod")

	k.ApplyModule(et.WithSettings(et.Module("mod", v1), addonutils.Values{"a": 2}), true)
	k.WaitLog(mark, "cancelled:upgrade:mod")
	release()

	k.WaitReport("mod", "a=2 applied", func(r model.Report) bool {
		return r.Current.AppliedValues["a"] == 2 && et.True(r.Current.ManifestsApplied)
	})
	s.exactly(k, mark, "mod", v1, 1)
}

// B8: a hook failing in a run reports HooksProcessed False/HookFailed, keeps Installed, is retried
// with backoff and recovers.
func (s *reconcileSuite) TestB8HookFailureAndRecovery() {
	k := s.start()
	mark := k.Mark()

	k.Faults.Fail("run:mod:beforeHelm", errBoom)
	s.Require().True(k.Nelm.Drift("mod"))

	r := k.WaitReport("mod", "HookFailed", func(r model.Report) bool {
		return et.False(r.Current.HooksProcessed, model.ReasonHookFailed)
	})
	s.Contains(r.Current.HooksProcessed.Message, "beforeHelm")
	s.True(et.True(r.Current.Installed), "a failure never uninstalls")
	s.True(et.True(r.Current.ManifestsApplied), "an unreached condition keeps its value within the revision")
	s.False(k.Engine.Converged(), "a failed run leaves the node unconverged")

	// Retried with the same inputs after the backoff.
	s.Equal(retry.Min, k.AdvanceToNextTimer("the run's retry"))
	k.Wait("hook retried", func() bool { return k.Count(mark, "run:mod:beforeHelm") == 2 })
	k.WaitArmed("the retry failed")

	k.Faults.Clear("run:mod:beforeHelm")
	s.Equal(2*retry.Min, k.AdvanceToNextTimer("the run's second retry"))
	k.WaitReport("mod", "recovered", func(r model.Report) bool { return et.Running(r, v1) })
	s.converged(k)
}

// B1: ManifestsApplied is False/ApplyingManifests while nelm applies a changed release, True after.
func (s *reconcileSuite) TestB1ApplyingManifestsWhileAChangedReleaseApplies() {
	k := s.start()
	mark := k.Mark()

	release := k.Faults.Block("upgrade:mod")
	defer release()

	k.ApplyModule(et.WithSettings(et.Module("mod", v1), addonutils.Values{"a": 2}), true)
	k.WaitLog(mark, "blocked:upgrade:mod")

	r := k.Report("mod")
	s.True(et.False(r.Current.ManifestsApplied, model.ReasonApplyingManifests), "report: %s", et.Dump(r))
	s.True(et.True(r.Current.Installed), "Installed stays True during a reconcile")

	release()
	k.WaitReport("mod", "applied", func(r model.Report) bool { return et.Running(r, v1) && r.Current.AppliedValues["a"] == 2 })
}

// B5: values an AfterHelm hook writes are applied inside the same run (reapply) and lead to no
// further run.
func (s *reconcileSuite) TestB5AfterHelmValuesAreReappliedInTheSameRun() {
	k := s.start()
	k.Hooks.WriteOn("mod", addontypes.AfterHelm, "afterHelm", "written")

	mark := k.Mark()
	s.Require().True(k.Nelm.Drift("mod"))
	s.exactly(k, mark, "mod", v1, 2)
	s.Equal(1, k.Count(mark, "run:mod:afterHelm"), "one run, two upgrades")

	mark = k.Mark()
	s.Require().True(k.Nelm.Drift("mod"))
	s.exactly(k, mark, "mod", v1, 1)
}

// A package disabled before it ever ran has applied nothing, so it writes no settings checkpoint:
// the webhooks read status.lastAppliedConfiguration as "has run" and freeze immutable fields on it.
func (s *reconcileSuite) TestNoCheckpointBeforeTheFirstRun() {
	k := et.New(s.T(), et.WithGlobal())
	k.MustBoot()

	k.ApplyModule(et.WithSettings(et.Module("mod", v1), addonutils.Values{"a": 1}), false)

	r := k.WaitReport("mod", "disabled", func(r model.Report) bool { return et.False(r.Current.Installed, model.ReasonDisabled) })
	s.Zero(k.Count(0, "upgraded:mod@"+v1), "never ran")
	s.Nil(r.Current.AppliedValues, "no checkpoint before the first run: %s", et.Dump(r))
}

// A package that ran, then was disabled, keeps its checkpoint current: every accept while disabled
// writes it (its enabled script reads the restored settings).
func (s *reconcileSuite) TestDisabledPackageThatRanKeepsWritingItsCheckpoint() {
	k := s.start()

	k.ApplyModule(et.WithSettings(et.Module("mod", v1), addonutils.Values{"a": 1}), false)
	k.WaitReport("mod", "disabled", func(r model.Report) bool { return et.False(r.Current.Installed, model.ReasonDisabled) })

	k.ApplyModule(et.WithSettings(et.Module("mod", v1), addonutils.Values{"a": 2}), false)

	r := k.WaitReport("mod", "a=2 checkpointed", func(r model.Report) bool { return r.Current.AppliedValues["a"] == 2 })
	s.Equal(r.Current.AcceptedChecksum, r.Current.AppliedChecksum)
}
