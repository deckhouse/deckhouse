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
	"github.com/Masterminds/semver/v3"

	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/schedule/checker"
	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/schedule/checker/condition"
	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/schedule/checker/dependency"
	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/schedule/checker/dependency/anyof"
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
	Order        Order               // Scheduling priority; lower values run first.
	Kubernetes   *semver.Constraints // Kubernetes version constraint (e.g., ">=1.21")
	Deckhouse    *semver.Constraints // Deckhouse version constraint (e.g., ">=1.60")
	Dependencies Dependencies
}

// Dependencies declares a Package's inter-package version requirements.
// Mandatory and Conditional are flat name→constraint maps; AnyOf expresses
// disjunctive groups where at least one member must be satisfied.
type Dependencies struct {
	Mandatory   map[string]*semver.Constraints `json:"mandatory" yaml:"mandatory"`     // Hard requirements: must be installed and match the constraint.
	Conditional map[string]*semver.Constraints `json:"conditional" yaml:"conditional"` // Soft requirements: only enforced when the dependency is installed.
	AnyOf       []AnyOfGroup                   `json:"any_of" yaml:"any_of"`           // Disjunctive groups; each group must have one satisfied member.
}

// AnyOfGroup is a "satisfy at least one" set of module dependencies: the group
// passes as soon as a single member is installed at a constraint-satisfying
// version. A nil constraint means any installed version of that module counts.
type AnyOfGroup struct {
	Name    string                         `json:"name" yaml:"name"`       // Group identifier; surfaced in error and status messages.
	Modules map[string]*semver.Constraints `json:"modules" yaml:"modules"` // Candidate modules keyed by name; only one needs to satisfy.
}

// Order is a numeric priority for scheduling: lower values are processed first.
type Order uint

// node is an internal graph vertex representing a registered Package.
// It tracks lifecycle state, dependency edges, and the checker chain
// used to evaluate eligibility on each scheduling pass.
type node struct {
	name    string          // Unique package name; also used as the graph vertex key.
	version *semver.Version // Current installed version; used by dependency checkers of followers.

	state nodeState // Lifecycle phase: idle → scheduled → active.
	order Order     // Scheduling priority; lower values run before higher ones.

	status checker.Result // Last computed enabled/disabled result from the checker chain.

	followees    map[string]struct{} // Packages this node waits for before it can be scheduled.
	followers    map[string]struct{} // Packages that are waiting on this node to become active.
	dependencies Dependencies        // Declared dependency constraints (version bounds, optional flag).

	checkers []checker.Checker // Ordered list of checkers to evaluate
}

// addNode creates a node from a Package, wires followee/follower edges in both
// directions, attaches version/condition/dependency checkers, and inserts the
// node into the graph. It does NOT trigger a scheduling pass — the caller is
// responsible for that.
//
// If a node with the same name already exists (version update), its stale
// reverse edges are cleaned up before the new node is inserted. This prevents
// old followees from keeping the package as a follower after its constraints change.
func (s *Scheduler) addNode(pkg Package) {
	// Clean up stale reverse edges from the previous node (if any).
	// Without this, a dependency dropped in the new version would still
	// hold a followers["name"] reference and spuriously trigger this node.
	if old, ok := s.nodes[pkg.GetName()]; ok {
		for dep := range old.followees {
			if parent, ok := s.nodes[dep]; ok {
				delete(parent.followers, old.name)
			}
		}
	}

	n := &node{
		name:         pkg.GetName(),
		version:      pkg.GetVersion(),
		state:        nodeStateIdle,
		followees:    make(map[string]struct{}),
		followers:    make(map[string]struct{}),
		dependencies: pkg.GetConstraints().Dependencies,
	}

	constraints := pkg.GetConstraints()

	n.order = constraints.Order

	if n.name != packageGlobal {
		n.followees[packageGlobal] = struct{}{}

		// all packages should be subscribed to global
		if global, ok := s.nodes[packageGlobal]; ok {
			global.followers[n.name] = struct{}{}
		}
	}

	for _, existing := range s.nodes {
		if _, ok := existing.followees[n.name]; ok {
			n.followers[existing.name] = struct{}{}
		}
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

	if s.dependencyGetter != nil {
		deps := make(map[string]dependency.Dependency)
		for name, dep := range constraints.Dependencies.Mandatory {
			deps[name] = dependency.Dependency{
				Constraint: dep,
			}
		}

		for name, dep := range constraints.Dependencies.Conditional {
			deps[name] = dependency.Dependency{
				Constraint: dep,
				Optional:   true,
			}
		}

		n.checkers = append(n.checkers, dependency.NewChecker(s.dependencyGetter, deps))

		anyOfDeps := make([]anyof.Group, 0, len(constraints.Dependencies.AnyOf))
		for _, group := range constraints.Dependencies.AnyOf {
			anyOfDeps = append(anyOfDeps, anyof.Group{
				Name:    group.Name,
				Modules: group.Modules,
			})
		}

		n.checkers = append(n.checkers, anyof.NewChecker(s.dependencyGetter, anyOfDeps))
	}

	s.nodes[pkg.GetName()] = n
}
