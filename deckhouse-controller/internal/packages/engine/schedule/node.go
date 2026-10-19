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
	"log/slog"
	"maps"

	"github.com/Masterminds/semver/v3"

	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/engine/schedule/rule"
	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/engine/schedule/rule/bundle"
	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/engine/schedule/rule/condition"
	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/engine/schedule/rule/dependency"
	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/engine/schedule/rule/dynamic"
	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/engine/schedule/rule/script"
	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/engine/schedule/rule/version"
	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/edition"
)

// Node is one revision of a package as AddNode registers it; a new revision is a new Node.
type Node struct {
	Name    string
	Version *semver.Version // Version is what dependents' rules compare against.
	// Revision is the identity in the protocol (Facts.Version, Event.Version) and in Global's
	// inputs: the image version. Not Version.String(): the image version may be spelled differently
	// (a leading v).
	Revision string
	// Module marks a module node: only enabled modules make up Global's enabled set and an enabled
	// script's global.enabledModules.
	Module      bool
	Constraints Constraints
	// EnabledScript returns the module's enabled script, nil when it ships none. Called on every
	// computation, so it reads the object's live settings.
	EnabledScript func() *script.Descriptor
	// Attachment is opaque to the scheduler: it rides on Global's event with an enabled module
	// (EnabledModule), so Global reads the revision the event names and no other.
	Attachment any
	// Output is the revision's Output, known when it is added: the node starts with it, so a
	// subscriber sees the new revision's Output at once, never an empty one until its Current
	// publishes the same value.
	Output string
}

// Constraints are the scheduling requirements of a package.
type Constraints struct {
	Order        Order                 // Order: lower values run first.
	Kubernetes   *semver.Constraints   // Kubernetes version constraint (e.g., ">=1.21")
	Deckhouse    *semver.Constraints   // Deckhouse version constraint (e.g., ">=1.60")
	Dependencies map[string]Dependency // Dependencies are hard ordering edges and gate inputs.
	AnyOf        []AnyOfGroup          // AnyOf members are soft edges, dropped where they close a cycle.
	NoneOf       []NoneOfGroup         // NoneOf is gate-only: no ordering edge.

	// Subscriptions name the nodes whose Output is part of this node's inputs. The engine
	// subscribes every package to global.
	Subscriptions map[string]struct{}
	// Licensing is the per-edition availability and bundle membership the bundle rule resolves
	// against the active edition.
	Licensing edition.Licensing

	// Floor is the lowest-precedence rule, the default when no intent rule has an opinion:
	// Static(Enable) for applications and Global, Static(Disable) for modules. A Disable floor also
	// gives the node the intent rules (addNode). Admission ignores it. A nil Floor leaves only the
	// gates, so the node resolves to Undefined and stays off.
	Floor rule.Rule
}

// Dependency is a requirement on another package; Optional skips the check when it is absent.
type Dependency struct {
	Constraint *semver.Constraints `json:"constraint" yaml:"constraint"`
	Optional   bool                `json:"optional" yaml:"optional"`
}

// AnyOfGroup passes when at least one member is enabled and satisfies its constraint (nil: any
// version). Name appears in failure messages.
type AnyOfGroup struct {
	Name    string                         `json:"name" yaml:"name"`
	Members map[string]*semver.Constraints `json:"members" yaml:"members"`
}

// NoneOfGroup passes when no member is enabled at a version matching its constraint (nil: any
// version). Name appears in failure messages.
type NoneOfGroup struct {
	Name    string                         `json:"name" yaml:"name"`
	Members map[string]*semver.Constraints `json:"members" yaml:"members"`
}

// Order is a scheduling priority: lower values run first.
type Order uint

// node is a graph vertex: what AddNode registered, the facts its Current published and the result
// of the last computation.
type node struct {
	name       string
	version    *semver.Version
	revision   string
	module     bool
	attachment any

	order          Order
	effectiveOrder Order // effectiveOrder is the tier the gate enforces (computeOrdering).

	dependencies map[string]Dependency

	anyOf      map[string]struct{} // anyOf are the members of every AnyOf group.
	anyOfWaits map[string]struct{} // anyOfWaits are the members the last computation kept as edges.

	subscriptions map[string]struct{}

	rules []rule.Rule

	// facts start with the node's Output only and are dropped when the node is replaced, until the
	// first Publish; a zero Applied never equals an inputs hash.
	facts Facts

	// Written only by compute.
	decision  rule.Decision
	inputs    string
	converged bool
	gate      bool
}

func (n *node) enabled() bool { return n.decision.Kind == rule.Enable }

// addNode builds the node of nd with its rule chain and inserts it, replacing a node of the same
// name and its facts. It does not compute.
func (s *Scheduler) addNode(nd Node) {
	constraints := nd.Constraints

	n := &node{
		name:       nd.Name,
		version:    nd.Version,
		revision:   nd.Revision,
		module:     nd.Module,
		attachment: nd.Attachment,
		order:      constraints.Order,
		// seeded so a node added between computations is not tier 0 until compute refines it
		effectiveOrder: constraints.Order,
		dependencies:   maps.Clone(constraints.Dependencies),
		anyOf:          anyOfMembers(constraints.AnyOf),
		subscriptions:  maps.Clone(constraints.Subscriptions),
		facts:          Facts{Output: nd.Output},
	}

	// The rule chain, by precedence (rule.Resolve: the first Forbid wins, else the last soft
	// vote): the floor first, then the gates, which can only Forbid, then the intent rules.
	if constraints.Floor != nil {
		n.rules = append(n.rules, constraints.Floor)
	}

	n.rules = append(n.rules, s.gateRules(constraints)...)

	// Intent rules, only for an intent-driven package (a Disable floor; the engine gives every
	// module one and sets Module too):
	//   - bundle: Enable for the active bundle, else a soft Disable;
	//   - dynamic: the intent (intentOf): an enable is a soft Enable, a disable a Forbid that also
	//     keeps the script from running;
	//   - script: veto-only: false is a Forbid, true has no opinion, an error keeps the last
	//     verdict (a Forbid before the first one).
	if constraints.Floor != nil && constraints.Floor == rule.Static(rule.Disable) {
		if s.bundleChecker != nil {
			n.rules = append(n.rules, bundle.NewRule(s.bundleChecker, constraints.Licensing))
		}

		n.rules = append(n.rules, dynamic.NewRule(s.intentOf, nd.Name))

		if nd.EnabledScript != nil {
			n.rules = append(n.rules, script.NewRule(nd.EnabledScript, s.enabledBefore, s.logger.With(slog.String("module", nd.Name))))
		}
	}

	s.nodes[nd.Name] = n
}

// gateRules are the requirement gates of constraints (Undefined or Forbid only), shared by the
// rule chain and by admission.
func (s *Scheduler) gateRules(constraints Constraints) []rule.Rule {
	var rules []rule.Rule

	if constraints.Kubernetes != nil && s.kubeVersionGetter != nil {
		rules = append(rules, version.NewRule(s.kubeVersionGetter, constraints.Kubernetes, subjectKubernetes, reasonRequirementsKubernetes))
	}

	if constraints.Deckhouse != nil && s.deckhouseVersionGetter != nil {
		rules = append(rules, version.NewRule(s.deckhouseVersionGetter, constraints.Deckhouse, subjectDeckhouse, reasonRequirementsDeckhouse))
	}

	if constraints.Order == FunctionalOrder && s.bootstrapCondition != nil {
		rules = append(rules, condition.NewRule(s.bootstrapCondition, reasonRequirementsBootstrap, messageRequirementsBootstrap))
	}

	if len(constraints.Dependencies) > 0 {
		deps := make(map[string]dependency.Dependency, len(constraints.Dependencies))
		for depName, dep := range constraints.Dependencies {
			deps[depName] = dependency.Dependency{
				Constraint: dep.Constraint,
				Optional:   dep.Optional,
			}
		}

		rules = append(rules, dependency.NewRule(s.dependencyGetter, deps))
	}

	if len(constraints.AnyOf) > 0 {
		rules = append(rules, dependency.NewAnyOfRule(s.dependencyGetter, toAnyOfGroups(constraints.AnyOf)))
	}

	if len(constraints.NoneOf) > 0 {
		rules = append(rules, dependency.NewNoneOfRule(s.dependencyGetter, toNoneOfGroups(constraints.NoneOf)))
	}

	return rules
}

func anyOfMembers(groups []AnyOfGroup) map[string]struct{} {
	if len(groups) == 0 {
		return nil
	}

	members := make(map[string]struct{})
	for _, g := range groups {
		for name := range g.Members {
			members[name] = struct{}{}
		}
	}

	return members
}

// toAnyOfGroups converts to the dependency package's type, so the public Constraints do not leak
// it; the members are cloned against later mutation of the caller's Constraints.
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

// toNoneOfGroups converts to the dependency package's type, as toAnyOfGroups.
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
