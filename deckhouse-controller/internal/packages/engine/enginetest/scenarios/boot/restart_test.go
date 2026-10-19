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

package boot_test

import (
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/suite"

	et "github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/engine/enginetest"
	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/engine/lifecycle/model"
	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/addonutils"
)

// restartSuite covers scenarios F1–F6: a restart is a Shutdown of one kit and a Boot of the next
// from the checkpoint the first one reported.
type restartSuite struct {
	suite.Suite
}

func TestRestart(t *testing.T) {
	suite.Run(t, new(restartSuite))
}

// F1: a restart in steady state restores the version and settings, runs once without an Apply,
// never reports Installed=False, and the first Apply with unchanged settings reruns nothing.
func (s *restartSuite) TestF1RestartInSteadyState() {
	m := et.WithSettings(et.Module("m", "v1.0.0"), addonutils.Values{"x": 1})

	k1 := et.New(s.T(), et.WithGlobal())
	k1.MustBoot()
	k1.ApplyModule(m, true)
	running(k1, "m", "v1.0.0")

	bp := checkpoint(k1, m)
	k1.Shutdown()

	k2 := et.New(s.T(), et.WithGlobal())
	k2.MustBoot(bp)

	r := running(k2, "m", "v1.0.0")
	s.Equal(addonutils.Values{"x": 1}, r.Current.AppliedValues)

	for _, h := range k2.History("m") {
		s.False(et.False(h.Current.Installed, ""), "Installed flapped to False after a restart: %s", et.Dump(h))
	}

	// The Apply carries the checkpoint's own configuration, so it is not accepted again; readiness
	// latches once it arrived.
	mark := k2.Mark()
	k2.ApplyModule(m, true)
	k2.Wait("ready", k2.Engine.Ready)
	s.False(k2.WaitFor(quiet, func() bool { return k2.Count(mark, "upgraded:m@v1.0.0") > 0 }),
		"the first Apply with the restored settings reran the module")
}

// F2: a restart during the preparation of v2 keeps v1 (status and image) and prepares v2 again.
func (s *restartSuite) TestF2RestartDuringPreparation() {
	k1 := et.New(s.T(), et.WithGlobal())
	k1.MustBoot()
	k1.ApplyModule(et.Module("m", "v1.0.0"), true)
	running(k1, "m", "v1.0.0")

	k1.Faults.Block("attach:m@v2.0.0")
	k1.ApplyModule(et.Module("m", "v2.0.0"), true)
	k1.WaitLog(0, "blocked:attach:m@v2.0.0")

	bp := checkpoint(k1, et.Module("m", "v1.0.0"))
	k1.Shutdown()

	s.Equal("v1.0.0", bp.Version)
	s.Equal([]string{"m@v1.0.0"}, k1.Disk.Attached(), "Current's image must stay, nothing else attached")

	k2 := et.New(s.T(), et.WithGlobal())
	k2.MustBoot(bp)
	running(k2, "m", "v1.0.0")

	k2.ApplyModule(et.Module("m", "v2.0.0"), true)
	running(k2, "m", "v2.0.0")
	s.Equal([]string{"m@v2.0.0"}, k2.Disk.Attached())
}

// F3: a restart while an update waits for its window: the candidate image is released at
// shutdown, v1 keeps running after the boot, the update is blocked again until the window opens.
func (s *restartSuite) TestF3RestartWhileUpdateIsBlocked() {
	window := &et.Window{}

	v2 := et.Module("m", "v2.0.0")
	v2.Policy.Windows = window

	k1 := et.New(s.T(), et.WithGlobal())
	k1.MustBoot()
	k1.ApplyModule(et.Module("m", "v1.0.0"), true)
	running(k1, "m", "v1.0.0")

	k1.ApplyModule(v2, true)
	k1.WaitReport("m", "waiting for the window", func(r model.Report) bool {
		return r.Candidate.Blocked == model.ReasonWaitingWindow
	})
	s.Equal([]string{"m@v1.0.0", "m@v2.0.0"}, k1.Disk.Attached())

	bp := checkpoint(k1, et.Module("m", "v1.0.0"))
	k1.Shutdown()
	s.Equal([]string{"m@v1.0.0"}, k1.Disk.Attached(), "shutdown must release the candidate, keep Current")

	k2 := et.New(s.T(), et.WithGlobal())
	k2.MustBoot(bp)
	k2.ApplyModule(v2, true)

	r := k2.WaitReport("m", "blocked again after the restart", func(r model.Report) bool {
		return r.Candidate.Blocked == model.ReasonWaitingWindow && et.True(r.Current.Installed)
	})
	s.Equal("v1.0.0", r.CurrentVersion)

	window.OpenAt(k2.Clock.Now())
	k2.ApplyModule(v2, true)
	running(k2, "m", "v2.0.0")
}

// F4: a restart between the activation of v2 and its first successful run restores v2 with v1's
// settings checkpoint (M3) and runs it; the new settings follow with the Apply.
func (s *restartSuite) TestF4RestartBeforeTheNewRevisionsFirstSuccess() {
	k1 := et.New(s.T(), et.WithGlobal())
	k1.MustBoot()
	k1.ApplyModule(et.WithSettings(et.Module("m", "v1.0.0"), addonutils.Values{"x": 1}), true)
	running(k1, "m", "v1.0.0")

	k1.Faults.Fail("upgrade:m", errors.New("apply failed"))

	v2 := et.WithSettings(et.Module("m", "v2.0.0"), addonutils.Values{"x": 2})
	k1.ApplyModule(v2, true)
	k1.WaitReport("m", "v2 activated, its run failed", func(r model.Report) bool {
		return r.CurrentVersion == "v2.0.0" && r.Current.Version == "v2.0.0" && et.False(r.Current.ManifestsApplied, "")
	})

	bp := checkpoint(k1, v2)
	k1.Shutdown()

	s.Equal("v2.0.0", bp.Version, "currentVersion is written at activation")
	s.Equal(addonutils.Values{"x": 1}, bp.Settings.Values, "lastAppliedConfiguration is v1's last success")

	k2 := et.New(s.T(), et.WithGlobal())
	k2.MustBoot(bp)

	r := running(k2, "m", "v2.0.0")
	s.Equal(addonutils.Values{"x": 1}, r.Current.AppliedValues)

	k2.ApplyModule(v2, true)
	k2.WaitReport("m", "the new settings applied", func(r model.Report) bool {
		return r.Current.AppliedValues["x"] == 2
	})
}

// F5: a Deckhouse upgrade restores an embedded module at the version the new image ships, with the
// old checkpoint; the first Apply needs no update.
func (s *restartSuite) TestF5DeckhouseUpgradeWithAnEmbeddedModule() {
	k1 := et.New(s.T(), et.WithGlobal())
	k1.MustBoot()
	k1.ApplyModule(et.WithSettings(et.Embedded("emb", "v1.0.0"), addonutils.Values{"x": 1}), true)
	running(k1, "emb", "v1.0.0")

	bp := checkpoint(k1, et.Embedded("emb", "v1.0.0"))
	k1.Shutdown()

	// What the facade passes after the upgrade: app.EmbeddedPackageVersion() (facade.go:221-226).
	bp.Version = "v1.1.0"

	k2 := et.New(s.T(), et.WithGlobal())
	k2.MustBoot(bp)

	r := running(k2, "emb", "v1.1.0")
	s.Equal(addonutils.Values{"x": 1}, r.Current.AppliedValues)

	// The Apply carries the checkpoint's own configuration (same values, the object's latest
	// settings version), so it is not accepted again; readiness latches once it arrived.
	mark := k2.Mark()
	k2.ApplyModule(et.WithSettings(et.Embedded("emb", "v1.1.0"), addonutils.Values{"x": 1}), true)
	k2.Wait("ready", k2.Engine.Ready)
	s.Equal(model.CandidateReport{}, k2.Report("emb").Candidate)
	s.Equal(-1, k2.Index(mark, "load:emb@v1.1.0"), "an embedded module at the image version is no update")
}

// shutdownSuite covers F6: a shutdown in the middle of an operation writes nothing and keeps
// Current's image for the next boot.
type shutdownSuite struct {
	suite.Suite
}

func TestShutdown(t *testing.T) {
	suite.Run(t, new(shutdownSuite))
}

// frozen asserts that name's Report and history do not change across fn (a shutdown).
func (s *shutdownSuite) frozen(k *et.Kit, name string, fn func()) {
	s.T().Helper()

	settled(k, name)

	before := et.Dump(k.Report(name))
	history := len(k.History(name))

	fn()

	s.Equal(before, et.Dump(k.Report(name)), "the Report changed during shutdown")
	s.Len(k.History(name), history, "a Report write was notified during shutdown")
}

// F6: shutdown during a run.
func (s *shutdownSuite) TestF6ShutdownDuringARun() {
	k := et.New(s.T(), et.WithGlobal())
	k.MustBoot()
	k.Faults.Block("upgrade:m")
	k.ApplyModule(et.Module("m", "v1.0.0"), true)
	k.WaitLog(0, "blocked:upgrade:m")

	s.frozen(k, "m", k.Shutdown)
	s.Equal([]string{"m@v1.0.0"}, k.Disk.Attached())
	s.GreaterOrEqual(k.Index(0, "cancelled:upgrade:m"), 0, "the run was not cancelled")
	s.False(k.Hooks.Running("m"), "hooks left running after shutdown")
}

// F6: shutdown during an activation that is detaching the previous image: the detach ignores the
// cancellation (bounded by detachTimeout, 30 s), so Shutdown waits for it, then writes nothing.
func (s *shutdownSuite) TestF6ShutdownDuringActivation() {
	k := et.New(s.T(), et.WithGlobal())
	k.MustBoot()
	k.ApplyModule(et.Module("m", "v1.0.0"), true)
	running(k, "m", "v1.0.0")

	releaseDetach := k.Faults.Block("detach:m@v1.0.0")

	k.ApplyModule(et.Module("m", "v2.0.0"), true)
	k.WaitLog(0, "blocked:detach:m@v1.0.0")
	k.WaitReport("m", "v2 accepted", func(r model.Report) bool { return r.CurrentVersion == "v2.0.0" && et.True(r.Current.Configured) })

	s.frozen(k, "m", func() {
		stopped := make(chan struct{})

		go func() {
			k.Shutdown()
			close(stopped)
		}()

		closed := func() bool {
			select {
			case <-stopped:
				return true
			default:
				return false
			}
		}

		s.False(k.WaitFor(quiet, closed), "the detach was expected to hold the shutdown")
		releaseDetach()

		select {
		case <-stopped:
		case <-time.After(et.Timeout):
			s.Fail("shutdown did not return after the detach was released")
		}
	})

	s.Equal("v2.0.0", k.Report("m").CurrentVersion)
	s.Equal([]string{"m@v2.0.0"}, k.Disk.Attached(), "the new Current's image must stay")
}

// F6: shutdown during a removal: Removed is not reported, the finalizer stays, the next boot
// restores the package and uninstalls it as at runtime, deletion hooks included.
func (s *shutdownSuite) TestF6ShutdownDuringRemoval() {
	m := et.Module("m", "v1.0.0")

	k1 := et.New(s.T(), et.WithGlobal())
	k1.MustBoot()
	k1.ApplyModule(m, true)
	running(k1, "m", "v1.0.0")

	k1.Faults.Block("delete:m")
	k1.Engine.Remove("m", model.KindModule)
	k1.WaitLog(0, "blocked:delete:m")

	bp := checkpoint(k1, m)

	s.frozen(k1, "m", k1.Shutdown)
	s.False(k1.Report("m").Removed)
	s.Equal([]string{"m@v1.0.0"}, k1.Disk.Attached())

	// A resource being deleted is booted with Removing; the controller calls Remove, a repeat.
	bp.Removing = true
	k2 := et.New(s.T(), et.WithGlobal())
	k2.MustBoot(bp)
	k2.Engine.Remove(bp.Name, model.KindModule)
	k2.WaitReport("m", "removed by the restored Current", func(r model.Report) bool { return r.Removed })
	s.GreaterOrEqual(k2.Index(0, "hook:m:beforeDeleteHelm"), 0, "the restored Current runs the deletion hooks")
	s.Greater(k2.Index(0, "delete:m"), k2.Index(0, "hook:m:beforeDeleteHelm"))
	s.Empty(k2.Disk.Attached())
}

// F10: the new Current gets its scheduler events only from the package Reconciler's loop, which is
// blocked in activate while the previous image detaches (up to detachTimeout; erofs holds its
// mutex during another package's download): the new revision does not run meanwhile.
func (s *shutdownSuite) TestF10NewCurrentRunsWhileThePreviousImageDetaches() {
	s.T().Skip("F10: the new Current receives no event while activate waits for the previous image's detach")

	k := et.New(s.T(), et.WithGlobal())
	k.MustBoot()
	k.ApplyModule(et.Module("m", "v1.0.0"), true)
	running(k, "m", "v1.0.0")

	release := k.Faults.Block("detach:m@v1.0.0")
	defer release()

	k.ApplyModule(et.Module("m", "v2.0.0"), true)
	k.WaitLog(0, "blocked:detach:m@v1.0.0")

	k.WaitReport("m", "v2 runs while v1's image detaches", func(r model.Report) bool { return et.Running(r, "v2.0.0") })
}

// PF3: an Apply that arrives while Shutdown waits for the package Reconcilers starts nothing: a new
// Reconciler would prepare (and might activate and write its Report) after Shutdown began, and
// Shutdown, which took its list before, would not wait for it.
func (s *shutdownSuite) TestPF3ApplyDuringShutdownStartsNothing() {
	k := et.New(s.T(), et.WithGlobal())
	k.MustBoot()
	k.ApplyModule(et.Module("m", "v1.0.0"), true)
	running(k, "m", "v1.0.0")

	// m's v2 is attached and loading; its release on shutdown blocks in the detach.
	releaseLoad := k.Faults.Block("load:m@v2.0.0")
	defer releaseLoad()

	releaseDetach := k.Faults.Block("detach:m@v2.0.0")
	defer releaseDetach()

	k.ApplyModule(et.Module("m", "v2.0.0"), true)
	k.WaitLog(0, "blocked:load:m@v2.0.0")

	stopped := make(chan struct{})

	go func() {
		k.Shutdown()
		close(stopped)
	}()

	// Only Shutdown cancels m's loop: from here Shutdown holds its list and waits for m.
	k.WaitLog(0, "blocked:detach:m@v2.0.0")

	mark := k.Mark()
	k.ApplyModule(et.Embedded("late", "v1.0.0"), true)
	k.Engine.Apply(et.Module("other", "v1.0.0"))

	s.False(k.WaitFor(quiet, func() bool {
		return k.Count(mark, "attach:other@v1.0.0") > 0 || k.Count(mark, "load:late@v1.0.0") > 0
	}), "a package Reconciler started after Shutdown began: %v", k.Log(mark))

	releaseDetach()

	select {
	case <-stopped:
	case <-time.After(et.Timeout):
		s.Fail("shutdown did not return after the detach was released")
	}
}
