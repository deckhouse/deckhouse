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
	"context"
	"errors"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/suite"

	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/engine/core"
	et "github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/engine/enginetest"
	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/engine/lifecycle/model"
	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/engine/schedule"
	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/addonutils"
)

// installSuite covers scenarios A2–A6: first installs and boot.
type installSuite struct {
	suite.Suite
}

func TestInstall(t *testing.T) {
	suite.Run(t, new(installSuite))
}

// A2: an embedded module installs from the image directory: nothing is attached or detached.
func (s *installSuite) TestA2FirstInstallOfAnEmbeddedModule() {
	k := et.New(s.T(), et.WithGlobal())
	k.MustBoot()

	mark := k.Mark()
	k.ApplyModule(et.Embedded("emb", "v1.0.0"), true)

	r := running(k, "emb", "v1.0.0")
	s.Equal("v1.0.0", r.CurrentVersion)
	s.Empty(k.Disk.Attached())

	for _, e := range k.Log(mark) {
		s.False(strings.HasPrefix(e, "attach:") || strings.HasPrefix(e, "detach:"), "disk touched: %s", e)
	}

	s.GreaterOrEqual(k.Index(mark, "load:emb@v1.0.0"), 0)
	s.GreaterOrEqual(k.Index(mark, "upgraded:emb@v1.0.0"), 0)
}

// A3: an application (no Global, V2 off) installs under its namespaced name.
func (s *installSuite) TestA3FirstInstallOfAnApplication() {
	k := et.New(s.T())
	k.MustBoot()

	mark := k.Mark()
	k.Engine.Apply(et.App("ns", "app", "v1.0.0"))

	running(k, "ns.app", "v1.0.0")
	s.Equal([]string{"ns.app@v1.0.0"}, k.Disk.Attached())
	s.GreaterOrEqual(k.Index(mark, "upgraded:ns.app@v1.0.0"), 0)
	s.Equal(model.ReleaseNamespace(model.KindApplication, "ns.app", et.ModulesNamespace), "ns")
}

// A4: on a fresh cluster the bootstrap rule holds functional modules only; a critical one
// installs, and the functional one follows once the bootstrap is done (a Recheck).
func (s *installSuite) TestA4BootstrapHoldsFunctionalModulesOnly() {
	var bootstrapped atomic.Bool

	k := et.New(s.T(), et.WithGlobal(), et.WithScheduler(schedule.WithBootstrapCondition(bootstrapped.Load)))
	k.Loader.Set("critical", "", et.Spec{Constraints: schedule.Constraints{Order: 10}})
	k.Loader.Set("functional", "", et.Spec{Constraints: schedule.Constraints{Order: schedule.FunctionalOrder}})
	k.MustBoot()

	critical := et.Module("critical", "v1.0.0")
	critical.Requirements.Order = 10
	functional := et.Module("functional", "v1.0.0")
	functional.Requirements.Order = schedule.FunctionalOrder

	k.ApplyModule(critical, true)
	k.ApplyModule(functional, true)

	running(k, "critical", "v1.0.0")

	r := k.WaitReport("functional", "held by the bootstrap rule", func(r model.Report) bool {
		return r.Candidate.Blocked == model.ReasonRequirementsNotMet
	})
	s.Equal("BootstrapRequirementsUnmet", r.Candidate.Reason, "the scheduler's own reason (G3)")
	s.Empty(r.CurrentVersion)

	bootstrapped.Store(true)
	k.Engine.Recheck()

	running(k, "functional", "v1.0.0")
}

// A5: the boot gate: restored modules run only after Global synchronized, once each, without an
// Apply.
func (s *installSuite) TestA5RestoredModulesRunAfterGlobalSyncWithoutApply() {
	k := et.New(s.T(), et.WithGlobal())
	release := k.Faults.Block("enable-hooks:global")

	m1 := et.Module("m1", "v1.0.0")
	m2 := et.Module("m2", "v1.0.0")

	booted := make(chan error, 1)

	go func() {
		booted <- k.Boot(enabled(et.Restored(m1, addonutils.Values{})), enabled(et.Restored(m2, addonutils.Values{})))
	}()

	k.WaitLog(0, "blocked:enable-hooks:global")
	s.False(k.WaitFor(quiet, func() bool { return k.Index(0, "upgrade:m1") >= 0 || k.Index(0, "upgrade:m2") >= 0 }),
		"a restored module ran before Global synchronized")
	s.Equal("v1.0.0", k.Report("m1").CurrentVersion, "the restored version is reported at once")

	release()
	s.Require().NoError(<-booted)

	running(k, "m1", "v1.0.0")
	running(k, "m2", "v1.0.0")
	s.Equal(1, k.Count(0, "upgraded:m1@v1.0.0"))
	s.Less(k.Index(0, "enable-hooks:global"), k.Index(0, "upgrade:m1"))
}

// A5: readiness latches only once every restored module was applied (M3), never waiting for a
// first install that Boot did not restore.
func (s *installSuite) TestA5ReadinessWaitsForRestoredModulesOnly() {
	k := et.New(s.T(), et.WithGlobal())

	m1 := et.Module("m1", "v1.0.0")
	m2 := et.Module("m2", "v1.0.0")
	first := model.BootPackage{Name: "m3", Kind: model.KindModule}

	k.MustBoot(enabled(et.Restored(m1, addonutils.Values{})), enabled(et.Restored(m2, addonutils.Values{})), first)
	running(k, "m1", "v1.0.0")
	running(k, "m2", "v1.0.0")

	k.Wait("converged", k.Engine.Converged)
	s.False(k.Engine.Ready(), "ready before the controllers applied the restored modules")

	k.ApplyModule(m1, true)
	s.False(k.Engine.Ready(), "ready with m2 not applied")

	k.ApplyModule(m2, true)
	k.Wait("ready", k.Engine.Ready)
}

// A5: a restored package without a settings checkpoint (installed before the field existed)
// waits for the first Apply instead of running with defaults.
func (s *installSuite) TestA5RestoredWithoutCheckpointWaitsForApply() {
	k := et.New(s.T(), et.WithGlobal())
	m := et.Module("m", "v1.0.0")

	k.MustBoot(enabled(et.Restored(m, nil)))

	s.False(k.WaitFor(quiet, func() bool { return k.Index(0, "upgrade:m") >= 0 }), "ran without settings")

	k.ApplyModule(et.WithSettings(m, addonutils.Values{"x": 1}), true)

	r := running(k, "m", "v1.0.0")
	s.Equal(addonutils.Values{"x": 1}, r.Current.AppliedValues)
}

// A5: a restored checkpoint holds effective values in the restored object's latest schema; it is
// applied at that version, never at the version the settings were once written for (BOOT-03).
func (s *installSuite) TestA5RestoredCheckpointIsAppliedAtTheLatestSchema() {
	k := et.New(s.T(), et.WithGlobal())
	k.Loader.Set("m", "v1.0.0", et.Spec{LatestSettingsVersion: 3})

	k.MustBoot(enabled(et.Restored(et.Module("m", "v1.0.0"), addonutils.Values{"x": 1})))
	running(k, "m", "v1.0.0")

	s.Equal(3, k.Loader.Latest("m").AppliedSettingsVersion())
}

// A6: the registry is unreachable at boot: Boot fails and nothing half-starts.
func (s *installSuite) TestA6BootFailsWhenRestoreCannotAttach() {
	k := et.New(s.T(), et.WithGlobal())
	k.Faults.Fail("attach:m@v1.0.0", errors.New("registry unreachable"))

	err := k.Boot(enabled(et.Restored(et.Module("m", "v1.0.0"), addonutils.Values{})),
		enabled(et.Restored(et.Module("other", "v1.0.0"), addonutils.Values{})))
	s.Require().Error(err)
	s.Contains(err.Error(), "registry unreachable")

	_, known := k.Engine.Report("m")
	s.False(known, "a package Reconciler was started after a failed restore")

	_, known = k.Engine.Report("other")
	s.False(known)

	s.False(k.Engine.Converged(), "the scheduler resumed after a failed boot")
	s.False(k.Engine.Ready())

	k.Shutdown() // must return after a failed Boot
}

// A5: the caller's context ends the boot gate's wait for Global's hooks before its own deadline.
func (s *installSuite) TestA5BootGateEndsWithTheCallersContext() {
	k := et.New(s.T(), et.WithGlobal())
	release := k.Faults.Block("enable-hooks:global")
	defer release()

	ctx, cancel := context.WithTimeout(context.Background(), quiet)
	defer cancel()

	err := k.Engine.Boot(ctx, &core.BootGlobal{Module: k.Global, Settings: k.GlobalSettings}, nil)
	s.Require().Error(err)
	s.ErrorIs(err, core.ErrGlobalNotSynced)
	s.False(k.Engine.Converged())
}

// A global synchronization that never completes fails Boot after ten minutes on the engine clock,
// also when the caller's context has no deadline.
func (s *installSuite) TestA5BootGateFailsAfterTenMinutes() {
	k := et.New(s.T(), et.WithGlobal())
	release := k.Faults.Block("enable-hooks:global")
	defer release()

	booted := make(chan error, 1)
	go func() {
		booted <- k.Engine.Boot(context.Background(), &core.BootGlobal{Module: k.Global, Settings: k.GlobalSettings}, nil)
	}()

	s.Equal(10*time.Minute, k.AdvanceToNextTimer("the boot deadline"))

	select {
	case err := <-booted:
		s.Require().ErrorIs(err, core.ErrGlobalNotSynced)
	case <-time.After(et.Timeout):
		s.FailNow("Boot did not return after its deadline")
	}

	s.False(k.Engine.Converged())
}
