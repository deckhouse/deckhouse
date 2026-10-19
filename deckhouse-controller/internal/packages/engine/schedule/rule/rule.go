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

// Package rule is the enable/disable signal the scheduler folds to decide whether a package runs.
// A Rule gives one opinion; Resolve combines an ordered chain. Requirement gates (version,
// dependency, bootstrap) and intent rules (bundle, enable intent, enabled script) are all Rules, so
// the scheduler branches only on the resolved Kind.
package rule

// Kind is a rule's opinion. There is deliberately no hard enable: requirements must always be able
// to block.
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

// Decision is a rule's verdict, with the reason and message a disabled package reports.
type Decision struct {
	Kind    Kind   `json:"kind"`
	Reason  string `json:"reason,omitempty"`
	Message string `json:"message,omitempty"`
}

// Rule gives one opinion. State outside the node is read through getters bound at construction,
// so Decide takes no arguments.
type Rule interface {
	Decide() Decision
}

type staticRule struct {
	decision Decision
}

func (r staticRule) Decide() Decision { return r.decision }

// Static returns a Rule that always emits kind; used for floors.
func Static(kind Kind) Rule {
	return staticRule{decision: Decision{Kind: kind}}
}

// Resolve folds an ordered chain into one decision: the first Forbid wins at once (later rules are
// not evaluated); otherwise the last soft vote (Enable or Disable) wins; Undefined is skipped, and
// a chain of Undefined only resolves to Undefined (not enabled).
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
