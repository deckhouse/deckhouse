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
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/Masterminds/semver/v3"
	"github.com/stretchr/testify/suite"

	et "github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/engine/enginetest"
	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/engine/lifecycle/model"
	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/engine/schedule"
	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/engine/schedule/rule/script"
)

// schedulerSuite covers scenarios H: dependencies, cycles, enabled scripts, the barrier and
// readiness.
type schedulerSuite struct {
	base
}

func TestScheduler(t *testing.T) {
	suite.Run(t, new(schedulerSuite))
}

func dependsOn(names ...string) schedule.Constraints {
	deps := make(map[string]schedule.Dependency, len(names))
	for _, name := range names {
		deps[name] = schedule.Dependency{Constraint: mustConstraint(">= 1.0.0")}
	}

	return schedule.Constraints{Dependencies: deps}
}

func mustConstraint(c string) *semver.Constraints {
	constraint, err := semver.NewConstraint(c)
	if err != nil {
		panic(err)
	}

	return constraint
}

func blocked(reason, cause string) func(model.Report) bool {
	return func(r model.Report) bool {
		return r.Candidate.Blocked == reason && r.Candidate.Reason == cause
	}
}

// H1: a requirement on the package version resource naming a disabled module blocks the first
// install before anything is downloaded; enabling the dependency unblocks it.
func (s *schedulerSuite) TestH1MandatoryRequirementBlocksFirstInstallUntilEnabled() {
	k := et.New(s.T(), et.WithGlobal())
	k.MustBoot()

	// Declared on the version resource and in the module itself, as a real module does: the
	// barrier orders the runs by the loaded object's dependencies.
	b := et.Module("b", v1)
	b.Requirements = dependsOn("a")
	k.Loader.Set("b", "", et.Spec{Constraints: dependsOn("a")})
	k.ApplyModule(b, true)

	r := k.WaitReport("b", "blocked", blocked(model.ReasonRequirementsNotMet, "DependencyNotEnabled"))
	s.Equal(v1, r.Candidate.Version)
	s.Empty(r.CurrentVersion)
	s.Equal(-1, k.Index(0, "attach:b@v1.0.0"), "nothing is downloaded before the requirements pass")

	k.ApplyModule(et.Module("a", v1), true)
	s.installed(k, "a", v1)
	s.installed(k, "b", v1)
	s.inOrder(k, 0, "upgraded:a@v1.0.0", "upgraded:b@v1.0.0")
}

// H1: an optional dependency that is absent does not block.
func (s *schedulerSuite) TestH1OptionalDependencyAbsentDoesNotBlock() {
	k := et.New(s.T(), et.WithGlobal())
	k.MustBoot()

	c := dependsOn("a")
	dep := c.Dependencies["a"]
	dep.Optional = true
	c.Dependencies["a"] = dep
	k.Loader.Set("b", "", et.Spec{Constraints: c})

	k.ApplyModule(et.Module("b", v1), true)
	s.installed(k, "b", v1)
}

// H1: the loaded revision's own dependency is checked at the final gate: the candidate is
// prepared, then blocked with the scheduler's reason.
func (s *schedulerSuite) TestH1ObjectDependencyBlocksAtTheFinalGate() {
	k := et.New(s.T(), et.WithGlobal())
	k.MustBoot()

	k.Loader.Set("b", "", et.Spec{Constraints: dependsOn("a")})
	k.ApplyModule(et.Module("b", v1), true)

	k.WaitReport("b", "blocked at the final gate", blocked(model.ReasonRequirementsNotMet, "DependencyNotEnabled"))
	s.GreaterOrEqual(k.Index(0, "load:b@v1.0.0"), 0, "prepared before the final gate")
	s.Empty(k.Report("b").CurrentVersion)

	k.ApplyModule(et.Module("a", v1), true)
	s.installed(k, "b", v1)
}

// H1: an installed dependent is disabled when its dependency is disabled, and enabled again with it.
func (s *schedulerSuite) TestH1DisablingADependencyDisablesItsDependent() {
	k := et.New(s.T(), et.WithGlobal())
	k.MustBoot()

	k.Loader.Set("b", "", et.Spec{Constraints: dependsOn("a")})
	k.ApplyModule(et.Module("a", v1), true)
	k.ApplyModule(et.Module("b", v1), true)
	s.installed(k, "a", v1)
	s.installed(k, "b", v1)

	k.ApplyModule(et.Module("a", v1), false)

	r := k.WaitReport("b", "disabled", func(r model.Report) bool {
		return et.False(r.Current.Installed, model.ReasonDisabled)
	})
	s.Require().NotNil(r.Current.Decision)
	s.False(r.Current.Decision.Enabled)
	s.Equal("DependencyNotEnabled", r.Current.Decision.Reason)
	s.False(k.Nelm.Installed("b"))

	k.ApplyModule(et.Module("a", v1), true)
	k.WaitReport("b", "enabled again", func(r model.Report) bool { return et.Running(r, v1) })
}

// H1: AnyOf passes once one member is enabled.
func (s *schedulerSuite) TestH1AnyOfPassesWithOneMember() {
	k := et.New(s.T(), et.WithGlobal())
	k.MustBoot()

	k.Loader.Set("c", "", et.Spec{Constraints: schedule.Constraints{AnyOf: []schedule.AnyOfGroup{
		{Name: "cni", Members: map[string]*semver.Constraints{"a": nil, "x": nil}},
	}}})
	k.ApplyModule(et.Module("c", v1), true)

	k.WaitReport("c", "blocked", func(r model.Report) bool {
		return r.Candidate.Blocked == model.ReasonRequirementsNotMet && r.Candidate.Reason != ""
	})

	k.ApplyModule(et.Module("a", v1), true)
	s.installed(k, "c", v1)
}

// H1: NoneOf blocks while a member is enabled and passes once it is disabled.
func (s *schedulerSuite) TestH1NoneOfBlocksWhileAMemberIsEnabled() {
	k := et.New(s.T(), et.WithGlobal())
	k.MustBoot()

	k.ApplyModule(et.Module("a", v1), true)
	s.installed(k, "a", v1)

	k.Loader.Set("d", "", et.Spec{Constraints: schedule.Constraints{NoneOf: []schedule.NoneOfGroup{
		{Name: "conflict", Members: map[string]*semver.Constraints{"a": nil}},
	}}})
	k.ApplyModule(et.Module("d", v1), true)

	k.WaitReport("d", "blocked", func(r model.Report) bool {
		return r.Candidate.Blocked == model.ReasonRequirementsNotMet && r.Candidate.Reason != ""
	})

	k.ApplyModule(et.Module("a", v1), false)
	s.installed(k, "d", v1)
}

// H2: an update that would close a dependency cycle is blocked; the old Current keeps running.
func (s *schedulerSuite) TestH2CycleIsRejectedAndTheOldCurrentKept() {
	k := et.New(s.T(), et.WithGlobal())
	k.MustBoot()

	k.ApplyModule(et.Module("a", v1), true)
	k.ApplyModule(et.Module("b", v1), true)
	s.installed(k, "a", v1)
	s.installed(k, "b", v1)

	// a v2 needs b: fine. b v2 needs a: a cycle.
	k.Loader.Set("a", "v2.0.0", et.Spec{Constraints: dependsOn("b")})
	k.Loader.Set("b", "v2.0.0", et.Spec{Constraints: dependsOn("a")})

	k.ApplyModule(et.Module("a", "v2.0.0"), true)
	s.installed(k, "a", "v2.0.0")

	mark := k.Mark()
	k.ApplyModule(et.Module("b", "v2.0.0"), true)

	r := k.WaitReport("b", "cycle", blocked(model.ReasonRequirementsNotMet, schedule.ReasonDependencyCycle))
	s.Equal("v2.0.0", r.Candidate.Version)
	s.Equal(v1, r.CurrentVersion)
	s.True(et.True(r.Current.Installed))
	s.True(k.Nelm.Installed("b"))
	s.Equal(-1, k.Index(mark, "add-node:b@v2.0.0"))
}

// H2: two updates that are each fine but together close a cycle, activated at the same window
// start: one activates, the other is rejected (at the final gate or at AddNode) and keeps v1.
func (s *schedulerSuite) TestH2ConcurrentActivationsClosingACycle() {
	k := et.New(s.T(), et.WithGlobal())
	k.MustBoot()

	k.ApplyModule(et.Module("a", v1), true)
	k.ApplyModule(et.Module("b", v1), true)
	s.installed(k, "a", v1)
	s.installed(k, "b", v1)

	k.Loader.Set("a", "v2.0.0", et.Spec{Constraints: dependsOn("b")})
	k.Loader.Set("b", "v2.0.0", et.Spec{Constraints: dependsOn("a")})

	window := &et.Window{}
	for _, name := range []string{"a", "b"} {
		d := et.Module(name, "v2.0.0")
		d.Policy.Windows = window
		k.ApplyModule(d, true)
	}

	for _, name := range []string{"a", "b"} {
		k.WaitReport(name, "waiting for the window", blocked(model.ReasonWaitingWindow, ""))
	}

	// Both wake at the same window start: a new input re-reads NextAllowedTime into each timer.
	opens := k.Clock.Now().Add(time.Minute)
	window.OpenAt(opens)
	k.Engine.Recheck()
	k.Wait("both wake timers at the window start", func() bool { return k.Clock.ArmedAt(opens) == 2 })
	k.Clock.AdvanceTo(opens)

	k.Wait("one activated, the other rejected", func() bool {
		ra, rb := k.Report("a"), k.Report("b")
		cycle := blocked(model.ReasonRequirementsNotMet, schedule.ReasonDependencyCycle)

		return (et.Running(ra, "v2.0.0") && cycle(rb) && rb.CurrentVersion == v1) ||
			(et.Running(rb, "v2.0.0") && cycle(ra) && ra.CurrentVersion == v1)
	})
}

// H3: the enabled script decides the module: false disables it after activation, true (after a
// recheck) enables and runs it.
func (s *schedulerSuite) TestH3EnabledScript() {
	dir := s.T().TempDir()
	flag := filepath.Join(dir, "enabled")
	path := filepath.Join(dir, "enabled.sh")

	content := "#!/bin/sh\nif [ -f " + flag + " ]; then echo true > \"$MODULE_ENABLED_RESULT\"; " +
		"else echo false > \"$MODULE_ENABLED_RESULT\"; echo 'flag missing' > \"$MODULE_ENABLED_REASON\"; fi\n"
	s.Require().NoError(os.WriteFile(path, []byte(content), 0o755))

	k := et.New(s.T(), et.WithGlobal())
	k.MustBoot()

	k.Loader.Set("mod", "", et.Spec{EnabledScript: &script.Descriptor{Path: path}})
	k.ApplyModule(et.Module("mod", v1), true)

	r := k.WaitReport("mod", "disabled by script", func(r model.Report) bool {
		return r.Current.Decision != nil && !r.Current.Decision.Enabled && et.False(r.Current.Installed, model.ReasonDisabled)
	})
	s.Equal("DisabledByScript", r.Current.Decision.Reason)
	s.Equal(v1, r.CurrentVersion, "activated, then disabled by its decision")
	s.Equal(-1, k.Index(0, "upgraded:mod@v1.0.0"))

	s.Require().NoError(os.WriteFile(flag, nil, 0o600))
	k.Engine.Recheck()

	s.installed(k, "mod", v1)
}

// H4: a dependent and a higher tier wait for their dependency's first run (the barrier and tiers).
func (s *schedulerSuite) TestH4DependentWaitsForItsDependencyToConverge() {
	k := et.New(s.T(), et.WithGlobal())
	k.MustBoot()

	k.Loader.Set("b", "", et.Spec{Constraints: dependsOn("a")})
	k.Loader.Set("c", "", et.Spec{Constraints: schedule.Constraints{Order: 10}})

	release := k.Faults.Block("upgrade:a")
	defer release()

	k.ApplyModule(et.Module("a", v1), true)
	k.WaitLog(0, "blocked:upgrade:a")

	k.ApplyModule(et.Module("b", v1), true)
	k.ApplyModule(et.Module("c", v1), true)
	k.WaitLog(0, "add-node:b@v1.0.0")
	k.WaitLog(0, "add-node:c@v1.0.0")

	s.False(k.WaitFor(quietPeriod, func() bool { return k.Index(0, "upgrade:b") >= 0 || k.Index(0, "upgrade:c") >= 0 }),
		"a dependent ran before its dependency converged: %v", k.Log(0))

	release()

	s.installed(k, "b", v1)
	s.installed(k, "c", v1)
	s.inOrder(k, 0, "upgraded:a@v1.0.0", "upgrade:b")
	s.inOrder(k, 0, "upgraded:a@v1.0.0", "upgrade:c")
}

// H5: readiness and convergence count modules only: a failing application holds neither.
func (s *schedulerSuite) TestH5FailingApplicationDoesNotHoldReadiness() {
	k := et.New(s.T(), et.WithGlobal())
	k.MustBoot()

	k.Faults.Fail("upgrade:ns.app", errBoom)
	k.Engine.Apply(et.App("ns", "app", v1))
	k.ApplyModule(et.Module("mod", v1), true)

	s.installed(k, "mod", v1)
	k.WaitReport("ns.app", "failing", func(r model.Report) bool {
		return et.False(r.Current.ManifestsApplied, model.ReasonManifestsApplyFailed)
	})

	s.converged(k)
	s.True(k.Engine.Ready())
}

// H2: a blocked candidate is neither attached again nor revalidated on every wake (the settings
// memo of the final gate).
func (s *schedulerSuite) TestH2BlockedCandidateIsNotPreparedAgainOnEveryWake() {
	k := et.New(s.T(), et.WithGlobal())
	k.MustBoot()

	k.ApplyModule(et.Module("a", v1), true)
	s.installed(k, "a", v1)

	k.Loader.Set("b", "", et.Spec{Constraints: dependsOn("missing")})
	k.ApplyModule(et.Module("b", v1), true)
	k.WaitReport("b", "blocked", blocked(model.ReasonRequirementsNotMet, "DependencyNotEnabled"))

	validations := k.Count(0, "validate:b@v1.0.0")

	for i := range 5 {
		mark := k.Mark()

		k.Engine.Recheck()
		s.Require().True(k.Nelm.Drift("a"), "iteration %d", i)
		k.WaitReport("a", "reran", func(model.Report) bool { return upgrades(k, mark, "a", v1) == 1 })
		s.converged(k)
	}

	s.Equal(1, k.Count(0, "attach:b@v1.0.0"))
	s.Equal(1, k.Count(0, "load:b@v1.0.0"))
	s.Equal(validations, k.Count(0, "validate:b@v1.0.0"), "the settings check is memoized")
}

// H1: updating a dependency within its constraint neither disables nor reruns its dependent; the
// update reruns Global only (its enabled set carries name@version, the published names do not
// change).
func (s *schedulerSuite) TestH1UpdatingADependencyNeitherDisablesNorRerunsItsDependent() {
	k := et.New(s.T(), et.WithGlobal())
	k.MustBoot()

	k.Loader.Set("b", "", et.Spec{Constraints: dependsOn("a")})
	k.ApplyModule(et.Module("a", v1), true)
	k.ApplyModule(et.Module("b", v1), true)
	s.installed(k, "a", v1)
	s.installed(k, "b", v1)
	s.converged(k)

	mark := k.Mark()
	globalRuns := k.Count(0, "run:global:beforeAll")

	k.ApplyModule(et.Module("a", "v2.0.0"), true)
	s.installed(k, "a", "v2.0.0")
	s.converged(k)

	s.False(k.WaitFor(quietPeriod, func() bool { return upgrades(k, mark, "b", v1) > 0 }), "b reran: %v", k.Log(mark))
	s.Equal(-1, k.Index(mark, "delete:b"), "b was disabled during a's activation")
	s.Greater(k.Count(0, "run:global:beforeAll"), globalRuns, "Global reran for a@v2.0.0")

	r := k.Report("b")
	s.Require().NotNil(r.Current.Decision)
	s.True(r.Current.Decision.Enabled)
}

// H1: an update the enabled dependent does not accept is blocked before the activation, as the old
// module dependency extender refused such a release: the dependency keeps its version and the
// dependent keeps running.
func (s *schedulerSuite) TestH1UpdateRejectedByAnEnabledDependentIsBlocked() {
	k := et.New(s.T(), et.WithGlobal())
	k.MustBoot()

	k.Loader.Set("b", "", et.Spec{Constraints: schedule.Constraints{
		Dependencies: map[string]schedule.Dependency{"a": {Constraint: mustConstraint("< 2.0.0")}},
	}})
	k.ApplyModule(et.Module("a", v1), true)
	k.ApplyModule(et.Module("b", v1), true)
	s.installed(k, "a", v1)
	s.installed(k, "b", v1)
	s.converged(k)

	mark := k.Mark()

	k.ApplyModule(et.Module("a", "v2.0.0"), true)

	r := k.WaitReport("a", "update blocked by the dependent", func(r model.Report) bool {
		return r.Candidate.Blocked == model.ReasonRequirementsNotMet
	})
	s.Equal(schedule.ReasonDependentRequirementsUnmet, r.Candidate.Reason)
	s.Contains(r.Candidate.Message, "'b' requires 'a'")
	s.Equal(v1, r.CurrentVersion, "a keeps its version")

	s.Equal(-1, k.Index(mark, "delete:b"), "b was disabled")

	b := k.Report("b")
	s.Require().NotNil(b.Current.Decision)
	s.True(b.Current.Decision.Enabled)
}

func (s *schedulerSuite) TestH1UpdateBlockedByADependentActivatesOnceItIsDisabled() {
	k := et.New(s.T(), et.WithGlobal())
	k.MustBoot()

	k.Loader.Set("b", "", et.Spec{Constraints: schedule.Constraints{
		Dependencies: map[string]schedule.Dependency{"a": {Constraint: mustConstraint("< 2.0.0")}},
	}})
	k.ApplyModule(et.Module("a", v1), true)
	k.ApplyModule(et.Module("b", v1), true)
	s.installed(k, "a", v1)
	s.installed(k, "b", v1)
	s.converged(k)

	k.ApplyModule(et.Module("a", "v2.0.0"), true)
	k.WaitReport("a", "update blocked by the dependent", func(r model.Report) bool {
		return r.Candidate.Reason == schedule.ReasonDependentRequirementsUnmet
	})

	// No input reaches a: only the computation that disables b wakes its blocked candidate.
	k.ApplyModule(et.Module("b", v1), false)
	k.WaitReport("a", "activated once the dependent is disabled", func(r model.Report) bool {
		return r.CurrentVersion == "v2.0.0" && r.Candidate == model.CandidateReport{}
	})
}
