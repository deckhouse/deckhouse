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

package schedule

import (
	"maps"

	"github.com/Masterminds/semver/v3"

	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/schedule/checker"
	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/schedule/checker/condition"
	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/schedule/checker/dependency"
	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/schedule/checker/version"
)

const (
	nodeStateIdle      nodeState = "idle"      // Waiting for eligibility; may be (re)scheduled.
	nodeStateScheduled nodeState = "scheduled" // Passed all checks; onSchedule callback fired.
	nodeStateActive    nodeState = "active"    // Processing complete; dependents may now proceed.
)

// nodeState represents the lifecycle phase of a node in the scheduling graph.
type nodeState string

// Package is the interface that graph participants must implement to be
// managed by the Scheduler.
type Package interface {
	GetName() string
	GetVersion() *semver.Version
	GetConstraints() Constraints
}

// Constraints defines the scheduling requirements for a Package:
// ordering priority, version bounds, and inter-package dependencies.
type Constraints struct {
	Order        Order                 // Scheduling priority; lower values run first.
	Kubernetes   *semver.Constraints   // Kubernetes version constraint (e.g., ">=1.21")
	Deckhouse    *semver.Constraints   // Deckhouse version constraint (e.g., ">=1.60")
	Dependencies map[string]Dependency // Inter-package dependencies; keyed by package name. Source of topological ordering and checker inputs.
}

// Dependency describes a requirement on another package, with an optional
// semver constraint and a flag to skip the check when the target is absent.
type Dependency struct {
	Constraint *semver.Constraints `json:"constraint" yaml:"constraint"` // Semver constraint the dependency must satisfy
	Optional   bool                `json:"optional" yaml:"optional"`     // If true, the check is skipped when the dependency is absent
}

// Order is a numeric priority for scheduling: lower values are processed first.
type Order uint

// node is an internal graph vertex representing a registered Package.
// It tracks lifecycle state, dependency edges, and the checker chain
// used to evaluate eligibility on each scheduling pass.
type node struct {
	name    string          // Unique package name; also used as the graph vertex key.
	version *semver.Version // Current installed version; used by dependency checkers of dependents.

	state nodeState // Lifecycle phase: idle → scheduled → active.
	order Order     // Scheduling priority; lower values run before higher ones.

	status checker.Result // Last computed enabled/disabled result from the checker chain.

	dependencies map[string]Dependency // Declared dependency constraints — source of topological ordering and checker inputs.

	checkers []checker.Checker // Ordered list of checkers to evaluate
}

// addNode creates a node from a Package, attaches the checker chain, and
// inserts the node into the graph. It does NOT trigger a scheduling pass —
// the caller is responsible for that.
//
// Ordering is derived from n.dependencies by topoSort; enable state is
// computed by the checker chain.
func (s *Scheduler) addNode(pkg Package) {
	constraints := pkg.GetConstraints()

	n := &node{
		name:         pkg.GetName(),
		version:      pkg.GetVersion(),
		state:        nodeStateIdle,
		order:        constraints.Order,
		dependencies: maps.Clone(constraints.Dependencies),
	}

	if constraints.Kubernetes != nil && s.kubeVersionGetter != nil {
		n.checkers = append(n.checkers, version.NewChecker(s.kubeVersionGetter, constraints.Kubernetes, reasonRequirementsKubernetes))
	}

	if constraints.Deckhouse != nil && s.deckhouseVersionGetter != nil {
		n.checkers = append(n.checkers, version.NewChecker(s.deckhouseVersionGetter, constraints.Deckhouse, reasonRequirementsDeckhouse))
	}

	if constraints.Order == FunctionalOrder && s.bootstrapCondition != nil {
		n.checkers = append(n.checkers, condition.NewChecker(s.bootstrapCondition, reasonRequirementsBootstrap))
	}

	if len(constraints.Dependencies) > 0 && s.dependencyGetter != nil {
		deps := make(map[string]dependency.Dependency, len(constraints.Dependencies))
		for name, dep := range constraints.Dependencies {
			deps[name] = dependency.Dependency{
				Constraint: dep.Constraint,
				Optional:   dep.Optional,
			}
		}

		n.checkers = append(n.checkers, dependency.NewChecker(s.dependencyGetter, deps))
	}

	s.nodes[pkg.GetName()] = n
}
