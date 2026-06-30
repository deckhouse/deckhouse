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

package condition

import (
	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/schedule/rule"
)

// Condition is a function that evaluates a boolean condition.
// Examples:
//   - Bootstrap ready check
//   - Leader election status
//   - External dependency availability
//   - Feature flag evaluation
type Condition func() bool

// Rule wraps a condition function as a rule.Rule. It is a gate: a false
// condition vetoes (Forbid), a true condition has no opinion (Undefined).
type Rule struct {
	condition Condition // Function to evaluate
	reason    string
}

// NewRule creates a condition rule.
func NewRule(condition Condition, reason string) *Rule {
	r := new(Rule)

	r.condition = condition
	r.reason = reason

	return r
}

// Decide evaluates the condition function: an unmet condition is a hard veto
// (Forbid) carrying the configured reason; a met condition yields Undefined.
func (r *Rule) Decide() rule.Decision {
	if !r.condition() {
		return rule.Decision{Kind: rule.Forbid, Reason: r.reason}
	}

	return rule.Decision{Kind: rule.Undefined}
}
