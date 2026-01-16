// Copyright 2025 Flant JSC
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

package condmapper

import metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

// State holds internal and external conditions for mapping.
type State struct {
	VersionChanged bool                        // true if package version changed
	Internal       map[string]metav1.Condition // source conditions
	External       map[string]metav1.Condition // previous external state
}

// Mapper applies rules to compute external conditions.
type Mapper struct {
	Rules []Rule
}

// Rule defines how to compute an external condition from internal state.
type Rule struct {
	Type    string    // external condition type name
	TrueIf  Predicate // set True when matched; source used for Reason/Message
	FalseIf Predicate // set False when matched; source used for Reason/Message
	OnlyIf  Predicate // precondition; skip rule if not matched
	Sticky  bool      // once True, stays True forever
}

// Map evaluates all rules and returns computed external conditions.
func (m Mapper) Map(state State) []metav1.Condition {
	result := make([]metav1.Condition, 0, len(m.Rules))

	for _, r := range m.Rules {
		if cond, ok := r.evaluate(state); ok {
			result = append(result, cond)
		}
	}

	return result
}

// evaluate applies a single rule and returns the resulting condition.
// Returns false if the rule should be skipped (OnlyIf not met, sticky already True, or no match).
func (r Rule) evaluate(state State) (metav1.Condition, bool) {
	// Check precondition
	if r.OnlyIf != nil && !r.OnlyIf(state).Ok {
		return metav1.Condition{}, false
	}

	// Sticky: skip if already True - no change needed
	if r.Sticky {
		if c, ok := state.External[r.Type]; ok && c.Status == metav1.ConditionTrue {
			return metav1.Condition{}, false
		}
	}

	// Evaluate predicates: FalseIf takes precedence over TrueIf
	res, status := r.evaluatePredicates(state)
	if !res.Ok {
		return metav1.Condition{}, false
	}

	// Build condition with Reason/Message from source
	cond := metav1.Condition{
		Type:   r.Type,
		Status: status,
	}
	if src, ok := state.Internal[res.Source]; ok {
		cond.Reason = src.Reason
		cond.Message = src.Message
	}

	return cond, true
}

// evaluatePredicates checks FalseIf and TrueIf predicates.
// FalseIf is checked first as failure state takes precedence.
func (r Rule) evaluatePredicates(state State) (match, metav1.ConditionStatus) {
	if r.FalseIf != nil {
		if res := r.FalseIf(state); res.Ok {
			return res, metav1.ConditionFalse
		}
	}

	if r.TrueIf != nil {
		if res := r.TrueIf(state); res.Ok {
			return res, metav1.ConditionTrue
		}
	}

	return match{}, ""
}
