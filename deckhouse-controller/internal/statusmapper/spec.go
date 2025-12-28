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

package statusmapper

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/status"
)

// =============================================================================
// Case & FirstMatch - Declarative DSL for condition evaluation
// =============================================================================

// Case represents a single condition evaluation case.
// Cases are evaluated in order; first match wins.
type Case struct {
	// When is the condition that must match.
	// If nil, case always matches (use for default/fallback case).
	When Matcher

	// Status is the condition status to set when case matches
	Status metav1.ConditionStatus

	// Reason is the condition reason to set
	Reason string

	// Message is the condition message to set
	Message string

	// MessageFrom specifies which internal condition to copy reason/message from.
	// If set, overrides empty Reason/Message with values from that condition.
	MessageFrom status.ConditionName
}

// FirstMatch evaluates cases in order and returns the first matching case.
// This is the primary DSL type for defining condition rules.
type FirstMatch []Case

// =============================================================================
// Spec
// =============================================================================

// Spec defines the complete specification for an external condition.
type Spec struct {
	// Type is the condition type this spec produces
	Type status.ConditionName

	// Rule defines the evaluation logic using FirstMatch DSL.
	// Cases are evaluated in order; first match wins.
	Rule FirstMatch

	// Sticky means once True, the condition never reverts to False.
	// Example: Installed - once installed, stays installed.
	Sticky bool

	// AppliesWhen determines if this condition should be evaluated at all.
	// If nil, condition always applies.
	// Example: UpdateInstalled only applies when version is changing.
	AppliesWhen Matcher
}

// Map finds the first matching case and returns the result.
// Returns nil if no cases match or condition doesn't apply.
func (s *Spec) Map(input *Input) *status.Condition {
	// Check if condition applies in current input
	if s.AppliesWhen != nil && !s.AppliesWhen.Match(input) {
		return nil
	}

	// Handle sticky conditions - once True, stays True
	if s.Sticky {
		if current, ok := input.ExternalConditions[s.Type]; ok {
			if current.Status == metav1.ConditionTrue {
				return &current
			}
		}
	}

	// Find first matching case (order in slice = priority)
	for _, c := range s.Rule {
		if c.When == nil || c.When.Match(input) {
			return s.buildResult(input, c)
		}
	}

	return nil
}

func (s *Spec) buildResult(input *Input, c Case) *status.Condition {
	result := status.Condition{
		Name:    s.Type,
		Status:  c.Status,
		Reason:  status.ConditionReason(c.Reason),
		Message: c.Message,
	}

	// Copy from internal condition if specified
	if c.MessageFrom != "" {
		if cond, ok := input.InternalConditions[c.MessageFrom]; ok {
			if result.Reason == "" {
				result.Reason = cond.Reason
			}
			if result.Message == "" {
				result.Message = cond.Message
			}
		}
	}

	return &result
}
