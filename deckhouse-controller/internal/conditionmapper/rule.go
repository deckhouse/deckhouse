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

package conditionmapper

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// Rule defines a mapping rule that converts internal conditions to an external condition.
// A rule evaluates its dependencies and produces a condition with appropriate status, reason, and message.
type Rule struct {
	TargetType string    // External condition type to produce (e.g., "Ready", "Installed")
	Filter     Predicate // Optional filter - rule only applies if predicate returns true
	Invert     bool      // Inverts the True/False logic (useful for conditions with negative semantics)
	DependOn   []string  // Internal condition types this rule depends on
}

// Map applies the rule to the given status and returns the resulting external condition.
// Returns nil if the rule doesn't apply (due to predicate or invalid configuration).
//
// Logic (when Invert=false, normal behavior):
//   - Returns True status when all dependencies are True
//   - Returns False status when any dependency is False (with first failure's reason/message)
//   - Returns nil when dependencies are in unknown/missing state
//
// Logic (when Invert=true, inverted behavior):
//   - Returns False status when all dependencies are True
//   - Returns True status when any dependency is False
//   - Returns nil when dependencies are in unknown/missing state
//
// Use Invert=true for conditions with negative semantics (e.g., "PartiallyDegraded" where
// True means "not degraded" and False means "is degraded").
func (r *Rule) Map(status Status) *metav1.Condition {
	// Validate rule configuration
	if len(r.DependOn) == 0 || len(r.TargetType) == 0 {
		return nil
	}

	// Check if rule applies based on predicate
	if r.Filter != nil && !r.Filter(status) {
		return nil
	}

	// All dependencies satisfied -> True
	if r.matchTrue(status) {
		var reason, message string
		if r.Invert {
			reason, message = r.getReasonAndMessage(status)
		}

		return &metav1.Condition{
			Type:    r.TargetType,
			Status:  metav1.ConditionTrue,
			Reason:  reason,
			Message: message,
		}
	}

	// Any dependency failed -> False with reason/message
	if r.matchFalse(status) {
		reason, message := r.getReasonAndMessage(status)
		return &metav1.Condition{
			Type:    r.TargetType,
			Status:  metav1.ConditionFalse,
			Reason:  reason,
			Message: message,
		}
	}

	// Dependencies in unknown state (missing conditions)
	return nil
}

// matchTrue returns true when the condition should have Status=True.
// Normal (Invert=false): true when all dependencies are True
// Inverted (Invert=true): true when NOT all dependencies are True (i.e., any is not True)
func (r *Rule) matchTrue(status Status) bool {
	res := allInternalTrue(r.DependOn...)(status)
	if r.Invert {
		return !res // Invert: True when deps are NOT all True
	}
	return res // Normal: True when all deps are True
}

// matchFalse returns true when the condition should have Status=False.
// Normal (Invert=false): true when any dependency is False
// Inverted (Invert=true): true when NO dependencies are False (i.e., all are True or missing)
func (r *Rule) matchFalse(status Status) bool {
	res := anyInternalFalse(r.DependOn...)(status)
	if r.Invert {
		return !res // Invert: False when NO deps are False
	}
	return res // Normal: False when any dep is False
}

// getReasonAndMessage extracts reason and message for a False condition status.
func (r *Rule) getReasonAndMessage(status Status) (string, string) {
	for _, cond := range r.DependOn {
		for _, internal := range status.Internal {
			if internal.Type != cond {
				continue
			}

			if internal.Status == metav1.ConditionFalse {
				return internal.Reason, internal.Message
			}
		}
	}

	return "", ""
}
