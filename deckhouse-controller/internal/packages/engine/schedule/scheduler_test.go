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

package schedule_test

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/Masterminds/semver/v3"
	"github.com/stretchr/testify/suite"

	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/engine/schedule"
	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/engine/schedule/rule"
	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/engine/schedule/rule/script"
	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/addonutils"
	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/edition"
	"github.com/deckhouse/deckhouse/pkg/log"
)

// globalName is the literal sentinel used by Scheduler internally; it lives
// here as a constant so the tests document the contract rather than scatter
// magic strings.
const globalName = "global"

// testPackage describes a node for the tests; production nodes are built by the package engine
// (lifecycle.schedulerNode) from apps.Application and modules.Module.
type testPackage struct {
	name          string
	version       *semver.Version
	module        bool
	constraints   schedule.Constraints
	enabledScript *script.Descriptor
	revision      string // revision overrides the default, version.String().
	attachment    any
}

// node returns the scheduler node of p, defaulting the Floor to an always-Enable rule (an
// application's "enabled once loaded") for cases that don't care about enablement. Cases
// exercising disablement set their own Floor.
func (p *testPackage) node() schedule.Node {
	c := p.constraints
	if c.Floor == nil {
		c.Floor = rule.Static(rule.Enable)
	}

	n := schedule.Node{Name: p.name, Version: p.version, Revision: p.revision, Module: p.module, Constraints: c, Attachment: p.attachment}
	if n.Revision == "" && p.version != nil {
		n.Revision = p.version.String()
	}

	if d := p.enabledScript; d != nil {
		n.EnabledScript = func() *script.Descriptor { return d }
	}

	return n
}

// mustVersion parses s into a *semver.Version or panics; tests use known-good values.
func mustVersion(s string) *semver.Version {
	v, err := semver.NewVersion(s)
	if err != nil {
		panic(err)
	}

	return v
}

// SchedulerSuite exercises the public Scheduler API end-to-end.
type SchedulerSuite struct {
	suite.Suite

	versions map[string]*semver.Version
	sched    *schedule.Scheduler

	// last is the latest event taken per node: converge reads the node's revision, own inputs and
	// Inputs from it, as a Current reads them from its own event.
	last map[string]schedule.Event
	// globalValues counts the global values changes recompute simulates.
	globalValues int
}

// TestSchedulerSuite is the testing.T entry point that runs the suite.
func TestSchedulerSuite(t *testing.T) {
	suite.Run(t, new(SchedulerSuite))
}

// SetupTest builds a fresh scheduler and version map for every test so cases
// remain isolated. The dependency getter reads from s.versions, letting each
// test simulate module presence/absence by direct map mutation.
func (s *SchedulerSuite) SetupTest() {
	s.versions = make(map[string]*semver.Version)
	s.last = make(map[string]schedule.Event)
	s.globalValues = 0
	s.sched = schedule.NewScheduler(
		log.NewNop(),
		schedule.WithDependencyGetter(func(name string) *semver.Version {
			return s.versions[name]
		}),
	)
}

// addNode adds the node of p.
func (s *SchedulerSuite) addNode(p *testPackage) error {
	return s.sched.AddNode(p.node())
}

// TearDownTest stops the scheduler, closing its wake channel.
func (s *SchedulerSuite) TearDownTest() {
	s.sched.Stop()
}

// activateGlobal registers the implicit global package, resumes the scheduler,
// and converges global. Drains all setup events so the test can assert only on
// the events it triggers itself.
func (s *SchedulerSuite) activateGlobal() {
	s.Require().NoError(s.addNode(&testPackage{
		name:    globalName,
		version: mustVersion("1.0.0"),
		constraints: schedule.Constraints{
			Order: 0,
		},
	}))

	s.sched.Resume()
	s.converge(globalName)
	s.drainEvents()
}

// take takes the pending events, remembers the latest one per node in s.last and returns them.
func (s *SchedulerSuite) take() []schedule.Event {
	events := s.sched.Take()
	for _, e := range events {
		s.last[e.Name] = e
	}

	return events
}

// converge publishes the facts of a successful run of the named node: Applied equals the Inputs of
// its latest event, with the own inputs that event echoed. The node is converged until its inputs
// change.
func (s *SchedulerSuite) converge(name string) {
	s.take()

	e, ok := s.last[name]
	s.Require().True(ok, "no event for %s", name)

	s.sched.Publish(name, schedule.Facts{Version: e.Version, OwnInputs: e.OwnInputs, Applied: e.Inputs})
}

// recompute simulates a change of the global values (which the version, bootstrap and hook
// getters read) followed by a successful global run: global publishes new own inputs, then its
// Applied for the new inputs. Each Publish computes, so the rule chains are evaluated again.
func (s *SchedulerSuite) recompute() {
	s.globalValues++
	ownInputs := fmt.Sprintf("global-values-%d", s.globalValues)

	s.take()
	e := s.last[globalName]
	s.sched.Publish(globalName, schedule.Facts{Version: e.Version, OwnInputs: ownInputs})

	s.take()
	e = s.last[globalName]
	s.sched.Publish(globalName, schedule.Facts{Version: e.Version, OwnInputs: ownInputs, Applied: e.Inputs})
}

// drainEvents empties the scheduler's pending events.
func (s *SchedulerSuite) drainEvents() {
	s.take()
}

// collectEvents takes the events of the latest computation. Scheduler operations are synchronous,
// so the events computed by the preceding call are pending by the time this runs.
func (s *SchedulerSuite) collectEvents() []schedule.Event {
	return s.take()
}

// gated returns the names of the events whose gate is open: the nodes that may run now.
func gated(events []schedule.Event) []string {
	var names []string
	for _, e := range events {
		if e.Gate {
			names = append(names, e.Name)
		}
	}

	return names
}

// disabled returns the names of the events whose decision is negative.
func disabled(events []schedule.Event) []string {
	var names []string
	for _, e := range events {
		if !e.Enabled {
			names = append(names, e.Name)
		}
	}

	return names
}

// TestAddNodeAndSchedule covers the happy-path lifecycle: a node added after
// global converged is enabled and its gate opens in the same computation.
func (s *SchedulerSuite) TestAddNodeAndSchedule() {
	s.activateGlobal()

	s.Require().NoError(s.addNode(&testPackage{
		name:    "alpha",
		version: mustVersion("1.0.0"),
		constraints: schedule.Constraints{
			Order: schedule.FunctionalOrder,
		},
	}))

	s.Contains(gated(s.collectEvents()), "alpha")
}

// TestFloorDisableKeepsNodeOff confirms the package-supplied floor governs
// enablement: a node whose Floor resolves to Disable never has its gate open,
// even though no requirement vetoes it.
func (s *SchedulerSuite) TestFloorDisableKeepsNodeOff() {
	s.activateGlobal()

	s.Require().NoError(s.addNode(&testPackage{
		name:    "alpha",
		version: mustVersion("1.0.0"),
		constraints: schedule.Constraints{
			Order: schedule.FunctionalOrder,
			Floor: rule.Static(rule.Disable),
		},
	}))

	s.NotContains(gated(s.collectEvents()), "alpha",
		"a node with a Disable floor must not be scheduled")
}

// TestOrderTierGate confirms that the gate's order-tier check holds a
// higher-tier node back until every lower-tier node is converged.
func (s *SchedulerSuite) TestOrderTierGate() {
	s.activateGlobal()

	s.Require().NoError(s.addNode(&testPackage{
		name:        "critical",
		version:     mustVersion("1.0.0"),
		constraints: schedule.Constraints{Order: 1},
	}))

	s.Require().NoError(s.addNode(&testPackage{
		name:        "functional",
		version:     mustVersion("1.0.0"),
		constraints: schedule.Constraints{Order: schedule.FunctionalOrder},
	}))

	scheduled := gated(s.collectEvents())
	s.Contains(scheduled, "critical")
	s.NotContains(scheduled, "functional", "functional must wait for critical to be converged")

	s.converge("critical")
	s.Contains(gated(s.collectEvents()), "functional")
}

// TestUnconvergedNamesRevisionAndBlocker confirms the debug view of the convergence: every
// unconverged module with the revision being converged and what holds its gate, a tier named by the
// nodes of the lowest unconverged one.
func (s *SchedulerSuite) TestUnconvergedNamesRevisionAndBlocker() {
	s.activateGlobal()

	for _, p := range []*testPackage{
		{name: "deckhouse", version: mustVersion("1.0.0"), module: true, revision: "v1.0.0", constraints: schedule.Constraints{Order: 2}},
		{name: "cni", version: mustVersion("1.0.0"), module: true, revision: "v1.0.0", constraints: schedule.Constraints{Order: 2}},
		{name: "node-manager", version: mustVersion("1.0.0"), module: true, revision: "v1.0.0", constraints: schedule.Constraints{Order: 40}},
		{name: "console", version: mustVersion("2.0.0"), module: true, revision: "v2.0.0", constraints: schedule.Constraints{Order: schedule.FunctionalOrder}},
	} {
		s.Require().NoError(s.addNode(p))
	}

	// The modules' revisions are Global's inputs: it is the barrier again until it runs.
	s.Equal("global", s.sched.Unconverged().Nodes["console"].WaitsFor)
	s.converge(globalName)

	s.Equal(map[string]schedule.UnconvergedNode{
		"deckhouse":    {Revision: "v1.0.0", Order: 2},
		"cni":          {Revision: "v1.0.0", Order: 2},
		"node-manager": {Revision: "v1.0.0", Order: 40, WaitsFor: "cni, deckhouse"},
		"console":      {Revision: "v2.0.0", Order: schedule.FunctionalOrder, WaitsFor: "cni, deckhouse"},
	}, s.sched.Unconverged().Nodes)

	s.converge("deckhouse")
	s.converge("cni")
	s.Equal("node-manager", s.sched.Unconverged().Nodes["console"].WaitsFor)
}

// TestDisabledNodeUnblocksHigherTier exercises "a disabled node is converged":
// a lower-tier node that loses eligibility must not stall higher tiers via the
// order-tier gate.
func (s *SchedulerSuite) TestDisabledNodeUnblocksHigherTier() {
	s.activateGlobal()

	s.Require().NoError(s.addNode(&testPackage{
		name:    "lower",
		version: mustVersion("1.0.0"),
		constraints: schedule.Constraints{
			Order: 1,
			Dependencies: map[string]schedule.Dependency{
				"never-installed": {},
			},
		},
	}))

	s.Require().NoError(s.addNode(&testPackage{
		name:        "higher",
		version:     mustVersion("1.0.0"),
		constraints: schedule.Constraints{Order: schedule.FunctionalOrder},
	}))

	scheduled := gated(s.collectEvents())
	s.NotContains(scheduled, "lower", "lower lost eligibility and must not be scheduled")
	s.Contains(scheduled, "higher", "higher must not be blocked by the disabled lower-tier node")
}

// TestMandatoryDependency verifies that a mandatory dep being absent disables
// the consumer, and that installing the dep flips it back to enabled.
func (s *SchedulerSuite) TestMandatoryDependency() {
	s.activateGlobal()

	s.Require().NoError(s.addNode(&testPackage{
		name:    "consumer",
		version: mustVersion("1.0.0"),
		constraints: schedule.Constraints{
			Order: schedule.FunctionalOrder,
			Dependencies: map[string]schedule.Dependency{
				"parent": {},
			},
		},
	}))

	// Consumer is born disabled (parent absent): its gate stays closed.
	s.NotContains(gated(s.collectEvents()), "consumer")

	s.versions["parent"] = mustVersion("1.0.0")
	s.recompute()

	s.Contains(gated(s.collectEvents()), "consumer")
}

// TestConditionalDependencyAbsentIsOK confirms that an optional dep being
// absent does NOT disable the consumer — only a version mismatch would.
func (s *SchedulerSuite) TestConditionalDependencyAbsentIsOK() {
	s.activateGlobal()

	s.Require().NoError(s.addNode(&testPackage{
		name:    "consumer",
		version: mustVersion("1.0.0"),
		constraints: schedule.Constraints{
			Order: schedule.FunctionalOrder,
			Dependencies: map[string]schedule.Dependency{
				"optional": {Optional: true},
			},
		},
	}))

	events := s.collectEvents()
	s.Contains(gated(events), "consumer")
	s.NotContains(disabled(events), "consumer")
}

// TestEnabledToDisabledFlipReportsDisabled verifies that the event of a
// previously-enabled node reports it disabled once it loses eligibility (e.g. its
// dependency is removed from the cluster).
func (s *SchedulerSuite) TestEnabledToDisabledFlipReportsDisabled() {
	s.activateGlobal()

	s.versions["parent"] = mustVersion("1.0.0")
	s.Require().NoError(s.addNode(&testPackage{
		name:    "consumer",
		version: mustVersion("1.0.0"),
		constraints: schedule.Constraints{
			Order: schedule.FunctionalOrder,
			Dependencies: map[string]schedule.Dependency{
				"parent": {},
			},
		},
	}))

	s.Contains(gated(s.collectEvents()), "consumer")

	// Parent disappears — consumer must flip enabled→disabled.
	delete(s.versions, "parent")
	s.recompute()

	s.Contains(disabled(s.collectEvents()), "consumer")
}

// TestAddNodeRejectsCyclicAddition pins AddNode as the authoritative cycle
// gate: when adding a node would close a dep cycle with an already-registered
// node, AddNode returns a *ConstraintError wrapping a *CycleError without
// mutating the graph. Subsequent higher-tier additions are gated normally
// because the cycle never entered the graph.
func (s *SchedulerSuite) TestAddNodeRejectsCyclicAddition() {
	s.activateGlobal()

	s.Require().NoError(s.addNode(&testPackage{
		name:    "alpha",
		version: mustVersion("1.0.0"),
		constraints: schedule.Constraints{
			Order: 1,
			Dependencies: map[string]schedule.Dependency{
				"beta": {},
			},
		},
	}))

	err := s.addNode(&testPackage{
		name:    "beta",
		version: mustVersion("1.0.0"),
		constraints: schedule.Constraints{
			Order: 1,
			Dependencies: map[string]schedule.Dependency{
				"alpha": {},
			},
		},
	})

	s.Require().Error(err)

	var cyc *schedule.CycleError
	s.Require().ErrorAs(err, &cyc)
	s.ElementsMatch([]string{"alpha", "beta"}, cyc.Members)

	// Cycle never entered the graph — a higher-tier consumer still schedules.
	s.Require().NoError(s.addNode(&testPackage{
		name:        "consumer",
		version:     mustVersion("1.0.0"),
		constraints: schedule.Constraints{Order: schedule.FunctionalOrder},
	}))

	s.Contains(gated(s.collectEvents()), "consumer")
}

// TestCheckConstraintsRejectsDependencyCycle pins the admission-time cycle
// gate: when checking constraints for a node whose dependencies would close
// a cycle with an already-registered node, CheckConstraints returns a
// *ConstraintError wrapping a *CycleError naming the participants. The graph
// is not mutated.
func (s *SchedulerSuite) TestCheckConstraintsRejectsDependencyCycle() {
	s.activateGlobal()

	// alpha depends on a future beta; no cycle yet (beta isn't in the graph).
	s.Require().NoError(s.addNode(&testPackage{
		name:    "alpha",
		version: mustVersion("1.0.0"),
		constraints: schedule.Constraints{
			Order: 1,
			Dependencies: map[string]schedule.Dependency{
				"beta": {},
			},
		},
	}))
	s.drainEvents()

	// Populate versions so the dep checker chain passes (proposed beta's dep
	// on alpha is satisfied); cycle simulation is what we want to evaluate.
	s.versions["alpha"] = mustVersion("1.0.0")

	// Proposed beta depends on alpha. Adding it would create alpha → beta → alpha.
	err := s.sched.CheckConstraints("beta", schedule.Constraints{
		Order: 1,
		Dependencies: map[string]schedule.Dependency{
			"alpha": {},
		},
	})

	s.Require().Error(err)

	var cyc *schedule.CycleError
	s.Require().ErrorAs(err, &cyc)
	s.ElementsMatch([]string{"alpha", "beta"}, cyc.Members)
}

// TestCheckDependentsRejectsAVersionAnEnabledDependentRefuses pins the reverse-dependency check:
// replacing lib by a version an enabled dependent's constraint rejects is refused, naming the
// dependent; a version within the constraint passes; a disabled dependent does not count; an
// optional dependency that is present counts as well. The graph is not mutated.
func (s *SchedulerSuite) TestCheckDependentsRejectsAVersionAnEnabledDependentRefuses() {
	s.activateGlobal()

	s.versions["lib"] = mustVersion("1.0.0")
	s.Require().NoError(s.addNode(&testPackage{name: "lib", version: mustVersion("1.0.0"), constraints: schedule.Constraints{Order: 1}}))

	below2 := func() *semver.Constraints {
		c, err := semver.NewConstraint("<2.0")
		s.Require().NoError(err)

		return c
	}

	s.Require().NoError(s.addNode(&testPackage{
		name:    "app",
		version: mustVersion("1.0.0"),
		constraints: schedule.Constraints{
			Order:        2,
			Dependencies: map[string]schedule.Dependency{"lib": {Constraint: below2()}},
		},
	}))
	s.Require().NoError(s.addNode(&testPackage{
		name:    "opt",
		version: mustVersion("1.0.0"),
		constraints: schedule.Constraints{
			Order:        2,
			Dependencies: map[string]schedule.Dependency{"lib": {Constraint: below2(), Optional: true}},
		},
	}))
	s.Require().NoError(s.addNode(&testPackage{
		name:    "off",
		version: mustVersion("1.0.0"),
		constraints: schedule.Constraints{
			Order:        2,
			Floor:        rule.Static(rule.Disable),
			Dependencies: map[string]schedule.Dependency{"lib": {Constraint: below2()}},
		},
	}))
	s.drainEvents()

	s.Require().NoError(s.sched.CheckDependents("lib", mustVersion("1.5.0")), "within the constraint")
	s.Require().NoError(s.sched.CheckDependents("lib", nil), "no version, nothing to compare")

	err := s.sched.CheckDependents("lib", mustVersion("2.0.0"))
	s.Require().Error(err)

	var ce *schedule.ConstraintError
	s.Require().ErrorAs(err, &ce)
	s.Equal(schedule.ReasonDependentRequirementsUnmet, ce.Reason)
	s.Contains(ce.Message, "'app' requires 'lib'")
	s.Contains(ce.Message, "'opt' requires 'lib'", "a present optional dependency must match too")
	s.NotContains(ce.Message, "'off'", "a disabled dependent is not disrupted")
}

// TestPauseSuppressesScheduling confirms that AddNode does not compute while
// the scheduler is paused, and that Resume computes once.
func (s *SchedulerSuite) TestPauseSuppressesScheduling() {
	// Scheduler starts paused by construction.
	s.Require().NoError(s.addNode(&testPackage{
		name:        globalName,
		version:     mustVersion("1.0.0"),
		constraints: schedule.Constraints{Order: 0},
	}))

	s.Empty(s.collectEvents(), "paused scheduler must not emit events")

	s.sched.Resume()
	s.Contains(gated(s.collectEvents()), globalName)
}

// mustConstraint parses s into a *semver.Constraints or panics; tests use
// known-good values.
func mustConstraint(s string) *semver.Constraints {
	c, err := semver.NewConstraint(s)
	if err != nil {
		panic(err)
	}

	return c
}

// TestAnyOfSatisfiedMemberEnables covers the happy path: a single AnyOf group
// with one installed member that meets its constraint enables the consumer.
func (s *SchedulerSuite) TestAnyOfSatisfiedMemberEnables() {
	s.activateGlobal()

	s.versions["gcp"] = mustVersion("1.5.0")

	s.Require().NoError(s.addNode(&testPackage{
		name:    "consumer",
		version: mustVersion("1.0.0"),
		constraints: schedule.Constraints{
			Order: schedule.FunctionalOrder,
			AnyOf: []schedule.AnyOfGroup{{
				Name: "cloud-provider",
				Members: map[string]*semver.Constraints{
					"gcp": mustConstraint(">=1.5.0"),
					"aws": mustConstraint(">=2.0.0"),
				},
			}},
		},
	}))

	s.Contains(gated(s.collectEvents()), "consumer")
}

// TestAnyOfNoInstalledMemberDisables verifies that an AnyOf group with no
// installed members keeps the consumer disabled, so its gate never opens.
func (s *SchedulerSuite) TestAnyOfNoInstalledMemberDisables() {
	s.activateGlobal()

	s.Require().NoError(s.addNode(&testPackage{
		name:    "consumer",
		version: mustVersion("1.0.0"),
		constraints: schedule.Constraints{
			Order: schedule.FunctionalOrder,
			AnyOf: []schedule.AnyOfGroup{{
				Name: "cloud-provider",
				Members: map[string]*semver.Constraints{
					"gcp": mustConstraint(">=1.5.0"),
					"aws": mustConstraint(">=2.0.0"),
				},
			}},
		},
	}))

	s.NotContains(gated(s.collectEvents()), "consumer")
}

// TestAnyOfInstalledButConstraintFailsDisables proves that the member's
// constraint is actually checked: a member installed at a version that fails
// the constraint is not counted as satisfying the group.
func (s *SchedulerSuite) TestAnyOfInstalledButConstraintFailsDisables() {
	s.activateGlobal()

	// gcp is installed but below the required floor; group is unmet.
	s.versions["gcp"] = mustVersion("1.4.0")

	s.Require().NoError(s.addNode(&testPackage{
		name:    "consumer",
		version: mustVersion("1.0.0"),
		constraints: schedule.Constraints{
			Order: schedule.FunctionalOrder,
			AnyOf: []schedule.AnyOfGroup{{
				Name: "cloud-provider",
				Members: map[string]*semver.Constraints{
					"gcp": mustConstraint(">=1.5.0"),
				},
			}},
		},
	}))

	s.NotContains(gated(s.collectEvents()), "consumer")
}

// TestAnyOfNilConstraintAcceptsAnyVersion pins the empty-constraint semantics:
// a member with nil constraint is satisfied as soon as it is installed at any
// version. Mirrors the DTO contract where an absent constraint string yields
// a nil *semver.Constraints meaning "any installed version is acceptable".
func (s *SchedulerSuite) TestAnyOfNilConstraintAcceptsAnyVersion() {
	s.activateGlobal()

	s.versions["gcp"] = mustVersion("0.0.1")

	s.Require().NoError(s.addNode(&testPackage{
		name:    "consumer",
		version: mustVersion("1.0.0"),
		constraints: schedule.Constraints{
			Order: schedule.FunctionalOrder,
			AnyOf: []schedule.AnyOfGroup{{
				Name: "cloud-provider",
				Members: map[string]*semver.Constraints{
					"gcp": nil,
				},
			}},
		},
	}))

	s.Contains(gated(s.collectEvents()), "consumer")
}

// TestAnyOfMultipleGroupsAllMustPass verifies that AnyOf groups are evaluated
// independently and ALL groups must pass for the consumer to be enabled.
// One satisfied group is not enough when a second group has no installed
// member — the consumer stays disabled.
func (s *SchedulerSuite) TestAnyOfMultipleGroupsAllMustPass() {
	s.activateGlobal()

	// First group satisfied via gcp; second group has no installed members.
	s.versions["gcp"] = mustVersion("1.5.0")

	s.Require().NoError(s.addNode(&testPackage{
		name:    "consumer",
		version: mustVersion("1.0.0"),
		constraints: schedule.Constraints{
			Order: schedule.FunctionalOrder,
			AnyOf: []schedule.AnyOfGroup{
				{
					Name: "cloud-provider",
					Members: map[string]*semver.Constraints{
						"gcp": mustConstraint(">=1.5.0"),
					},
				},
				{
					Name: "storage-backend",
					Members: map[string]*semver.Constraints{
						"minio": mustConstraint(">=2.0.0"),
						"s3":    mustConstraint(">=1.0.0"),
					},
				},
			},
		},
	}))

	s.NotContains(gated(s.collectEvents()), "consumer")

	// Installing a member of the second group satisfies all groups; consumer
	// schedules on the next pass.
	s.versions["minio"] = mustVersion("2.0.0")
	s.recompute()

	s.Contains(gated(s.collectEvents()), "consumer")
}

// TestAnyOfDoesNotCreateDependencyEdge pins that AnyOf edges are soft: two packages whose AnyOf
// groups reference each other do not produce a cycle. The same scenario with hard dependencies
// would be rejected as a *CycleError.
func (s *SchedulerSuite) TestAnyOfDoesNotCreateDependencyEdge() {
	s.activateGlobal()

	s.Require().NoError(s.addNode(&testPackage{
		name:    "alpha",
		version: mustVersion("1.0.0"),
		constraints: schedule.Constraints{
			Order: schedule.FunctionalOrder,
			AnyOf: []schedule.AnyOfGroup{{
				Name: "fallback",
				Members: map[string]*semver.Constraints{
					"beta": nil,
				},
			}},
		},
	}))

	// Adding beta whose AnyOf references alpha must NOT trigger a CycleError —
	// a soft edge that closes a cycle is dropped, not reported.
	s.Require().NoError(s.addNode(&testPackage{
		name:    "beta",
		version: mustVersion("1.0.0"),
		constraints: schedule.Constraints{
			Order: schedule.FunctionalOrder,
			AnyOf: []schedule.AnyOfGroup{{
				Name: "fallback",
				Members: map[string]*semver.Constraints{
					"alpha": nil,
				},
			}},
		},
	}))
}

// TestAnyOfMemberRunsBeforeConsumer confirms a consumer waits for an AnyOf member in the graph to
// finish.
func (s *SchedulerSuite) TestAnyOfMemberRunsBeforeConsumer() {
	s.activateGlobal()

	s.versions["gcp"] = mustVersion("1.5.0")

	s.Require().NoError(s.addNode(&testPackage{
		name:        "gcp",
		version:     mustVersion("1.5.0"),
		constraints: schedule.Constraints{Order: schedule.FunctionalOrder},
	}))
	s.Require().Contains(gated(s.collectEvents()), "gcp")

	s.Require().NoError(s.addNode(&testPackage{
		name:    "consumer",
		version: mustVersion("1.0.0"),
		constraints: schedule.Constraints{
			Order: schedule.FunctionalOrder,
			AnyOf: []schedule.AnyOfGroup{{
				Name: "cloud-provider",
				Members: map[string]*semver.Constraints{
					"gcp": mustConstraint(">=1.5.0"),
					"aws": nil,
				},
			}},
		},
	}))
	s.NotContains(gated(s.collectEvents()), "consumer", "a running member must hold the consumer back")

	s.converge("gcp")

	s.Contains(gated(s.collectEvents()), "consumer")
}

// TestAnyOfDisabledMemberDoesNotHoldConsumer confirms a not-enabled member, converged by
// definition, is not waited for.
func (s *SchedulerSuite) TestAnyOfDisabledMemberDoesNotHoldConsumer() {
	s.activateGlobal()

	s.versions["gcp"] = mustVersion("1.5.0")

	s.Require().NoError(s.addNode(&testPackage{
		name:    "aws",
		version: mustVersion("2.0.0"),
		constraints: schedule.Constraints{
			Order: schedule.FunctionalOrder,
			Floor: rule.Static(rule.Disable),
		},
	}))
	s.drainEvents()

	s.Require().NoError(s.addNode(&testPackage{
		name:    "consumer",
		version: mustVersion("1.0.0"),
		constraints: schedule.Constraints{
			Order: schedule.FunctionalOrder,
			AnyOf: []schedule.AnyOfGroup{{
				Name: "cloud-provider",
				Members: map[string]*semver.Constraints{
					"gcp": nil,
					"aws": nil,
				},
			}},
		},
	}))

	s.Contains(gated(s.collectEvents()), "consumer")
}

// TestAnyOfRaisesConsumerTier confirms a consumer declared below its member's order is raised, not
// deadlocked.
func (s *SchedulerSuite) TestAnyOfRaisesConsumerTier() {
	s.versions["gcp"] = mustVersion("1.5.0")

	s.Require().NoError(s.addNode(&testPackage{
		name:        globalName,
		version:     mustVersion("1.0.0"),
		constraints: schedule.Constraints{Order: 0},
	}))
	s.Require().NoError(s.addNode(&testPackage{
		name:    "consumer",
		version: mustVersion("1.0.0"),
		constraints: schedule.Constraints{
			Order: schedule.FunctionalOrder,
			AnyOf: []schedule.AnyOfGroup{{
				Name:    "cloud-provider",
				Members: map[string]*semver.Constraints{"gcp": nil},
			}},
		},
	}))
	s.Require().NoError(s.addNode(&testPackage{
		name:        "gcp",
		version:     mustVersion("1.5.0"),
		constraints: schedule.Constraints{Order: schedule.FunctionalOrder + 1},
	}))

	s.sched.Resume()
	s.converge(globalName)

	scheduled := gated(s.collectEvents())
	s.Contains(scheduled, "gcp", "the member must not wait behind its consumer's declared tier")
	s.NotContains(scheduled, "consumer")

	s.converge("gcp")

	s.Contains(gated(s.collectEvents()), "consumer")
}

// TestAnyOfMutualMembersDoNotDeadlock confirms a mutual AnyOf pair drops one edge and runs in name
// order.
func (s *SchedulerSuite) TestAnyOfMutualMembersDoNotDeadlock() {
	s.versions["alpha"] = mustVersion("1.0.0")
	s.versions["beta"] = mustVersion("1.0.0")

	s.Require().NoError(s.addNode(&testPackage{
		name:        globalName,
		version:     mustVersion("1.0.0"),
		constraints: schedule.Constraints{Order: 0},
	}))

	for name, member := range map[string]string{"alpha": "beta", "beta": "alpha"} {
		s.Require().NoError(s.addNode(&testPackage{
			name:    name,
			version: mustVersion("1.0.0"),
			constraints: schedule.Constraints{
				Order: schedule.FunctionalOrder,
				AnyOf: []schedule.AnyOfGroup{{
					Name:    "fallback",
					Members: map[string]*semver.Constraints{member: nil},
				}},
			},
		}))
	}

	s.sched.Resume()
	s.converge(globalName)

	scheduled := gated(s.collectEvents())
	s.Contains(scheduled, "alpha", "the cycle is broken at the lowest name")
	s.NotContains(scheduled, "beta")

	s.converge("alpha")

	s.Contains(gated(s.collectEvents()), "beta")
}

// TestCheckConstraintsAnyOfRejectsAtAdmission pins the admission-time parity:
// CheckConstraints (the webhook path) evaluates the AnyOf predicate identically
// to the persistent node checker chain, returning an error when no member of
// a group is installed.
func (s *SchedulerSuite) TestCheckConstraintsAnyOfRejectsAtAdmission() {
	s.activateGlobal()

	err := s.sched.CheckConstraints("proposed", schedule.Constraints{
		Order: schedule.FunctionalOrder,
		AnyOf: []schedule.AnyOfGroup{{
			Name: "cloud-provider",
			Members: map[string]*semver.Constraints{
				"gcp": mustConstraint(">=1.5.0"),
				"aws": mustConstraint(">=2.0.0"),
			},
		}},
	})

	s.Require().Error(err)
	s.Contains(err.Error(), "cloud-provider", "failure message must name the unmet group")
}

// TestCheckConstraintsVersionRejectionNamesSubject pins the admission-time
// message content for a version constraint. CheckConstraints surfaces only the
// decision message and drops the Reason that carries the subject, so the message
// itself must name what was checked, what was required and what the cluster has
// — semver's own error ("1.34.9 is less than v1.35") names none of the three.
//
// A local scheduler is built because the suite fixture wires no version getter.
func (s *SchedulerSuite) TestCheckConstraintsVersionRejectionNamesSubject() {
	sched := schedule.NewScheduler(log.NewNop(), schedule.WithKubeVersionGetter(func() (*semver.Version, error) {
		return mustVersion("1.34.9"), nil
	}))
	defer sched.Stop()

	err := sched.CheckConstraints("proposed", schedule.Constraints{
		Order:      schedule.FunctionalOrder,
		Kubernetes: mustConstraint(">= v1.35"),
	})

	s.Require().Error(err)
	s.Contains(err.Error(), "Kubernetes", "failure message must name the checked version")
	s.Contains(err.Error(), ">=v1.35", "failure message must name the violated constraint")
	s.Contains(err.Error(), "1.34.9", "failure message must name the current version")
}

// TestAnyOfMemberInstallTriggersReschedule confirms dynamic re-evaluation:
// a consumer born disabled (no AnyOf member installed) flips to enabled when
// a member becomes available and the scheduler computes again.
func (s *SchedulerSuite) TestAnyOfMemberInstallTriggersReschedule() {
	s.activateGlobal()

	s.Require().NoError(s.addNode(&testPackage{
		name:    "consumer",
		version: mustVersion("1.0.0"),
		constraints: schedule.Constraints{
			Order: schedule.FunctionalOrder,
			AnyOf: []schedule.AnyOfGroup{{
				Name: "cloud-provider",
				Members: map[string]*semver.Constraints{
					"gcp": mustConstraint(">=1.5.0"),
				},
			}},
		},
	}))

	s.NotContains(gated(s.collectEvents()), "consumer")

	s.versions["gcp"] = mustVersion("1.5.0")
	s.recompute()

	s.Contains(gated(s.collectEvents()), "consumer")
}

// TestNoneOfNoInstalledMemberEnables covers the happy path: with no forbidden
// module installed, the noneOf group passes and the consumer schedules.
func (s *SchedulerSuite) TestNoneOfNoInstalledMemberEnables() {
	s.activateGlobal()

	s.Require().NoError(s.addNode(&testPackage{
		name:    "consumer",
		version: mustVersion("1.0.0"),
		constraints: schedule.Constraints{
			Order: schedule.FunctionalOrder,
			NoneOf: []schedule.NoneOfGroup{{
				Name: "legacy-ingress",
				Members: map[string]*semver.Constraints{
					"nginx-ingress-legacy": mustConstraint("<2.0.0"),
					"haproxy-legacy":       nil,
				},
			}},
		},
	}))

	s.Contains(gated(s.collectEvents()), "consumer")
}

// TestNoneOfInstalledNilConstraintViolated pins the empty-constraint semantics
// for noneOf: a nil constraint forbids the module at any installed version.
// Once the module is installed, the consumer is disabled.
func (s *SchedulerSuite) TestNoneOfInstalledNilConstraintViolated() {
	s.activateGlobal()

	// Forbidden module is installed at any version — group violated from birth.
	s.versions["haproxy-legacy"] = mustVersion("0.0.1")

	s.Require().NoError(s.addNode(&testPackage{
		name:    "consumer",
		version: mustVersion("1.0.0"),
		constraints: schedule.Constraints{
			Order: schedule.FunctionalOrder,
			NoneOf: []schedule.NoneOfGroup{{
				Name: "legacy-ingress",
				Members: map[string]*semver.Constraints{
					"haproxy-legacy": nil,
				},
			}},
		},
	}))

	s.NotContains(gated(s.collectEvents()), "consumer")
}

// TestNoneOfInstalledInForbiddenRangeViolated verifies that a constraint
// narrows the forbidden range and the consumer is disabled only when a
// matching version is installed.
func (s *SchedulerSuite) TestNoneOfInstalledInForbiddenRangeViolated() {
	s.activateGlobal()

	// 1.9.0 matches "<2.0.0" — falls in the forbidden range.
	s.versions["nginx-ingress-legacy"] = mustVersion("1.9.0")

	s.Require().NoError(s.addNode(&testPackage{
		name:    "consumer",
		version: mustVersion("1.0.0"),
		constraints: schedule.Constraints{
			Order: schedule.FunctionalOrder,
			NoneOf: []schedule.NoneOfGroup{{
				Name: "legacy-ingress",
				Members: map[string]*semver.Constraints{
					"nginx-ingress-legacy": mustConstraint("<2.0.0"),
				},
			}},
		},
	}))

	s.NotContains(gated(s.collectEvents()), "consumer")
}

// TestNoneOfInstalledOutsideForbiddenRangeEnables confirms that a member
// installed at a version *outside* the forbidden range does not violate the
// group — the constraint is the forbidden range, not a "must not be present"
// shortcut.
func (s *SchedulerSuite) TestNoneOfInstalledOutsideForbiddenRangeEnables() {
	s.activateGlobal()

	// 2.0.0 is outside the "<2.0.0" forbidden range — group passes.
	s.versions["nginx-ingress-legacy"] = mustVersion("2.0.0")

	s.Require().NoError(s.addNode(&testPackage{
		name:    "consumer",
		version: mustVersion("1.0.0"),
		constraints: schedule.Constraints{
			Order: schedule.FunctionalOrder,
			NoneOf: []schedule.NoneOfGroup{{
				Name: "legacy-ingress",
				Members: map[string]*semver.Constraints{
					"nginx-ingress-legacy": mustConstraint("<2.0.0"),
				},
			}},
		},
	}))

	s.Contains(gated(s.collectEvents()), "consumer")
}

// TestNoneOfMultipleGroupsAllMustPass verifies that noneOf groups are
// evaluated independently and ALL must pass for the consumer to be enabled.
// A violation in any one group disables the consumer.
func (s *SchedulerSuite) TestNoneOfMultipleGroupsAllMustPass() {
	s.activateGlobal()

	// Second group violated via deprecated-storage.
	s.versions["deprecated-storage"] = mustVersion("1.0.0")

	s.Require().NoError(s.addNode(&testPackage{
		name:    "consumer",
		version: mustVersion("1.0.0"),
		constraints: schedule.Constraints{
			Order: schedule.FunctionalOrder,
			NoneOf: []schedule.NoneOfGroup{
				{
					Name: "legacy-ingress",
					Members: map[string]*semver.Constraints{
						"nginx-ingress-legacy": mustConstraint("<2.0.0"),
					},
				},
				{
					Name: "legacy-storage",
					Members: map[string]*semver.Constraints{
						"deprecated-storage": nil,
					},
				},
			},
		},
	}))

	s.NotContains(gated(s.collectEvents()), "consumer")

	// Removing the violator from the second group enables the consumer.
	delete(s.versions, "deprecated-storage")
	s.recompute()

	s.Contains(gated(s.collectEvents()), "consumer")
}

// TestNoneOfDoesNotCreateDependencyEdge is the load-bearing test for the
// design property: noneOf groups must not contribute to the topological
// graph, so two packages whose noneOf groups reference each other do not
// produce a cycle. Symmetric to TestAnyOfDoesNotCreateDependencyEdge.
func (s *SchedulerSuite) TestNoneOfDoesNotCreateDependencyEdge() {
	s.activateGlobal()

	s.Require().NoError(s.addNode(&testPackage{
		name:    "alpha",
		version: mustVersion("1.0.0"),
		constraints: schedule.Constraints{
			Order: schedule.FunctionalOrder,
			NoneOf: []schedule.NoneOfGroup{{
				Name: "conflict",
				Members: map[string]*semver.Constraints{
					"beta": nil,
				},
			}},
		},
	}))

	// Adding beta whose NoneOf references alpha must NOT trigger a CycleError —
	// noneOf members are not predecessors in the topo graph.
	s.Require().NoError(s.addNode(&testPackage{
		name:    "beta",
		version: mustVersion("1.0.0"),
		constraints: schedule.Constraints{
			Order: schedule.FunctionalOrder,
			NoneOf: []schedule.NoneOfGroup{{
				Name: "conflict",
				Members: map[string]*semver.Constraints{
					"alpha": nil,
				},
			}},
		},
	}))
}

// TestCheckConstraintsNoneOfRejectsAtAdmission pins admission-time parity:
// CheckConstraints (the webhook path) evaluates the noneOf predicate
// identically to the persistent node checker chain, returning an error
// naming the violated group when a forbidden module is installed.
func (s *SchedulerSuite) TestCheckConstraintsNoneOfRejectsAtAdmission() {
	s.activateGlobal()

	s.versions["haproxy-legacy"] = mustVersion("1.0.0")

	err := s.sched.CheckConstraints("proposed", schedule.Constraints{
		Order: schedule.FunctionalOrder,
		NoneOf: []schedule.NoneOfGroup{{
			Name: "legacy-ingress",
			Members: map[string]*semver.Constraints{
				"haproxy-legacy": nil,
			},
		}},
	})

	s.Require().Error(err)
	s.Contains(err.Error(), "legacy-ingress", "failure message must name the violated group")
	s.Contains(err.Error(), "haproxy-legacy", "failure message must name the offending member")
}

// TestNoneOfMemberInstallTriggersDisable confirms dynamic re-evaluation:
// a consumer is enabled while no forbidden module is installed, then flips
// to disabled when one becomes available and the scheduler re-runs.
func (s *SchedulerSuite) TestNoneOfMemberInstallTriggersDisable() {
	s.activateGlobal()

	s.Require().NoError(s.addNode(&testPackage{
		name:    "consumer",
		version: mustVersion("1.0.0"),
		constraints: schedule.Constraints{
			Order: schedule.FunctionalOrder,
			NoneOf: []schedule.NoneOfGroup{{
				Name: "legacy-ingress",
				Members: map[string]*semver.Constraints{
					"haproxy-legacy": nil,
				},
			}},
		},
	}))

	s.Contains(gated(s.collectEvents()), "consumer")

	// Forbidden module appears; consumer must flip enabled→disabled.
	s.versions["haproxy-legacy"] = mustVersion("1.0.0")
	s.recompute()

	s.Contains(disabled(s.collectEvents()), "consumer")
}

// boolPtr returns a pointer to b, for the tri-state dynamic getter.
func boolPtr(b bool) *bool { return &b }

// useDynamicScheduler replaces the suite scheduler with one wired to a
// controllable dynamic getter (the global hooks' enable signal, consulted when
// no intent was set through SetIntent), backed by the returned map: a
// *true/*false entry is an enable/disable signal, an absent entry is "no
// opinion" (nil). Tests drive enablement by mutating the map before a
// computation. The original scheduler is stopped.
func (s *SchedulerSuite) useDynamicScheduler() map[string]*bool {
	enabledState := make(map[string]*bool)

	s.sched.Stop()
	s.sched = schedule.NewScheduler(
		log.NewNop(),
		schedule.WithDependencyGetter(func(name string) *semver.Version {
			return s.versions[name]
		}),
		schedule.WithDynamicGetter(func(module string) *bool {
			return enabledState[module]
		}),
	)

	return enabledState
}

// TestDynamicRuleEnablesOverFloor verifies the dynamic rule's Enable vote turns
// on a module whose Disable floor would otherwise keep it off.
func (s *SchedulerSuite) TestDynamicRuleEnablesOverFloor() {
	enabledState := s.useDynamicScheduler()
	s.activateGlobal()

	enabledState["mod"] = boolPtr(true)
	// Order 0 (global tier) keeps the test focused on rule precedence, not
	// order gating.
	s.Require().NoError(s.addNode(&testPackage{
		name:    "mod",
		version: mustVersion("1.0.0"),
		constraints: schedule.Constraints{
			Order: 0,
			Floor: rule.Static(rule.Disable),
		},
	}))

	s.Contains(gated(s.collectEvents()), "mod",
		"an enable intent must override the module's Disable floor")
}

// TestDynamicRuleNilDefersToFloor verifies that with no intent the dynamic rule
// is Undefined and resolution falls through to the floor: a Disable-floored
// module stays off.
func (s *SchedulerSuite) TestDynamicRuleNilDefersToFloor() {
	s.useDynamicScheduler() // getter returns nil for everything
	s.activateGlobal()

	s.Require().NoError(s.addNode(&testPackage{
		name:    "mod",
		version: mustVersion("1.0.0"),
		constraints: schedule.Constraints{
			Order: schedule.FunctionalOrder,
			Floor: rule.Static(rule.Disable),
		},
	}))

	s.NotContains(gated(s.collectEvents()), "mod",
		"no intent must defer to the Disable floor")
}

// TestDynamicRuleDisableOverridesBundle verifies the dynamic rule outranks the
// bundle vote: a module the active bundle would enable is still turned off by an
// explicit disable intent, because the dynamic rule is appended after bundle.
func (s *SchedulerSuite) TestDynamicRuleDisableOverridesBundle() {
	enabledState := make(map[string]*bool)

	s.sched.Stop()
	s.sched = schedule.NewScheduler(
		log.NewNop(),
		schedule.WithDependencyGetter(func(name string) *semver.Version {
			return s.versions[name]
		}),
		// Bundle enables every module it is asked about.
		schedule.WithBundleChecker(func(edition.Licensing) bool { return true }),
		schedule.WithDynamicGetter(func(module string) *bool {
			return enabledState[module]
		}),
	)
	s.activateGlobal()

	// No intent: the bundle enable stands → module scheduled.
	s.Require().NoError(s.addNode(&testPackage{
		name:        "bundle-on",
		version:     mustVersion("1.0.0"),
		constraints: schedule.Constraints{Order: 0, Floor: rule.Static(rule.Disable)},
	}))
	s.Contains(gated(s.collectEvents()), "bundle-on",
		"with no intent, a bundle enable must turn the module on")

	// Explicit disable: overrides the bundle enable → module stays off.
	enabledState["bundle-off"] = boolPtr(false)
	s.Require().NoError(s.addNode(&testPackage{
		name:        "bundle-off",
		version:     mustVersion("1.0.0"),
		constraints: schedule.Constraints{Order: 0, Floor: rule.Static(rule.Disable)},
	}))
	s.NotContains(gated(s.collectEvents()), "bundle-off",
		"a disable intent must override the bundle enable")
}

// TestDynamicRuleRemovedSignalDisables verifies that a global hook's signal, once removed, no longer
// enables the module: the next Recheck falls back to the Disable floor, as addon-operator's
// DynamicallyEnabled extender drops a removed key.
func (s *SchedulerSuite) TestDynamicRuleRemovedSignalDisables() {
	enabledState := s.useDynamicScheduler()
	s.activateGlobal()

	enabledState["mod"] = boolPtr(true)
	s.Require().NoError(s.addNode(&testPackage{
		name:        "mod",
		version:     mustVersion("1.0.0"),
		module:      true,
		constraints: schedule.Constraints{Order: 0, Floor: rule.Static(rule.Disable)},
	}))
	s.NotContains(disabled(s.collectEvents()), "mod", "the hook's signal must enable the module")

	delete(enabledState, "mod")
	s.sched.Recheck()

	s.Contains(disabled(s.collectEvents()), "mod", "a removed signal must withdraw the enable")
}

// writeEnabledScript writes an executable enabled script with the given /bin/sh
// body into a per-test temp dir and returns a descriptor pointing at it. The
// body runs with the MODULE_ENABLED_RESULT / MODULE_ENABLED_REASON environment
// variables the runner sets.
func (s *SchedulerSuite) writeEnabledScript(body string) *script.Descriptor {
	path := filepath.Join(s.T().TempDir(), "enabled")
	s.Require().NoError(os.WriteFile(path, []byte("#!/bin/sh\n"+body+"\n"), 0o755))

	return &script.Descriptor{
		Path:     path,
		Settings: addonutils.Values{},
		Values:   addonutils.Values{},
	}
}

// TestScriptRuleTrueDoesNotEnableOverFloor verifies a true enabled-script result
// is not an enable vote: the script can only veto a module, never turn one on, so
// a true result leaves the module's Disable floor in force and the module stays off.
func (s *SchedulerSuite) TestScriptRuleTrueDoesNotEnableOverFloor() {
	s.activateGlobal()

	s.Require().NoError(s.addNode(&testPackage{
		name:    "mod",
		version: mustVersion("1.0.0"),
		constraints: schedule.Constraints{
			Order: 0,
			Floor: rule.Static(rule.Disable),
		},
		enabledScript: s.writeEnabledScript("echo true > $MODULE_ENABLED_RESULT"),
	}))

	s.NotContains(gated(s.collectEvents()), "mod",
		"a true enabled-script result must not override the module's Disable floor")
}

// TestScriptRuleTrueDoesNotForbid verifies a true enabled-script result abstains
// rather than vetoes: a module another intent rule enables stays scheduled when
// its script passes. Paired with TestScriptRuleTrueDoesNotEnableOverFloor, this
// pins the true result to Undefined (neither enables nor forbids).
func (s *SchedulerSuite) TestScriptRuleTrueDoesNotForbid() {
	enabledState := s.useDynamicScheduler()
	s.activateGlobal()

	enabledState["mod"] = boolPtr(true) // dynamic intent enables it
	s.Require().NoError(s.addNode(&testPackage{
		name:    "mod",
		version: mustVersion("1.0.0"),
		constraints: schedule.Constraints{
			Order: 0,
			Floor: rule.Static(rule.Disable),
		},
		enabledScript: s.writeEnabledScript("echo true > $MODULE_ENABLED_RESULT"),
	}))

	s.Contains(gated(s.collectEvents()), "mod",
		"a true enabled-script result must not veto a dynamically enabled module")
}

// TestScriptRuleFalseForbids verifies a false enabled-script result vetoes the
// module even when another intent rule would enable it: the dynamic enable is
// overridden by the script's Forbid (which wins from any position).
func (s *SchedulerSuite) TestScriptRuleFalseForbids() {
	enabledState := s.useDynamicScheduler()
	s.activateGlobal()

	enabledState["mod"] = boolPtr(true) // dynamic intent alone would enable it
	s.Require().NoError(s.addNode(&testPackage{
		name:    "mod",
		version: mustVersion("1.0.0"),
		constraints: schedule.Constraints{
			Order: 0,
			Floor: rule.Static(rule.Disable),
		},
		enabledScript: s.writeEnabledScript("echo false > $MODULE_ENABLED_RESULT"),
	}))

	s.NotContains(gated(s.collectEvents()), "mod",
		"a false enabled-script result must veto the module despite the dynamic enable")
}

// TestScriptRuleErrorForbids verifies that a script which fails to run with no earlier verdict
// vetoes the module (fail-closed): a new node never starts on a broken script.
func (s *SchedulerSuite) TestScriptRuleErrorForbids() {
	enabledState := s.useDynamicScheduler()
	s.activateGlobal()

	enabledState["mod"] = boolPtr(true)
	s.Require().NoError(s.addNode(&testPackage{
		name:    "mod",
		version: mustVersion("1.0.0"),
		constraints: schedule.Constraints{
			Order: 0,
			Floor: rule.Static(rule.Disable),
		},
		enabledScript: s.writeEnabledScript("exit 1"),
	}))

	s.NotContains(gated(s.collectEvents()), "mod",
		"a failing enabled script must veto the module (fail-closed)")
}

// TestScriptRuleErrorKeepsLastVerdict verifies that a script failing after a true result keeps the
// module enabled: a transient error must not disable a running module.
func (s *SchedulerSuite) TestScriptRuleErrorKeepsLastVerdict() {
	enabledState := s.useDynamicScheduler()
	s.activateGlobal()

	enabledState["mod"] = boolPtr(true)
	descriptor := s.writeEnabledScript("echo true > $MODULE_ENABLED_RESULT")
	s.Require().NoError(s.addNode(&testPackage{
		name:    "mod",
		version: mustVersion("1.0.0"),
		constraints: schedule.Constraints{
			Order: 0,
			Floor: rule.Static(rule.Disable),
		},
		enabledScript: descriptor,
	}))
	s.Require().Contains(gated(s.collectEvents()), "mod")

	s.Require().NoError(os.WriteFile(descriptor.Path, []byte("#!/bin/sh\nexit 1\n"), 0o755))
	s.recompute()

	events := s.collectEvents()
	s.Require().NotEmpty(events)
	s.NotContains(disabled(events), "mod", "a script error after a true result must keep the module enabled")
}

// TestScriptRuleDisableSurfacesReason verifies that when a module flips off
// because its enabled script now reports false, its event carries the
// script's reason and the DisabledByScript condition reason. The module is
// dynamically enabled (a true script only abstains, it cannot enable); the same
// descriptor path is rewritten between passes to change the verdict.
func (s *SchedulerSuite) TestScriptRuleDisableSurfacesReason() {
	enabledState := s.useDynamicScheduler()
	s.activateGlobal()

	enabledState["mod"] = boolPtr(true) // dynamic intent enables it; the passing script abstains
	descriptor := s.writeEnabledScript("echo true > $MODULE_ENABLED_RESULT")
	s.Require().NoError(s.addNode(&testPackage{
		name:    "mod",
		version: mustVersion("1.0.0"),
		constraints: schedule.Constraints{
			Order: 0,
			Floor: rule.Static(rule.Disable),
		},
		enabledScript: descriptor,
	}))
	s.Require().Contains(gated(s.collectEvents()), "mod")

	// Rewrite the same script to report false with a reason, then compute again
	// so the module flips off.
	s.Require().NoError(os.WriteFile(descriptor.Path,
		[]byte("#!/bin/sh\necho false > $MODULE_ENABLED_RESULT\necho 'not today' > $MODULE_ENABLED_REASON\n"),
		0o755))
	s.recompute()

	var disable *schedule.Event
	for _, e := range s.collectEvents() {
		if !e.Enabled && e.Name == "mod" {
			event := e
			disable = &event

			break
		}
	}

	s.Require().NotNil(disable, "flipping the script to false must report the module disabled")
	s.Equal("DisabledByScript", disable.Reason)
	s.Contains(disable.Message, "not today")
}

// TestEnabledScriptSeesModulesEnabledBefore pins what an enabled script reads as
// global.enabledModules: the modules the same computation decided enabled before it (sorted,
// modules only), not the list Global published after its last run, which is empty right after a
// restart. Every case is the very first computation after Resume, as at boot.
func (s *SchedulerSuite) TestEnabledScriptSeesModulesEnabledBefore() {
	module := func(name string, order schedule.Order) *testPackage {
		return &testPackage{
			name:        name,
			version:     mustVersion("1.0.0"),
			module:      true,
			constraints: schedule.Constraints{Order: order, Floor: rule.Static(rule.Disable)},
		}
	}

	cases := []struct {
		name    string
		others  []*testPackage
		intents map[string]bool
		// requires makes the script node depend on cni.
		requires bool
		seen     []string
		enabled  bool
	}{
		{
			name:    "a module enabled at a lower order is seen",
			others:  []*testPackage{module("cni", 10)},
			intents: map[string]bool{"cni": true},
			seen:    []string{"cni"},
			enabled: true,
		},
		{
			name:    "a module at a higher order is not seen yet",
			others:  []*testPackage{module("cni", 30)},
			intents: map[string]bool{"cni": true},
			seen:    []string{},
			enabled: false,
		},
		{
			name:     "a dependency is seen whatever its declared order",
			others:   []*testPackage{module("cni", 30)},
			intents:  map[string]bool{"cni": true},
			requires: true,
			seen:     []string{"cni"},
			enabled:  true,
		},
		{
			name: "every enabled lower module is seen at once; disabled modules and applications are not",
			others: []*testPackage{
				module("cni", 10),
				module("alpha", 1),
				module("off", 5),
				{name: "app", version: mustVersion("1.0.0"), constraints: schedule.Constraints{Order: 2}},
			},
			intents: map[string]bool{"cni": true, "alpha": true, "off": false},
			seen:    []string{"alpha", "cni"},
			enabled: true,
		},
	}

	for _, tc := range cases {
		sched := schedule.NewScheduler(log.NewNop())

		record := filepath.Join(s.T().TempDir(), "values.json")
		kube := module("kube", 20)
		kube.enabledScript = s.writeEnabledScript(fmt.Sprintf(
			`cp "$VALUES_PATH" %q
if grep -q '"cni"' "$VALUES_PATH"; then echo true; else echo false; fi > "$MODULE_ENABLED_RESULT"`, record))

		if tc.requires {
			kube.constraints.Dependencies = map[string]schedule.Dependency{"cni": {}}
		}

		for _, p := range append(tc.others, kube) {
			s.Require().NoError(sched.AddNode(p.node()), tc.name)
		}

		for name, on := range tc.intents {
			sched.SetIntent(name, &on)
		}

		sched.SetIntent("kube", boolPtr(true))
		sched.Resume()

		events := sched.Take()
		sched.Stop()

		data, err := os.ReadFile(record)
		s.Require().NoError(err, tc.name)

		var values struct {
			Global struct {
				EnabledModules []string `json:"enabledModules"`
			} `json:"global"`
		}

		s.Require().NoError(json.Unmarshal(data, &values), tc.name)

		seen := values.Global.EnabledModules
		if seen == nil {
			seen = []string{}
		}

		s.Equal(tc.seen, seen, tc.name)
		s.Equal(tc.enabled, !slices.Contains(disabled(events), "kube"), tc.name)
	}
}

// publishGlobal publishes global's facts for its latest event: Applied is the event's Inputs when
// the run succeeded and unknown otherwise; output is the checksum of the global values it
// published.
func (s *SchedulerSuite) publishGlobal(succeeded bool, output string) {
	s.take()

	e := s.last[globalName]

	applied := ""
	if succeeded {
		applied = e.Inputs
	}

	s.sched.Publish(globalName, schedule.Facts{Version: e.Version, OwnInputs: e.OwnInputs, Applied: applied, Output: output})
}

// TestSubscribersFollowGlobalOutput pins that a subscriber's inputs carry what Global published
// (Facts.Output), not Global's Applied: a module upgrade reruns Global but nobody else unless the
// published values change, a failed Global run leaves the subscribers' inputs alone (no in-flight
// run is cancelled), and the global barrier still holds every gate until Global converges.
func (s *SchedulerSuite) TestSubscribersFollowGlobalOutput() {
	subscriber := schedule.Constraints{Order: schedule.FunctionalOrder, Subscriptions: map[string]struct{}{globalName: {}}}
	moduleAt := func(version string) *testPackage {
		return &testPackage{name: "mod", version: mustVersion(version), module: true, constraints: subscriber}
	}

	cases := []struct {
		name string
		act  func()
		// globalInputs: global's inputs changed; appInputs: the application's inputs changed.
		globalInputs bool
		appInputs    bool
		// appGate and modGate: the gates are open after act.
		appGate bool
		modGate bool
	}{
		{
			name:         "a module upgrade reruns global, the module waits behind the barrier",
			act:          func() { s.Require().NoError(s.addNode(moduleAt("2.0.0"))) },
			globalInputs: true,
		},
		{
			name: "global converges on the upgrade without a new output: nobody else reruns",
			act: func() {
				s.Require().NoError(s.addNode(moduleAt("2.0.0")))
				s.publishGlobal(true, "values-1")
			},
			globalInputs: true,
			appGate:      true,
			modGate:      true,
		},
		{
			name:      "global publishes new values: every subscriber reruns",
			act:       func() { s.publishGlobal(true, "values-2") },
			appInputs: true,
			appGate:   true,
			modGate:   true,
		},
		{
			name: "a failed global run changes no subscriber's inputs but holds the barrier",
			act:  func() { s.publishGlobal(false, "values-1") },
		},
		{
			name: "a module enable reruns global; subscribers rerun only once global publishes",
			act: func() {
				s.Require().NoError(s.addNode(&testPackage{name: "extra", version: mustVersion("1.0.0"), module: true}))
			},
			globalInputs: true,
		},
	}

	for _, tc := range cases {
		s.SetupTest()

		s.Require().NoError(s.addNode(&testPackage{name: globalName, version: mustVersion("1.0.0")}))
		s.Require().NoError(s.addNode(moduleAt("1.0.0")))
		s.Require().NoError(s.addNode(&testPackage{name: "app", version: mustVersion("1.0.0"), constraints: subscriber}))
		s.sched.Resume()

		s.publishGlobal(true, "values-1")
		s.converge("mod")
		s.converge("app")
		s.take()

		globalBefore, appBefore := s.last[globalName].Inputs, s.last["app"].Inputs

		tc.act()
		s.take()

		s.Equal(tc.globalInputs, s.last[globalName].Inputs != globalBefore, tc.name+": global inputs")
		s.Equal(tc.appInputs, s.last["app"].Inputs != appBefore, tc.name+": app inputs")
		s.Equal(tc.appGate, s.last["app"].Gate, tc.name+": app gate")
		s.Equal(tc.modGate, s.last["mod"].Gate, tc.name+": module gate")

		s.sched.Stop()
	}
}

// TestGlobalEventCarriesEnabledModules pins Global's enabled set: every enabled module with its
// identity (name@revision) and its node's attachment,
// sorted by identity, computed together, so a replaced node brings its new attachment in the same
// event. Disabled modules and applications are not listed.
func (s *SchedulerSuite) TestGlobalEventCarriesEnabledModules() {
	s.Require().NoError(s.addNode(&testPackage{name: globalName, version: mustVersion("1.0.0")}))
	s.Require().NoError(s.addNode(&testPackage{name: "beta", version: mustVersion("2.0.0"), module: true, attachment: "view-beta-2"}))
	s.Require().NoError(s.addNode(&testPackage{
		name: "alpha", version: mustVersion("1.0.0"), module: true, revision: "v1.0.0", attachment: "view-alpha",
	}))
	s.Require().NoError(s.addNode(&testPackage{
		name: "off", version: mustVersion("1.0.0"), module: true, attachment: "view-off",
		constraints: schedule.Constraints{Floor: rule.Static(rule.Disable)},
	}))
	s.Require().NoError(s.addNode(&testPackage{name: "app", version: mustVersion("1.0.0"), attachment: "view-app"}))
	s.sched.Resume()
	s.take()

	s.Equal([]schedule.EnabledModule{
		{ID: "alpha@v1.0.0", Attachment: "view-alpha"},
		{ID: "beta@2.0.0", Attachment: "view-beta-2"},
	}, s.last[globalName].EnabledModules)
	s.Empty(s.last["beta"].EnabledModules, "only the global node's event carries the set")

	s.Require().NoError(s.addNode(&testPackage{name: "beta", version: mustVersion("3.0.0"), module: true, attachment: "view-beta-3"}))
	s.take()

	s.Equal([]schedule.EnabledModule{
		{ID: "alpha@v1.0.0", Attachment: "view-alpha"},
		{ID: "beta@3.0.0", Attachment: "view-beta-3"},
	}, s.last[globalName].EnabledModules)
}
