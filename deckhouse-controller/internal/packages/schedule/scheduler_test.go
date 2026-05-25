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

// GetConstraints returns the package's scheduler constraints.
func (p *testPackage) GetConstraints() schedule.Constraints { return p.constraints }

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
