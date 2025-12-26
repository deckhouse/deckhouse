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

package types

import (
	corev1 "k8s.io/api/core/v1"
)

// =============================================================================
// MappingRule
// =============================================================================

// MappingRule describes a single transition rule for an external condition.
// Rules are evaluated in order; first match wins.
type MappingRule struct {
	// Name identifies this rule (for debugging and validation)
	Name string

	// Matcher determines when this rule triggers
	Matcher Matcher

	// Status is the condition status to set when rule matches
	Status corev1.ConditionStatus

	// Reason is the condition reason to set
	Reason string

	// Message is the condition message to set
	Message string

	// MessageFrom specifies which internal condition to copy reason/message from.
	// If set, overrides empty Reason/Message with values from that internal condition.
	MessageFrom InternalConditionName
}

// =============================================================================
// MappingSpec
// =============================================================================

// MappingSpec defines the complete specification for an external condition.
type MappingSpec struct {
	// Type is the external condition type this spec produces
	Type ExternalConditionType

	// MappingRules are evaluated in order; first match wins
	MappingRules []MappingRule

	// Sticky means once True, the condition never reverts to False.
	// Example: Installed - once installed, stays installed.
	Sticky bool

	// AppliesWhen determines if this condition should be evaluated at all.
	// If nil, condition always applies.
	// Example: UpdateInstalled only applies when version is changing.
	AppliesWhen Matcher
}

// Map finds the first matching rule and returns the result.
// Returns nil if no rules match or condition doesn't apply.
func (s *MappingSpec) Map(input *MappingInput) *ExternalCondition {
	// Check if condition applies in current input
	if s.AppliesWhen != nil && !s.AppliesWhen.Match(input) {
		return nil
	}

	// Handle sticky conditions - once True, stays True
	if s.Sticky {
		if current, ok := input.CurrentConditions[s.Type]; ok {
			if current.Status == corev1.ConditionTrue {
				return &current
			}
		}
	}

	// Find first matching rule (order in slice = priority)
	for _, rule := range s.MappingRules {
		if rule.Matcher.Match(input) {
			return s.buildResult(input, rule)
		}
	}

	return nil
}

func (s *MappingSpec) buildResult(input *MappingInput, rule MappingRule) *ExternalCondition {
	result := ExternalCondition{
		Type:    s.Type,
		Status:  rule.Status,
		Reason:  rule.Reason,
		Message: rule.Message,
	}

	// Copy from internal condition if specified
	if rule.MessageFrom != "" {
		if internal, ok := input.InternalConditions[string(rule.MessageFrom)]; ok {
			if result.Reason == "" {
				result.Reason = internal.Reason
			}
			if result.Message == "" {
				result.Message = internal.Message
			}
		}
	}

	return &result
}
