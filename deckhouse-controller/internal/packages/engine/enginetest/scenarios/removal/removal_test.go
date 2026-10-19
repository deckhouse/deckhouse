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

package removal_test

import (
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Masterminds/semver/v3"
	"github.com/stretchr/testify/suite"

	et "github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/engine/enginetest"
	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/engine/lifecycle/model"
	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/engine/lifecycle/retry"
	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/engine/schedule"
	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/addonutils"
)

const (
	mod = "mod"
	v1  = "v1.0.0"
	v2  = "v2.0.0"
)

var errBoom = errors.New("boom")

// quiet bounds a "nothing happens" check: the engine reacts within milliseconds in the kit.
const quiet = 400 * time.Millisecond

// disableSuite covers scenarios D: disable and re-enable.
type disableSuite struct {
	suite.Suite
}

func TestDisable(t *testing.T) {
	suite.Run(t, new(disableSuite))
}

// removalSuite covers scenarios E: removal.
type removalSuite struct {
	suite.Suite
}

func TestRemoval(t *testing.T) {
	suite.Run(t, new(removalSuite))
}

// installed boots a kit with Global and installs mod at v1.
func installed(t *testing.T) *et.Kit {
	t.Helper()

	k := et.New(t, et.WithGlobal())
	k.MustBoot()
	k.ApplyModule(et.Module(mod, v1), true)
	k.WaitReport(mod, "installed", func(r model.Report) bool { return et.Running(r, v1) })

	return k
}

func disabled(r model.Report) bool {
	return r.Current.Decision != nil && !r.Current.Decision.Enabled
}

func (s *disableSuite) inOrder(k *et.Kit, mark int, entries ...string) {
	s.T().Helper()

	last := -1
	for _, e := range entries {
		i := k.Index(mark, e)
		s.Require().GreaterOrEqual(i, 0, "missing %s in %v", e, k.Log(mark))
		s.Greater(i, last, "%s out of order in %v", e, k.Log(mark))
		last = i
	}
}

// D1: the decision is reported at once; Installed stays True until the disable succeeded (G9).
func (s *disableSuite) TestD1DecisionAtOnceInstalledAfterSuccess() {
	k := installed(s.T())

	release := k.Faults.Block("delete:" + mod)
	k.ApplyModule(et.Module(mod, v1), false)

	r := k.WaitReport(mod, "decision reported while the delete is blocked", func(r model.Report) bool {
		return disabled(r) && k.Index(0, "blocked:delete:"+mod) >= 0
	})
	s.True(et.True(r.Current.Installed), "Installed stays True while the release is still there")
	s.NotEmpty(r.Current.Decision.Reason, "a disable carries the scheduler's reason")

	release()

	r = k.WaitReport(mod, "disabled", func(r model.Report) bool { return et.False(r.Current.Installed, model.ReasonDisabled) })
	s.True(et.True(r.Current.HooksProcessed))
	s.True(et.True(r.Current.ManifestsApplied))
}

// D1: the disable steps run in today's order and the hooks stop; the node and the image stay.
func (s *disableSuite) TestD1DisableStepsOrder() {
	k := installed(s.T())
	mark := k.Mark()

	k.ApplyModule(et.Module(mod, v1), false)
	k.WaitReport(mod, "disabled", func(r model.Report) bool { return et.False(r.Current.Installed, model.ReasonDisabled) })

	s.inOrder(k, mark, "remove-monitor:"+mod, "hook:"+mod+":beforeDeleteHelm", "delete:"+mod,
		"hook:"+mod+":afterDeleteHelm", "disable:"+mod, "disable-hooks:"+mod)
	s.False(k.Hooks.Running(mod))
	s.False(k.Nelm.Installed(mod))
	s.Equal(-1, k.Index(mark, "remove-node:"+mod), "a disabled package keeps its node")
	s.Equal([]string{mod + "@" + v1}, k.Disk.Attached(), "a disabled package keeps its image")
	s.Equal(v1, k.Report(mod).CurrentVersion)
	s.NotEmpty(k.Report(mod).Current.AppliedChecksum, "the checkpoint stays for the next boot")
}

// D1: a restored module that is disabled never started its hooks: only the release is deleted.
func (s *disableSuite) TestD1RestoredDisabledModuleSkipsDeletionHooks() {
	k := et.New(s.T(), et.WithGlobal())

	bp := et.Restored(et.Module(mod, v1), addonutils.Values{})
	bp.Intent = ptr(false)
	k.MustBoot(bp)

	r := k.WaitReport(mod, "disabled", func(r model.Report) bool { return et.False(r.Current.Installed, model.ReasonDisabled) })
	s.True(disabled(r))
	s.GreaterOrEqual(k.Index(0, "delete:"+mod), 0, "the release is deleted by the disable")
	s.Equal(-1, k.Index(0, "hook:"+mod+":beforeDeleteHelm"))
	s.Equal(-1, k.Index(0, "hook:"+mod+":afterDeleteHelm"))
	s.Equal(-1, k.Index(0, "enable-hooks:"+mod), "no hook of a disabled module starts at boot")
}

// D1: disabling resets the observed runtime hook failure.
func (s *disableSuite) TestD1DisableResetsObservedHookFailure() {
	k := installed(s.T())

	k.Hooks.RuntimeFailure(mod, "watch-things", "boom")
	s.NotEmpty(k.Observed.HookFailure(mod))

	k.ApplyModule(et.Module(mod, v1), false)
	k.WaitReport(mod, "disabled", func(r model.Report) bool { return et.False(r.Current.Installed, model.ReasonDisabled) })

	s.Empty(k.Observed.HookFailure(mod))
}

// D1: an application disabled by a requirement (the Kubernetes version) is disabled like a module.
func (s *disableSuite) TestD1ApplicationDisabledByRequirement() {
	var kube atomic.Pointer[semver.Version]
	kube.Store(semver.MustParse("1.31.0"))

	k := et.New(s.T(), et.WithScheduler(schedule.WithKubeVersionGetter(func() (*semver.Version, error) {
		return kube.Load(), nil
	})))
	k.MustBoot()

	app := et.App("ns", "app", v1)
	k.Loader.Set(app.Name, "", et.Spec{Constraints: schedule.Constraints{
		Order:      schedule.FunctionalOrder,
		Kubernetes: mustConstraint(">= 1.30"),
	}})

	k.Engine.Apply(app)
	k.WaitReport(app.Name, "installed", func(r model.Report) bool { return et.Running(r, v1) })

	kube.Store(semver.MustParse("1.29.0"))
	k.Engine.Recheck()

	r := k.WaitReport(app.Name, "disabled", func(r model.Report) bool { return et.False(r.Current.Installed, model.ReasonDisabled) })
	s.Equal("KubernetesRequirementsUnmet", r.Current.Decision.Reason)
	s.False(k.Nelm.Installed(app.Name))
	s.Zero(k.Count(0, "purge:"+app.Name), "a disable never purges")
}

// A failing disable is retried once per backoff step, however often a runtime hook changes the own
// inputs meanwhile: a disable deletes the release whatever the values are.
func (s *disableSuite) TestDisableRetryIgnoresRuntimeHookFirings() {
	k := installed(s.T())

	k.Faults.Fail("delete:"+mod, errBoom)
	k.ApplyModule(et.Module(mod, v1), false)
	k.WaitReport(mod, "the disable failed", func(r model.Report) bool {
		return et.False(r.Current.ManifestsApplied, model.ReasonReleaseDeleteFailed)
	})

	fire := func() {
		s.T().Helper()

		mark := k.Mark()
		for i := range 3 {
			s.Require().True(k.Hooks.RuntimeChange(mod, "x", i), "the hooks keep running while the disable fails")
		}

		k.Wait("the firings published", func() bool { return k.Count(mark, "runtime-hook:"+mod) == 3 })
	}

	fire()
	s.False(k.WaitFor(quiet, func() bool { return k.Count(0, "delete:"+mod) > 1 }), "retried outside the backoff: %v", k.Log(0))

	s.Equal(retry.Min, k.AdvanceToNextTimer("the first retry"))
	k.Wait("the first retry", func() bool { return k.Count(0, "delete:"+mod) == 2 })

	fire()
	s.False(k.WaitFor(quiet, func() bool { return k.Count(0, "delete:"+mod) > 2 }), "retried outside the backoff: %v", k.Log(0))

	s.Equal(2*retry.Min, k.AdvanceToNextTimer("the second retry"))
	k.Wait("the second retry", func() bool { return k.Count(0, "delete:"+mod) == 3 })
	k.WaitArmed("the second retry failed")

	k.Faults.Clear("delete:" + mod)
	s.Equal(4*retry.Min, k.AdvanceToNextTimer("the third retry"))
	k.WaitReport(mod, "disabled", func(r model.Report) bool { return et.False(r.Current.Installed, model.ReasonDisabled) })
}

// D2: a failing release delete keeps Installed True, reports ReleaseDeleteFailed and retries;
// readiness and convergence hold (D5).
func (s *disableSuite) TestD2DisableFailsAndRetries() {
	k := installed(s.T())
	// The facts of the first run reach the scheduler after the Report shows it.
	k.Wait("ready", k.Engine.Ready)

	k.Faults.Fail("delete:"+mod, errBoom)
	k.ApplyModule(et.Module(mod, v1), false)

	r := k.WaitReport(mod, "the disable failed", func(r model.Report) bool {
		return et.False(r.Current.ManifestsApplied, model.ReasonReleaseDeleteFailed)
	})
	s.True(et.True(r.Current.Installed), "the release is still there")
	s.True(disabled(r))

	s.Equal(retry.Min, k.AdvanceToNextTimer("the disable's retry"))
	k.Wait("a retry of the delete", func() bool { return k.Count(0, "delete:"+mod) == 2 })
	k.WaitArmed("the retry failed")
	// Disabling changes Global's enabled set, so Global reruns first; then everything converges.
	k.Wait("converged while the disable keeps failing", k.Engine.Converged)
	s.False(k.Report(mod).Removed)
	s.True(et.True(k.Report(mod).Current.Installed), "still True after the retries")
	s.True(k.Engine.Ready())

	k.Faults.Clear("delete:" + mod)
	s.Equal(2*retry.Min, k.AdvanceToNextTimer("the disable's second retry"))
	k.WaitReport(mod, "disabled after the fault cleared", func(r model.Report) bool {
		return et.False(r.Current.Installed, model.ReasonDisabled) && et.True(r.Current.ManifestsApplied)
	})
}

// D3: re-enabling installs again with a fresh enable of the hooks (OnStartup again).
func (s *disableSuite) TestD3ReEnable() {
	k := installed(s.T())

	k.ApplyModule(et.Module(mod, v1), false)
	k.WaitReport(mod, "disabled", func(r model.Report) bool { return et.False(r.Current.Installed, model.ReasonDisabled) })

	mark := k.Mark()
	k.ApplyModule(et.Module(mod, v1), true)

	r := k.WaitReport(mod, "enabled again", func(r model.Report) bool {
		return et.Running(r, v1) && r.Current.Decision != nil && r.Current.Decision.Enabled
	})
	s.True(et.True(r.Current.Installed))
	s.inOrder(k, mark, "enable-hooks:"+mod, "hook:"+mod+":onStartup", "upgraded:"+mod+"@"+v1)
	s.True(k.Hooks.Running(mod))
	s.True(k.Nelm.Installed(mod))
}

func (s *removalSuite) inOrder(k *et.Kit, mark int, entries ...string) {
	s.T().Helper()

	last := -1
	for _, e := range entries {
		i := k.Index(mark, e)
		s.Require().GreaterOrEqual(i, 0, "missing %s in %v", e, k.Log(mark))
		s.Greater(i, last, "%s out of order in %v", e, k.Log(mark))
		last = i
	}
}

func removed(r model.Report) bool { return r.Removed }

// E1: removing a module with a Current removes the node, uninstalls it, detaches the image.
func (s *removalSuite) TestE1RemoveModuleWithCurrent() {
	k := installed(s.T())
	mark := k.Mark()

	k.Engine.Remove(mod, model.KindModule)
	k.WaitReport(mod, "removed", removed)

	s.inOrder(k, mark, "remove-node:"+mod, "remove-monitor:"+mod, "hook:"+mod+":beforeDeleteHelm", "delete:"+mod,
		"hook:"+mod+":afterDeleteHelm", "disable:"+mod, "detach:"+mod+"@"+v1)
	s.Zero(k.Count(mark, "purge:"+mod), "a module has no purge")
	s.Empty(k.Disk.Attached())
	s.False(k.Hooks.Running(mod))
	s.False(k.Nelm.Installed(mod))
}

// E1: a repeated Remove keeps reporting Removed and does nothing more.
func (s *removalSuite) TestE1RepeatedRemoveKeepsRemoved() {
	k := installed(s.T())

	k.Engine.Remove(mod, model.KindModule)
	k.WaitReport(mod, "removed", removed)

	mark := k.Mark()
	k.Engine.Remove(mod, model.KindModule)

	s.True(k.Report(mod).Removed)
	for _, e := range k.Log(mark) {
		s.NotContains(e, ":"+mod, "a second Remove of a removed package acts on nothing")
	}
}

// E1: an Apply after the removal finished starts a fresh package (a recreated resource).
func (s *removalSuite) TestE1ApplyAfterRemovalStartsFresh() {
	k := installed(s.T())

	k.Engine.Remove(mod, model.KindModule)
	k.WaitReport(mod, "removed", removed)

	// The controller applies a recreated resource long after it saw Removed; Apply until the
	// exited package is replaced (see F40 for an Apply right at Removed).
	k.Wait("a fresh package takes the Apply", func() bool {
		k.ApplyModule(et.Module(mod, v1), true)

		return !k.Report(mod).Removed
	})

	r := k.WaitReport(mod, "installed again", func(r model.Report) bool { return et.Running(r, v1) })
	s.False(r.Removed)
	s.Equal([]string{mod + "@" + v1}, k.Disk.Attached())
}

// E1: an Apply while the removal runs is ignored (Kubernetes sends only Remove for a deleted object).
func (s *removalSuite) TestE1ApplyIgnoredWhileRemoving() {
	k := installed(s.T())

	release := k.Faults.Block("delete:" + mod)
	k.Engine.Remove(mod, model.KindModule)
	k.WaitLog(0, "blocked:delete:"+mod)

	k.ApplyModule(et.Module(mod, v2), true)
	release()

	k.WaitReport(mod, "removed", removed)
	s.Equal(-1, k.Index(0, "attach:"+mod+"@"+v2), "the Apply during the removal was dropped")
}

// E2: removing an application purges its leftovers after the uninstall.
func (s *removalSuite) TestE2RemoveApplicationPurgesLast() {
	k := et.New(s.T())
	k.MustBoot()

	app := et.App("ns", "app", v1)
	k.Engine.Apply(app)
	k.WaitReport(app.Name, "installed", func(r model.Report) bool { return et.Running(r, v1) })

	mark := k.Mark()
	k.Engine.Remove(app.Name, model.KindApplication)
	k.WaitReport(app.Name, "removed", removed)

	s.inOrder(k, mark, "remove-node:"+app.Name, "delete:"+app.Name, "disable:"+app.Name, "purge:"+app.Name,
		"detach:"+app.Name+"@"+v1)
}

// E3: Remove of a name the engine does not know deletes the release by name and reports Removed;
// a failure is reported in RemoveError and cleared on success.
func (s *removalSuite) TestE3RemoveUnknownDeletesByName() {
	k := et.New(s.T(), et.WithGlobal())
	k.MustBoot()

	k.Faults.FailTimes("delete:ghost", 1, errBoom)
	k.Engine.Remove("ghost", model.KindModule)

	k.WaitReport("ghost", "the failure reported", func(r model.Report) bool { return r.RemoveError != "" })
	s.Equal(retry.Min, k.AdvanceToNextTimer("the delete's retry"))
	r := k.WaitReport("ghost", "removed after the retry", removed)
	s.Empty(r.RemoveError, "cleared on success")
	s.Equal(2, k.Count(0, "delete:ghost"))
	s.GreaterOrEqual(k.Index(0, "remove-monitor:ghost"), 0)
	s.Equal(-1, k.Index(0, "hook:ghost:beforeDeleteHelm"), "no deletion hooks without a Current")
}

// E3: the by-name delete uses the release namespace of the kind (the fake nelm does not record
// namespaces, so the namespace function remove.go calls is checked directly).
func (s *removalSuite) TestE3ReleaseNamespaceByKind() {
	s.Equal(et.ModulesNamespace, model.ReleaseNamespace(model.KindModule, mod, et.ModulesNamespace))
	s.Equal("ns", model.ReleaseNamespace(model.KindApplication, "ns.app", et.ModulesNamespace))
}

// E3: a package that was never activated (its preparation keeps failing) is removed by name.
func (s *removalSuite) TestE3RemoveNeverActivated() {
	k := et.New(s.T())
	k.MustBoot()

	app := et.App("ns", "app", v1)
	k.Faults.Fail("attach:"+app.Name+"@"+v1, errBoom)
	k.Engine.Apply(app)
	k.WaitReport(app.Name, "preparation failed", func(r model.Report) bool { return r.Candidate.Step == "attach" })

	k.Engine.Remove(app.Name, model.KindApplication)
	k.WaitReport(app.Name, "removed", removed)

	s.GreaterOrEqual(k.Index(0, "delete:"+app.Name), 0)
	s.Equal(-1, k.Index(0, "purge:"+app.Name), "without a Current nothing is purged")
	s.Empty(k.Disk.Attached())
}

// E4: Remove during the first preparation cancels it, detaches what it attached, deletes by name.
func (s *removalSuite) TestE4RemoveDuringPreparation() {
	k := et.New(s.T(), et.WithGlobal())
	k.MustBoot()

	release := k.Faults.Block("load:" + mod + "@" + v1)
	defer release()

	k.ApplyModule(et.Module(mod, v1), true)
	k.WaitLog(0, "blocked:load:"+mod+"@"+v1)

	k.Engine.Remove(mod, model.KindModule)
	k.WaitReport(mod, "removed", removed)

	s.inOrder(k, 0, "cancelled:load:"+mod+"@"+v1, "detach:"+mod+"@"+v1, "delete:"+mod)
	s.Empty(k.Disk.Attached())
	s.Empty(k.Report(mod).CurrentVersion, "never activated")
}

// E5: Remove while an update waits for its window detaches the candidate and uninstalls Current.
func (s *removalSuite) TestE5RemoveWhileUpdateBlocked() {
	k := installed(s.T())

	d := et.Module(mod, v2)
	d.Policy.Windows = &et.Window{}
	k.ApplyModule(d, true)

	k.WaitReport(mod, "update blocked by the window", func(r model.Report) bool {
		return r.Candidate.Blocked == model.ReasonWaitingWindow
	})
	s.Equal([]string{mod + "@" + v1, mod + "@" + v2}, k.Disk.Attached())

	mark := k.Mark()
	k.Engine.Remove(mod, model.KindModule)
	k.WaitReport(mod, "removed", removed)

	s.inOrder(k, mark, "detach:"+mod+"@"+v2, "remove-node:"+mod, "delete:"+mod, "detach:"+mod+"@"+v1)
	s.Empty(k.Disk.Attached())
}

// E6: a failing release delete holds the finalizer and shows the failure in the Current section
// (the Deleting summary reads it, L3); the removal finishes once the fault clears.
func (s *removalSuite) TestE6UninstallDeleteFails() {
	k := installed(s.T())

	k.Faults.Fail("delete:"+mod, errBoom)
	k.Engine.Remove(mod, model.KindModule)

	r := k.WaitReport(mod, "uninstall failed", func(r model.Report) bool {
		return et.False(r.Current.ManifestsApplied, model.ReasonReleaseDeleteFailed)
	})
	s.False(r.Removed, "the finalizer is held")
	s.True(et.True(r.Current.Installed), "the release is still there")

	s.Equal(retry.Min, k.AdvanceToNextTimer("the uninstall's retry"))
	k.Wait("a retry", func() bool { return k.Count(0, "delete:"+mod) == 2 })
	k.WaitArmed("the retry failed")
	s.False(k.Report(mod).Removed)

	k.Faults.Clear("delete:" + mod)
	s.Equal(2*retry.Min, k.AdvanceToNextTimer("the uninstall's second retry"))
	k.WaitReport(mod, "removed after the fault cleared", removed)
	s.Empty(k.Disk.Attached())
}

// E6: a failing deletion hook holds the finalizer with HooksProcessed False.
func (s *removalSuite) TestE6UninstallHookFails() {
	k := installed(s.T())

	key := "run:" + mod + ":beforeDeleteHelm"
	k.Faults.Fail(key, errBoom)
	k.Engine.Remove(mod, model.KindModule)

	r := k.WaitReport(mod, "uninstall hook failed", func(r model.Report) bool {
		return et.False(r.Current.HooksProcessed, model.ReasonHookFailed)
	})
	s.False(r.Removed)
	s.Equal(-1, k.Index(0, "delete:"+mod), "the release stays while BeforeDeleteHelm fails")

	k.Faults.Clear(key)
	s.Equal(retry.Min, k.AdvanceToNextTimer("the uninstall's retry"))
	k.WaitReport(mod, "removed after the fault cleared", removed)
}

// E1: removing a disabled module runs no deletion hooks again (its hooks are stopped) and deletes
// the already absent release.
func (s *removalSuite) TestE1RemoveDisabledModule() {
	k := installed(s.T())

	k.ApplyModule(et.Module(mod, v1), false)
	k.WaitReport(mod, "disabled", func(r model.Report) bool { return et.False(r.Current.Installed, model.ReasonDisabled) })

	mark := k.Mark()
	k.Engine.Remove(mod, model.KindModule)
	k.WaitReport(mod, "removed", removed)

	s.Zero(k.Count(mark, "hook:"+mod+":beforeDeleteHelm"))
	s.Equal(1, k.Count(mark, "delete:"+mod))
	s.Empty(k.Disk.Attached())
}

func mustConstraint(c string) *semver.Constraints {
	constraint, err := semver.NewConstraint(c)
	if err != nil {
		panic(err)
	}

	return constraint
}

func ptr(b bool) *bool { return &b }

// E1 (F40): an Apply that arrives right after Removed was reported, maybe before the removed
// package's loop exited, starts a new package: Removed is the one signal of a finished removal.
func (s *removalSuite) TestE1ApplyRightAfterRemovedIsNotLost() {
	k := installed(s.T())

	k.Engine.Remove(mod, model.KindModule)
	k.WaitReport(mod, "removed", removed)

	k.ApplyModule(et.Module(mod, v1), true)

	k.WaitReport(mod, "installed again", func(r model.Report) bool { return !r.Removed && et.Running(r, v1) })
	s.Equal([]string{mod + "@" + v1}, k.Disk.Attached())
}
