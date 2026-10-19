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
	"testing"

	"github.com/stretchr/testify/suite"

	et "github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/engine/enginetest"
	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/engine/lifecycle/model"
	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/engine/lifecycle/retry"
	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/engine/schedule"
	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/addonutils"
)

// globalSuite covers scenarios G: Global and its barrier.
type globalSuite struct {
	base
}

func TestGlobal(t *testing.T) {
	suite.Run(t, new(globalSuite))
}

func applyGlobal(k *et.Kit, values addonutils.Values) {
	k.Engine.Apply(model.Desired{
		Name:          schedule.GlobalNode,
		Kind:          model.KindGlobal,
		Configuration: model.Configuration{Settings: model.Settings{Values: values}},
	})
}

// G1: Boot waits for Global's hook synchronization before the scheduler resumes; a restored module
// runs only after it.
func (s *globalSuite) TestG1GlobalSynchronizesBeforeResume() {
	k := et.New(s.T(), et.WithGlobal())
	release := k.Faults.Block("enable-hooks:global")

	booted := make(chan error, 1)
	enabled := true
	mod := et.Restored(et.Module("mod", v1), addonutils.Values{})
	mod.Intent = &enabled

	go func() { booted <- k.Boot(mod) }()

	k.WaitLog(0, "blocked:enable-hooks:global")
	s.False(k.WaitFor(quietPeriod, func() bool { return len(booted) > 0 || k.Index(0, "upgrade:mod") >= 0 }),
		"boot returned or a module ran before Global synchronized: %v", k.Log(0))
	s.False(k.Engine.Converged())

	release()
	s.Require().NoError(<-booted)

	s.installed(k, "mod", v1)
	s.inOrder(k, 0, "enable-hooks:global", "run:global:beforeAll", "upgraded:mod@v1.0.0")
}

// G1: a failed synchronization is retried; Boot returns once it succeeds.
func (s *globalSuite) TestG1FailedSynchronizationIsRetried() {
	k := et.New(s.T(), et.WithGlobal())
	k.Faults.FailTimes("enable-hooks:global", 1, errBoom)

	start := k.Clock.Now()

	booted := make(chan error, 1)
	go func() { booted <- k.Boot() }()

	// Boot's own 10-minute deadline is armed as well, often first: wait for the retry's timer
	// before moving the clock, so the advance goes to the retry, not to the deadline.
	k.Wait("the sync retry armed", func() bool { return k.Clock.ArmedAt(start.Add(retry.Min)) > 0 })
	s.Equal(retry.Min, k.AdvanceToNextTimer("the sync retry"))
	s.Require().NoError(<-booted)

	s.Equal(2, k.Count(0, "enable-hooks:global"))
	// The failed attempt is undone before the retry, which starts from fresh controllers.
	s.inOrder(k, 0, "enable-hooks:global", "disable:global", "initialize-hooks:global", "enable-hooks:global")
	r := k.WaitReport("global", "sync recovered", func(r model.Report) bool { return et.True(r.Current.HooksProcessed) })
	s.True(et.True(r.Current.HooksProcessed))
}

// G2: a global settings change reruns Global, then every subscribed package exactly once.
func (s *globalSuite) TestG2GlobalSettingsChangeRerunsEverySubscriberOnce() {
	k := et.New(s.T(), et.WithGlobal())
	k.MustBoot()

	k.ApplyModule(et.Module("a", v1), true)
	k.ApplyModule(et.Module("b", v1), true)
	k.Engine.Apply(et.App("ns", "app", v1))
	s.installed(k, "a", v1)
	s.installed(k, "b", v1)
	s.installed(k, "ns.app", v1)
	s.converged(k)

	mark := k.Mark()
	applyGlobal(k, addonutils.Values{"x": 1})

	k.WaitReport("global", "applied", func(r model.Report) bool { return r.Current.AppliedValues["x"] == 1 })

	for _, name := range []string{"a", "b", "ns.app"} {
		s.exactly(k, mark, name, v1, 1)
	}

	s.inOrder(k, mark, "run:global:beforeAll", "upgraded:a@v1.0.0")
}

// G3: a failed Global run cancels no run in flight and reruns no package, but holds new runs until
// it succeeds.
func (s *globalSuite) TestG3FailedGlobalRunHoldsNewRunsButCancelsNone() {
	k := et.New(s.T(), et.WithGlobal())
	k.MustBoot()

	k.ApplyModule(et.Module("a", v1), true)
	k.ApplyModule(et.Module("b", v1), true)
	s.installed(k, "a", v1)
	s.installed(k, "b", v1)
	s.converged(k)

	mark := k.Mark()

	// a run of a is in flight when Global fails
	release := k.Faults.Block("upgrade:a")
	defer release()

	s.Require().True(k.Nelm.Drift("a"))
	k.WaitLog(mark, "blocked:upgrade:a")

	// The first run and its retry fail; the second retry succeeds.
	k.Faults.FailTimes("crds:b", 2, errBoom)
	applyGlobal(k, addonutils.Values{"x": 1})

	k.WaitReport("global", "CRDsApplyFailed", func(r model.Report) bool {
		return et.False(r.Current.ManifestsApplied, model.ReasonCRDsApplyFailed)
	})
	s.False(k.Engine.Converged(), "Global is not converged")

	// a new run of b is held by the barrier
	s.Require().True(k.Nelm.Drift("b"))
	s.Equal(retry.Min, k.AdvanceToTimerOf(et.GlobalTimers, "Global's first retry"))
	k.Wait("Global's retry failed", func() bool { return k.Count(mark, "crds:b") == 2 })
	s.False(k.WaitFor(quietPeriod, func() bool { return upgrades(k, mark, "b", v1) > 0 }),
		"b ran behind a failing Global: %v", k.Log(mark))

	// the run of a in flight is not cancelled and finishes
	release()
	k.Wait("a's run finished", func() bool { return upgrades(k, mark, "a", v1) >= 1 })
	s.Equal(-1, k.Index(mark, "cancelled:upgrade:a"))

	s.Equal(2*retry.Min, k.AdvanceToTimerOf(et.GlobalTimers, "Global's second retry"))
	k.WaitReport("global", "recovered", func(r model.Report) bool { return et.True(r.Current.ManifestsApplied) })
	k.Wait("b ran", func() bool { return upgrades(k, mark, "b", v1) >= 1 })
	s.converged(k)
}

// G4: an update of a module reruns Global, which ensures the module's CRDs and webhooks before
// the new revision's first run.
func (s *globalSuite) TestG4ModuleCRDsAreEnsuredWithTheNewRevision() {
	k := et.New(s.T(), et.WithGlobal())
	k.MustBoot()

	k.ApplyModule(et.Module("mod", v1), true)
	s.installed(k, "mod", v1)
	s.converged(k)

	mark := k.Mark()
	k.ApplyModule(et.Module("mod", "v2.0.0"), true)
	s.installed(k, "mod", "v2.0.0")

	s.inOrder(k, mark, "add-node:mod@v2.0.0", "crds:mod", "webhooks:mod", "upgraded:mod@v2.0.0")
	s.Equal([]string{"mod"}, k.Global.EnabledModules())
}

// G5: an update that ships other images reruns Global, the module and its subscribers only: Global
// publishes the new digests outside its Output, and the module's own Output tells its subscribers.
func (s *globalSuite) TestG5ModuleUpdateRerunsOnlyItsSubscribers() {
	k := et.New(s.T(), et.WithGlobal())
	k.Loader.Set("images", v1, et.Spec{Digests: map[string]string{"image": "sha256:one"}})
	k.Loader.Set("images", "v2.0.0", et.Spec{Digests: map[string]string{"image": "sha256:two"}})
	k.Loader.Set("subscriber", "", et.Spec{Constraints: schedule.Constraints{
		Subscriptions: map[string]struct{}{"images": {}},
	}})
	k.MustBoot()

	k.ApplyModule(et.Module("images", v1), true)
	k.ApplyModule(et.Module("subscriber", v1), true)
	k.ApplyModule(et.Module("other", v1), true)
	s.installed(k, "images", v1)
	s.installed(k, "subscriber", v1)
	s.installed(k, "other", v1)
	s.converged(k)
	s.Equal(map[string]string{"image": "sha256:one"}, k.Global.ImagesDigests("images"))

	mark := k.Mark()
	k.ApplyModule(et.Module("images", "v2.0.0"), true)
	s.installed(k, "images", "v2.0.0")

	s.exactly(k, mark, "subscriber", v1, 1)
	s.exactly(k, mark, "other", v1, 0)
	s.Equal(map[string]string{"image": "sha256:two"}, k.Global.ImagesDigests("images"))
	s.inOrder(k, mark, "add-node:images@v2.0.0", "run:global:beforeAll", "upgraded:subscriber@v1.0.0")
}

// G5: a Candidate waiting for its window is attached beside the running revision, but Global
// publishes only the running revision's digests.
func (s *globalSuite) TestG5CandidateWaitingForTheWindowPublishesNoDigests() {
	k := et.New(s.T(), et.WithGlobal())
	k.Loader.Set("images", v1, et.Spec{Digests: map[string]string{"image": "sha256:one"}})
	k.Loader.Set("images", "v2.0.0", et.Spec{Digests: map[string]string{"image": "sha256:two"}})
	k.MustBoot()

	k.ApplyModule(et.Module("images", v1), true)
	s.installed(k, "images", v1)

	v2 := et.Module("images", "v2.0.0")
	v2.Policy.Windows = &et.Window{}
	k.ApplyModule(v2, true)
	k.WaitReport("images", "waiting for the window", func(r model.Report) bool {
		return r.Candidate.Blocked == model.ReasonWaitingWindow
	})
	s.converged(k)

	s.Equal(map[string]string{"image": "sha256:one"}, k.Global.ImagesDigests("images"))
}

// G2: global settings Global cannot accept keep the last accepted ones in force and rerun nothing;
// they are retried and recover.
func (s *globalSuite) TestG2InvalidGlobalSettingsKeepTheLastInForce() {
	k := et.New(s.T(), et.WithGlobal())
	k.MustBoot()

	k.ApplyModule(et.Module("mod", v1), true)
	s.installed(k, "mod", v1)
	s.converged(k)

	mark := k.Mark()
	k.Hooks.SettingsError("global", errBoom)
	applyGlobal(k, addonutils.Values{"x": 1})

	k.WaitReport("global", "SettingsInvalid", func(r model.Report) bool {
		return et.False(r.Current.Configured, model.ReasonSettingsInvalid)
	})
	s.False(k.WaitFor(quietPeriod, func() bool { return upgrades(k, mark, "mod", v1) > 0 }))
	s.True(k.Engine.Converged(), "the last accepted settings stay in force and converged")

	k.Hooks.SettingsError("global", nil)
	s.Equal(retry.Min, k.AdvanceToNextTimer("the accept retry"))
	k.WaitReport("global", "accepted on retry", func(r model.Report) bool {
		return et.True(r.Current.Configured) && r.Current.AppliedValues["x"] == 1
	})
	s.exactly(k, mark, "mod", v1, 1)
}

// G3: disabling the module whose CRD breaks every Global run unblocks Global: its enabled set
// changes, and the run without that module succeeds.
func (s *globalSuite) TestG3DisablingTheModuleWithABrokenCRDUnblocksGlobal() {
	k := et.New(s.T(), et.WithGlobal())
	k.MustBoot()

	k.ApplyModule(et.Module("a", v1), true)
	s.installed(k, "a", v1)

	k.Faults.Fail("crds:broken", errBoom)
	k.ApplyModule(et.Module("broken", v1), true)

	k.WaitReport("global", "CRDsApplyFailed", func(r model.Report) bool {
		return et.False(r.Current.ManifestsApplied, model.ReasonCRDsApplyFailed)
	})
	s.False(k.Engine.Converged())

	k.ApplyModule(et.Module("broken", v1), false)

	k.WaitReport("global", "recovered", func(r model.Report) bool { return et.True(r.Current.ManifestsApplied) })
	s.converged(k)
	s.NotContains(k.Global.EnabledModules(), "broken")
}

// G1: without a settings checkpoint (a fresh V2 cluster) Global synchronizes, then holds every
// package behind the barrier until its first settings arrive.
func (s *globalSuite) TestG1WithoutCheckpointGlobalHoldsTheBarrierUntilItsFirstSettings() {
	k := et.New(s.T(), et.WithGlobal())
	k.GlobalSettings = nil
	k.MustBoot()

	k.ApplyModule(et.Module("mod", v1), true)
	k.WaitLog(0, "add-node:mod@v1.0.0")

	s.False(k.WaitFor(quietPeriod, func() bool { return k.Index(0, "upgrade:mod") >= 0 }),
		"a module ran before Global had settings: %v", k.Log(0))
	s.False(k.Engine.Ready())

	applyGlobal(k, addonutils.Values{})
	s.installed(k, "mod", v1)
	s.converged(k)
	s.True(k.Engine.Ready())
}
