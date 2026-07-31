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

// Package rule models the single enable/disable signal type the scheduler folds
// to decide whether a package may run. A Rule contributes one opinion over the
// Kind lattice; Resolve combines an ordered list of them. Both the old "gate"
// checks (version, dependency, bootstrap) and future intent signals (bundle,
// user config, enabled script) are expressed uniformly as Rules — the scheduler
// never branches on rule type, only on the resolved Kind.
package rule

// Kind classifies a single rule's opinion about whether a package should run.
// The four values form a lattice with two soft states (overridable votes) and
// one hard state (a non-overridable veto); there is deliberately no hard
// "force-enable" — requirements must always retain the power to block.
type Kind string

const (
	// Undefined means the rule has no opinion; resolution defers to other rules.
	Undefined Kind = "Undefined"
	// Enable is a soft vote to turn the package on; a later soft vote overrides it.
	Enable Kind = "Enable"
	// Disable is a soft vote to turn the package off; a later soft vote overrides it.
	Disable Kind = "Disable"
	// Forbid is a hard veto: the package is off and no other rule can re-enable it.
	Forbid Kind = "Forbid"
)

// Decision is a single rule's verdict: its opinion plus a reason and message
// suitable for surfacing as a Kubernetes status condition when the package
// ends up disabled.
type Decision struct {
	Kind    Kind   `json:"kind"`
	Reason  string `json:"reason,omitempty"`
	Message string `json:"message,omitempty"`
}

// Rule contributes one opinion about whether a package should run. Cross-module
// state (current versions, dependency presence, and the like) is captured by
// getters at construction time, so Decide takes no arguments.
type Rule interface {
	// Decide evaluates the rule and returns its verdict.
	Decide() Decision
}

// staticRule is a Rule that always returns the same decision; see Static.
type staticRule struct {
	decision Decision
}

// Decide returns the preset decision.
func (r staticRule) Decide() Decision { return r.decision }

// Static returns a Rule that always emits the given kind with no reason or
// message. Use it for constant floors — e.g. an always-Enable floor for
// packages that should run whenever they are loaded.
func Static(kind Kind) Rule {
	return staticRule{decision: Decision{Kind: kind}}
}

// Resolve folds an ordered list of rules into a single decision:
//   - the first Forbid wins immediately — a hard veto cannot be overridden, so
//     its position in the list does not affect the outcome;
//   - among soft votes (Enable/Disable) the last one wins, so later rules take
//     precedence over earlier ones;
//   - Undefined rules are skipped; if every rule is Undefined the result is
//     Undefined and the caller's floor decides the default.
func Resolve(rules ...Rule) Decision {
	out := Decision{Kind: Undefined}

	for _, r := range rules {
		switch d := r.Decide(); d.Kind {
		case Undefined:
			continue
		case Forbid:
			return d
		case Enable, Disable:
			out = d
		}
	}

	return out
}
