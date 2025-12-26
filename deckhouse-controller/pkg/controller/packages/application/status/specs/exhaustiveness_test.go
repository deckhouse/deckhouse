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

package specs

import (
	"slices"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/status"
	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/controller/packages/application/status/types"
)

// TestEverySpecIsStructurallyValid ensures all specs have valid structure.
func TestEverySpecIsStructurallyValid(t *testing.T) {
	for _, spec := range DefaultSpecs() {
		t.Run(string(spec.Type), func(t *testing.T) {
			require.NotEmpty(t, spec.Type, "spec has empty Type")
			require.NotEmpty(t, spec.MappingRules, "spec has no rules")

			// Check rule names are non-empty and unique
			ruleNames := make(map[string]bool)
			for _, rule := range spec.MappingRules {
				require.NotEmpty(t, rule.Name, "rule has empty name")
				require.False(t, ruleNames[rule.Name], "duplicate rule name: %s", rule.Name)
				ruleNames[rule.Name] = true
			}
		})
	}
}

// TestEverySpecHasFallbackRule ensures every spec has Always{} as last rule.
// This guarantees that Map() never returns nil for applicable conditions.
func TestEverySpecHasFallbackRule(t *testing.T) {
	for _, spec := range DefaultSpecs() {
		t.Run(string(spec.Type), func(t *testing.T) {
			require.NotEmpty(t, spec.MappingRules, "spec has no rules")

			lastRule := spec.MappingRules[len(spec.MappingRules)-1]
			_, isAlways := lastRule.Matcher.(types.Always)

			assert.True(t, isAlways,
				"last rule must be Always{} for exhaustiveness, got %s", lastRule.Matcher.String())
		})
	}
}

// TestEveryExternalConditionHasSpec ensures every defined external condition type
// has a corresponding MappingSpec in DefaultSpecs().
func TestEveryExternalConditionHasSpec(t *testing.T) {
	specs := DefaultSpecs()
	specTypes := make(map[types.ExternalConditionType]bool)
	for _, s := range specs {
		specTypes[s.Type] = true
	}

	for _, condType := range types.AllExternalConditions {
		assert.True(t, specTypes[condType],
			"external condition %q has no MappingSpec", condType)
	}

	// Verify no orphan specs (specs for unknown condition types)
	for _, s := range specs {
		assert.True(t, slices.Contains(types.AllExternalConditions, s.Type),
			"spec exists for unknown condition type %q (add it to AllExternalConditions)", s.Type)
	}
}

// TestEveryInternalConditionHasMapping ensures all internal conditions from operator
// are used in at least one MappingSpec. Uses status.AllConditions as single source of truth.
func TestEveryInternalConditionHasMapping(t *testing.T) {
	usageMap := make(map[status.ConditionName]bool)
	for _, name := range status.AllConditions {
		usageMap[name] = false
	}

	// Scan all specs for internal condition references
	for _, spec := range DefaultSpecs() {
		for _, rule := range spec.MappingRules {
			collectInternalConditionUsage(rule.Matcher, usageMap)
			if rule.MessageFrom != "" {
				usageMap[rule.MessageFrom] = true
			}
		}
	}

	for name, isUsed := range usageMap {
		if slices.Contains(status.UnusedConditions, name) {
			continue // Intentionally unused (reserved for future)
		}
		assert.True(t, isUsed,
			"operator condition %q has no mapping in specs (add to spec or status.UnusedConditions)", name)
	}
}

// collectInternalConditionUsage recursively marks internal conditions as used.
func collectInternalConditionUsage(m types.Matcher, usageMap map[status.ConditionName]bool) {
	switch v := m.(type) {
	case types.InternalIs:
		usageMap[v.Name] = true
	case types.InternalNotTrue:
		usageMap[v.Name] = true
	case types.AllOf:
		for _, sub := range v {
			collectInternalConditionUsage(sub, usageMap)
		}
	case types.AnyOf:
		for _, sub := range v {
			collectInternalConditionUsage(sub, usageMap)
		}
	}
}
