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
	Order        Order
	Kubernetes   *semver.Constraints // Kubernetes version constraint (e.g., ">=1.21")
	Deckhouse    *semver.Constraints // Deckhouse version constraint
	Dependencies map[string]Dependency
}

// Dependency describes a requirement on another package, with an optional
// semver constraint and a flag to skip the check when the target is absent.
type Dependency struct {
	Constraint *semver.Constraints // Semver constraint the dependency must satisfy
	Optional   bool                // If true, the check is skipped when the dependency is absent
}

// Order is a numeric priority for scheduling: lower values are processed first.
type Order uint

// node is an internal graph vertex representing a registered Package.
// It tracks lifecycle state, dependency edges, and the checker chain
// used to evaluate eligibility on each scheduling pass.
type node struct {
	name    string
	version *semver.Version

	state nodeState
	order Order

	status checker.Result

	followees map[string]struct{}
	followers map[string]struct{}

	checkers []checker.Checker // Ordered list of checkers to evaluate
}

// addNode creates a node from a Package, wires followee/follower edges in both
// directions, attaches version/condition/dependency checkers, and inserts the
// node into the graph. It does NOT trigger a scheduling pass — the caller is
// responsible for that.
func (s *Scheduler) addNode(pkg Package) {
	n := &node{
		name:      pkg.GetName(),
		version:   pkg.GetVersion(),
		state:     nodeStateIdle,
		followees: make(map[string]struct{}),
		followers: make(map[string]struct{}),
	}

	constraints := pkg.GetConstraints()

	n.order = constraints.Order

	for dep := range constraints.Dependencies {
		n.followees[dep] = struct{}{}

		if parent, ok := s.nodes[dep]; ok {
			parent.followers[n.name] = struct{}{}
		}
	}

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
		n.checkers = append(n.checkers, version.NewChecker(s.kubeVersionGetter, constraints.Kubernetes, ""))
	}

	if constraints.Deckhouse != nil && s.deckhouseVersionGetter != nil {
		n.checkers = append(n.checkers, version.NewChecker(s.deckhouseVersionGetter, constraints.Deckhouse, ""))
	}

	if len(constraints.Dependencies) > 0 {
		deps := make(map[string]dependency.Dependency)
		for name, dep := range constraints.Dependencies {
			deps[name] = dependency.Dependency{
				Constraint: dep.Constraint,
				Optional:   dep.Optional,
			}
		}

		n.checkers = append(n.checkers, dependency.NewChecker(s.getVersion, deps))
	}

	if constraints.Order == functionalWeight && s.bootstrapCondition != nil {
		n.checkers = append(n.checkers, condition.NewChecker(s.bootstrapCondition, ""))
	}

	s.nodes[pkg.GetName()] = n
}
