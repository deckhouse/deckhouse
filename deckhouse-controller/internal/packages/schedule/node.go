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

	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/schedule/rule"
	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/schedule/rule/bundle"
	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/schedule/rule/condition"
	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/schedule/rule/dependency"
	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/schedule/rule/dynamic"
	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/schedule/rule/version"
	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/edition"
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
	Dependencies map[string]Dependency // Inter-package dependencies; keyed by package name. Source of topological ordering and gate-rule inputs.
	AnyOf        []AnyOfGroup          // Groups of alternative dependencies. Gate-only: never contributes edges to the topological graph, so fallback chains across packages do not produce cycles.
	NoneOf       []NoneOfGroup         // Groups of forbidden dependencies. Gate-only: "must not be installed" is an admission predicate, not an ordering relation.

	Subscriptions map[string]struct{} // Subscriptions to other nodes: this node will be notified when the subscribed node changes state.
	// Licensing carries the package's per-edition availability and bundle
	// membership. It is the data the edition gate and bundle floor consume,
	// resolved live against the active edition. The package supplies only the
	// data — resolution logic lives in the edition package.
	Licensing edition.Licensing

	// Floor is the package's lowest-precedence rule: its default decision when no
	// higher-precedence intent rule (bundle, user, script) has an opinion. Apps and Global
	// set rule.Static(rule.Enable) (on whenever loaded); modules set
	// rule.Static(rule.Disable). It is the only behavior-carrying field here — admission
	// (CheckConstraints) ignores it, since the floor is intent, not a
	// requirement. A nil Floor means no floor: with gates-only the package
	// resolves to Undefined and stays off, so every package must set one.
	Floor rule.Rule
}

// Dependency describes a requirement on another package, with an optional
// semver constraint and a flag to skip the check when the target is absent.
type Dependency struct {
	Constraint *semver.Constraints `json:"constraint" yaml:"constraint"` // Semver constraint the dependency must satisfy
	Optional   bool                `json:"optional" yaml:"optional"`     // If true, the check is skipped when the dependency is absent
}

// AnyOfGroup is a group of alternative dependencies: at least one member must
// be installed and satisfy its constraint for the group to pass. A nil
// constraint on a member means "any installed version is acceptable". Name is
// the stable identifier used by the scheduler in failure diagnostics.
type AnyOfGroup struct {
	Name    string                         `json:"name" yaml:"name"`
	Members map[string]*semver.Constraints `json:"members" yaml:"members"`
}

// NoneOfGroup is a group of forbidden dependencies: no member may be installed
// in a way that matches its constraint. A nil constraint on a member forbids
// the module at any installed version; a non-nil constraint narrows the
// forbidden range. Name is the stable identifier used by the scheduler in
// failure diagnostics.
type NoneOfGroup struct {
	Name    string                         `json:"name" yaml:"name"`
	Members map[string]*semver.Constraints `json:"members" yaml:"members"`
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

	decision rule.Decision // Last computed decision from the rule chain; the node is enabled iff Kind == rule.Enable.

	dependencies map[string]Dependency // Declared dependency constraints — source of topological ordering and rule inputs.

	subscriptions map[string]struct{} // Subscriptions to other nodes: this node will be notified when the subscribed node changes state.
	subscribers   map[string]struct{} // Set of nodes that are subscribed to this node's state changes.

	rescheduleOnEnable bool // If true, this node flipping to enabled triggers a full-graph reschedule in compute() (every node reverts to idle), not just the global node.

	rules []rule.Rule // Ordered rule chain evaluated on each scheduling pass.
}

// enabled reports whether the node's last resolved decision turns it on. Only
// a soft Enable counts — Disable, Forbid, and Undefined all mean "not enabled".
func (n *node) enabled() bool { return n.decision.Kind == rule.Enable }

// addNode creates a node from a Package, attaches the checker chain, and
// inserts the node into the graph. It does NOT trigger a scheduling pass —
// the caller is responsible for that.
//
// Ordering is derived from n.dependencies by topoSort; enable state is
// computed by the rule chain.
func (s *Scheduler) addNode(pkg Package) {
	constraints := pkg.GetConstraints()

	n := &node{
		name:          pkg.GetName(),
		version:       pkg.GetVersion(),
		state:         nodeStateIdle,
		order:         constraints.Order,
		dependencies:  maps.Clone(constraints.Dependencies),
		subscriptions: maps.Clone(constraints.Subscriptions),
		subscribers:   make(map[string]struct{}),
	}

	// Modules (floor = Static(Disable)) trigger a full-graph reschedule when they
	// flip to enabled: a dynamically-enabled module may install CRDs that other
	// packages render against, and those template-level deps are not tracked.
	if constraints.Floor != nil && constraints.Floor == rule.Static(rule.Disable) {
		n.rescheduleOnEnable = true

		// Intent rule, appended after the gates: a dynamic Enable is a soft vote that
		// overrides the floor's bundle decision (e.g. a module enabled at runtime by
		// an enabled script), while gates still veto via Forbid from any position.
		if s.dynamicGetter != nil {
			n.rules = append(n.rules, dynamic.NewRule(s.dynamicGetter, pkg.GetName()))
		}

		// Only a package whose licensing actually names enabling bundles gets the
		// bundle floor. Packages that carry editions purely for availability (e.g.
		// applications) have no bundle membership, so adding the rule would soft-
		// disable them and override their Enable floor.
		if s.bundleChecker != nil {
			n.rules = append(n.rules, bundle.NewRule(s.bundleChecker, constraints.Licensing))
		}
	}

	// The package's floor sits first (lowest precedence): gates appended after
	// it still veto via Forbid, and future intent rules placed after the gates
	// override the floor's soft vote. A nil Floor leaves the node with gates
	// only, so it resolves to Undefined and stays off.
	if constraints.Floor != nil {
		n.rules = append(n.rules, constraints.Floor)
	}

	if constraints.Kubernetes != nil && s.kubeVersionGetter != nil {
		n.rules = append(n.rules, version.NewRule(s.kubeVersionGetter, constraints.Kubernetes, reasonRequirementsKubernetes))
	}

	if constraints.Deckhouse != nil && s.deckhouseVersionGetter != nil {
		n.rules = append(n.rules, version.NewRule(s.deckhouseVersionGetter, constraints.Deckhouse, reasonRequirementsDeckhouse))
	}

	if constraints.Order == FunctionalOrder && s.bootstrapCondition != nil {
		n.rules = append(n.rules, condition.NewRule(s.bootstrapCondition, reasonRequirementsBootstrap))
	}

	if len(constraints.Dependencies) > 0 && s.dependencyGetter != nil {
		deps := make(map[string]dependency.Dependency, len(constraints.Dependencies))
		for name, dep := range constraints.Dependencies {
			deps[name] = dependency.Dependency{
				Constraint: dep.Constraint,
				Optional:   dep.Optional,
			}
		}

		n.rules = append(n.rules, dependency.NewRule(s.dependencyGetter, deps))
	}

	if len(constraints.AnyOf) > 0 && s.dependencyGetter != nil {
		n.rules = append(n.rules, dependency.NewAnyOfRule(s.dependencyGetter, toAnyOfGroups(constraints.AnyOf)))
	}

	if len(constraints.NoneOf) > 0 && s.dependencyGetter != nil {
		n.rules = append(n.rules, dependency.NewNoneOfRule(s.dependencyGetter, toNoneOfGroups(constraints.NoneOf)))
	}

	s.nodes[pkg.GetName()] = n

	// Adding (or replacing) a node changes the subscription graph, so recompute
	// the reverse index. Cheap relative to node churn and always correct on update.
	s.rebuildSubscribers()
}

// rebuildSubscribers recomputes every node's subscribers set from the
// subscriptions declared across the graph. A node lists the nodes it subscribes
// to (subscriptions); the reverse index — who is subscribed to a given node
// (subscribers) — is what Reschedule fans out to. Rebuilding wholesale keeps the
// index correct after any add, update, or remove without per-edge bookkeeping.
func (s *Scheduler) rebuildSubscribers() {
	for _, n := range s.nodes {
		n.subscribers = make(map[string]struct{})
	}

	for name, n := range s.nodes {
		for target := range n.subscriptions {
			if t, ok := s.nodes[target]; ok {
				t.subscribers[name] = struct{}{}
			}
		}
	}
}

// toAnyOfGroups translates schedule.AnyOfGroup values into the dependency
// package's AnyOfGroup shape. The two types are structurally identical; the
// translation exists so the schedule package's public contract does not leak
// the dependency package's types to callers. Members maps are cloned so the
// scheduler's view is isolated from later mutation of the caller's Constraints
// (mirrors the maps.Clone of constraints.Dependencies in addNode).
func toAnyOfGroups(in []AnyOfGroup) []dependency.AnyOfGroup {
	out := make([]dependency.AnyOfGroup, 0, len(in))
	for _, g := range in {
		out = append(out, dependency.AnyOfGroup{
			Name:    g.Name,
			Members: maps.Clone(g.Members),
		})
	}

	return out
}

// toNoneOfGroups translates schedule.NoneOfGroup values into the dependency
// package's NoneOfGroup shape. The two types are structurally identical; the
// translation exists so the schedule package's public contract does not leak
// the dependency package's types to callers.
func toNoneOfGroups(in []NoneOfGroup) []dependency.NoneOfGroup {
	out := make([]dependency.NoneOfGroup, 0, len(in))
	for _, g := range in {
		out = append(out, dependency.NoneOfGroup{
			Name:    g.Name,
			Members: g.Members,
		})
	}

	return out
}
