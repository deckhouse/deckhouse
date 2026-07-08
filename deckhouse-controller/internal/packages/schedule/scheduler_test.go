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
	"testing"

	"github.com/Masterminds/semver/v3"
	"github.com/stretchr/testify/suite"

	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/schedule"
	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/schedule/rule"
	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/schedule/rule/script"
	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/edition"
	"github.com/deckhouse/deckhouse/pkg/log"
)

// globalName is the literal sentinel used by Scheduler internally; it lives
// here as a constant so the tests document the contract rather than scatter
// magic strings.
const globalName = "global"

// testPackage is a minimal Package implementation used to drive Scheduler
// behavior from tests; production code uses apps.Application / modules.Module.
type testPackage struct {
	name        string
	version     *semver.Version
	constraints schedule.Constraints
}

// GetName returns the package identifier.
func (p *testPackage) GetName() string { return p.name }

// GetVersion returns the parsed package version.
func (p *testPackage) GetVersion() *semver.Version { return p.version }

// GetConstraints returns the package's scheduler constraints, defaulting the
// Floor to an always-Enable rule so cases that don't care about enablement keep
// the legacy "enabled once loaded" behavior. Cases exercising disablement set
// their own Floor.
func (p *testPackage) GetConstraints() schedule.Constraints {
	if p.constraints.Floor == nil {
		p.constraints.Floor = rule.Static(rule.Enable)
	}

	return p.constraints
}

func (p *testPackage) GetEnabledScriptDescriptor() *script.Descriptor {
	return nil
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
	s.sched = schedule.NewScheduler(
		log.NewNop(),
		schedule.WithDependencyGetter(func(name string) *semver.Version {
			return s.versions[name]
		}),
	)
}

// TearDownTest closes the event channel so a leaked goroutine never wedges
// the next test.
func (s *SchedulerSuite) TearDownTest() {
	s.sched.Stop()
}

// activateGlobal registers the implicit global package, resumes the scheduler,
// and drives global to active. Drains all setup events so the test can assert
// only on the events it triggers itself.
func (s *SchedulerSuite) activateGlobal() {
	s.Require().NoError(s.sched.AddNode(&testPackage{
		name:    globalName,
		version: mustVersion("1.0.0"),
		constraints: schedule.Constraints{
			Order: 0,
		},
	}))

	s.sched.Resume()
	s.sched.Complete(globalName)
	s.drainEvents()
}

// drainEvents non-blockingly empties the scheduler's event channel.
func (s *SchedulerSuite) drainEvents() {
	for {
		select {
		case <-s.sched.Ch():
		default:
			return
		}
	}
}

// collectEvents non-blockingly drains and returns the events currently buffered
// on the scheduler's event channel. Scheduler operations are synchronous, so
// all events emitted by the preceding call are present by the time this runs.
func (s *SchedulerSuite) collectEvents() []schedule.Event {
	var events []schedule.Event
	for {
		select {
		case e := <-s.sched.Ch():
			events = append(events, e)
		default:
			return events
		}
	}
}

// eventNames returns the names of events of the given kind from the slice.
func eventNames(events []schedule.Event, kind schedule.EventKind) []string {
	var names []string
	for _, e := range events {
		if e.Kind == kind {
			names = append(names, e.Name)
		}
	}

	return names
}

// TestAddNodeAndSchedule covers the happy-path lifecycle: a node added after
// global completes is enabled and scheduled on the same scheduling pass.
func (s *SchedulerSuite) TestAddNodeAndSchedule() {
	s.activateGlobal()

	s.Require().NoError(s.sched.AddNode(&testPackage{
		name:    "alpha",
		version: mustVersion("1.0.0"),
		constraints: schedule.Constraints{
			Order: schedule.FunctionalOrder,
		},
	}))

	s.Contains(eventNames(s.collectEvents(), schedule.EventSchedule), "alpha")
}

// TestFloorDisableKeepsNodeOff confirms the package-supplied floor governs
// enablement: a node whose Floor resolves to Disable is never scheduled, even
// though no gate vetoes it.
func (s *SchedulerSuite) TestFloorDisableKeepsNodeOff() {
	s.activateGlobal()

	s.Require().NoError(s.sched.AddNode(&testPackage{
		name:    "alpha",
		version: mustVersion("1.0.0"),
		constraints: schedule.Constraints{
			Order: schedule.FunctionalOrder,
			Floor: rule.Static(rule.Disable),
		},
	}))

	s.NotContains(eventNames(s.collectEvents(), schedule.EventSchedule), "alpha",
		"a node with a Disable floor must not be scheduled")
}

// TestOrderTierGate confirms that canSchedule's order-tier check holds a
// higher-tier node back until every lower-tier node is active.
func (s *SchedulerSuite) TestOrderTierGate() {
	s.activateGlobal()

	s.Require().NoError(s.sched.AddNode(&testPackage{
		name:        "critical",
		version:     mustVersion("1.0.0"),
		constraints: schedule.Constraints{Order: 1},
	}))

	s.Require().NoError(s.sched.AddNode(&testPackage{
		name:        "functional",
		version:     mustVersion("1.0.0"),
		constraints: schedule.Constraints{Order: schedule.FunctionalOrder},
	}))

	scheduled := eventNames(s.collectEvents(), schedule.EventSchedule)
	s.Contains(scheduled, "critical")
	s.NotContains(scheduled, "functional", "functional must wait for critical to be active")

	s.sched.Complete("critical")
	s.Contains(eventNames(s.collectEvents(), schedule.EventSchedule), "functional")
}

// TestDisabledNodeUnblocksHigherTier exercises the "mark disabled nodes active"
// behavior in compute(): a lower-tier node that loses eligibility must not
// stall higher tiers via the order-tier gate.
func (s *SchedulerSuite) TestDisabledNodeUnblocksHigherTier() {
	s.activateGlobal()

	s.Require().NoError(s.sched.AddNode(&testPackage{
		name:    "lower",
		version: mustVersion("1.0.0"),
		constraints: schedule.Constraints{
			Order: 1,
			Dependencies: map[string]schedule.Dependency{
				"never-installed": {},
			},
		},
	}))

	s.Require().NoError(s.sched.AddNode(&testPackage{
		name:        "higher",
		version:     mustVersion("1.0.0"),
		constraints: schedule.Constraints{Order: schedule.FunctionalOrder},
	}))

	scheduled := eventNames(s.collectEvents(), schedule.EventSchedule)
	s.NotContains(scheduled, "lower", "lower lost eligibility and must not be scheduled")
	s.Contains(scheduled, "higher", "higher must not be blocked by the disabled lower-tier node")
}

// TestMandatoryDependency verifies that a mandatory dep being absent disables
// the consumer, and that installing the dep flips it back to enabled.
func (s *SchedulerSuite) TestMandatoryDependency() {
	s.activateGlobal()

	s.Require().NoError(s.sched.AddNode(&testPackage{
		name:    "consumer",
		version: mustVersion("1.0.0"),
		constraints: schedule.Constraints{
			Order: schedule.FunctionalOrder,
			Dependencies: map[string]schedule.Dependency{
				"parent": {},
			},
		},
	}))

	// Consumer is born disabled (parent absent); compute() emits no event in
	// this case because there was no enabled→disabled flip. Just assert it
	// stays out of the scheduled set.
	s.NotContains(eventNames(s.collectEvents(), schedule.EventSchedule), "consumer")

	s.versions["parent"] = mustVersion("1.0.0")
	s.sched.Schedule()

	s.Contains(eventNames(s.collectEvents(), schedule.EventSchedule), "consumer")
}

// TestConditionalDependencyAbsentIsOK confirms that an optional dep being
// absent does NOT disable the consumer — only a version mismatch would.
func (s *SchedulerSuite) TestConditionalDependencyAbsentIsOK() {
	s.activateGlobal()

	s.Require().NoError(s.sched.AddNode(&testPackage{
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
	s.Contains(eventNames(events, schedule.EventSchedule), "consumer")
	s.NotContains(eventNames(events, schedule.EventDisable), "consumer")
}

// TestEnabledToDisabledFlipEmitsEventDisable verifies that compute() fires
// EventDisable when a previously-enabled node loses eligibility (e.g. its
// dependency is removed from the cluster).
func (s *SchedulerSuite) TestEnabledToDisabledFlipEmitsEventDisable() {
	s.activateGlobal()

	s.versions["parent"] = mustVersion("1.0.0")
	s.Require().NoError(s.sched.AddNode(&testPackage{
		name:    "consumer",
		version: mustVersion("1.0.0"),
		constraints: schedule.Constraints{
			Order: schedule.FunctionalOrder,
			Dependencies: map[string]schedule.Dependency{
				"parent": {},
			},
		},
	}))

	s.Contains(eventNames(s.collectEvents(), schedule.EventSchedule), "consumer")

	// Parent disappears — consumer must flip enabled→disabled.
	delete(s.versions, "parent")
	s.sched.Schedule()

	s.Contains(eventNames(s.collectEvents(), schedule.EventDisable), "consumer")
}

// TestStatusFlipResetsOnlyAffectedNode is the regression guard for the
// reconverge removal: when one node's Enabled status flips, other nodes'
// state must not be reset.
func (s *SchedulerSuite) TestStatusFlipResetsOnlyAffectedNode() {
	s.activateGlobal()

	s.Require().NoError(s.sched.AddNode(&testPackage{
		name:        "stable",
		version:     mustVersion("1.0.0"),
		constraints: schedule.Constraints{Order: schedule.FunctionalOrder},
	}))

	s.Require().NoError(s.sched.AddNode(&testPackage{
		name:    "flapper",
		version: mustVersion("1.0.0"),
		constraints: schedule.Constraints{
			Order: schedule.FunctionalOrder,
			Dependencies: map[string]schedule.Dependency{
				"absent": {},
			},
		},
	}))

	s.sched.Complete("stable")
	s.drainEvents()

	// stable is now active. Flip flapper from disabled → enabled by installing
	// its dep. compute() must reset flapper to idle but leave stable alone.
	s.versions["absent"] = mustVersion("1.0.0")
	s.sched.Schedule()

	events := s.collectEvents()
	scheduled := eventNames(events, schedule.EventSchedule)
	s.Contains(scheduled, "flapper")
	s.NotContains(scheduled, "stable", "stable was already active; status flip on flapper must not reset it")
}

// TestAddNodeRejectsCyclicAddition pins AddNode as the authoritative cycle
// gate: when adding a node would close a dep cycle with an already-registered
// node, AddNode returns a *CycleError without mutating the graph. Subsequent
// higher-tier additions schedule normally because the cycle never entered
// s.nodes.
func (s *SchedulerSuite) TestAddNodeRejectsCyclicAddition() {
	s.activateGlobal()

	s.Require().NoError(s.sched.AddNode(&testPackage{
		name:    "alpha",
		version: mustVersion("1.0.0"),
		constraints: schedule.Constraints{
			Order: 1,
			Dependencies: map[string]schedule.Dependency{
				"beta": {},
			},
		},
	}))

	err := s.sched.AddNode(&testPackage{
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
	s.Require().NoError(s.sched.AddNode(&testPackage{
		name:        "consumer",
		version:     mustVersion("1.0.0"),
		constraints: schedule.Constraints{Order: schedule.FunctionalOrder},
	}))

	s.Contains(eventNames(s.collectEvents(), schedule.EventSchedule), "consumer")
}

// TestCheckConstraintsRejectsDependencyCycle pins the admission-time cycle
// gate: when checking constraints for a node whose dependencies would close
// a cycle with an already-registered node, CheckConstraints returns a
// *CycleError naming the participants. The graph is not mutated.
func (s *SchedulerSuite) TestCheckConstraintsRejectsDependencyCycle() {
	s.activateGlobal()

	// alpha depends on a future beta; no cycle yet (beta isn't in the graph).
	s.Require().NoError(s.sched.AddNode(&testPackage{
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

// TestPauseSuppressesScheduling confirms that AddNode does not advance state
// while the scheduler is paused, and that Resume drains the pending work.
func (s *SchedulerSuite) TestPauseSuppressesScheduling() {
	// Scheduler starts paused by construction.
	s.Require().NoError(s.sched.AddNode(&testPackage{
		name:        globalName,
		version:     mustVersion("1.0.0"),
		constraints: schedule.Constraints{Order: 0},
	}))

	s.Empty(s.collectEvents(), "paused scheduler must not emit events")

	s.sched.Resume()
	s.Contains(eventNames(s.collectEvents(), schedule.EventSchedule), globalName)
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

	s.Require().NoError(s.sched.AddNode(&testPackage{
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

	s.Contains(eventNames(s.collectEvents(), schedule.EventSchedule), "consumer")
}

// TestAnyOfNoInstalledMemberDisables verifies that an AnyOf group with no
// installed members keeps the consumer disabled. Consumer is born disabled,
// so no EventSchedule is emitted in the first place.
func (s *SchedulerSuite) TestAnyOfNoInstalledMemberDisables() {
	s.activateGlobal()

	s.Require().NoError(s.sched.AddNode(&testPackage{
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

	s.NotContains(eventNames(s.collectEvents(), schedule.EventSchedule), "consumer")
}

// TestAnyOfInstalledButConstraintFailsDisables proves that the member's
// constraint is actually checked: a member installed at a version that fails
// the constraint is not counted as satisfying the group.
func (s *SchedulerSuite) TestAnyOfInstalledButConstraintFailsDisables() {
	s.activateGlobal()

	// gcp is installed but below the required floor; group is unmet.
	s.versions["gcp"] = mustVersion("1.4.0")

	s.Require().NoError(s.sched.AddNode(&testPackage{
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

	s.NotContains(eventNames(s.collectEvents(), schedule.EventSchedule), "consumer")
}

// TestAnyOfNilConstraintAcceptsAnyVersion pins the empty-constraint semantics:
// a member with nil constraint is satisfied as soon as it is installed at any
// version. Mirrors the DTO contract where an absent constraint string yields
// a nil *semver.Constraints meaning "any installed version is acceptable".
func (s *SchedulerSuite) TestAnyOfNilConstraintAcceptsAnyVersion() {
	s.activateGlobal()

	s.versions["gcp"] = mustVersion("0.0.1")

	s.Require().NoError(s.sched.AddNode(&testPackage{
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

	s.Contains(eventNames(s.collectEvents(), schedule.EventSchedule), "consumer")
}

// TestAnyOfMultipleGroupsAllMustPass verifies that AnyOf groups are evaluated
// independently and ALL groups must pass for the consumer to be enabled.
// One satisfied group is not enough when a second group has no installed
// member — the consumer stays disabled.
func (s *SchedulerSuite) TestAnyOfMultipleGroupsAllMustPass() {
	s.activateGlobal()

	// First group satisfied via gcp; second group has no installed members.
	s.versions["gcp"] = mustVersion("1.5.0")

	s.Require().NoError(s.sched.AddNode(&testPackage{
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

	s.NotContains(eventNames(s.collectEvents(), schedule.EventSchedule), "consumer")

	// Installing a member of the second group satisfies all groups; consumer
	// schedules on the next pass.
	s.versions["minio"] = mustVersion("2.0.0")
	s.sched.Schedule()

	s.Contains(eventNames(s.collectEvents(), schedule.EventSchedule), "consumer")
}

// TestAnyOfDoesNotCreateDependencyEdge is the load-bearing test for the
// design decision in ENG-7: AnyOf groups must not contribute to the
// topological graph, so two packages whose AnyOf groups reference each other
// do not produce a cycle. The same scenario expressed with hard dependencies
// would be rejected as a *CycleError.
func (s *SchedulerSuite) TestAnyOfDoesNotCreateDependencyEdge() {
	s.activateGlobal()

	s.Require().NoError(s.sched.AddNode(&testPackage{
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
	// AnyOf members are not predecessors in the topo graph.
	s.Require().NoError(s.sched.AddNode(&testPackage{
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

// TestAnyOfMemberInstallTriggersReschedule confirms dynamic re-evaluation:
// a consumer born disabled (no AnyOf member installed) flips to enabled when
// a member becomes available and the scheduler re-runs.
func (s *SchedulerSuite) TestAnyOfMemberInstallTriggersReschedule() {
	s.activateGlobal()

	s.Require().NoError(s.sched.AddNode(&testPackage{
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

	s.NotContains(eventNames(s.collectEvents(), schedule.EventSchedule), "consumer")

	s.versions["gcp"] = mustVersion("1.5.0")
	s.sched.Schedule()

	s.Contains(eventNames(s.collectEvents(), schedule.EventSchedule), "consumer")
}

// TestNoneOfNoInstalledMemberEnables covers the happy path: with no forbidden
// module installed, the noneOf group passes and the consumer schedules.
func (s *SchedulerSuite) TestNoneOfNoInstalledMemberEnables() {
	s.activateGlobal()

	s.Require().NoError(s.sched.AddNode(&testPackage{
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

	s.Contains(eventNames(s.collectEvents(), schedule.EventSchedule), "consumer")
}

// TestNoneOfInstalledNilConstraintViolated pins the empty-constraint semantics
// for noneOf: a nil constraint forbids the module at any installed version.
// Once the module is installed, the consumer is disabled.
func (s *SchedulerSuite) TestNoneOfInstalledNilConstraintViolated() {
	s.activateGlobal()

	// Forbidden module is installed at any version — group violated from birth.
	s.versions["haproxy-legacy"] = mustVersion("0.0.1")

	s.Require().NoError(s.sched.AddNode(&testPackage{
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

	s.NotContains(eventNames(s.collectEvents(), schedule.EventSchedule), "consumer")
}

// TestNoneOfInstalledInForbiddenRangeViolated verifies that a constraint
// narrows the forbidden range and the consumer is disabled only when a
// matching version is installed.
func (s *SchedulerSuite) TestNoneOfInstalledInForbiddenRangeViolated() {
	s.activateGlobal()

	// 1.9.0 matches "<2.0.0" — falls in the forbidden range.
	s.versions["nginx-ingress-legacy"] = mustVersion("1.9.0")

	s.Require().NoError(s.sched.AddNode(&testPackage{
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

	s.NotContains(eventNames(s.collectEvents(), schedule.EventSchedule), "consumer")
}

// TestNoneOfInstalledOutsideForbiddenRangeEnables confirms that a member
// installed at a version *outside* the forbidden range does not violate the
// group — the constraint is the forbidden range, not a "must not be present"
// shortcut.
func (s *SchedulerSuite) TestNoneOfInstalledOutsideForbiddenRangeEnables() {
	s.activateGlobal()

	// 2.0.0 is outside the "<2.0.0" forbidden range — group passes.
	s.versions["nginx-ingress-legacy"] = mustVersion("2.0.0")

	s.Require().NoError(s.sched.AddNode(&testPackage{
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

	s.Contains(eventNames(s.collectEvents(), schedule.EventSchedule), "consumer")
}

// TestNoneOfMultipleGroupsAllMustPass verifies that noneOf groups are
// evaluated independently and ALL must pass for the consumer to be enabled.
// A violation in any one group disables the consumer.
func (s *SchedulerSuite) TestNoneOfMultipleGroupsAllMustPass() {
	s.activateGlobal()

	// Second group violated via deprecated-storage.
	s.versions["deprecated-storage"] = mustVersion("1.0.0")

	s.Require().NoError(s.sched.AddNode(&testPackage{
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

	s.NotContains(eventNames(s.collectEvents(), schedule.EventSchedule), "consumer")

	// Removing the violator from the second group enables the consumer.
	delete(s.versions, "deprecated-storage")
	s.sched.Schedule()

	s.Contains(eventNames(s.collectEvents(), schedule.EventSchedule), "consumer")
}

// TestNoneOfDoesNotCreateDependencyEdge is the load-bearing test for the
// design property: noneOf groups must not contribute to the topological
// graph, so two packages whose noneOf groups reference each other do not
// produce a cycle. Symmetric to TestAnyOfDoesNotCreateDependencyEdge.
func (s *SchedulerSuite) TestNoneOfDoesNotCreateDependencyEdge() {
	s.activateGlobal()

	s.Require().NoError(s.sched.AddNode(&testPackage{
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
	s.Require().NoError(s.sched.AddNode(&testPackage{
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

	s.Require().NoError(s.sched.AddNode(&testPackage{
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

	s.Contains(eventNames(s.collectEvents(), schedule.EventSchedule), "consumer")

	// Forbidden module appears; consumer must flip enabled→disabled.
	s.versions["haproxy-legacy"] = mustVersion("1.0.0")
	s.sched.Schedule()

	s.Contains(eventNames(s.collectEvents(), schedule.EventDisable), "consumer")
}

// TestRescheduleFansOutToDirectSubscribers verifies that Reschedule reverts the
// named node AND its direct subscribers to idle — re-emitting EventSchedule for
// both — while leaving unrelated nodes and second-level subscribers untouched.
// The cascade is one level deep: a subscriber's own subscribers are not reverted.
func (s *SchedulerSuite) TestRescheduleFansOutToDirectSubscribers() {
	s.activateGlobal()

	s.Require().NoError(s.sched.AddNode(&testPackage{
		name:        "publisher",
		version:     mustVersion("1.0.0"),
		constraints: schedule.Constraints{Order: 0},
	}))
	s.sched.Complete("publisher")

	// Direct subscriber of publisher — must be reverted on Reschedule.
	s.Require().NoError(s.sched.AddNode(&testPackage{
		name:    "subscriber",
		version: mustVersion("1.0.0"),
		constraints: schedule.Constraints{
			Order:         0,
			Subscriptions: map[string]struct{}{"publisher": {}},
		},
	}))
	s.sched.Complete("subscriber")

	// Subscribes to subscriber, not publisher — the one-level-deep cascade must
	// not reach it.
	s.Require().NoError(s.sched.AddNode(&testPackage{
		name:    "second-level",
		version: mustVersion("1.0.0"),
		constraints: schedule.Constraints{
			Order:         0,
			Subscriptions: map[string]struct{}{"subscriber": {}},
		},
	}))
	s.sched.Complete("second-level")

	// No subscription relationship at all — must stay active.
	s.Require().NoError(s.sched.AddNode(&testPackage{
		name:        "unrelated",
		version:     mustVersion("1.0.0"),
		constraints: schedule.Constraints{Order: 0},
	}))
	s.sched.Complete("unrelated")
	s.drainEvents()

	s.sched.Reschedule("publisher")

	scheduled := eventNames(s.collectEvents(), schedule.EventSchedule)
	s.Contains(scheduled, "publisher", "the rescheduled node must be re-scheduled")
	s.Contains(scheduled, "subscriber", "a direct subscriber must be reverted and re-scheduled")
	s.NotContains(scheduled, "second-level", "the cascade must stop at one level — a subscriber's subscribers are untouched")
	s.NotContains(scheduled, "unrelated", "a node with no subscription must not be rescheduled")
}

// TestRescheduleOnEnableRevertsEntireGraph verifies the full-graph reschedule
// path: when a module (Floor = Static(Disable)) flips to enabled, compute()
// reverts every node to idle, not just the module. A module turning on may
// install CRDs other packages render against, and those template-level deps are
// untracked, so the whole graph must re-converge. An already-active, unrelated
// bystander being re-scheduled is the observable signal of that reconverge.
func (s *SchedulerSuite) TestRescheduleOnEnableRevertsEntireGraph() {
	enabledState := s.useDynamicScheduler()

	s.activateGlobal()

	// An always-on bystander, unrelated to the module, driven to active. Its
	// re-schedule after the module enables proves the whole graph reconverged.
	s.Require().NoError(s.sched.AddNode(&testPackage{
		name:        "bystander",
		version:     mustVersion("1.0.0"),
		constraints: schedule.Constraints{Order: 0},
	}))
	s.sched.Complete("bystander")

	// A module that is off (floor disables it) until dynamically enabled.
	s.Require().NoError(s.sched.AddNode(&testPackage{
		name:    "dynamic-mod",
		version: mustVersion("1.0.0"),
		constraints: schedule.Constraints{
			Order: 0,
			Floor: rule.Static(rule.Disable),
		},
	}))
	s.drainEvents()

	// Flip the module on and run a pass: it enables, triggering a full-graph
	// reschedule that reverts and re-schedules the bystander too.
	enabledState["dynamic-mod"] = boolPtr(true)
	s.sched.Schedule()

	scheduled := eventNames(s.collectEvents(), schedule.EventSchedule)
	s.Contains(scheduled, "dynamic-mod", "the newly enabled module must be scheduled")
	s.Contains(scheduled, "bystander", "reschedule-on-enable must revert and re-schedule the whole graph")
}

// boolPtr returns a pointer to b, for the tri-state dynamic getter.
func boolPtr(b bool) *bool { return &b }

// useDynamicScheduler replaces the suite scheduler with one wired to a
// controllable dynamic getter, backed by the returned map: a *true/*false entry
// is an explicit enable/disable intent (as the global module would resolve from
// ModuleConfig and dynamic hooks), an absent entry is "no opinion" (nil). Tests
// drive enablement by mutating the map before a scheduling pass. The original
// scheduler is stopped to avoid leaking its event goroutine.
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
	// Order 0 (global tier): enabling a Disable-floored module triggers a
	// full-graph reschedule, which reverts global to idle; a higher-tier module
	// would then be held by canSchedule until global re-completes. Same tier keeps
	// the test focused on rule precedence, not order gating.
	s.Require().NoError(s.sched.AddNode(&testPackage{
		name:    "mod",
		version: mustVersion("1.0.0"),
		constraints: schedule.Constraints{
			Order: 0,
			Floor: rule.Static(rule.Disable),
		},
	}))

	s.Contains(eventNames(s.collectEvents(), schedule.EventSchedule), "mod",
		"an enable intent must override the module's Disable floor")
}

// TestDynamicRuleNilDefersToFloor verifies that with no intent the dynamic rule
// is Undefined and resolution falls through to the floor: a Disable-floored
// module stays off.
func (s *SchedulerSuite) TestDynamicRuleNilDefersToFloor() {
	s.useDynamicScheduler() // getter returns nil for everything
	s.activateGlobal()

	s.Require().NoError(s.sched.AddNode(&testPackage{
		name:    "mod",
		version: mustVersion("1.0.0"),
		constraints: schedule.Constraints{
			Order: schedule.FunctionalOrder,
			Floor: rule.Static(rule.Disable),
		},
	}))

	s.NotContains(eventNames(s.collectEvents(), schedule.EventSchedule), "mod",
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
	s.Require().NoError(s.sched.AddNode(&testPackage{
		name:        "bundle-on",
		version:     mustVersion("1.0.0"),
		constraints: schedule.Constraints{Order: 0, Floor: rule.Static(rule.Disable)},
	}))
	s.Contains(eventNames(s.collectEvents(), schedule.EventSchedule), "bundle-on",
		"with no intent, a bundle enable must turn the module on")

	// Explicit disable: overrides the bundle enable → module stays off.
	enabledState["bundle-off"] = boolPtr(false)
	s.Require().NoError(s.sched.AddNode(&testPackage{
		name:        "bundle-off",
		version:     mustVersion("1.0.0"),
		constraints: schedule.Constraints{Order: 0, Floor: rule.Static(rule.Disable)},
	}))
	s.NotContains(eventNames(s.collectEvents(), schedule.EventSchedule), "bundle-off",
		"a disable intent must override the bundle enable")
}
