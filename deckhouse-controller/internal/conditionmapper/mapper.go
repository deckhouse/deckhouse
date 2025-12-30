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
	"maps"
	"slices"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// Mapper applies a set of rules to convert internal conditions to external conditions.
type Mapper struct {
	Rules []Rule // Ordered list of mapping rules to apply
}

// Status represents the current state used for condition mapping.
type Status struct {
	VersionChanged bool                        // True if package version changed since last update
	Internal       map[string]metav1.Condition // Internal conditions from package operator
	External       map[string]metav1.Condition // Existing external conditions on Application
}

// Map applies all rules to the given status and returns the resulting external conditions.
// Each rule may produce zero or one condition. If multiple rules produce the same condition type,
// the last one wins (rules are processed in order).
func (m Mapper) Map(status Status) []metav1.Condition {
	tmp := make(map[string]metav1.Condition)

	// Apply each rule
	for _, rule := range m.Rules {
		cond := rule.Map(status)
		if cond == nil {
			continue // Rule didn't apply or dependencies not ready
		}

		// Store condition (overwrites if duplicate type)
		tmp[cond.Type] = *cond
	}

	// Convert map to slice
	return slices.Collect(maps.Values(tmp))
}
